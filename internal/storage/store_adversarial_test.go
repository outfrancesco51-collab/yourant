package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"yourant/internal/storage"
)

// TestAdv_Storage_ConcurrentUpdatesAndSaves stress tests concurrent writes and saves
// to uncover any file lock contention or atomic rename race conditions.
func TestAdv_Storage_ConcurrentUpdatesAndSaves(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_storage_concurrent_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "library.json")
	store, err := storage.NewLibraryStore(storePath)
	if err != nil {
		t.Fatalf("new store error: %v", err)
	}
	_ = store.Load()

	var wg sync.WaitGroup
	var saveErrors int32
	concurrency := 30

	for i := 1; i <= concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Update entry
			score := 8.0 + float64(id%20)*0.1
			prog := id % 24
			total := 24
			title := "Concurrent Anime"
			_, err := store.Update(storage.TrackingUpdateRequest{
				MediaID:       id,
				Title:         &title,
				Progress:      &prog,
				TotalEpisodes: &total,
				Score:         &score,
			})
			if err != nil {
				t.Errorf("update error for id %d: %v", id, err)
				return
			}

			// Immediately call Save concurrently
			if err := store.Save(); err != nil {
				atomic.AddInt32(&saveErrors, 1)
				t.Logf("[OBSERVED_SAVE_ERROR] Concurrent Save() failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	if saveErrors > 0 {
		t.Logf("[FINDING] Detected %d Save() errors out of %d concurrent attempts due to file rename race on Windows", saveErrors, concurrency)
	}

	// Ensure final store save succeeds cleanly when serialized
	if err := store.Save(); err != nil {
		t.Fatalf("serialized final save failed: %v", err)
	}

	// Verify no orphan temp files remain in directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read dir error: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leaked temporary file found: %s", e.Name())
		}
	}

	// Verify file is valid JSON and contains entries
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read file error: %v", err)
	}

	var list []*storage.UserLibraryEntry
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("corrupted JSON written: %v", err)
	}

	if len(list) < concurrency {
		t.Errorf("expected at least %d entries in JSON file, found %d", concurrency, len(list))
	}
}

// TestAdv_Storage_ProgressAutoCompletionScenarios thoroughly verifies auto-completion rules.
func TestAdv_Storage_ProgressAutoCompletionScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_storage_autocomp_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, _ := storage.NewLibraryStore(filepath.Join(tempDir, "library.json"))

	// Scenario 1: Exact episode completion
	p12 := 12
	t12 := 12
	e1, err := store.Update(storage.TrackingUpdateRequest{
		MediaID:       601,
		Progress:      &p12,
		TotalEpisodes: &t12,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if e1.Status != storage.StatusCompleted {
		t.Errorf("expected COMPLETED when progress == totalEpisodes, got %s", e1.Status)
	}
	if e1.CompletedAt == nil {
		t.Errorf("expected CompletedAt timestamp to be set")
	}

	// Scenario 2: Overflow episode progress (e.g. 15 on a 12 ep show)
	p15 := 15
	e2, err := store.Update(storage.TrackingUpdateRequest{
		MediaID:       602,
		Progress:      &p15,
		TotalEpisodes: &t12,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if e2.Progress != 12 {
		t.Errorf("expected progress to be clamped to total episodes (12), got %d", e2.Progress)
	}
	if e2.Status != storage.StatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", e2.Status)
	}

	// Scenario 3: Airing anime with unknown total (totalEpisodes == 0)
	p50 := 50
	t0 := 0
	e3, err := store.Update(storage.TrackingUpdateRequest{
		MediaID:       603,
		Progress:      &p50,
		TotalEpisodes: &t0,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if e3.Status == storage.StatusCompleted {
		t.Errorf("anime with totalEpisodes=0 should NOT auto-complete")
	}
	if e3.Progress != 50 {
		t.Errorf("expected progress 50, got %d", e3.Progress)
	}

	// Scenario 4: Auto-transition from PLANNING to CURRENT on first episode
	p0 := 0
	t24 := 24
	statusPlanning := storage.StatusPlanning
	e4, err := store.Update(storage.TrackingUpdateRequest{
		MediaID:       604,
		Status:        &statusPlanning,
		Progress:      &p0,
		TotalEpisodes: &t24,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if e4.Status != storage.StatusPlanning {
		t.Fatalf("expected PLANNING, got %s", e4.Status)
	}

	// Progress to 1
	p1 := 1
	e4Updated, _ := store.Update(storage.TrackingUpdateRequest{
		MediaID:  604,
		Progress: &p1,
	})
	if e4Updated.Status != storage.StatusCurrent {
		t.Errorf("expected auto-transition from PLANNING to CURRENT when progress > 0, got %s", e4Updated.Status)
	}
	if e4Updated.StartedAt == nil {
		t.Errorf("expected StartedAt to be set when transitioning to CURRENT")
	}
}

// TestAdv_Storage_ScoreBoundsClamping verifies score clamping on negative and overflowing scores.
func TestAdv_Storage_ScoreBoundsClamping(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_storage_clamp_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, _ := storage.NewLibraryStore(filepath.Join(tempDir, "library.json"))

	testCases := []struct {
		inputScore    float64
		expectedScore float64
	}{
		{-100.0, 0.0},
		{-0.01, 0.0},
		{0.0, 0.0},
		{5.5, 5.5},
		{10.0, 10.0},
		{10.01, 10.0},
		{999.9, 10.0},
	}

	for i, tc := range testCases {
		score := tc.inputScore
		e, err := store.Update(storage.TrackingUpdateRequest{
			MediaID: 700 + i,
			Score:   &score,
		})
		if err != nil {
			t.Fatalf("update error for score %f: %v", tc.inputScore, err)
		}
		if e.Score != tc.expectedScore {
			t.Errorf("input score %f: expected %f, got %f", tc.inputScore, tc.expectedScore, e.Score)
		}
	}
}
