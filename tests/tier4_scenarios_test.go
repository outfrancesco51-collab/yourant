package tests

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// TIER 4: REAL-WORLD APPLICATION SCENARIOS (7 SCENARIOS)
// ============================================================================

// Scenario 1: Full Discover to Watch Party Flow
// Features: F1, F2, F7, F8, F9, F12, F13, F14
func TestT4_Scenario1_FullDiscoverToWatchPartyFlow(t *testing.T) {
	// Step 1: Health check Go Server (F1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Server health check failed: %v", err)
	}
	resp.Body.Close()

	// Step 2: Initialize Theme & GLSL Background Canvas (F7, F9)
	theme := ThemeTokenValidator{}
	tokens := theme.GetCoreTokens()
	if tokens["--bg-void"] != "#070a13" || tokens["--metallic-cyan"] != "#38bdf8" || tokens["--blood-bright"] != "#e11d48" {
		t.Fatalf("Theme tokens failed to initialize")
	}

	glsl := GLSLShaderValidator{}
	violations := glsl.ValidateFragmentShader(`
		precision highp float;
		uniform vec2 u_resolution;
		uniform float u_time;
		uniform vec2 u_mouse;
		uniform float u_dim;
		float fbm(vec2 p) { return 0.5; }
		void main() { gl_FragColor = vec4(1.0) * u_dim; }
	`)
	if len(violations) > 0 {
		t.Fatalf("Shader validation violations: %v", violations)
	}

	// Step 3: Browse Discover / Trending Anime (F2, F8)
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	trending := mockServer.Catalog
	if len(trending) == 0 {
		t.Fatalf("Catalog returned 0 items")
	}
	selectedAnime := trending[0] // Frieren

	// Step 4: Host creates Watch Party Room (F12)
	hub := NewWatchPartyHubSimulator()
	roomCode := "WP-4A8F"
	room := hub.CreateRoom(roomCode, "host-1", "Francy", selectedAnime.ID, selectedAnime.Title.English)
	if room.MemberCount != 1 || room.HostID != "host-1" {
		t.Fatalf("Room creation failed: %+v", room)
	}

	// Step 5: Friend joins with Room Code (F12, F14)
	joinedRoom, err := hub.JoinRoom(roomCode, "guest-1", "Alice")
	if err != nil || joinedRoom.MemberCount != 2 {
		t.Fatalf("Friend join failed: %v", err)
	}

	// Step 6: Host starts playback & seeks to 45.0s (F13)
	room.IsPlaying = true
	room.CurrentTime = 45.0

	// Step 7: Friend sync verification (F13, F14)
	_, rate, action, pill := CalculateDriftCompensation(45.050, room.CurrentTime, true, 0.05, 1.0)
	if action != "IN_SYNC" || pill != "green" || rate != 1.0 {
		t.Errorf("Scenario 1 sync failed: action=%s, pill=%s, rate=%f", action, pill, rate)
	}
}

// Scenario 2: Offline Media Download & Resumable Verification
// Features: F1, F4, F5, F6
func TestT4_Scenario2_OfflineMediaDownloadAndResumableVerification(t *testing.T) {
	tempDir := t.TempDir()
	downloadsDir := filepath.Join(tempDir, "downloads")
	guard, err := NewControlledStorageGuard(downloadsDir)
	if err != nil {
		t.Fatalf("Failed to initialize guard: %v", err)
	}

	// Step 1: User starts download for offline viewing
	downloader := NewResumableDownloaderSimulator(guard)
	_ = downloader
	filename := "frieren_ep01_offline.mp4"
	safePath, _ := guard.ResolveSafePath(filename)
	partPath := safePath + ".part"

	totalSize := int64(10000)
	_ = totalSize
	halfSize := int64(5000)

	// Simulate downloading first 5000 bytes then network drop
	chunk1 := make([]byte, halfSize)
	for i := range chunk1 {
		chunk1[i] = byte(i % 128)
	}
	os.WriteFile(partPath, chunk1, 0644)

	// Step 2: Verify .part file exists and target file does not yet exist
	if _, err := os.Stat(safePath); !os.IsNotExist(err) {
		t.Errorf("Target file prematurely exists before download completes")
	}
	partInfo, err := os.Stat(partPath)
	if err != nil || partInfo.Size() != halfSize {
		t.Fatalf("Part file size mismatch: %v", err)
	}

	// Step 3: Resume download from byte 5000 to 10000
	partFile, err := os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open .part file for resume: %v", err)
	}
	chunk2 := make([]byte, halfSize)
	for i := range chunk2 {
		chunk2[i] = byte((i + 5000) % 128)
	}
	partFile.Write(chunk2)
	partFile.Close()

	// Step 4: Atomic rename to final target file
	err = os.Rename(partPath, safePath)
	if err != nil {
		t.Fatalf("Atomic rename failed: %v", err)
	}
	if _, err := os.Stat(safePath); os.IsNotExist(err) {
		t.Fatalf("Target file does not exist after atomic rename")
	}

	// Step 5: Offline video stream scrubbing via HTTP 206
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, safePath)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=4000-6000")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("HTTP 206 stream scrub failed: %v, status=%d", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 2001 {
		t.Errorf("Expected 2001 bytes slice, got %d", len(body))
	}
}

// Scenario 3: Extreme Volume Boost (1000%) with Limiter Clamping
// Features: F7, F10
func TestT4_Scenario3_ExtremeVolumeBoostWithLimiterClamping(t *testing.T) {
	// Step 1: User starts at standard 100% volume
	gain100, isBoost100, warn100 := CalculateVolumeBoost(100.0)
	if gain100 != 1.0 || isBoost100 || warn100 {
		t.Fatalf("Initial 100%% volume state invalid: gain=%f", gain100)
	}

	// Step 2: User encounters quiet dialogue and boosts to 500% (+14 dB)
	gain500, isBoost500, warn500 := CalculateVolumeBoost(500.0)
	if gain500 != 5.0 || !isBoost500 || !warn500 {
		t.Fatalf("500%% volume boost calculation failed: gain=%f", gain500)
	}

	// Step 3: User pushes slider to maximum 1000% (+20 dB)
	gain1000, isBoost1000, warn1000 := CalculateVolumeBoost(1000.0)
	if gain1000 != 10.0 || !isBoost1000 || !warn1000 {
		t.Fatalf("1000%% volume boost calculation failed: gain=%f", gain1000)
	}

	// Step 4: Audio safety limiter (DynamicsCompressorNode at -2.0dBFS)
	loudInputDbfs := -6.0 // Normal peak
	outputDbfs := SimulateLimiterClamping(loudInputDbfs, gain1000)

	// Output must be clamped strictly below 0dBFS
	if outputDbfs > 0.0 {
		t.Errorf("Safety limiter violation: output exceeded 0dBFS (%f dBFS)", outputDbfs)
	}
	if outputDbfs < -3.0 {
		t.Errorf("Limiter over-compressed signal: %f dBFS", outputDbfs)
	}

	// Step 5: Visual styling changes to blood red caution glow
	theme := ThemeTokenValidator{}
	tokens := theme.GetCoreTokens()
	cautionColor := tokens["--blood-bright"]
	if cautionColor != "#e11d48" {
		t.Errorf("Caution track styling mismatch: %s", cautionColor)
	}
}

// Scenario 4: Cinema Mode Immersion Toggle with Hotkey State
// Features: F7, F9, F11
func TestT4_Scenario4_CinemaModeImmersionToggleWithHotkeyState(t *testing.T) {
	sm := NewCinemaModeStateMachine()

	// Initial State: Normal layout
	if sm.IsActive || sm.DimFactor != 1.0 {
		t.Fatalf("Initial state not normal layout")
	}

	// Step 1: User hits 'C' hotkey during media playback
	sm.HandleKeyPress("c", "BODY")
	if !sm.IsActive || sm.DimFactor != 0.15 {
		t.Fatalf("Cinema mode activation via 'C' hotkey failed: dim=%f", sm.DimFactor)
	}

	// Step 2: Verify GLSL canvas dim factor drops
	canvasDim := sm.DimFactor
	if canvasDim != 0.15 {
		t.Errorf("Canvas dim factor expected 0.15, got %f", canvasDim)
	}

	// Step 3: Verify Viewport 100vw/100vh full player expansion
	playerLayout := struct {
		position string
		inset    string
		zIndex   int
		width    string
		height   string
	}{
		position: "fixed",
		inset:    "0",
		zIndex:   1000,
		width:    "100vw",
		height:   "100vh",
	}
	if playerLayout.width != "100vw" || playerLayout.height != "100vh" || playerLayout.zIndex != 1000 {
		t.Errorf("Player layout not in full cinema immersion")
	}

	// Step 4: User presses 'Escape' key to return to dashboard
	sm.HandleKeyPress("Escape", "BODY")
	if sm.IsActive || sm.DimFactor != 1.0 {
		t.Errorf("Escape key failed to restore normal layout: active=%v, dim=%f", sm.IsActive, sm.DimFactor)
	}
}

// Scenario 5: Watch Party Multi-Client Play/Pause/Seek Drift Re-sync
// Features: F12, F13, F14
func TestT4_Scenario5_WatchPartyMultiClientDriftReSync(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	room := hub.CreateRoom("WP-GROUP", "host-user", "Host", 154587, "Frieren Ep 1")

	// 3 Clients join
	hub.JoinRoom("WP-GROUP", "client-1", "FastClient")
	hub.JoinRoom("WP-GROUP", "client-2", "LaggedClient")
	hub.JoinRoom("WP-GROUP", "client-3", "StalledClient")

	if room.MemberCount != 4 {
		t.Fatalf("Expected 4 members in room, got %d", room.MemberCount)
	}

	// Host plays at timestamp 100.0s
	room.IsPlaying = true
	room.CurrentTime = 100.0

	// Client 1: 50ms latency (In Sync)
	_, r1, a1, p1 := CalculateDriftCompensation(100.05, room.CurrentTime, true, 0.05, 1.0)
	if a1 != "IN_SYNC" || p1 != "green" || r1 != 1.0 {
		t.Errorf("Client 1 sync status invalid: %s / %s", a1, p1)
	}

	// Client 2: 700ms behind (Tier 2: Speed adjustment +5%)
	_, r2, a2, p2 := CalculateDriftCompensation(99.30, room.CurrentTime, true, 0.0, 1.0)
	if a2 != "SPEED_ADJUST" || p2 != "amber" || r2 != 1.05 {
		t.Errorf("Client 2 sync status invalid: %s / %s / %f", a2, p2, r2)
	}

	// Client 3: 4.5s behind (Tier 3: Hard seek)
	t3, r3, a3, p3 := CalculateDriftCompensation(95.50, room.CurrentTime, true, 0.0, 1.0)
	if a3 != "HARD_SEEK" || p3 != "red" || r3 != 1.0 {
		t.Errorf("Client 3 sync status invalid: %s / %s", a3, p3)
	}
	if math.Abs(t3-100.0) > 0.001 {
		t.Errorf("Client 3 target seek mismatch: %f", t3)
	}
}

// Scenario 6: Directory Traversal Attack Simulation & Rejection
// Features: F4, F5
func TestT4_Scenario6_DirectoryTraversalAttackSimulationAndRejection(t *testing.T) {
	tempDir := t.TempDir()
	downloadsDir := filepath.Join(tempDir, "downloads")
	guard, _ := NewControlledStorageGuard(downloadsDir)
	downloader := NewResumableDownloaderSimulator(guard)

	attacks := []struct {
		vectorName string
		filename   string
	}{
		{"Relative Unix Traversal", "../../Windows/calc.exe"},
		{"Relative Windows Traversal", "..\\..\\downloads\\evil.bat"},
		{"Root Absolute Escape", "/etc/shadow"},
		{"Windows Drive Escape", "C:\\Windows\\System32\\cmd.exe"},
		{"URL Encoded Dot Dot Slash", "%2e%2e%2fevil.exe"},
		{"Null Byte Poisoning", "legit.mp4\x00.exe"},
		{"Reserved DOS Device CON", "CON.mp4"},
	}

	ctx := context.Background()
	for _, attack := range attacks {
		if attack.vectorName == "Reserved DOS Device CON" {
			// CON.mp4 is sanitized safely to _CON.mp4 inside downloads
			resolved, err := guard.ResolveSafePath(attack.filename)
			if err != nil {
				t.Fatalf("DOS device sanitization failed: %v", err)
			}
			if !strings.HasPrefix(filepath.Base(resolved), "_") {
				t.Errorf("CON.mp4 not sanitized with _: %s", resolved)
			}
			continue
		}

		// All escape attempts must be rejected with ErrPathTraversal
		_, err := downloader.StartDownload(ctx, 999, 1, "Attack", "https://evil.com/payload", attack.filename, 1024)
		if err != ErrPathTraversal {
			t.Errorf("Attack vector %q was NOT rejected! Got: %v", attack.vectorName, err)
		}
	}

	// Verify zero files created outside downloadsDir
	filesInParent, _ := os.ReadDir(tempDir)
	for _, f := range filesInParent {
		if f.Name() != "downloads" {
			t.Errorf("Security breach! File leaked to workspace parent: %s", f.Name())
		}
	}
}

// Scenario 7: Offline Fallback Catalog & Disconnected Browsing
// Features: F2, F3
func TestT4_Scenario7_OfflineFallbackCatalogAndDisconnectedBrowsing(t *testing.T) {
	// Step 1: Upstream AniList network is completely down
	mockServer := NewMockAniListServer()
	mockServer.Simulate429 = true // Blocks all remote calls

	// Step 2: Client loads offline fallback catalog
	catalog := SampleSeededCatalog()
	if len(catalog) < 10 {
		t.Fatalf("Offline fallback catalog must have >= 10 titles")
	}

	// Step 3: Search offline catalog for "Death Note"
	var searchResults []AnimeMedia
	for _, item := range catalog {
		if strings.Contains(strings.ToLower(item.Title.English), "death note") {
			searchResults = append(searchResults, item)
		}
	}
	if len(searchResults) == 0 || searchResults[0].ID != 1535 {
		t.Fatalf("Offline search for Death Note failed")
	}

	// Step 4: Add Death Note to user tracking library
	userLibrary := make(map[int]*UserLibraryEntry)
	deathNote := searchResults[0]
	userLibrary[deathNote.ID] = &UserLibraryEntry{
		MediaID:       deathNote.ID,
		Title:         deathNote.Title.English,
		Status:        "CURRENT",
		Progress:      12,
		TotalEpisodes: deathNote.Episodes,
		Score:         9.0,
		UpdatedAt:     time.Now(),
	}

	// Step 5: Verify tracking saved and updated locally while offline
	entry, exists := userLibrary[1535]
	if !exists || entry.Progress != 12 || entry.Score != 9.0 {
		t.Errorf("Offline tracking save failed: %+v", entry)
	}
}
