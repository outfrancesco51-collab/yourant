package downloader_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"yourant/internal/downloader"
)

func setupTestServer(t *testing.T, payload []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		rangeHeader := r.Header.Get("Range")

		if rangeHeader == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
			return
		}

		if !strings.HasPrefix(rangeHeader, "bytes=") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rangeVal := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.Split(rangeVal, "-")
		start, err := strconv.Atoi(parts[0])
		if err != nil || start >= len(payload) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		end := len(payload) - 1
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)-start))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[start:])
	}))
}

func TestDownloader_CompleteLifecycleAndAtomicRename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_dl_lifecycle_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, err := downloader.NewStorageGuard(tempDir)
	if err != nil {
		t.Fatalf("guard creation error: %v", err)
	}
	dl := downloader.NewDownloader(guard, nil, 2)
	defer dl.Close()

	payload := bytes.Repeat([]byte("YOURANT_STREAM_MEDIA_CHUNK_TEST_DATA_"), 16384)
	server := setupTestServer(t, payload)
	defer server.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       101,
		EpisodeNumber: 1,
		Title:         "Test Episode 1",
		URL:           server.URL,
		FileName:      "test_ep_01.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, ok := dl.GetTask(task.ID)
		if ok && snap.Status == downloader.StatusCompleted {
			break
		}
		if ok && snap.Status == downloader.StatusFailed {
			t.Fatalf("task failed unexpectedly: %s", snap.Error)
		}
		time.Sleep(50 * time.Millisecond)
	}

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusCompleted {
		t.Fatalf("expected task status COMPLETED, got %s", snap.Status)
	}

	// Verify .part is gone
	if _, err := os.Stat(task.PartFilePath); !os.IsNotExist(err) {
		t.Errorf("expected .part file to be removed after atomic rename, but it exists: %s", task.PartFilePath)
	}

	// Verify final file exists and matches sha256 checksum
	downloadedData, err := os.ReadFile(task.FilePath)
	if err != nil {
		t.Fatalf("failed to read completed file: %v", err)
	}

	expectedSum := sha256.Sum256(payload)
	actualSum := sha256.Sum256(downloadedData)
	if expectedSum != actualSum {
		t.Errorf("checksum mismatch! Expected %x, got %x", expectedSum, actualSum)
	}
}

func TestDownloader_PauseAndResumeWithRange(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_dl_pause_resume_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, _ := downloader.NewStorageGuard(tempDir)
	payload := bytes.Repeat([]byte("A"), 1024*1024)
	server := setupTestServer(t, payload)
	defer server.Close()

	dl := downloader.NewDownloader(guard, nil, 1)
	defer dl.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       202,
		EpisodeNumber: 2,
		Title:         "Resumable Anime",
		URL:           server.URL,
		FileName:      "resume_ep_02.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	_ = dl.Pause(task.ID)

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusPaused && snap.Status != downloader.StatusCompleted {
		t.Fatalf("expected task status PAUSED, got: %s", snap.Status)
	}

	if snap.Status == downloader.StatusPaused {
		err = dl.Resume(task.ID)
		if err != nil {
			t.Fatalf("resume failed: %v", err)
		}

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			s, _ := dl.GetTask(task.ID)
			if s.Status == downloader.StatusCompleted {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		data, err := os.ReadFile(task.FilePath)
		if err != nil {
			t.Fatalf("failed to read resumed file: %v", err)
		}
		if len(data) != len(payload) {
			t.Errorf("file size mismatch: expected %d, got %d", len(payload), len(data))
		}
	}
}

func TestDownloader_CancellationCleansUpPartFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_dl_cancel_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, _ := downloader.NewStorageGuard(tempDir)
	payload := bytes.Repeat([]byte("Z"), 5*1024*1024)
	server := setupTestServer(t, payload)
	defer server.Close()

	dl := downloader.NewDownloader(guard, nil, 1)
	defer dl.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       303,
		EpisodeNumber: 3,
		Title:         "Cancel Episode",
		URL:           server.URL,
		FileName:      "cancel_ep_03.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
	_ = dl.Cancel(task.ID)

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusCancelled {
		t.Errorf("expected CANCELLED status, got: %s", snap.Status)
	}

	for i := 0; i < 20; i++ {
		if _, err := os.Stat(task.PartFilePath); os.IsNotExist(err) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := os.Stat(task.PartFilePath); !os.IsNotExist(err) {
		t.Errorf("expected .part file to be removed after cancellation: %s", task.PartFilePath)
	}
}
