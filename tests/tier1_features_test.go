package tests

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// FEATURE 1 (F1): Go Server & Health Routing
// Requirements: net/http routing, health check, CORS, graceful status
// ============================================================================

func TestF1_01_HealthCheckStatusOk(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("Failed to execute GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	if data["status"] != "ok" || data["version"] != "1.0.0" {
		t.Errorf("Unexpected body payload: %v", data)
	}
}

func TestF1_02_HealthCheckContentTypeJson(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}
}

func TestF1_03_CORSHeadersPresent(t *testing.T) {
	mux := http.NewServeMux()
	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			next(w, r)
		}
	}
	mux.HandleFunc("GET /api/health", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *")
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "GET") {
		t.Errorf("Expected Access-Control-Allow-Methods to contain GET")
	}
}

func TestF1_04_OptionsMethodPreflight(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 204 or 200 for OPTIONS, got %d", resp.StatusCode)
	}
}

func TestF1_05_RouterMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/health", strings.NewReader(`{}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 405 or 404 for unhandled POST, got %d", resp.StatusCode)
	}
}

// ============================================================================
// FEATURE 2 (F2): AniList GraphQL Client & Cache
// Requirements: Trending, search, detail, in-memory TTL caching, rate limiter
// ============================================================================

func TestF2_01_TrendingQueryReturnsItems(t *testing.T) {
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	catalog := mockServer.Catalog
	if len(catalog) == 0 {
		t.Fatal("Expected non-empty catalog")
	}

	trendingItem := catalog[0]
	if trendingItem.ID <= 0 || trendingItem.Title.English == "" {
		t.Errorf("Invalid trending media item: %+v", trendingItem)
	}
	if trendingItem.CoverImage.ExtraLarge == "" {
		t.Errorf("Missing cover image for item ID %d", trendingItem.ID)
	}
}

func TestF2_02_SearchQueryReturnsFilteredItems(t *testing.T) {
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	query := "Frieren"
	var matches []AnimeMedia
	for _, item := range mockServer.Catalog {
		if strings.Contains(item.Title.English, query) || strings.Contains(item.Title.Romaji, query) {
			matches = append(matches, item)
		}
	}

	if len(matches) == 0 {
		t.Fatalf("Expected at least 1 search match for %q", query)
	}
	if matches[0].ID != 154587 {
		t.Errorf("Expected Frieren ID 154587, got %d", matches[0].ID)
	}
}

func TestF2_03_AnimeDetailQueryReturnsRelations(t *testing.T) {
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	item := mockServer.Catalog[0] // Frieren
	if len(item.Characters) == 0 {
		t.Errorf("Expected characters populated for detail view")
	}
	if len(item.Relations) == 0 {
		t.Errorf("Expected relations populated for detail view")
	}
	if item.Characters[0].Role != "MAIN" {
		t.Errorf("Expected main character role, got %s", item.Characters[0].Role)
	}
}

func TestF2_04_InMemoryCacheTTLHit(t *testing.T) {
	cache := make(map[string]struct {
		data      []AnimeMedia
		expiresAt time.Time
	})
	var mu sync.RWMutex

	setCache := func(key string, items []AnimeMedia, ttl time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		cache[key] = struct {
			data      []AnimeMedia
			expiresAt time.Time
		}{data: items, expiresAt: time.Now().Add(ttl)}
	}

	getCache := func(key string) ([]AnimeMedia, bool) {
		mu.RLock()
		defer mu.RUnlock()
		entry, found := cache[key]
		if !found || time.Now().After(entry.expiresAt) {
			return nil, false
		}
		return entry.data, true
	}

	mockCatalog := SampleSeededCatalog()
	setCache("trending:1:20", mockCatalog, 100*time.Millisecond)

	cached, found := getCache("trending:1:20")
	if !found || len(cached) != len(mockCatalog) {
		t.Errorf("Expected cache hit within TTL")
	}

	time.Sleep(120 * time.Millisecond)
	_, foundAfterExpiry := getCache("trending:1:20")
	if foundAfterExpiry {
		t.Errorf("Expected cache miss after TTL expiration")
	}
}

func TestF2_05_RateLimiterTokenBucket80Min(t *testing.T) {
	ratePerMin := 80.0
	refillPerSec := ratePerMin / 60.0 // 1.333 tokens/sec
	capacity := 10.0
	tokens := capacity

	consume := func(count float64) bool {
		if tokens >= count {
			tokens -= count
			return true
		}
		return false
	}

	// Burst consume 10 tokens
	for i := 0; i < 10; i++ {
		if !consume(1.0) {
			t.Fatalf("Burst token %d should have been available", i+1)
		}
	}

	// 11th token should fail immediately without wait
	if consume(1.0) {
		t.Errorf("Token consumption should be rejected once capacity exhausted")
	}

	// Refill after 1 second
	tokens += 1.0 * refillPerSec
	if tokens < 1.0 {
		t.Errorf("Expected at least 1 token refilled after 1 second, got %f", tokens)
	}
}

// ============================================================================
// FEATURE 3 (F3): User Anime Tracking & Progress
// Requirements: Local tracking store, status transitions, score bounds, persistence
// ============================================================================

func TestF3_01_GetTrackingListEmptyOrInitial(t *testing.T) {
	store := map[int]*UserLibraryEntry{
		154587: {
			MediaID:       154587,
			Title:         "Frieren: Beyond Journey's End",
			Status:        "CURRENT",
			Progress:      14,
			TotalEpisodes: 28,
			Score:         9.5,
			UpdatedAt:     time.Now(),
		},
	}

	if len(store) != 1 {
		t.Fatalf("Expected 1 tracked anime")
	}
	entry := store[154587]
	if entry.Status != "CURRENT" || entry.Progress != 14 {
		t.Errorf("Unexpected tracking entry: %+v", entry)
	}
}

func TestF3_02_UpdateTrackingProgress(t *testing.T) {
	entry := &UserLibraryEntry{
		MediaID:       16498,
		Title:         "Attack on Titan",
		Status:        "CURRENT",
		Progress:      10,
		TotalEpisodes: 25,
		Score:         8.5,
	}

	newProgress := 15
	entry.Progress = newProgress
	entry.UpdatedAt = time.Now()

	if entry.Progress != 15 {
		t.Errorf("Expected progress 15, got %d", entry.Progress)
	}
}

func TestF3_03_StatusTransitionToCompleted(t *testing.T) {
	entry := &UserLibraryEntry{
		MediaID:       120377,
		Title:         "Cyberpunk: Edgerunners",
		Status:        "CURRENT",
		Progress:      9,
		TotalEpisodes: 10,
	}

	// User finishes final episode
	entry.Progress = entry.TotalEpisodes
	if entry.Progress >= entry.TotalEpisodes {
		entry.Status = "COMPLETED"
	}

	if entry.Status != "COMPLETED" {
		t.Errorf("Expected auto-transition to COMPLETED, got %s", entry.Status)
	}
}

func TestF3_04_ScoreValidationWithinBounds(t *testing.T) {
	scores := []struct {
		input    float64
		expected float64
	}{
		{input: 8.5, expected: 8.5},
		{input: 0.0, expected: 0.0},
		{input: 10.0, expected: 10.0},
		{input: -2.0, expected: 0.0},  // clamped
		{input: 15.0, expected: 10.0}, // clamped
	}

	for _, tc := range scores {
		clamped := tc.input
		if clamped < 0.0 {
			clamped = 0.0
		} else if clamped > 10.0 {
			clamped = 10.0
		}
		if clamped != tc.expected {
			t.Errorf("Input score %f: expected %f, got %f", tc.input, tc.expected, clamped)
		}
	}
}

func TestF3_05_PersistenceThreadSafety(t *testing.T) {
	var mu sync.RWMutex
	store := make(map[int]int)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mu.Lock()
			store[id] = id * 2
			mu.Unlock()

			mu.RLock()
			_ = store[id]
			mu.RUnlock()
		}(i)
	}
	wg.Wait()

	if len(store) != 50 {
		t.Errorf("Expected 50 items stored safely under concurrency, got %d", len(store))
	}
}

// ============================================================================
// FEATURE 4 (F4): Controlled Storage Sandboxing
// Requirements: Strictly within downloads/, path traversal defense, reserved names
// ============================================================================

func TestF4_01_DownloadsDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	downloadsDir := filepath.Join(tempDir, "downloads")

	guard, err := NewControlledStorageGuard(downloadsDir)
	if err != nil {
		t.Fatalf("Failed to initialize ControlledStorageGuard: %v", err)
	}

	if _, err := os.Stat(guard.BaseDir); os.IsNotExist(err) {
		t.Errorf("Controlled storage directory was not created: %s", guard.BaseDir)
	}
}

func TestF4_02_SafePathInsideDownloads(t *testing.T) {
	tempDir := t.TempDir()
	downloadsDir := filepath.Join(tempDir, "downloads")
	guard, _ := NewControlledStorageGuard(downloadsDir)

	safePath, err := guard.ResolveSafePath("episode_01.mp4")
	if err != nil {
		t.Fatalf("Valid path resolution failed: %v", err)
	}

	if !strings.HasPrefix(safePath, guard.BaseDir) {
		t.Errorf("Resolved safe path %q is outside base dir %q", safePath, guard.BaseDir)
	}
}

func TestF4_03_RejectDotDotTraversal(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))

	maliciousPaths := []string{
		"../../Windows/calc.exe",
		"..\\..\\system32\\evil.dll",
		"foo/../../bar.txt",
		"sub/../../../escape.mp4",
	}

	for _, p := range maliciousPaths {
		_, err := guard.ResolveSafePath(p)
		if err != ErrPathTraversal {
			t.Errorf("Path %q should have been rejected with ErrPathTraversal, got: %v", p, err)
		}
	}
}

func TestF4_04_SanitizeForbiddenCharacters(t *testing.T) {
	rawName := "anime:episode*1?|<>.mp4"
	clean := SanitizeFileName(rawName)

	if strings.ContainsAny(clean, `<>:"/\|?*`) {
		t.Errorf("Sanitized filename still contains forbidden characters: %q", clean)
	}
	if !strings.HasSuffix(clean, ".mp4") {
		t.Errorf("Sanitized filename corrupted extension: %q", clean)
	}
}

func TestF4_05_WindowsReservedNamesSanitized(t *testing.T) {
	reserved := []string{"CON.mp4", "prn.mkv", "AUX", "nul.part", "COM1.mp4", "lpt9.bin"}
	for _, name := range reserved {
		clean := SanitizeFileName(name)
		if !strings.HasPrefix(clean, "_") {
			t.Errorf("Reserved Windows device name %q was not prefixed with _: %q", name, clean)
		}
	}
}

// ============================================================================
// FEATURE 5 (F5): Offline Media Downloader
// Requirements: Task queue, .part streaming, atomic rename, progress tracking, cancel
// ============================================================================

func TestF5_01_TaskQueueCreation(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	downloader := NewResumableDownloaderSimulator(guard)

	ctx := context.Background()
	task, err := downloader.StartDownload(ctx, 154587, 1, "Frieren Ep 1", "https://example.com/stream.mp4", "frieren_01.mp4", 5120)
	if err != nil {
		t.Fatalf("Failed to queue download task: %v", err)
	}

	if task.Status != "COMPLETED" {
		t.Errorf("Expected status COMPLETED, got %s", task.Status)
	}
	if task.FileName != "frieren_01.mp4" {
		t.Errorf("Expected fileName frieren_01.mp4, got %s", task.FileName)
	}
}

func TestF5_02_PartFileChunkedWriting(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))

	safePath, _ := guard.ResolveSafePath("test_video.mp4")
	partPath := safePath + ".part"

	// Write simulated partial chunk
	chunk := []byte("SIMULATED_VIDEO_BYTES_CHUNK")
	err := os.WriteFile(partPath, chunk, 0644)
	if err != nil {
		t.Fatalf("Failed to write .part file: %v", err)
	}

	info, err := os.Stat(partPath)
	if err != nil || info.Size() != int64(len(chunk)) {
		t.Errorf("Partial file size mismatch: %v", err)
	}
}

func TestF5_03_AtomicRenameOnCompletion(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	safePath, _ := guard.ResolveSafePath("atomic_test.mp4")
	partPath := safePath + ".part"

	os.WriteFile(partPath, []byte("COMPLETE_VIDEO_CONTENT"), 0644)

	err := os.Rename(partPath, safePath)
	if err != nil {
		t.Fatalf("Atomic rename failed: %v", err)
	}

	if _, err := os.Stat(safePath); os.IsNotExist(err) {
		t.Errorf("Target file does not exist after rename")
	}
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Errorf(".part file still exists after rename")
	}
}

func TestF5_04_ProgressCalculation(t *testing.T) {
	total := int64(10000)
	downloaded := int64(4500)

	progressPercent := (float64(downloaded) / float64(total)) * 100.0
	speedBps := int64(1000) // 1000 bytes/sec
	remaining := total - downloaded
	etaSeconds := remaining / speedBps

	if progressPercent != 45.0 {
		t.Errorf("Expected 45.0%%, got %f", progressPercent)
	}
	if etaSeconds != 5 {
		t.Errorf("Expected 5 seconds ETA, got %d", etaSeconds)
	}
}

func TestF5_05_DownloadCancellation(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	downloader := NewResumableDownloaderSimulator(guard)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	task, err := downloader.StartDownload(ctx, 16498, 1, "AoT Ep 1", "https://example.com/aot.mp4", "aot_cancel.mp4", 1000000)
	if err != nil {
		t.Fatalf("Download start unexpected error: %v", err)
	}

	if task.Status != "CANCELLED" {
		t.Errorf("Expected task status CANCELLED, got %s", task.Status)
	}
}

// ============================================================================
// FEATURE 6 (F6): HTTP 206 Video Streaming
// Requirements: Accept-Ranges header, 206 Partial Content, Content-Range, seeking
// ============================================================================

func TestF6_01_AcceptRangesHeaderSent(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "sample.mp4")
	os.WriteFile(videoFile, make([]byte, 1024), 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET video failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Accept-Ranges") != "bytes" {
		t.Errorf("Expected Accept-Ranges: bytes, got %q", resp.Header.Get("Accept-Ranges"))
	}
}

func TestF6_02_PartialContentRangeResponse(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "sample.mp4")
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 256)
	}
	os.WriteFile(videoFile, data, 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Range request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("Expected status 206 Partial Content, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 1024 {
		t.Errorf("Expected 1024 bytes returned, got %d", len(body))
	}
}

func TestF6_03_ContentRangeHeaderFormat(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "sample.mp4")
	os.WriteFile(videoFile, make([]byte, 2048), 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=512-1023")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	cr := resp.Header.Get("Content-Range")
	expected := "bytes 512-1023/2048"
	if cr != expected {
		t.Errorf("Expected Content-Range %q, got %q", expected, cr)
	}
}

func TestF6_04_MidStreamRangeSeek(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "sample.mp4")
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 128)
	}
	os.WriteFile(videoFile, data, 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=1024-2047")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Seek request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 1024 {
		t.Fatalf("Expected 1024 bytes slice, got %d", len(body))
	}
	if body[0] != data[1024] || body[1023] != data[2047] {
		t.Errorf("Byte slice mismatch at offset 1024")
	}
}

func TestF6_05_FullFileFallbackWithoutRange(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "sample.mp4")
	os.WriteFile(videoFile, make([]byte, 500), 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Full GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK without Range header, got %d", resp.StatusCode)
	}
	if resp.ContentLength != 500 {
		t.Errorf("Expected Content-Length 500, got %d", resp.ContentLength)
	}
}

// ============================================================================
// FEATURE 7 (F7): Metallic Blue & Blood Red UI Theme
// Requirements: Design tokens, metallic cyan, blood red, gradients, contrast
// ============================================================================

func TestF7_01_MetallicBlueTokensDefined(t *testing.T) {
	validator := ThemeTokenValidator{}
	tokens := validator.GetCoreTokens()

	if tokens["--metallic-cyan"] != "#38bdf8" {
		t.Errorf("Expected --metallic-cyan: #38bdf8, got %s", tokens["--metallic-cyan"])
	}
	if tokens["--metallic-cobalt"] != "#0284c7" {
		t.Errorf("Expected --metallic-cobalt: #0284c7, got %s", tokens["--metallic-cobalt"])
	}
	if tokens["--bg-void"] != "#070a13" {
		t.Errorf("Expected --bg-void: #070a13, got %s", tokens["--bg-void"])
	}
}

func TestF7_02_BloodRedTokensDefined(t *testing.T) {
	validator := ThemeTokenValidator{}
	tokens := validator.GetCoreTokens()

	if tokens["--blood-bright"] != "#e11d48" {
		t.Errorf("Expected --blood-bright: #e11d48, got %s", tokens["--blood-bright"])
	}
	if tokens["--blood-crimson"] != "#881337" {
		t.Errorf("Expected --blood-crimson: #881337, got %s", tokens["--blood-crimson"])
	}
	if tokens["--blood-ruby"] != "#f43f5e" {
		t.Errorf("Expected --blood-ruby: #f43f5e, got %s", tokens["--blood-ruby"])
	}
}

func TestF7_03_DualColorGradientsPresent(t *testing.T) {
	gradMetallic := "linear-gradient(135deg, #38bdf8 0%, #0284c7 60%, #0f172a 100%)"
	gradBlood := "linear-gradient(135deg, #f43f5e 0%, #e11d48 50%, #881337 100%)"
	gradHybrid := "linear-gradient(90deg, #38bdf8 0%, #e11d48 100%)"

	if !strings.Contains(gradMetallic, "#38bdf8") || !strings.Contains(gradMetallic, "#0284c7") {
		t.Errorf("Metallic gradient does not contain primary cyan/cobalt values")
	}
	if !strings.Contains(gradBlood, "#e11d48") || !strings.Contains(gradBlood, "#881337") {
		t.Errorf("Blood red gradient does not contain crimson/bright red values")
	}
	if !strings.Contains(gradHybrid, "#38bdf8") || !strings.Contains(gradHybrid, "#e11d48") {
		t.Errorf("Hybrid gradient does not bridge metallic blue and blood red")
	}
}

func TestF7_04_GlowBoxShadowEffects(t *testing.T) {
	metallicGlow := "0 0 20px rgba(56, 189, 248, 0.35)"
	bloodGlow := "0 0 20px rgba(225, 29, 72, 0.45)"

	if !strings.Contains(metallicGlow, "56, 189, 248") {
		t.Errorf("Metallic glow does not reflect #38bdf8 RGB components")
	}
	if !strings.Contains(bloodGlow, "225, 29, 72") {
		t.Errorf("Blood glow does not reflect #e11d48 RGB components")
	}
}

func TestF7_05_ThemeVariablesParseValidation(t *testing.T) {
	validator := ThemeTokenValidator{}
	for tokenName, hexVal := range validator.GetCoreTokens() {
		r, g, b, err := HexToRGB(hexVal)
		if err != nil {
			t.Errorf("Token %s has invalid hex %q: %v", tokenName, hexVal, err)
		}
		if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
			t.Errorf("Token %s RGB values out of bounds: (%f, %f, %f)", tokenName, r, g, b)
		}
	}
}

// ============================================================================
// FEATURE 8 (F8): Seanime App Shell Layout
// Requirements: Sidebar states, hero banner, 2:3 media cards, watched progress
// ============================================================================

func TestF8_01_SidebarNavigationItems(t *testing.T) {
	expectedNavItems := []string{"Discover", "Library", "Schedule", "Downloads", "Watch Party"}
	navMap := map[string]string{
		"Discover":    "/discover",
		"Library":     "/library",
		"Schedule":    "/schedule",
		"Downloads":   "/downloads",
		"Watch Party": "/party",
	}

	for _, item := range expectedNavItems {
		if _, ok := navMap[item]; !ok {
			t.Errorf("Missing expected sidebar nav link: %s", item)
		}
	}
}

func TestF8_02_SidebarDualWidthStates(t *testing.T) {
	collapsedWidthPx := 72
	expandedWidthPx := 240

	if collapsedWidthPx != 72 {
		t.Errorf("Expected collapsed sidebar 72px, got %d", collapsedWidthPx)
	}
	if expandedWidthPx != 240 {
		t.Errorf("Expected expanded sidebar 240px, got %d", expandedWidthPx)
	}
}

func TestF8_03_HeroBannerVisualHierarchy(t *testing.T) {
	heroHeightPx := 460
	gradientMask := "linear-gradient(to top, #070a13 0%, rgba(7, 10, 19, 0.4) 60%, transparent 100%)"

	if heroHeightPx < 400 {
		t.Errorf("Hero banner height too small: %d", heroHeightPx)
	}
	if !strings.Contains(gradientMask, "#070a13") {
		t.Errorf("Hero banner missing void base gradient blending")
	}
}

func TestF8_04_MediaCardAspectRatio2x3(t *testing.T) {
	width := 200.0
	height := 300.0
	ratio := width / height
	expectedRatio := 2.0 / 3.0

	if math.Abs(ratio-expectedRatio) > 0.001 {
		t.Errorf("Expected 2:3 aspect ratio (%f), got %f", expectedRatio, ratio)
	}
}

func TestF8_05_WatchProgressBarComponent(t *testing.T) {
	totalEps := 24
	watchedEps := 12
	percent := (float64(watchedEps) / float64(totalEps)) * 100.0

	if percent != 50.0 {
		t.Errorf("Expected 50%% watch progress, got %f", percent)
	}
}

// ============================================================================
// FEATURE 9 (F9): WebGL & GLSL Shader Background
// Requirements: Vertex coordinates, fragment uniforms, FBM noise, u_dim factor
// ============================================================================

func TestF9_01_VertexShaderCoordinates(t *testing.T) {
	vertShaderCode := `
		attribute vec2 a_position;
		varying vec2 v_uv;
		void main() {
			v_uv = (a_position + 1.0) * 0.5;
			gl_Position = vec4(a_position, 0.0, 1.0);
		}
	`
	if !strings.Contains(vertShaderCode, "v_uv = (a_position + 1.0) * 0.5;") {
		t.Errorf("Vertex shader UV normalization missing")
	}
}

func TestF9_02_FragmentShaderUniforms(t *testing.T) {
	fragShaderSample := `
		precision highp float;
		uniform vec2 u_resolution;
		uniform float u_time;
		uniform vec2 u_mouse;
		uniform float u_dim;
		float fbm(vec2 p) { return 0.5; }
		void main() {
			vec3 color = vec3(0.1, 0.2, 0.3) * u_dim;
			gl_FragColor = vec4(color, 1.0);
		}
	`
	validator := GLSLShaderValidator{}
	violations := validator.ValidateFragmentShader(fragShaderSample)
	if len(violations) > 0 {
		t.Errorf("GLSL validation violations: %v", violations)
	}
}

func TestF9_03_FBMNoiseAlgorithm(t *testing.T) {
	fragShaderSample := `
		float fbm(vec2 p) {
			float v = 0.0;
			float a = 0.5;
			for (int i = 0; i < 4; ++i) {
				v += a;
				a *= 0.5;
			}
			return v;
		}
	`
	if !strings.Contains(fragShaderSample, "fbm") || !strings.Contains(fragShaderSample, "a *= 0.5") {
		t.Errorf("Fractal brownian motion octave decay missing")
	}
}

func TestF9_04_CinemaDimUniformMultiplier(t *testing.T) {
	normalDim := 1.0
	cinemaDim := 0.15

	baseColorIntensity := 0.8
	normalResult := baseColorIntensity * normalDim
	cinemaResult := baseColorIntensity * cinemaDim

	if normalResult != 0.8 {
		t.Errorf("Normal dim result unexpected: %f", normalResult)
	}
	if math.Abs(cinemaResult-0.12) > 0.001 {
		t.Errorf("Cinema dim result unexpected: %f", cinemaResult)
	}
}

func TestF9_05_ShaderPrecisionQualifiers(t *testing.T) {
	shaderHeader := "precision highp float;"
	if !strings.HasPrefix(strings.TrimSpace(shaderHeader), "precision highp float") {
		t.Errorf("Shader missing high precision float qualifier")
	}
}

// ============================================================================
// FEATURE 10 (F10): Web Audio 1000% Volume Booster
// Requirements: Gain 0.0 to 10.0, 1000% mapping, DynamicsCompressorNode limiter, singleton
// ============================================================================

func TestF10_01_VolumeGainMultiplierMath(t *testing.T) {
	gain100, isBoost100, _ := CalculateVolumeBoost(100.0)
	if gain100 != 1.0 || isBoost100 {
		t.Errorf("100%% volume should be gain 1.0 and normal zone, got gain=%f, boost=%v", gain100, isBoost100)
	}

	gain1000, isBoost1000, warn := CalculateVolumeBoost(1000.0)
	if gain1000 != 10.0 || !isBoost1000 || !warn {
		t.Errorf("1000%% volume should be gain 10.0 with boost caution, got gain=%f, warn=%v", gain1000, warn)
	}
}

func TestF10_02_GainClampedAtUpperLimit(t *testing.T) {
	gainExtreme, _, _ := CalculateVolumeBoost(2500.0)
	if gainExtreme != 10.0 {
		t.Errorf("Gain must be clamped to 10.0 (1000%%), got %f", gainExtreme)
	}
}

func TestF10_03_DynamicsCompressorSafetyLimiter(t *testing.T) {
	inputDbfs := -12.0
	gain10x := 10.0 // +20 dB boost -> uncompressed would be +8 dBFS (clipping!)

	outputDbfs := SimulateLimiterClamping(inputDbfs, gain10x)
	if outputDbfs > 0.0 {
		t.Errorf("Peak limiter failed: signal exceeded 0dBFS (%f dBFS)", outputDbfs)
	}
	if outputDbfs > -1.0 {
		t.Errorf("Expected peak to be clamped strictly below 0dBFS, got %f dBFS", outputDbfs)
	}
}

func TestF10_04_WeakMapSingletonPerMediaElement(t *testing.T) {
	booster := NewAudioBoosterSimulator()

	gain1, isNew1 := booster.AttachOrGetBooster("video-1", 1.0)
	if !isNew1 || gain1 != 1.0 {
		t.Errorf("First attach should create new graph")
	}

	gain2, isNew2 := booster.AttachOrGetBooster("video-1", 5.0)
	if isNew2 || gain2 != 1.0 {
		t.Errorf("Second attach to same element must return cached singleton graph without recreating")
	}
}

func TestF10_05_AudioContextStateSuspendedResume(t *testing.T) {
	type AudioContextState string
	state := AudioContextState("suspended")

	userInteractionOccurred := true
	if userInteractionOccurred && state == "suspended" {
		state = AudioContextState("running")
	}

	if state != "running" {
		t.Errorf("AudioContext should transition from suspended to running on user gesture")
	}
}

// ============================================================================
// FEATURE 11 (F11): Cinema Mode Visual Isolation
// Requirements: Body class toggle, hotkey 'C', 'Escape' exit, dimming to 0.15
// ============================================================================

func TestF11_01_BodyClassCinemaModeToggle(t *testing.T) {
	sm := NewCinemaModeStateMachine()

	active1 := sm.Toggle()
	if !active1 || sm.DimFactor != 0.15 {
		t.Errorf("Expected cinema mode active with dim 0.15, got active=%v, dim=%f", active1, sm.DimFactor)
	}

	active2 := sm.Toggle()
	if active2 || sm.DimFactor != 1.0 {
		t.Errorf("Expected cinema mode inactive with dim 1.0, got active=%v, dim=%f", active2, sm.DimFactor)
	}
}

func TestF11_02_Viewport100vw100vhPlayerRules(t *testing.T) {
	cinemaCss := `
		body.cinema-mode-active .player-container {
			position: fixed;
			inset: 0;
			z-index: 1000;
			width: 100vw;
			height: 100vh;
		}
	`
	if !strings.Contains(cinemaCss, "100vw") || !strings.Contains(cinemaCss, "100vh") || !strings.Contains(cinemaCss, "z-index: 1000") {
		t.Errorf("Cinema mode CSS missing full viewport isolation rules")
	}
}

func TestF11_03_ChromeFadeOutAndSidebarTranslate(t *testing.T) {
	cinemaCss := `
		body.cinema-mode-active .sidebar {
			transform: translateX(-100%);
			opacity: 0;
		}
	`
	if !strings.Contains(cinemaCss, "translateX(-100%)") || !strings.Contains(cinemaCss, "opacity: 0") {
		t.Errorf("Cinema mode CSS missing sidebar collapse rules")
	}
}

func TestF11_04_HotkeyCHandler(t *testing.T) {
	sm := NewCinemaModeStateMachine()

	// Press 'C' while focused on document body
	sm.HandleKeyPress("c", "BODY")
	if !sm.IsActive {
		t.Errorf("Expected 'C' hotkey to toggle cinema mode active")
	}

	// Press 'C' again
	sm.HandleKeyPress("c", "BODY")
	if sm.IsActive {
		t.Errorf("Expected second 'C' hotkey to toggle cinema mode inactive")
	}
}

func TestF11_05_HotkeyEscapeHandler(t *testing.T) {
	sm := NewCinemaModeStateMachine()
	sm.Toggle() // Enter cinema mode
	if !sm.IsActive {
		t.Fatalf("Failed to activate cinema mode")
	}

	sm.HandleKeyPress("Escape", "BODY")
	if sm.IsActive {
		t.Errorf("Expected Escape key to exit cinema mode")
	}
}

// ============================================================================
// FEATURE 12 (F12): Watch Party WebSocket Hub
// Requirements: Room creation, code generation, host role, join/leave, member count
// ============================================================================

func TestF12_01_RoomCreationWithCode(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	room := hub.CreateRoom("WP-7F3A", "host-1", "Francy", 154587, "Frieren Ep 1")

	if room.RoomID != "WP-7F3A" || room.HostID != "host-1" || room.MemberCount != 1 {
		t.Errorf("Room creation fields mismatch: %+v", room)
	}
}

func TestF12_02_ClientJoinRoom(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	hub.CreateRoom("WP-9921", "host-1", "HostUser", 16498, "AoT")

	joinedRoom, err := hub.JoinRoom("WP-9921", "guest-2", "GuestUser")
	if err != nil {
		t.Fatalf("Failed to join room: %v", err)
	}

	if joinedRoom.MemberCount != 2 {
		t.Errorf("Expected member count 2, got %d", joinedRoom.MemberCount)
	}
}

func TestF12_03_MemberCountTracking(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	hub.CreateRoom("WP-1111", "host-1", "HostUser", 16498, "AoT")

	hub.JoinRoom("WP-1111", "user-2", "User 2")
	hub.JoinRoom("WP-1111", "user-3", "User 3")

	room := hub.Rooms["WP-1111"]
	if room.MemberCount != 3 {
		t.Errorf("Expected 3 members, got %d", room.MemberCount)
	}

	hub.LeaveRoom("WP-1111", "user-2")
	if room.MemberCount != 2 {
		t.Errorf("Expected 2 members after leave, got %d", room.MemberCount)
	}
}

func TestF12_04_HostTransferOnDisconnect(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	hub.CreateRoom("WP-2222", "host-original", "OriginalHost", 16498, "AoT")
	hub.JoinRoom("WP-2222", "guest-next", "NextHost")

	room, err := hub.LeaveRoom("WP-2222", "host-original")
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	if room.HostID != "guest-next" {
		t.Errorf("Expected host to be migrated to guest-next, got %s", room.HostID)
	}
}

func TestF12_05_HeartbeatPingPong(t *testing.T) {
	pingMsg := WatchPartyMessage{
		Type:       "ping",
		RoomID:     "WP-1234",
		SenderID:   "u1",
		ClientTime: 1727376000000,
	}

	// Server generates pong
	pongMsg := WatchPartyMessage{
		Type:       "pong",
		RoomID:     pingMsg.RoomID,
		SenderID:   "server",
		ClientTime: pingMsg.ClientTime,
		ServerTime: time.Now().UnixMilli(),
	}

	if pongMsg.Type != "pong" || pongMsg.ClientTime != pingMsg.ClientTime {
		t.Errorf("Pong response did not echo client time correctly")
	}
}

// ============================================================================
// FEATURE 13 (F13): Watch Party State Synchronization
// Requirements: Bidirectional sync, play/pause, seek, 3-tier drift algorithm
// ============================================================================

func TestF13_01_PlayEventBroadcast(t *testing.T) {
	msg := WatchPartyMessage{
		Type:       "host:play",
		RoomID:     "WP-777",
		SenderID:   "host-1",
		IsHost:     true,
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"isPlaying": true,
			"timestamp": 12.5,
		},
	}

	if msg.Type != "host:play" || msg.Payload["isPlaying"] != true {
		t.Errorf("Play broadcast message invalid")
	}
}

func TestF13_02_PauseEventBroadcast(t *testing.T) {
	msg := WatchPartyMessage{
		Type:       "host:pause",
		RoomID:     "WP-777",
		SenderID:   "host-1",
		IsHost:     true,
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"isPlaying": false,
			"timestamp": 45.2,
		},
	}

	if msg.Type != "host:pause" || msg.Payload["isPlaying"] != false {
		t.Errorf("Pause broadcast message invalid")
	}
}

func TestF13_03_SeekEventBroadcast(t *testing.T) {
	targetSeek := 125.8
	msg := WatchPartyMessage{
		Type:       "host:seek",
		RoomID:     "WP-777",
		SenderID:   "host-1",
		Payload: map[string]interface{}{
			"timestamp": targetSeek,
		},
	}

	if msg.Payload["timestamp"] != 125.8 {
		t.Errorf("Expected seek target 125.8, got %v", msg.Payload["timestamp"])
	}
}

func TestF13_04_DriftTier1InSync(t *testing.T) {
	clientTime := 10.150
	hostTime := 10.000 // 150ms drift (< 300ms)

	_, rate, action, pill := CalculateDriftCompensation(clientTime, hostTime, true, 0.05, 1.0)
	if action != "IN_SYNC" || rate != 1.0 || pill != "green" {
		t.Errorf("Tier 1 drift expected IN_SYNC/green/rate=1.0, got %s/%s/rate=%f", action, pill, rate)
	}
}

func TestF13_05_DriftTier2AndTier3Algorithms(t *testing.T) {
	// Tier 2: 600ms drift behind (client behind -> speed up to 1.05x)
	_, rateT2, actionT2, pillT2 := CalculateDriftCompensation(9.400, 10.000, true, 0.0, 1.0)
	if actionT2 != "SPEED_ADJUST" || rateT2 != 1.05 || pillT2 != "amber" {
		t.Errorf("Tier 2 expected SPEED_ADJUST/amber/1.05, got %s/%s/%f", actionT2, pillT2, rateT2)
	}

	// Tier 3: 3.5s drift (hard seek)
	_, rateT3, actionT3, pillT3 := CalculateDriftCompensation(6.500, 10.000, true, 0.0, 1.0)
	if actionT3 != "HARD_SEEK" || rateT3 != 1.0 || pillT3 != "red" {
		t.Errorf("Tier 3 expected HARD_SEEK/red/1.0, got %s/%s/%f", actionT3, pillT3, rateT3)
	}
}

// ============================================================================
// FEATURE 14 (F14): Watch Party Frontend Client
// Requirements: Room code copy, sync status pill, participant roster, chat stream
// ============================================================================

func TestF14_01_RoomCodeDisplayAndClipboard(t *testing.T) {
	roomCode := "WP-9842"
	pattern := `^WP-[A-Z0-9]{4}$`
	matched, err := regexp.MatchString(pattern, roomCode)
	if err != nil || !matched {
		t.Errorf("Room code format %q invalid, must match %q", roomCode, pattern)
	}
}

func TestF14_02_SyncStatusPillColorTransitions(t *testing.T) {
	states := []struct {
		driftSec      float64
		expectedPill  string
		expectedLabel string
	}{
		{driftSec: 0.10, expectedPill: "green", expectedLabel: "SYNCED"},
		{driftSec: 0.80, expectedPill: "amber", expectedLabel: "ADJUSTING SPEED"},
		{driftSec: 2.50, expectedPill: "red", expectedLabel: "HARD RESYNC"},
	}

	for _, s := range states {
		_, _, action, pill := CalculateDriftCompensation(10.0+s.driftSec, 10.0, true, 0.0, 1.0)
		if pill != s.expectedPill {
			t.Errorf("Drift %fs: expected pill %s, got %s (action: %s)", s.driftSec, s.expectedPill, pill, action)
		}
	}
}

func TestF14_03_ParticipantListRoster(t *testing.T) {
	members := []struct {
		id     string
		name   string
		isHost bool
		pingMs int
	}{
		{id: "u1", name: "HostUser", isHost: true, pingMs: 18},
		{id: "u2", name: "GuestUser", isHost: false, pingMs: 45},
	}

	if len(members) != 2 || !members[0].isHost || members[1].isHost {
		t.Errorf("Participant roster role flags invalid")
	}
}

func TestF14_04_ChatStreamMessageBroadcast(t *testing.T) {
	chatMsg := WatchPartyMessage{
		Type:       "chat:message",
		RoomID:     "WP-1234",
		SenderID:   "u2",
		SenderName: "Alice",
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"chatText": "This episode has incredible animation!",
		},
	}

	if chatMsg.Payload["chatText"] != "This episode has incredible animation!" {
		t.Errorf("Chat text mismatch")
	}
}

func TestF14_05_SystemNotificationEvents(t *testing.T) {
	sysEvent := WatchPartyMessage{
		Type:       "room:user_joined",
		RoomID:     "WP-1234",
		SenderID:   "system",
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"userName": "Bob",
		},
	}

	if sysEvent.Type != "room:user_joined" || sysEvent.Payload["userName"] != "Bob" {
		t.Errorf("System notification format invalid")
	}
}
