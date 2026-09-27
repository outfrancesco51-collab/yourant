package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

// TestAdv_API_InvalidPayloadsAndEdgeCases tests HTTP API responses against malformed inputs.
func TestAdv_API_InvalidPayloadsAndEdgeCases(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	// 1. Malformed JSON to tracking update
	req := httptest.NewRequest(http.MethodPost, "/api/user/tracking/update", strings.NewReader("{invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed JSON, got %d", rr.Code)
	}

	// 2. Negative mediaId to tracking update
	req = httptest.NewRequest(http.MethodPost, "/api/user/tracking/update", strings.NewReader(`{"mediaId": -10}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for negative mediaId, got %d", rr.Code)
	}

	// 3. Download request with invalid scheme (ftp / file)
	downloadPayloads := []string{
		`{"url": "ftp://malicious.com/file.mp4"}`,
		`{"url": "file:///c:/windows/system32/cmd.exe"}`,
		`{"url": "javascript:alert(1)"}`,
		`{"url": ""}`,
	}
	for _, payload := range downloadPayloads {
		req = httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid download payload %q, got %d", payload, rr.Code)
		}
	}

	// 4. Delete file traversal attempt
	req = httptest.NewRequest(http.MethodDelete, "/api/downloads/files/..%2fevil.mp4", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal in DELETE, got %d", rr.Code)
	}

	// 5. Anime detail with non-existent ID returns 404 (not 500)
	req = httptest.NewRequest(http.MethodGet, "/api/anime/9999999", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for missing anime, got %d", rr.Code)
	}
}

// TestAdv_API_ConcurrentTrackingUpdates tests concurrent HTTP POSTs to /api/user/tracking/update.
func TestAdv_API_ConcurrentTrackingUpdates(t *testing.T) {
	router, tempDir := setupTestApp(t)
	defer os.RemoveAll(tempDir)

	var wg sync.WaitGroup
	concurrency := 20

	for i := 1; i <= concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]any{
				"mediaId":       154587,
				"progress":      id,
				"totalEpisodes": 28,
				"score":         9.0,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/user/tracking/update", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected 200 OK for concurrent update %d, got %d (body: %s)", id, rr.Code, rr.Body.String())
			}
		}(i)
	}

	wg.Wait()
}
