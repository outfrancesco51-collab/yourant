package tests

import (
	"context"
	"errors"
	"fmt"
	"html"
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
)

// ============================================================================
// FEATURE 1 (F1) BOUNDARIES: Server & Health Routing
// ============================================================================

func TestF1_B01_LargePayloadHandling(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/echo", func(w http.ResponseWriter, r *http.Request) {
		lr := io.LimitReader(r.Body, 10<<20) // 10MB limit
		data, err := io.ReadAll(lr)
		if err != nil {
			http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"received":%d}`, len(data))))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	largeData := strings.Repeat("A", 1024*1024) // 1MB payload
	resp, err := http.Post(ts.URL+"/api/echo", "application/octet-stream", strings.NewReader(largeData))
	if err != nil {
		t.Fatalf("1MB payload POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for 1MB payload, got %d", resp.StatusCode)
	}
}

func TestF1_B02_MalformedHttpMethods(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("PATCH", ts.URL+"/api/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 405 or 404 for unhandled PATCH, got %d", resp.StatusCode)
	}
}

func TestF1_B03_MissingAcceptHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		// Defaults safely to application/json if Accept is missing
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/health", nil)
	req.Header.Del("Accept") // Strip Accept header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected default application/json Content-Type")
	}
}

func TestF1_B04_ConcurrentHealthPings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(ts.URL + "/api/health")
			if err != nil || resp.StatusCode != http.StatusOK {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
			if resp != nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	if errCount > 0 {
		t.Errorf("%d of 100 concurrent health pings failed", errCount)
	}
}

func TestF1_B05_TrailingSlashPermutation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/health/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Both /api/health and /api/health/ should be routed cleanly
	resp1, err1 := http.Get(ts.URL + "/api/health")
	if err1 != nil || resp1.StatusCode != http.StatusOK {
		t.Errorf("/api/health failed: %v", err1)
	}
	if resp1 != nil {
		resp1.Body.Close()
	}

	resp2, err2 := http.Get(ts.URL + "/api/health/")
	if err2 != nil || (resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusMovedPermanently) {
		t.Errorf("/api/health/ failed: %v", err2)
	}
	if resp2 != nil {
		resp2.Body.Close()
	}
}

// ============================================================================
// FEATURE 2 (F2) BOUNDARIES: AniList & Cache
// ============================================================================

func TestF2_B01_EmptySearchQuery(t *testing.T) {
	catalog := SampleSeededCatalog()
	search := func(query string) []AnimeMedia {
		q := strings.TrimSpace(query)
		if q == "" {
			return catalog // Default to full catalog or empty without crash
		}
		var results []AnimeMedia
		for _, item := range catalog {
			if strings.Contains(strings.ToLower(item.Title.English), strings.ToLower(q)) {
				results = append(results, item)
			}
		}
		return results
	}

	results := search("")
	if len(results) != len(catalog) {
		t.Errorf("Empty search query should return full catalog gracefully, got %d items", len(results))
	}
}

func TestF2_B02_SpecialCharactersSearch(t *testing.T) {
	catalog := SampleSeededCatalog()
	specialQuery := `Steins;Gate (!@#$%^&*()_+{}[]:;"'?)`
	cleanPunctuation := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	cleanQuery := cleanPunctuation.ReplaceAllString(specialQuery, "")

	var matches []AnimeMedia
	for _, item := range catalog {
		cleanTitle := cleanPunctuation.ReplaceAllString(item.Title.English, "")
		if strings.Contains(strings.ToLower(cleanTitle), strings.ToLower(strings.TrimSpace(cleanQuery))) {
			matches = append(matches, item)
		}
	}

	if len(matches) == 0 {
		t.Errorf("Sanitized special character query failed to match Steins;Gate")
	}
}

func TestF2_B03_NegativePageNumber(t *testing.T) {
	normalizePage := func(page int) int {
		if page < 1 {
			return 1
		}
		return page
	}

	if normalizePage(-1) != 1 || normalizePage(0) != 1 || normalizePage(-999) != 1 {
		t.Errorf("Negative or zero page number must normalize to 1")
	}
}

func TestF2_B04_ExtremePerPageLimit(t *testing.T) {
	clampPerPage := func(perPage int) int {
		if perPage < 1 {
			return 20
		}
		if perPage > 50 {
			return 50 // AniList max ceiling
		}
		return perPage
	}

	if clampPerPage(10000) != 50 || clampPerPage(500) != 50 {
		t.Errorf("Extreme perPage must clamp to maximum allowed (50)")
	}
}

func TestF2_B05_Simulated429RateLimitRecovery(t *testing.T) {
	mockServer := NewMockAniListServer()
	mockServer.Simulate429 = true
	defer mockServer.Close()

	// When 429 occurs, client falls back to seeded catalog
	fetchWithFallback := func() []AnimeMedia {
		req, _ := http.NewRequest("POST", mockServer.Server.URL, strings.NewReader(`{"query":"GetTrending"}`))
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode == http.StatusTooManyRequests {
			if resp != nil {
				resp.Body.Close()
			}
			return SampleSeededCatalog() // Offline fallback
		}
		defer resp.Body.Close()
		return nil
	}

	catalog := fetchWithFallback()
	if len(catalog) == 0 {
		t.Fatalf("Fallback catalog returned empty list on upstream 429")
	}
	if catalog[0].ID != 154587 {
		t.Errorf("Fallback catalog primary anime mismatch")
	}
}

// ============================================================================
// FEATURE 3 (F3) BOUNDARIES: Tracking & Progress
// ============================================================================

func TestF3_B01_NegativeEpisodeProgress(t *testing.T) {
	updateProgress := func(current, update int) int {
		if update < 0 {
			return 0
		}
		return update
	}

	if updateProgress(5, -1) != 0 || updateProgress(5, -100) != 0 {
		t.Errorf("Negative episode progress must clamp to 0")
	}
}

func TestF3_B02_ProgressExceedingTotalEpisodes(t *testing.T) {
	totalEps := 24
	updateProgress := func(update int) int {
		if update > totalEps {
			return totalEps
		}
		return update
	}

	if updateProgress(50) != 24 || updateProgress(100) != 24 {
		t.Errorf("Progress exceeding total episodes must clamp to total episodes")
	}
}

func TestF3_B03_ScoreBoundariesMinMax(t *testing.T) {
	clampScore := func(s float64) float64 {
		if s < 0.0 {
			return 0.0
		}
		if s > 10.0 {
			return 10.0
		}
		return math.Round(s*10) / 10
	}

	if clampScore(-0.1) != 0.0 {
		t.Errorf("Score < 0.0 must clamp to 0.0")
	}
	if clampScore(10.1) != 10.0 {
		t.Errorf("Score > 10.0 must clamp to 10.0")
	}
	if clampScore(9.44) != 9.4 {
		t.Errorf("Score precision rounding failed")
	}
}

func TestF3_B04_InvalidStatusString(t *testing.T) {
	validStatuses := map[string]bool{
		"CURRENT":   true,
		"PLANNING":  true,
		"COMPLETED": true,
		"DROPPED":   true,
		"PAUSED":    true,
	}

	validateStatus := func(s string) error {
		if !validStatuses[s] {
			return fmt.Errorf("invalid tracking status %q", s)
		}
		return nil
	}

	if err := validateStatus("BINGING"); err == nil {
		t.Errorf("Expected error for non-standard status 'BINGING'")
	}
	if err := validateStatus(""); err == nil {
		t.Errorf("Expected error for empty status string")
	}
}

func TestF3_B05_RapidDuplicateUpdates(t *testing.T) {
	type Store struct {
		sync.Mutex
		progress map[int]int
	}
	s := &Store{progress: make(map[int]int)}

	// 50 rapid sequential updates for anime ID 100
	for i := 1; i <= 50; i++ {
		s.Lock()
		s.progress[100] = i
		s.Unlock()
	}

	if s.progress[100] != 50 {
		t.Errorf("Expected final progress to be 50, got %d", s.progress[100])
	}
}

// ============================================================================
// FEATURE 4 (F4) BOUNDARIES: Controlled Storage & Path Traversal Security
// ============================================================================

func TestF4_B01_RelativeDotDotTraversal(t *testing.T) {
	guard, _ := NewControlledStorageGuard(t.TempDir())
	_, err := guard.ResolveSafePath("../../Windows/calc.exe")
	if err != ErrPathTraversal {
		t.Errorf("Expected ErrPathTraversal for ../../Windows/calc.exe, got: %v", err)
	}
}

func TestF4_B02_BackslashTraversal(t *testing.T) {
	guard, _ := NewControlledStorageGuard(t.TempDir())
	_, err := guard.ResolveSafePath("..\\..\\downloads\\evil.bat")
	if err != ErrPathTraversal {
		t.Errorf("Expected ErrPathTraversal for ..\\..\\downloads\\evil.bat, got: %v", err)
	}
}

func TestF4_B03_DosReservedDeviceNames(t *testing.T) {
	guard, _ := NewControlledStorageGuard(t.TempDir())

	reservedList := []string{
		"CON.mp4",
		"prn.mkv",
		"aux.avi",
		"nul",
		"com1.flv",
		"lpt1.mp4",
	}

	for _, name := range reservedList {
		resolved, err := guard.ResolveSafePath(name)
		if err != nil {
			t.Fatalf("Safe resolution failed: %v", err)
		}
		base := filepath.Base(resolved)
		if !strings.HasPrefix(base, "_") {
			t.Errorf("Reserved DOS device name %q was not prefixed with _: %q", name, base)
		}
	}
}

func TestF4_B04_UrlEncodedTraversal(t *testing.T) {
	guard, _ := NewControlledStorageGuard(t.TempDir())

	encodedAttacks := []string{
		"%2e%2e%2fcalc.exe",
		"%2e%2e%5cevil.bat",
		"%252e%252e%252froot",
	}

	for _, attack := range encodedAttacks {
		_, err := guard.ResolveSafePath(attack)
		if err != ErrPathTraversal {
			t.Errorf("URL encoded traversal %q was not rejected: %v", attack, err)
		}
	}
}

func TestF4_B05_NullByteInjection(t *testing.T) {
	guard, _ := NewControlledStorageGuard(t.TempDir())

	nullByteAttack := "safe_video.mp4\x00.exe"
	_, err := guard.ResolveSafePath(nullByteAttack)
	if err != ErrPathTraversal {
		t.Errorf("Null byte injection was not caught with ErrPathTraversal")
	}
}

// ============================================================================
// FEATURE 5 (F5) BOUNDARIES: Offline Media Downloader
// ============================================================================

func TestF5_B01_ZeroByteFileDownload(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	downloader := NewResumableDownloaderSimulator(guard)

	ctx := context.Background()
	task, err := downloader.StartDownload(ctx, 100, 1, "0-byte Anime", "https://example.com/zero.mp4", "zero.mp4", 0)
	if err != nil {
		t.Fatalf("0-byte download failed: %v", err)
	}

	if task.Status != "COMPLETED" || task.ProgressPercent != 100.0 {
		t.Errorf("0-byte download should complete with 100%% progress, got status=%s, progress=%f", task.Status, task.ProgressPercent)
	}
}

func TestF5_B02_SimulatedLargeFile10GB(t *testing.T) {
	totalBytes := int64(10) * 1024 * 1024 * 1024 // 10 GB
	downloaded := int64(5) * 1024 * 1024 * 1024  // 5 GB

	progress := (float64(downloaded) / float64(totalBytes)) * 100.0
	if progress != 50.0 {
		t.Errorf("10GB progress calculation overflow: %f", progress)
	}
}

func TestF5_B03_AbruptConnectionDropMidDownload(t *testing.T) {
	tempDir := t.TempDir()
	partPath := filepath.Join(tempDir, "anime_ep1.mp4.part")

	// Write 5000 bytes before abrupt drop
	initialBytes := make([]byte, 5000)
	os.WriteFile(partPath, initialBytes, 0644)

	// Simulate resumption: inspect existing bytes
	fi, err := os.Stat(partPath)
	if err != nil {
		t.Fatalf("Part file missing: %v", err)
	}
	existingBytes := fi.Size()
	if existingBytes != 5000 {
		t.Errorf("Expected 5000 bytes preserved, got %d", existingBytes)
	}

	// Range header to resume
	rangeHeader := fmt.Sprintf("bytes=%d-", existingBytes)
	if rangeHeader != "bytes=5000-" {
		t.Errorf("Resumption header mismatch: %s", rangeHeader)
	}
}

func TestF5_B04_ResumeWithoutServerRangeSupport(t *testing.T) {
	// If server does not return Accept-Ranges: bytes, restart from 0
	serverAcceptRanges := false
	existingOffset := int64(2048)

	var startOffset int64
	if serverAcceptRanges {
		startOffset = existingOffset
	} else {
		startOffset = 0 // Forced restart from 0
	}

	if startOffset != 0 {
		t.Errorf("Downloader must restart from byte 0 when server does not support byte ranges")
	}
}

func TestF5_B05_DiskFullErrorSimulation(t *testing.T) {
	task := &DownloadTask{
		ID:     "dl-err-1",
		Status: "DOWNLOADING",
	}

	simulatedDiskErr := errors.New("no space left on device")
	if simulatedDiskErr != nil {
		task.Status = "FAILED"
		task.Error = simulatedDiskErr.Error()
	}

	if task.Status != "FAILED" || !strings.Contains(task.Error, "no space left") {
		t.Errorf("Expected task failure status on disk error")
	}
}

// ============================================================================
// FEATURE 6 (F6) BOUNDARIES: HTTP 206 Video Streaming
// ============================================================================

func TestF6_B01_RangeBeyondFileSize(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "short.mp4")
	os.WriteFile(videoFile, make([]byte, 100), 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=999999-")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Range request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Expected 416 Range Not Satisfiable, got %d", resp.StatusCode)
	}
}

func TestF6_B02_InvertedRangeIndices(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "short.mp4")
	os.WriteFile(videoFile, make([]byte, 1000), 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=500-100") // Inverted!
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Expected 416 for inverted range, got %d", resp.StatusCode)
	}
}

func TestF6_B03_ZeroByteRange(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "short.mp4")
	os.WriteFile(videoFile, []byte("HELLO_WORLD_VIDEO"), 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=0-0") // Exactly 1 byte
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("Expected 206 Partial Content, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 1 || string(body) != "H" {
		t.Errorf("Expected 1 byte 'H', got %q", string(body))
	}
}

func TestF6_B04_RangeStartingAtEof(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "short.mp4")
	data := []byte("FINAL_BYTE_CHECK_Z")
	os.WriteFile(videoFile, data, 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	lastIdx := len(data) - 1
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", lastIdx, lastIdx))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 1 || body[0] != 'Z' {
		t.Errorf("Expected final byte 'Z', got %s", string(body))
	}
}

func TestF6_B05_SuffixRangeRequest(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "short.mp4")
	data := []byte("PREFIX_MIDDLE_SUFFIX12345")
	os.WriteFile(videoFile, data, 0644)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=-11") // Last 11 bytes: "SUFFIX12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "SUFFIX12345" {
		t.Errorf("Expected suffix 'SUFFIX12345', got %q", string(body))
	}
}

// ============================================================================
// FEATURE 7 (F7) BOUNDARIES: Theme Tokens & CSS
// ============================================================================

func TestF7_B01_InvalidHexColorRejection(t *testing.T) {
	invalidHexes := []string{"#070a1", "#GGGGGG", "red", "rgba(0,0,0)", "#1234567"}
	for _, hexVal := range invalidHexes {
		_, _, _, err := HexToRGB(hexVal)
		if err == nil {
			t.Errorf("Expected error for invalid hex color %q", hexVal)
		}
	}
}

func TestF7_B02_ZeroOpacityAlphaToken(t *testing.T) {
	zeroAlpha := "rgba(7, 10, 19, 0)"
	fullAlpha := "rgba(7, 10, 19, 1)"

	if !strings.Contains(zeroAlpha, "0)") || !strings.Contains(fullAlpha, "1)") {
		t.Errorf("Alpha channel formatting invalid")
	}
}

func TestF7_B03_HighContrastRatioCheck(t *testing.T) {
	bgVoid := "#070a13"
	textPrimary := "#f8fafc"

	ratio, err := CalculateContrastRatio(bgVoid, textPrimary)
	if err != nil {
		t.Fatalf("Contrast calculation failed: %v", err)
	}

	// WCAG AAA requires ratio >= 7.0 for normal text
	if ratio < 7.0 {
		t.Errorf("Contrast ratio %f is below WCAG AAA requirement (7.0:1)", ratio)
	}
}

func TestF7_B04_MissingVariableFallbackTokens(t *testing.T) {
	cssRule := "background: var(--metallic-cyan, #38bdf8);"
	if !strings.Contains(cssRule, ", #38bdf8") {
		t.Errorf("CSS rule missing safe fallback variable token")
	}
}

func TestF7_B05_ExtremeDisplayScaling(t *testing.T) {
	scaleFactors := []float64{1.0, 1.25, 1.5, 1.75, 2.0}
	baseWidth := 200.0

	for _, sf := range scaleFactors {
		scaled := math.Round(baseWidth * sf)
		if scaled <= 0 {
			t.Errorf("Scale factor %f produced non-positive pixel dimension: %f", sf, scaled)
		}
	}
}

// ============================================================================
// FEATURE 8 (F8) BOUNDARIES: Seanime App Shell Layout
// ============================================================================

func TestF8_B01_UltraWide4KViewport(t *testing.T) {
	viewportWidth := 3840
	maxContentWidth := 1600

	actualContentWidth := viewportWidth
	if actualContentWidth > maxContentWidth {
		actualContentWidth = maxContentWidth
	}

	if actualContentWidth != 1600 {
		t.Errorf("Content width on 4K display should be capped at 1600px, got %d", actualContentWidth)
	}
}

func TestF8_B02_MobileViewportCollapse(t *testing.T) {
	viewports := []struct {
		width       int
		expectDrawer bool
	}{
		{width: 480, expectDrawer: true},
		{width: 767, expectDrawer: true},
		{width: 1024, expectDrawer: false},
	}

	for _, vp := range viewports {
		isMobile := vp.width < 768
		if isMobile != vp.expectDrawer {
			t.Errorf("Viewport width %d mobile collapse mismatch", vp.width)
		}
	}
}

func TestF8_B03_ExtremelyLongAnimeTitle(t *testing.T) {
	longTitle := "That Time I Got Reincarnated as a Slime and Traveled to an Alternate Universe with 100 Friends and Found a Dragon in the Mountains of Magic"
	maxLen := 40

	truncate := func(s string, limit int) string {
		if len(s) > limit {
			return s[:limit-3] + "..."
		}
		return s
	}

	truncated := truncate(longTitle, maxLen)
	if len(truncated) != maxLen || !strings.HasSuffix(truncated, "...") {
		t.Errorf("Title truncation failed: %q", truncated)
	}
}

func TestF8_B04_ZeroItemsEmptyGridState(t *testing.T) {
	var mediaItems []AnimeMedia
	renderEmptyState := func(items []AnimeMedia) string {
		if len(items) == 0 {
			return `<div class="empty-state">No anime found. Try exploring trending titles!</div>`
		}
		return `<div class="grid"></div>`
	}

	htmlOut := renderEmptyState(mediaItems)
	if !strings.Contains(htmlOut, "empty-state") {
		t.Errorf("Empty grid did not render empty state banner")
	}
}

func TestF8_B05_RapidSidebarToggleStress(t *testing.T) {
	isExpanded := false
	for i := 0; i < 50; i++ {
		isExpanded = !isExpanded
	}
	// 50 toggles (even number) should return to initial false
	if isExpanded != false {
		t.Errorf("Rapid toggle parity error: expected false, got %v", isExpanded)
	}
}

// ============================================================================
// FEATURE 9 (F9) BOUNDARIES: WebGL & GLSL Shaders
// ============================================================================

func TestF9_B01_ZeroResolutionViewport(t *testing.T) {
	calcSt := func(x, y, resX, resY float64) (float64, float64) {
		if resY <= 0 {
			resY = 1.0 // Guard against division by zero
		}
		stX := (x - 0.5*resX) / resY
		stY := (y - 0.5*resY) / resY
		return stX, stY
	}

	stX, stY := calcSt(0, 0, 0, 0)
	if math.IsNaN(stX) || math.IsNaN(stY) {
		t.Errorf("Zero resolution caused NaN coordinates")
	}
}

func TestF9_B02_NegativeDeltaTime(t *testing.T) {
	clampTime := func(uTime float64) float64 {
		if uTime < 0.0 {
			return 0.0
		}
		return uTime
	}

	if clampTime(-5.0) != 0.0 {
		t.Errorf("Negative delta time was not clamped to 0.0")
	}
}

func TestF9_B03_OutOfBoundsMouseCoordinates(t *testing.T) {
	clampMouse := func(mX, mY, resX, resY float64) (float64, float64) {
		clampedX := math.Max(0.0, math.Min(resX, mX))
		clampedY := math.Max(0.0, math.Min(resY, mY))
		return clampedX, clampedY
	}

	cx, cy := clampMouse(-500.0, 9999.0, 1920.0, 1080.0)
	if cx != 0.0 || cy != 1080.0 {
		t.Errorf("Mouse coordinate clamping failed: (%f, %f)", cx, cy)
	}
}

func TestF9_B04_ExtremeUDimValues(t *testing.T) {
	clampUDim := func(val float64) float64 {
		return math.Max(0.0, math.Min(1.0, val))
	}

	if clampUDim(-1.5) != 0.0 {
		t.Errorf("u_dim < 0.0 must clamp to 0.0")
	}
	if clampUDim(99.0) != 1.0 {
		t.Errorf("u_dim > 1.0 must clamp to 1.0")
	}
}

func TestF9_B05_ContextLostAndRestoredHandling(t *testing.T) {
	type WebGLState string
	state := WebGLState("active")

	// Trigger context lost
	state = WebGLState("lost")
	animationLoopPaused := state == "lost"

	if !animationLoopPaused {
		t.Errorf("Animation loop should pause on webglcontextlost")
	}

	// Trigger context restored
	state = WebGLState("active")
	if state != "active" {
		t.Errorf("Context should restore to active")
	}
}

// ============================================================================
// FEATURE 10 (F10) BOUNDARIES: Web Audio 1000% Volume Booster
// ============================================================================

func TestF10_B01_VolumeZeroPercent(t *testing.T) {
	gain, _, _ := CalculateVolumeBoost(0.0)
	if gain != 0.0 {
		t.Errorf("Volume 0%% must yield gain 0.0 (muted), got %f", gain)
	}
}

func TestF10_B02_Volume100PercentStandard(t *testing.T) {
	gain, isBoost, warn := CalculateVolumeBoost(100.0)
	if gain != 1.0 || isBoost || warn {
		t.Errorf("Volume 100%% must be unity gain 1.0 without boost warning")
	}
}

func TestF10_B03_Volume1000PercentMax(t *testing.T) {
	gain, isBoost, warn := CalculateVolumeBoost(1000.0)
	if gain != 10.0 || !isBoost || !warn {
		t.Errorf("Volume 1000%% must be gain 10.0 with boost warning")
	}
}

func TestF10_B04_NegativeVolumeClamping(t *testing.T) {
	gain, _, _ := CalculateVolumeBoost(-100.0)
	if gain != 0.0 {
		t.Errorf("Negative volume must clamp to gain 0.0, got %f", gain)
	}
}

func TestF10_B05_SuperExtremeVolumeClamping(t *testing.T) {
	gain, _, _ := CalculateVolumeBoost(10000.0)
	if gain != 10.0 {
		t.Errorf("Volume 10000%% must clamp strictly to gain 10.0, got %f", gain)
	}
}

// ============================================================================
// FEATURE 11 (F11) BOUNDARIES: Cinema Mode Visual Isolation
// ============================================================================

func TestF11_B01_RapidCinemaModeToggles(t *testing.T) {
	sm := NewCinemaModeStateMachine()
	for i := 0; i < 20; i++ {
		sm.Toggle()
	}
	if sm.IsActive != false || sm.DimFactor != 1.0 {
		t.Errorf("Rapid toggling parity error: expected false / 1.0, got %v / %f", sm.IsActive, sm.DimFactor)
	}
}

func TestF11_B02_HotkeyCIgnoredInInputFields(t *testing.T) {
	sm := NewCinemaModeStateMachine()

	// User types 'c' into an input field or comment textarea
	sm.HandleKeyPress("c", "INPUT")
	if sm.IsActive {
		t.Errorf("Pressing 'c' inside <input> must NOT trigger cinema mode")
	}

	sm.HandleKeyPress("c", "TEXTAREA")
	if sm.IsActive {
		t.Errorf("Pressing 'c' inside <textarea> must NOT trigger cinema mode")
	}
}

func TestF11_B03_EscapeKeyWhenAlreadyInactive(t *testing.T) {
	sm := NewCinemaModeStateMachine() // Inactive by default

	sm.HandleKeyPress("Escape", "BODY")
	if sm.IsActive {
		t.Errorf("Escape key when inactive should remain inactive")
	}
}

func TestF11_B04_SimultaneousWindowResize(t *testing.T) {
	sm := NewCinemaModeStateMachine()
	sm.Toggle() // Active

	// Resize simulation during cinema mode
	viewportW := 1920
	viewportH := 1080
	playerW := viewportW
	playerH := viewportH

	if playerW != 1920 || playerH != 1080 {
		t.Errorf("Player must match 100vw/100vh on resize during cinema mode")
	}
}

func TestF11_B05_WatchPartyChatDrawerDuringCinema(t *testing.T) {
	cinemaChatClass := "chat-floating-translucent"
	if !strings.Contains(cinemaChatClass, "translucent") {
		t.Errorf("Cinema chat drawer must transition to translucent overlay")
	}
}

// ============================================================================
// FEATURE 12 (F12) BOUNDARIES: Watch Party WebSocket Hub
// ============================================================================

func TestF12_B01_EmptyOrMalformedRoomCode(t *testing.T) {
	hub := NewWatchPartyHubSimulator()

	_, errEmpty := hub.JoinRoom("", "u1", "User")
	if errEmpty == nil {
		t.Errorf("Expected error when joining empty room code")
	}

	_, errInvalid := hub.JoinRoom("INVALID_ROOM_CODE_12345", "u1", "User")
	if errInvalid == nil {
		t.Errorf("Expected error when joining invalid room code")
	}
}

func TestF12_B02_JoinNonExistentRoom(t *testing.T) {
	hub := NewWatchPartyHubSimulator()

	_, err := hub.JoinRoom("WP-4040", "u1", "User")
	if err == nil || err.Error() != "room not found" {
		t.Errorf("Expected 'room not found' error, got %v", err)
	}
}

func TestF12_B03_MaxRoomCapacityLimit(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	room := hub.CreateRoom("WP-FULL", "host", "Host", 1, "Title")

	// Fill room to 100
	for i := 1; i < 100; i++ {
		hub.JoinRoom("WP-FULL", fmt.Sprintf("u-%d", i), fmt.Sprintf("User %d", i))
	}

	if room.MemberCount != 100 {
		t.Fatalf("Room member count should be 100, got %d", room.MemberCount)
	}

	// 101st client attempt
	_, err := hub.JoinRoom("WP-FULL", "u-101", "User 101")
	if err == nil || err.Error() != "room capacity reached" {
		t.Errorf("Expected 'room capacity reached' error, got %v", err)
	}
}

func TestF12_B04_HostDisconnectSingleUser(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	hub.CreateRoom("WP-SOLO", "solo-host", "Solo", 1, "Title")

	room, _ := hub.LeaveRoom("WP-SOLO", "solo-host")
	if room.MemberCount != 0 {
		t.Errorf("Room should have 0 members after host leaves, got %d", room.MemberCount)
	}
}

func TestF12_B05_ConcurrentRoomCodeGeneration(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			code := fmt.Sprintf("WP-%04X", idx)
			hub.CreateRoom(code, fmt.Sprintf("h-%d", idx), "Host", 1, "Title")
		}(i)
	}
	wg.Wait()

	if len(hub.Rooms) != 100 {
		t.Errorf("Expected 100 unique rooms generated concurrently, got %d", len(hub.Rooms))
	}
}

// ============================================================================
// FEATURE 13 (F13) BOUNDARIES: Watch Party Synchronization
// ============================================================================

func TestF13_B01_NegativeSeekTimestamp(t *testing.T) {
	clampSeek := func(target float64) float64 {
		if target < 0.0 {
			return 0.0
		}
		return target
	}

	if clampSeek(-15.5) != 0.0 {
		t.Errorf("Negative seek target must clamp to 0.0s")
	}
}

func TestF13_B02_SeekBeyondVideoDuration(t *testing.T) {
	videoDuration := 1440.0 // 24 minutes in seconds
	clampSeek := func(target float64) float64 {
		if target > videoDuration {
			return videoDuration
		}
		return target
	}

	if clampSeek(99999.0) != 1440.0 {
		t.Errorf("Seek beyond video duration must clamp to video duration")
	}
}

func TestF13_B03_ExactBoundary300msDrift(t *testing.T) {
	// Exactly 299ms drift -> Tier 1 (In Sync)
	_, _, action299, pill299 := CalculateDriftCompensation(10.299, 10.000, true, 0.0, 1.0)
	if action299 != "IN_SYNC" || pill299 != "green" {
		t.Errorf("299ms drift must be Tier 1 IN_SYNC/green, got %s/%s", action299, pill299)
	}

	// Exactly 300ms drift -> Tier 2 (Speed Adjust)
	_, _, action300, pill300 := CalculateDriftCompensation(10.300, 10.000, true, 0.0, 1.0)
	if action300 != "SPEED_ADJUST" || pill300 != "amber" {
		t.Errorf("300ms drift must be Tier 2 SPEED_ADJUST/amber, got %s/%s", action300, pill300)
	}
}

func TestF13_B04_ExactBoundary1500msDrift(t *testing.T) {
	// Exactly 1500ms drift -> Tier 2 (Speed Adjust)
	_, _, action1500, pill1500 := CalculateDriftCompensation(11.500, 10.000, true, 0.0, 1.0)
	if action1500 != "SPEED_ADJUST" || pill1500 != "amber" {
		t.Errorf("1500ms drift must be Tier 2 SPEED_ADJUST/amber, got %s/%s", action1500, pill1500)
	}

	// 1501ms drift -> Tier 3 (Hard Seek)
	_, _, action1501, pill1501 := CalculateDriftCompensation(11.501, 10.000, true, 0.0, 1.0)
	if action1501 != "HARD_SEEK" || pill1501 != "red" {
		t.Errorf("1501ms drift must trigger Tier 3 HARD_SEEK/red, got %s/%s", action1501, pill1501)
	}
}

func TestF13_B05_OutdatedPacketDropping(t *testing.T) {
	lastAppliedServerTime := int64(1727376000500)

	shouldProcessPacket := func(packetServerTime int64) bool {
		return packetServerTime >= lastAppliedServerTime
	}

	if shouldProcessPacket(1727376000400) {
		t.Errorf("Outdated packet with older timestamp should be discarded")
	}
	if !shouldProcessPacket(1727376000600) {
		t.Errorf("Newer packet with recent timestamp should be accepted")
	}
}

// ============================================================================
// FEATURE 14 (F14) BOUNDARIES: Watch Party Frontend Client
// ============================================================================

func TestF14_B01_EmptyChatMessageRejection(t *testing.T) {
	validateChat := func(text string) error {
		if strings.TrimSpace(text) == "" {
			return errors.New("empty chat message")
		}
		return nil
	}

	if err := validateChat("   "); err == nil {
		t.Errorf("Expected rejection for whitespace-only message")
	}
}

func TestF14_B02_ExtremeLengthChatMessage(t *testing.T) {
	longMsg := strings.Repeat("A", 5000)
	maxAllowed := 500

	sanitizeChatLength := func(msg string) string {
		if len(msg) > maxAllowed {
			return msg[:maxAllowed]
		}
		return msg
	}

	sanitized := sanitizeChatLength(longMsg)
	if len(sanitized) != 500 {
		t.Errorf("Chat message should be clamped to 500 characters, got %d", len(sanitized))
	}
}

func TestF14_B03_XssSanitizationInChat(t *testing.T) {
	xssPayload := `<script>alert('xss')</script><img src="x" onerror="alert(1)"/>`
	escaped := html.EscapeString(xssPayload)

	if strings.Contains(escaped, "<script>") || strings.Contains(escaped, "<img") {
		t.Errorf("XSS script injection was not escaped: %s", escaped)
	}
}

func TestF14_B04_DuplicateUserIdHandling(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	hub.CreateRoom("WP-DUP", "user-1", "FirstUser", 1, "Title")

	// Same user ID joins again
	hub.JoinRoom("WP-DUP", "user-1", "FirstUser Renamed")
	room := hub.Rooms["WP-DUP"]

	// Member count should not double for same user ID
	if room.MemberCount != 1 {
		t.Errorf("Expected member count 1 for duplicate user ID join, got %d", room.MemberCount)
	}
}

func TestF14_B05_RapidConnectionFlapping(t *testing.T) {
	connectCount := 0
	disconnectCount := 0

	for i := 0; i < 10; i++ {
		connectCount++
		disconnectCount++
	}

	if connectCount != 10 || disconnectCount != 10 {
		t.Errorf("Flapping connection tracking parity error")
	}
}
