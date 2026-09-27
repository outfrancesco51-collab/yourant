package downloader_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yourant/internal/downloader"
)

// TestAdv_Downloader_NetworkInterruptionAndResume simulates network cutoff mid-stream
// and verifies that the partial file is preserved and can be resumed with HTTP 206.
func TestAdv_Downloader_NetworkInterruptionAndResume(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_dl_interrupt_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, err := downloader.NewStorageGuard(tempDir)
	if err != nil {
		t.Fatalf("guard error: %v", err)
	}

	payload := bytes.Repeat([]byte("CHALLENGER_NETWORK_INTERRUPTION_RESUME_TEST_DATA_"), 8192) // ~400 KB
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Accept-Ranges", "bytes")
		rangeHeader := r.Header.Get("Range")

		if count == 1 {
			// First request: simulate interruption after sending 50 KB
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload[:50*1024])
			// Abruptly terminate connection
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
			}
			return
		}

		// Second request: resume from byte range
		if strings.HasPrefix(rangeHeader, "bytes=") {
			rangeVal := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(rangeVal, "-")
			start, _ := strconv.Atoi(parts[0])
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)-start))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[start:])
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dl := downloader.NewDownloader(guard, nil, 1)
	defer dl.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       501,
		EpisodeNumber: 1,
		Title:         "Interrupted Anime",
		URL:           server.URL,
		FileName:      "interrupted.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	// Wait for task to fail due to connection interruption
	deadline := time.Now().Add(5 * time.Second)
	var failed bool
	for time.Now().Before(deadline) {
		snap, _ := dl.GetTask(task.ID)
		if snap.Status == downloader.StatusFailed {
			failed = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !failed {
		snap, _ := dl.GetTask(task.ID)
		t.Fatalf("expected task to fail on network cut, got status %s", snap.Status)
	}

	// Verify .part file exists and has ~50 KB
	fi, err := os.Stat(task.PartFilePath)
	if err != nil {
		t.Fatalf("expected .part file to be preserved after failure: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatalf("expected non-zero .part file size after cut")
	}

	// Resume the task
	if err := dl.Resume(task.ID); err != nil {
		t.Fatalf("resume error: %v", err)
	}

	// Wait for completion
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, _ := dl.GetTask(task.ID)
		if snap.Status == downloader.StatusCompleted {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusCompleted {
		t.Fatalf("expected COMPLETED after resume, got %s (err: %s)", snap.Status, snap.Error)
	}

	// Verify checksum of final file
	finalData, err := os.ReadFile(task.FilePath)
	if err != nil {
		t.Fatalf("failed to read completed file: %v", err)
	}

	if sha256.Sum256(finalData) != sha256.Sum256(payload) {
		t.Fatalf("data corrupted after interruption and resume")
	}
}

// TestAdv_Downloader_ServerIgnoresRangeRestartsCleanly verifies that when a server
// ignores the Range request and responds with HTTP 200 OK, the downloader truncates
// the .part file and restarts from byte 0 without appending duplicate bytes.
func TestAdv_Downloader_ServerIgnoresRangeRestartsCleanly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_dl_no_range_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, _ := downloader.NewStorageGuard(tempDir)
	payload := []byte("PRECISE_NON_CORRUPTED_STREAM_CONTENT_NO_DUPLICATE_BYTES")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately ignore Range header and return HTTP 200 OK
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	// Pre-create corrupt .part file with 50 bytes of garbage
	targetPart := filepath.Join(tempDir, "norange.mp4.part")
	_ = os.WriteFile(targetPart, bytes.Repeat([]byte("GARBAGE"), 10), 0644)

	dl := downloader.NewDownloader(guard, nil, 1)
	defer dl.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       502,
		EpisodeNumber: 1,
		Title:         "No Range Support",
		URL:           server.URL,
		FileName:      "norange.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, _ := dl.GetTask(task.ID)
		if snap.Status == downloader.StatusCompleted {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusCompleted {
		t.Fatalf("expected COMPLETED, got %s", snap.Status)
	}

	data, err := os.ReadFile(task.FilePath)
	if err != nil {
		t.Fatalf("read final file error: %v", err)
	}

	if len(data) != len(payload) || !bytes.Equal(data, payload) {
		t.Fatalf("expected exact payload without garbage, got len %d vs %d", len(data), len(payload))
	}
}

// TestAdv_Downloader_CancellationDuringActiveStream tests the Windows file lock race
// where Cancel is invoked while worker is actively streaming.
func TestAdv_Downloader_CancellationDuringActiveStream(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_dl_cancel_race_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, _ := downloader.NewStorageGuard(tempDir)

	// Slow streaming server
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10000000")
		w.WriteHeader(http.StatusOK)
		close(started)
		chunk := bytes.Repeat([]byte("SLOW"), 1024)
		for i := 0; i < 50; i++ {
			_, err := w.Write(chunk)
			if err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer server.Close()

	dl := downloader.NewDownloader(guard, nil, 1)
	defer dl.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       503,
		EpisodeNumber: 1,
		Title:         "Cancel Test",
		URL:           server.URL,
		FileName:      "cancel_race.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	<-started
	time.Sleep(50 * time.Millisecond) // Let it write a few KB

	// Cancel while actively streaming
	if err := dl.Cancel(task.ID); err != nil {
		t.Fatalf("cancel error: %v", err)
	}

	// Give worker time to exit and close handle
	time.Sleep(300 * time.Millisecond)

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusCancelled {
		t.Errorf("expected status CANCELLED, got %s", snap.Status)
	}

	// Check if .part file was cleaned up or leaked
	if _, err := os.Stat(task.PartFilePath); !os.IsNotExist(err) {
		t.Logf("[OBSERVED_BEHAVIOR] .part file was NOT deleted immediately by Cancel because file was open in worker: %s", task.PartFilePath)
	}
}

// TestAdv_Downloader_AtomicRenameCollision tests behavior when target file already exists
func TestAdv_Downloader_AtomicRenameCollision(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "adv_dl_rename_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, _ := downloader.NewStorageGuard(tempDir)
	payload := []byte("NEW_OVERWRITTEN_FILE_PAYLOAD")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	// Pre-create destination file
	destFile := filepath.Join(tempDir, "collision.mp4")
	_ = os.WriteFile(destFile, []byte("OLD_STALE_FILE_DATA"), 0644)

	dl := downloader.NewDownloader(guard, nil, 1)
	defer dl.Close()

	task, err := dl.Enqueue(downloader.DownloadRequest{
		AnimeID:       504,
		EpisodeNumber: 1,
		Title:         "Collision Test",
		URL:           server.URL,
		FileName:      "collision.mp4",
	})
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, _ := dl.GetTask(task.ID)
		if snap.Status == downloader.StatusCompleted {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	snap, _ := dl.GetTask(task.ID)
	if snap.Status != downloader.StatusCompleted {
		t.Fatalf("expected COMPLETED despite existing file, got %s (err: %s)", snap.Status, snap.Error)
	}

	data, _ := os.ReadFile(destFile)
	if !bytes.Equal(data, payload) {
		t.Fatalf("expected old file to be replaced with new payload, got: %s", string(data))
	}
}
