package storage_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"yourant/internal/storage"
)

func TestStorage_ConcurrencySafety(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_storage_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "tracking.json")
	store, err := storage.NewLibraryStore(storePath)
	if err != nil {
		t.Fatalf("new store error: %v", err)
	}

	var wg sync.WaitGroup
	// 50 concurrent writers and readers
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = store.Upsert(&storage.UserLibraryEntry{
				MediaID:       id,
				Title:         "Anime Title",
				Status:        storage.StatusCurrent,
				Progress:      id % 12,
				TotalEpisodes: 12,
				Score:         8.5,
			})
			_ = store.GetList("")
			_, _ = store.GetEntry(id)

			prog := 12
			_, _ = store.Update(storage.TrackingUpdateRequest{
				MediaID:  id,
				Progress: &prog,
			})
		}(i)
	}

	wg.Wait()

	list := store.GetList("")
	if len(list) != 50 {
		t.Fatalf("expected 50 entries, got %d", len(list))
	}
}

func TestStorage_AtomicPersistenceAndReload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_persist_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "library.json")
	store1, err := storage.NewLibraryStore(storePath)
	if err != nil {
		t.Fatalf("new store error: %v", err)
	}

	// 1. Initial Load should create pre-seeded demo entries
	if err := store1.Load(); err != nil {
		t.Fatalf("store1 load error: %v", err)
	}

	entries := store1.GetList("")
	if len(entries) < 2 {
		t.Fatalf("expected pre-seeded demo entries, got %d", len(entries))
	}

	// 2. Add new entry and save
	_, err = store1.Upsert(&storage.UserLibraryEntry{
		MediaID:       113415,
		Title:         "Jujutsu Kaisen",
		Status:        storage.StatusCurrent,
		Progress:      10,
		TotalEpisodes: 24,
		Score:         8.8,
	})
	if err != nil {
		t.Fatalf("upsert error: %v", err)
	}

	if err := store1.Save(); err != nil {
		t.Fatalf("store1 save error: %v", err)
	}

	// 3. New store instance loading the same file
	store2, err := storage.NewLibraryStore(storePath)
	if err != nil {
		t.Fatalf("store2 new error: %v", err)
	}

	if err := store2.Load(); err != nil {
		t.Fatalf("store2 load error: %v", err)
	}

	jjk, ok := store2.GetEntry(113415)
	if !ok {
		t.Fatalf("expected Jujutsu Kaisen to exist in reloaded store")
	}
	if jjk.Progress != 10 || jjk.Score != 8.8 {
		t.Fatalf("reloaded data mismatch: progress=%d, score=%f", jjk.Progress, jjk.Score)
	}
}

func TestStorage_ProgressAutoCompletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_autocomp_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, _ := storage.NewLibraryStore(filepath.Join(tempDir, "library.json"))

	entry, err := store.Upsert(&storage.UserLibraryEntry{
		MediaID:       999,
		Title:         "Short OVA",
		Status:        storage.StatusCurrent,
		Progress:      1,
		TotalEpisodes: 2,
		Score:         7.0,
	})
	if err != nil {
		t.Fatalf("upsert error: %v", err)
	}
	if entry.Status != storage.StatusCurrent {
		t.Fatalf("expected CURRENT status, got %s", entry.Status)
	}

	// Update to progress = 2 (TotalEpisodes)
	prog := 2
	updated, err := store.Update(storage.TrackingUpdateRequest{
		MediaID:  999,
		Progress: &prog,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}

	if updated.Status != storage.StatusCompleted {
		t.Fatalf("expected auto-transition to COMPLETED, got %s", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set upon auto-completion")
	}
}

func TestStorage_ScoreAndProgressClamping(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_clamp_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, _ := storage.NewLibraryStore(filepath.Join(tempDir, "library.json"))

	// Test score > 10.0 clamped to 10.0, score < 0 clamped to 0
	e, _ := store.Upsert(&storage.UserLibraryEntry{
		MediaID:       100,
		Title:         "Clamp Test",
		Status:        storage.StatusPlanning,
		Score:         15.5,
		Progress:      -5,
		TotalEpisodes: 12,
	})

	if e.Score != 10.0 {
		t.Errorf("expected score 10.0, got %f", e.Score)
	}
	if e.Progress != 0 {
		t.Errorf("expected progress 0, got %d", e.Progress)
	}
}
