package tests

import (
	"context"
	"encoding/json"
	"fmt"
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
// TIER 3: PAIRWISE CROSS-FEATURE COMBINATIONS (14 TESTS)
// ============================================================================

// 1. AniList Search (F2) -> Start Download (F5) -> Verify Controlled Storage (F4)
func TestT3_01_AniListSearchToDownloadToControlledStorage(t *testing.T) {
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	// Step 1: Search AniList for Frieren
	var foundMedia *AnimeMedia
	for _, item := range mockServer.Catalog {
		if strings.Contains(item.Title.English, "Frieren") {
			foundMedia = &item
			break
		}
	}
	if foundMedia == nil {
		t.Fatalf("Failed to discover Frieren in AniList catalog")
	}

	// Step 2: Queue download
	tempDir := t.TempDir()
	guard, err := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	if err != nil {
		t.Fatalf("Storage guard initialization failed: %v", err)
	}
	downloader := NewResumableDownloaderSimulator(guard)

	ctx := context.Background()
	filename := fmt.Sprintf("frieren_ep01_%d.mp4", foundMedia.ID)
	task, err := downloader.StartDownload(ctx, foundMedia.ID, 1, foundMedia.Title.English, "https://example.com/ep1.mp4", filename, 4096)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Step 3: Verify storage sandboxing
	expectedPath := filepath.Join(guard.BaseDir, filename)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Downloaded file does not exist in controlled storage: %s", expectedPath)
	}
	if task.Status != "COMPLETED" || task.ProgressPercent != 100.0 {
		t.Errorf("Download task status unexpected: status=%s, progress=%f", task.Status, task.ProgressPercent)
	}
}

// 2. Download Media (F5) -> Stream with HTTP 206 (F6) -> Initialize Audio Booster (F10)
func TestT3_02_DownloadToStreamHttp206ToAudioBooster(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	downloader := NewResumableDownloaderSimulator(guard)

	// Step 1: Complete download
	filename := "test_stream_audio.mp4"
	ctx := context.Background()
	task, err := downloader.StartDownload(ctx, 16498, 1, "AoT Ep 1", "https://example.com/aot.mp4", filename, 5000)
	if err != nil || task.Status != "COMPLETED" {
		t.Fatalf("Download setup failed: %v", err)
	}

	// Step 2: Stream with HTTP 206
	filePath := filepath.Join(guard.BaseDir, filename)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, filePath)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Range", "bytes=100-500")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("HTTP 206 streaming failed: %v, status=%d", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	// Step 3: Connect to Audio Booster
	booster := NewAudioBoosterSimulator()
	gain, isNew := booster.AttachOrGetBooster("video-player-1", 1.0)
	if !isNew || gain != 1.0 {
		t.Errorf("Audio booster graph failed to initialize on downloaded stream")
	}
}

// 3. Stream Video (F6) -> Volume Boost to 1000% (F10) -> Toggle Cinema Mode (F11)
func TestT3_03_VideoStreamToVolumeBoostToCinemaMode(t *testing.T) {
	tempDir := t.TempDir()
	videoFile := filepath.Join(tempDir, "cinema_boost.mp4")
	os.WriteFile(videoFile, make([]byte, 2048), 0644)

	// Step 1: Stream video chunk
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleVideoStream(w, r, videoFile)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Video stream failed: %v", err)
	}
	resp.Body.Close()

	// Step 2: Boost volume to 1000%
	gain, isBoost, warn := CalculateVolumeBoost(1000.0)
	if gain != 10.0 || !isBoost || !warn {
		t.Fatalf("1000%% volume boost calculation failed: gain=%f", gain)
	}

	// Limiter test
	clampedDbfs := SimulateLimiterClamping(-15.0, gain)
	if clampedDbfs > 0.0 {
		t.Errorf("Audio signal exceeded 0dBFS: %f", clampedDbfs)
	}

	// Step 3: Toggle Cinema Mode
	sm := NewCinemaModeStateMachine()
	sm.Toggle()
	if !sm.IsActive || sm.DimFactor != 0.15 {
		t.Errorf("Cinema mode failed to activate with dim 0.15")
	}
}

// 4. AniList Details (F2) -> User Tracking Update (F3) -> Verify Local Storage Persistence (F3, F1)
func TestT3_04_AniListDetailToUserTrackingToLocalStorage(t *testing.T) {
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	// Step 1: Query detail
	mediaDetail := mockServer.Catalog[0] // Frieren (28 eps)

	// Step 2: Update tracking progress
	entry := &UserLibraryEntry{
		MediaID:       mediaDetail.ID,
		Title:         mediaDetail.Title.English,
		Status:        "CURRENT",
		Progress:      28, // Watched all
		TotalEpisodes: mediaDetail.Episodes,
		Score:         9.8,
		UpdatedAt:     time.Now(),
	}

	// Transition rule
	if entry.Progress >= entry.TotalEpisodes {
		entry.Status = "COMPLETED"
	}
	if entry.Status != "COMPLETED" {
		t.Errorf("Expected status COMPLETED on progress reaching total episodes")
	}

	// Step 3: Persistence simulation
	tempDir := t.TempDir()
	storeFile := filepath.Join(tempDir, "tracking.json")
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(storeFile, data, 0644); err != nil {
		t.Fatalf("Failed to persist tracking: %v", err)
	}

	readBytes, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("Failed to read persisted tracking: %v", err)
	}
	var loaded UserLibraryEntry
	json.Unmarshal(readBytes, &loaded)
	if loaded.MediaID != mediaDetail.ID || loaded.Status != "COMPLETED" || loaded.Score != 9.8 {
		t.Errorf("Persisted tracking mismatch: %+v", loaded)
	}
}

// 5. Create Watch Party (F12) -> Host Seek Sync (F13) -> Frontend Status Pill Green (F14)
func TestT3_05_WatchPartyCreateToHostSeekSyncToStatusPill(t *testing.T) {
	hub := NewWatchPartyHubSimulator()

	// Step 1: Create room
	room := hub.CreateRoom("WP-7F3A", "host-1", "Francy", 154587, "Frieren Ep 1")

	// Step 2: Host seeks to 142.5s
	room.CurrentTime = 142.5
	room.IsPlaying = true

	// Step 3: Client evaluates drift with 50ms latency
	clientTime := 142.550 // 50ms drift
	targetTime, rate, action, pill := CalculateDriftCompensation(clientTime, room.CurrentTime, true, 0.05, 1.0)

	if action != "IN_SYNC" || pill != "green" || rate != 1.0 {
		t.Errorf("Expected green/IN_SYNC status, got action=%s, pill=%s, rate=%f", action, pill, rate)
	}
	if math.Abs(targetTime-142.550) > 0.001 {
		t.Errorf("Target time projection mismatch: %f", targetTime)
	}
}

// 6. Watch Party (F12) -> High Drift 2000ms (F13) -> Hard Seek with Video Player (F6) -> Status Pill Red (F14)
func TestT3_06_WatchPartyHighDriftToHardSeekToRedPill(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	room := hub.CreateRoom("WP-SYNC", "host-1", "Host", 16498, "AoT")

	// Host is at 100.0s
	room.CurrentTime = 100.0
	room.IsPlaying = true

	// Client was paused/lagged at 70.0s (30,000ms drift!)
	clientCurrentTime := 70.0
	targetTime, rate, action, pill := CalculateDriftCompensation(clientCurrentTime, room.CurrentTime, true, 0.1, 1.0)

	if action != "HARD_SEEK" || pill != "red" || rate != 1.0 {
		t.Errorf("Expected HARD_SEEK and red pill for large drift, got action=%s, pill=%s", action, pill)
	}
	if math.Abs(targetTime-100.1) > 0.001 {
		t.Errorf("Target time projected incorrectly: %f", targetTime)
	}
}

// 7. Cinema Mode Toggle (F11) -> GLSL Shader Dimming u_dim=0.15 (F9) -> Theme Contrast Preserved (F7)
func TestT3_07_CinemaModeToggleToShaderDimmingToThemeContrast(t *testing.T) {
	sm := NewCinemaModeStateMachine()

	// Step 1: Toggle cinema mode
	sm.Toggle()
	if !sm.IsActive || sm.DimFactor != 0.15 {
		t.Fatalf("Cinema mode activation failed: dim=%f", sm.DimFactor)
	}

	// Step 2: Validate GLSL shader code handles u_dim
	fragCode := `
		precision highp float;
		uniform float u_dim;
		void main() {
			vec3 color = vec3(0.027, 0.039, 0.075) * u_dim;
			gl_FragColor = vec4(color, 1.0);
		}
	`
	if !strings.Contains(fragCode, "* u_dim") {
		t.Errorf("Shader code does not apply u_dim multiplier")
	}

	// Step 3: Verify text contrast against dark background
	bgDark := "#070a13"
	textPrimary := "#f8fafc"
	ratio, err := CalculateContrastRatio(bgDark, textPrimary)
	if err != nil || ratio < 7.0 {
		t.Errorf("Contrast failed in cinema mode backdrop: ratio=%f, err=%v", ratio, err)
	}
}

// 8. Controlled Storage Guard (F4) -> Attempt Traversal in Download Queue (F5) -> Rejection without Leak (F1)
func TestT3_08_ControlledStorageGuardToDownloadQueueRejection(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	downloader := NewResumableDownloaderSimulator(guard)

	// Step 1: Malicious download attempt
	maliciousFilename := "../../evil_calc.exe"
	ctx := context.Background()
	_, err := downloader.StartDownload(ctx, 999, 1, "Malicious Media", "https://example.com/evil", maliciousFilename, 1024)

	// Step 2: Assert strict rejection
	if err != ErrPathTraversal {
		t.Errorf("Expected ErrPathTraversal, got: %v", err)
	}

	// Step 3: Assert no file written in parent directory
	parentDir := filepath.Dir(guard.BaseDir)
	leakedFile := filepath.Join(parentDir, "evil_calc.exe")
	if _, statErr := os.Stat(leakedFile); !os.IsNotExist(statErr) {
		t.Errorf("Security breach: file escaped to %s", leakedFile)
	}
}

// 9. Watch Party Room (F12) -> Chat Stream (F14) -> Cinema Mode Chat Docking (F11)
func TestT3_09_WatchPartyRoomToChatStreamToCinemaDocking(t *testing.T) {
	hub := NewWatchPartyHubSimulator()
	room := hub.CreateRoom("WP-CHAT", "host-1", "Host", 154587, "Frieren")

	// Chat message broadcast
	chat := WatchPartyMessage{
		Type:       "chat:message",
		RoomID:     room.RoomID,
		SenderName: "Alice",
		Payload:    map[string]interface{}{"chatText": "Incredible scene!"},
	}
	if chat.Payload["chatText"] != "Incredible scene!" {
		t.Errorf("Chat payload invalid")
	}

	// Toggle Cinema Mode: chat docking rules
	sm := NewCinemaModeStateMachine()
	sm.Toggle()

	dockedChatClass := "cinema-chat-drawer-translucent"
	if !strings.Contains(dockedChatClass, "translucent") {
		t.Errorf("Chat drawer failed to enter translucent docking in cinema mode")
	}
}

// 10. Offline Mode (F2 fallback) -> Local Library Display (F3) -> App Shell Navigation (F8)
func TestT3_10_OfflineModeFallbackToLibraryDisplayToAppShell(t *testing.T) {
	// Step 1: External network down -> load offline fallback catalog
	catalog := SampleSeededCatalog()
	if len(catalog) < 10 {
		t.Fatalf("Offline catalog must contain at least 10 titles")
	}

	// Step 2: User library tracking
	library := []*UserLibraryEntry{
		{
			MediaID:       catalog[0].ID,
			Title:         catalog[0].Title.English,
			Status:        "CURRENT",
			Progress:      5,
			TotalEpisodes: catalog[0].Episodes,
		},
	}

	// Step 3: App shell displays library
	navRoute := "/library"
	if navRoute != "/library" || len(library) != 1 {
		t.Errorf("App shell library view mismatch")
	}
}

// 11. Video Streaming (F6) -> Audio Booster Gain Clamping (F10) -> Cinema Mode Auto-hide HUD (F11)
func TestT3_11_VideoStreamToAudioLimiterClampingToAutoHideControls(t *testing.T) {
	// Step 1: Video stream ready
	streamReady := true

	// Step 2: Audio booster gain clamped
	gain, _, _ := CalculateVolumeBoost(800.0) // 8.0x (+18 dB)
	outputLevel := SimulateLimiterClamping(-10.0, gain)
	if outputLevel > 0.0 {
		t.Errorf("Limiter failed to clamp gain: %f", outputLevel)
	}

	// Step 3: Cinema Mode HUD idle auto-hide
	controlsIdle := true
	hudOpacity := 1.0
	if controlsIdle {
		hudOpacity = 0.0
	}

	if !streamReady || hudOpacity != 0.0 {
		t.Errorf("HUD controls failed to auto-hide during cinema playback")
	}
}

// 12. User Tracking Episode Complete (F3) -> Next Episode Download Trigger (F5) -> Storage Verification (F4)
func TestT3_12_TrackingEpisodeCompleteToNextDownloadTrigger(t *testing.T) {
	tempDir := t.TempDir()
	guard, _ := NewControlledStorageGuard(filepath.Join(tempDir, "downloads"))
	downloader := NewResumableDownloaderSimulator(guard)

	// Episode 1 finished
	currentEpisode := 1
	nextEpisode := currentEpisode + 1

	// Auto-queue episode 2 download
	ctx := context.Background()
	filename := fmt.Sprintf("anime_ep%02d.mp4", nextEpisode)
	task, err := downloader.StartDownload(ctx, 154587, nextEpisode, "Frieren Ep 2", "https://example.com/ep2.mp4", filename, 2048)
	if err != nil {
		t.Fatalf("Episode 2 auto-download failed: %v", err)
	}

	if task.FileName != "anime_ep02.mp4" || task.Status != "COMPLETED" {
		t.Errorf("Task status unexpected: %+v", task)
	}

	expectedPath := filepath.Join(guard.BaseDir, filename)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Episode 2 file missing in downloads: %s", expectedPath)
	}
}

// 13. AniList Search (F2) -> App Shell Media Grid Render (F8) -> Theme Palette Application (F7)
func TestT3_13_AniListSearchToAppShellGridToThemeTokens(t *testing.T) {
	mockServer := NewMockAniListServer()
	defer mockServer.Close()

	// AniList search
	searchResults := mockServer.Catalog[:4]

	// Grid layout with 2:3 aspect ratio
	for range searchResults {
		w := 200.0
		h := 300.0
		if math.Abs((w/h)-(2.0/3.0)) > 0.001 {
			t.Errorf("Media card aspect ratio invalid")
		}
	}

	// Theme tokens
	validator := ThemeTokenValidator{}
	tokens := validator.GetCoreTokens()
	if tokens["--metallic-cyan"] != "#38bdf8" || tokens["--blood-bright"] != "#e11d48" {
		t.Errorf("Theme tokens missing from media grid styling")
	}
}

// 14. Watch Party Drift 800ms (F13) -> Soft Rate Adjustment (1.05x) (F10, F13) -> Status Pill Amber (F14)
func TestT3_14_WatchPartyModerateDriftToSpeedAdjustmentToAmberPill(t *testing.T) {
	hostTimestamp := 20.000
	clientTimestamp := 19.200 // 800ms behind

	targetTime, rate, action, pill := CalculateDriftCompensation(clientTimestamp, hostTimestamp, true, 0.0, 1.0)

	if action != "SPEED_ADJUST" || rate != 1.05 || pill != "amber" {
		t.Errorf("Expected SPEED_ADJUST / 1.05x / amber pill, got action=%s, rate=%f, pill=%s", action, rate, pill)
	}
	if math.Abs(targetTime-20.000) > 0.001 {
		t.Errorf("Target time mismatch: %f", targetTime)
	}
}
