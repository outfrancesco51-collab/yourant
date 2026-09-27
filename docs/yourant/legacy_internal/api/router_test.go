package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yourant/internal/anilist"
	"yourant/internal/api"
	"yourant/internal/downloader"
	"yourant/internal/storage"
)

func setupTestApp(t *testing.T) (http.Handler, string) {
	tempDir, err := os.MkdirTemp("", "yourant_api_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}

	downloadsDir := filepath.Join(tempDir, "downloads")
	_ = os.MkdirAll(downloadsDir, 0755)

	storageFile := filepath.Join(tempDir, "library.json")

	aniClient := anilist.NewClient(anilist.Config{
		RateLimitRPM: 1000.0,
	})

	store, err := storage.NewLibraryStore(storageFile)
	if err != nil {
		t.Fatalf("store init error: %v", err)
	}
	_ = store.Load()

	dlManager, err := downloader.NewManager(downloadsDir)
	if err != nil {
		t.Fatalf("downloader init error: %v", err)
	}

	router := api.NewRouter(api.RouterConfig{
		DownloadsDir: downloadsDir,
		Version:      "1.0.0",
	}, aniClient, store, dlManager)

	return router, tempDir
}

func TestHealthEndpoint(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON error: %v", err)
	}

	if body["status"] != "ok" || body["version"] != "1.0.0" {
		t.Fatalf("unexpected health payload: %v", body)
	}
}

func TestCORSMiddlewarePreflight(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	req := httptest.NewRequest(http.MethodOptions, "/api/anime/trending", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content for OPTIONS, got %d", rr.Code)
	}

	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %s", origin)
	}

	methods := rr.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Errorf("missing Access-Control-Allow-Methods")
	}
}

func TestAnimeEndpoints(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	// 1. Trending
	req := httptest.NewRequest(http.MethodGet, "/api/anime/trending?page=1&perPage=5", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("trending error status: %d", rr.Code)
	}

	// 2. Search
	req = httptest.NewRequest(http.MethodGet, "/api/anime/search?q=Frieren", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("search error status: %d", rr.Code)
	}

	// 3. Detail
	req = httptest.NewRequest(http.MethodGet, "/api/anime/154587", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("detail error status: %d", rr.Code)
	}

	// 4. Invalid ID
	req = httptest.NewRequest(http.MethodGet, "/api/anime/notanid", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
	}
}

func TestTrackingListAndUpdate(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	// 1. Update tracking
	updatePayload := []byte(`{
		"mediaId": 16498,
		"title": "Attack on Titan",
		"status": "CURRENT",
		"progress": 5,
		"score": 9.5
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/tracking/update", bytes.NewReader(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("update error status: %d, body: %s", rr.Code, rr.Body.String())
	}

	// 2. Get tracking list
	req = httptest.NewRequest(http.MethodGet, "/api/user/tracking/list", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list error status: %d", rr.Code)
	}

	var resp storage.UserLibraryResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode library response error: %v", err)
	}

	found := false
	for _, item := range resp.Lists {
		if item.MediaID == 16498 && item.Progress == 5 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected updated item in library response")
	}
}

func TestDownloadsAndStreaming(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	downloadsDir := filepath.Join(tempDir, "downloads")
	videoPath := filepath.Join(downloadsDir, "stream_test.mp4")

	// Create 1024-byte test video payload
	dummyVideo := bytes.Repeat([]byte("X"), 1024)
	if err := os.WriteFile(videoPath, dummyVideo, 0644); err != nil {
		t.Fatalf("write dummy video error: %v", err)
	}

	// 1. Full content stream (HTTP 200)
	req := httptest.NewRequest(http.MethodGet, "/api/downloads/stream/stream_test.mp4", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for full video, got %d", rr.Code)
	}
	if rr.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("missing Accept-Ranges: bytes")
	}
	if rr.Body.Len() != 1024 {
		t.Fatalf("expected body length 1024, got %d", rr.Body.Len())
	}

	// 2. Partial content stream: first 256 bytes (0-255) (HTTP 206)
	req = httptest.NewRequest(http.MethodGet, "/api/downloads/stream/stream_test.mp4", nil)
	req.Header.Set("Range", "bytes=0-255")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Range") != "bytes 0-255/1024" {
		t.Fatalf("expected Content-Range 'bytes 0-255/1024', got %q", rr.Header().Get("Content-Range"))
	}
	if rr.Body.Len() != 256 {
		t.Fatalf("expected 256 bytes, got %d", rr.Body.Len())
	}

	// 3. Partial content stream: middle 512 bytes (256-767) (HTTP 206)
	req = httptest.NewRequest(http.MethodGet, "/api/downloads/stream/stream_test.mp4", nil)
	req.Header.Set("Range", "bytes=256-767")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Range") != "bytes 256-767/1024" {
		t.Fatalf("expected Content-Range 'bytes 256-767/1024', got %q", rr.Header().Get("Content-Range"))
	}
	if rr.Body.Len() != 512 {
		t.Fatalf("expected 512 bytes, got %d", rr.Body.Len())
	}

	// 4. Out of bounds range (HTTP 416)
	req = httptest.NewRequest(http.MethodGet, "/api/downloads/stream/stream_test.mp4", nil)
	req.Header.Set("Range", "bytes=2000-3000")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected 416 Range Not Satisfiable, got %d", rr.Code)
	}

	// 5. Path traversal attack rejection
	req = httptest.NewRequest(http.MethodGet, "/api/downloads/stream/..%2f..%2fwindows%2fnotepad.exe", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Fatalf("SECURITY VIOLATION: expected 400 or 404 for traversal, got %d", rr.Code)
	}
}

func TestListFilesAndDeleteFile(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	downloadsDir := filepath.Join(tempDir, "downloads")
	fileA := filepath.Join(downloadsDir, "fileA.mp4")
	_ = os.WriteFile(fileA, []byte("mediaA"), 0644)

	// List files
	req := httptest.NewRequest(http.MethodGet, "/api/downloads/files", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list files error: %d", rr.Code)
	}

	var files []downloader.FileInfo
	if err := json.NewDecoder(rr.Body).Decode(&files); err != nil {
		t.Fatalf("decode files error: %v", err)
	}
	if len(files) != 1 || files[0].FileName != "fileA.mp4" {
		t.Fatalf("unexpected files list: %v", files)
	}

	// Delete file
	req = httptest.NewRequest(http.MethodDelete, "/api/downloads/files/fileA.mp4", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("delete file error: %d", rr.Code)
	}

	if _, err := os.Stat(fileA); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted")
	}
}

func TestWatchPartyEndpoint(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	// 1. GET /api/watchparty/code generates valid room code
	reqCode := httptest.NewRequest(http.MethodGet, "/api/watchparty/code", nil)
	rrCode := httptest.NewRecorder()
	router.ServeHTTP(rrCode, reqCode)

	if rrCode.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/watchparty/code, got %d", rrCode.Code)
	}

	var codeResp map[string]string
	if err := json.Unmarshal(rrCode.Body.Bytes(), &codeResp); err != nil {
		t.Fatalf("failed to decode code response: %v", err)
	}
	if !strings.HasPrefix(codeResp["roomId"], "WP-") {
		t.Fatalf("expected room code with prefix WP-, got %s", codeResp["roomId"])
	}

	// 2. GET /api/watchparty/ws with invalid room code returns 400 Bad Request
	reqInvalid := httptest.NewRequest(http.MethodGet, "/api/watchparty/ws?roomId=invalid-code", nil)
	rrInvalid := httptest.NewRecorder()
	router.ServeHTTP(rrInvalid, reqInvalid)

	if rrInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid room code, got %d", rrInvalid.Code)
	}

	// 3. GET /api/watchparty/ws with valid room code but without WS handshake headers returns 400 (handshake error)
	reqValid := httptest.NewRequest(http.MethodGet, "/api/watchparty/ws?roomId="+codeResp["roomId"], nil)
	rrValid := httptest.NewRecorder()
	router.ServeHTTP(rrValid, reqValid)

	if rrValid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for non-websocket request, got %d", rrValid.Code)
	}
}
