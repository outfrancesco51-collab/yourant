package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type DownloadStatus string

const (
	StatusQueued      DownloadStatus = "QUEUED"
	StatusDownloading DownloadStatus = "DOWNLOADING"
	StatusPaused      DownloadStatus = "PAUSED"
	StatusCompleted   DownloadStatus = "COMPLETED"
	StatusFailed      DownloadStatus = "FAILED"
	StatusCancelled   DownloadStatus = "CANCELLED"
)

// DownloadTask represents an individual media download operation.
type DownloadTask struct {
	ID              string         `json:"id"`
	AnimeID         int            `json:"animeId"`
	EpisodeNumber   int            `json:"episodeNumber"`
	Title           string         `json:"title"`
	URL             string         `json:"url"`
	FileName        string         `json:"fileName"`
	FilePath        string         `json:"filePath"`
	PartFilePath    string         `json:"partFilePath"`
	TotalBytes      int64          `json:"totalBytes"`
	DownloadedBytes int64          `json:"downloadedBytes"`
	SpeedBps        int64          `json:"speedBps"`
	ProgressPercent float64        `json:"progressPercent"`
	ETASeconds      int64          `json:"etaSeconds"`
	Status          DownloadStatus `json:"status"`
	Error           string         `json:"error,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	CompletedAt     *time.Time     `json:"completedAt,omitempty"`

	mu         sync.RWMutex       `json:"-"`
	cancelFunc context.CancelFunc `json:"-"`
}

// DownloadRecord is an alias for DownloadTask to satisfy PROJECT.md REST contracts.
type DownloadRecord = DownloadTask

// DownloadRequest represents user submission for a new download.
type DownloadRequest struct {
	AnimeID       int    `json:"animeId"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	FileName      string `json:"fileName"`
}

// FileInfo represents a file on disk in the downloads directory.
type FileInfo struct {
	FileName string    `json:"fileName"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
}

// Downloader coordinates background downloads within the sandboxed storage engine.
type Downloader struct {
	guard         *StorageGuard
	httpClient    *http.Client
	maxConcurrent int

	mu     sync.RWMutex
	tasks  map[string]*DownloadTask
	queue  chan *DownloadTask
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a Downloader with a StorageGuard wrapping baseDir.
func NewManager(baseDir string) (*Downloader, error) {
	guard, err := NewStorageGuard(baseDir)
	if err != nil {
		return nil, err
	}
	return NewDownloader(guard, nil, 3), nil
}

// NewDownloader creates and starts a background streaming downloader.
func NewDownloader(guard *StorageGuard, client *http.Client, maxConcurrent int) *Downloader {
	if client == nil {
		client = &http.Client{Timeout: 0} // Streaming needs no timeout
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &Downloader{
		guard:         guard,
		httpClient:    client,
		maxConcurrent: maxConcurrent,
		tasks:         make(map[string]*DownloadTask),
		queue:         make(chan *DownloadTask, 100),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start worker pool
	for i := 0; i < maxConcurrent; i++ {
		d.wg.Add(1)
		go d.worker()
	}

	return d
}

// Guard returns the internal StorageGuard instance.
func (d *Downloader) Guard() *StorageGuard {
	return d.guard
}

// ResolveSafePath delegates path resolution to the StorageGuard.
func (d *Downloader) ResolveSafePath(fileName string) (string, error) {
	return d.guard.ResolveSafePath(fileName)
}

// Enqueue registers a new download task and enqueues it for streaming.
func (d *Downloader) Enqueue(req DownloadRequest) (*DownloadTask, error) {
	if req.URL == "" {
		return nil, errors.New("url cannot be empty")
	}

	rawName := req.FileName
	if rawName == "" {
		rawName = fmt.Sprintf("anime_%d_ep_%d.mp4", req.AnimeID, req.EpisodeNumber)
	}

	finalPath, err := d.guard.ResolveSafePath(rawName)
	if err != nil {
		return nil, fmt.Errorf("invalid download destination: %w", err)
	}

	fileName := filepath.Base(finalPath)
	partPath := finalPath + ".part"

	taskID := fmt.Sprintf("dl_%d_%d_%d", req.AnimeID, req.EpisodeNumber, time.Now().UnixNano())

	task := &DownloadTask{
		ID:            taskID,
		AnimeID:       req.AnimeID,
		EpisodeNumber: req.EpisodeNumber,
		Title:         req.Title,
		URL:           req.URL,
		FileName:      fileName,
		FilePath:      finalPath,
		PartFilePath:  partPath,
		Status:        StatusQueued,
		CreatedAt:     time.Now(),
	}

	if fi, err := os.Stat(partPath); err == nil {
		task.DownloadedBytes = fi.Size()
	}

	d.mu.Lock()
	d.tasks[taskID] = task
	d.mu.Unlock()

	select {
	case d.queue <- task:
	default:
	}

	return task, nil
}

// StartDownload is an alias to Enqueue for interface compatibility.
func (d *Downloader) StartDownload(ctx context.Context, req DownloadRequest) (*DownloadTask, error) {
	return d.Enqueue(req)
}

// Pause pauses an active or queued download.
func (d *Downloader) Pause(taskID string) error {
	d.mu.RLock()
	task, exists := d.tasks[taskID]
	d.mu.RUnlock()

	if !exists {
		return errors.New("task not found")
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status != StatusDownloading && task.Status != StatusQueued {
		return fmt.Errorf("task cannot be paused from status: %s", task.Status)
	}

	task.Status = StatusPaused
	if task.cancelFunc != nil {
		task.cancelFunc()
		task.cancelFunc = nil
	}

	return nil
}

// Resume re-enqueues a paused or failed task.
func (d *Downloader) Resume(taskID string) error {
	d.mu.RLock()
	task, exists := d.tasks[taskID]
	d.mu.RUnlock()

	if !exists {
		return errors.New("task not found")
	}

	task.mu.Lock()
	if task.Status != StatusPaused && task.Status != StatusFailed {
		task.mu.Unlock()
		return fmt.Errorf("task cannot be resumed from status: %s", task.Status)
	}
	task.Status = StatusQueued
	task.Error = ""
	task.mu.Unlock()

	d.queue <- task
	return nil
}

// Cancel terminates a download and deletes partial .part files.
func (d *Downloader) Cancel(taskID string) error {
	d.mu.RLock()
	task, exists := d.tasks[taskID]
	d.mu.RUnlock()

	if !exists {
		return errors.New("task not found")
	}

	task.mu.Lock()
	task.Status = StatusCancelled
	if task.cancelFunc != nil {
		task.cancelFunc()
		task.cancelFunc = nil
	}
	task.mu.Unlock()

	_ = os.Remove(task.PartFilePath)
	return nil
}

// CancelDownload is an alias for Cancel.
func (d *Downloader) CancelDownload(id string) error {
	return d.Cancel(id)
}

// GetTask returns a snapshot of task status.
func (d *Downloader) GetTask(taskID string) (*DownloadTask, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	task, exists := d.tasks[taskID]
	if !exists {
		return nil, false
	}
	task.mu.RLock()
	cp := *task
	task.mu.RUnlock()
	return &cp, true
}

// GetDownload is an alias for GetTask.
func (d *Downloader) GetDownload(id string) (*DownloadTask, bool) {
	return d.GetTask(id)
}

// ListTasks returns all tracked downloads.
func (d *Downloader) ListTasks() []*DownloadTask {
	d.mu.RLock()
	defer d.mu.RUnlock()
	res := make([]*DownloadTask, 0, len(d.tasks))
	for _, t := range d.tasks {
		t.mu.RLock()
		cp := *t
		t.mu.RUnlock()
		res = append(res, &cp)
	}
	return res
}

// GetDownloads is an alias for ListTasks.
func (d *Downloader) GetDownloads() []*DownloadTask {
	return d.ListTasks()
}

// ListFiles lists all completed media files in the downloads directory.
func (d *Downloader) ListFiles() ([]FileInfo, error) {
	entries, err := os.ReadDir(d.guard.BaseDir())
	if err != nil {
		return nil, err
	}

	var res []FileInfo
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, ".part") || strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		res = append(res, FileInfo{
			FileName: name,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		})
	}
	return res, nil
}

// DeleteFile deletes a completed media file from the downloads directory.
func (d *Downloader) DeleteFile(fileName string) error {
	safePath, err := d.guard.ResolveSafePath(fileName)
	if err != nil {
		return err
	}
	return os.Remove(safePath)
}

// Close shuts down the worker pool gracefully.
func (d *Downloader) Close() error {
	d.cancel()
	d.wg.Wait()
	return nil
}

func (d *Downloader) worker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case task, ok := <-d.queue:
			if !ok {
				return
			}
			d.executeDownload(task)
		}
	}
}

func (d *Downloader) executeDownload(task *DownloadTask) {
	task.mu.Lock()
	if task.Status != StatusQueued {
		task.mu.Unlock()
		return
	}
	task.Status = StatusDownloading

	taskCtx, taskCancel := context.WithCancel(d.ctx)
	task.cancelFunc = taskCancel
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.cancelFunc = nil
		task.mu.Unlock()
	}()

	var existingBytes int64 = 0
	if fi, err := os.Stat(task.PartFilePath); err == nil {
		existingBytes = fi.Size()
	}

	req, err := http.NewRequestWithContext(taskCtx, "GET", task.URL, nil)
	if err != nil {
		d.failTask(task, fmt.Errorf("failed to create request: %w", err))
		return
	}

	if existingBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingBytes))
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		if errors.Is(taskCtx.Err(), context.Canceled) {
			return
		}
		d.failTask(task, fmt.Errorf("http request failed: %w", err))
		return
	}
	defer resp.Body.Close()

	var openFlag int
	var startOffset int64

	switch resp.StatusCode {
	case http.StatusPartialContent: // HTTP 206
		openFlag = os.O_CREATE | os.O_APPEND | os.O_WRONLY
		startOffset = existingBytes
		total := parseContentRangeTotal(resp.Header.Get("Content-Range"))
		if total > 0 {
			task.mu.Lock()
			task.TotalBytes = total
			task.mu.Unlock()
		} else if resp.ContentLength > 0 {
			task.mu.Lock()
			task.TotalBytes = existingBytes + resp.ContentLength
			task.mu.Unlock()
		}

	case http.StatusOK: // HTTP 200 (Range ignored or fresh download)
		openFlag = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
		startOffset = 0
		task.mu.Lock()
		task.TotalBytes = resp.ContentLength
		task.mu.Unlock()

	case http.StatusRequestedRangeNotSatisfiable: // HTTP 416
		if existingBytes > 0 {
			task.mu.Lock()
			if task.TotalBytes > 0 && existingBytes >= task.TotalBytes {
				task.DownloadedBytes = existingBytes
				task.mu.Unlock()
				d.completeTask(task)
				return
			}
			task.mu.Unlock()
		}
		d.failTask(task, errors.New("range not satisfiable"))
		return

	default:
		d.failTask(task, fmt.Errorf("unexpected http status: %d %s", resp.StatusCode, resp.Status))
		return
	}

	partFile, err := os.OpenFile(task.PartFilePath, openFlag, 0644)
	if err != nil {
		d.failTask(task, fmt.Errorf("failed to open part file: %w", err))
		return
	}

	pw := &progressStreamer{
		task:            task,
		writer:          partFile,
		downloadedBytes: startOffset,
		lastUpdate:      time.Now(),
		lastBytes:       startOffset,
	}

	buf := make([]byte, 64*1024)
	_, copyErr := io.CopyBuffer(pw, resp.Body, buf)

	_ = partFile.Sync()
	_ = partFile.Close()

	if copyErr != nil {
		if errors.Is(taskCtx.Err(), context.Canceled) {
			_ = os.Remove(task.PartFilePath)
			return
		}
		d.failTask(task, fmt.Errorf("streaming error: %w", copyErr))
		return
	}

	d.completeTask(task)
}

func (d *Downloader) completeTask(task *DownloadTask) {
	task.mu.Lock()
	defer task.mu.Unlock()

	// Windows: remove target file if it already exists before renaming
	if _, err := os.Stat(task.FilePath); err == nil {
		_ = os.Remove(task.FilePath)
	}

	if err := os.Rename(task.PartFilePath, task.FilePath); err != nil {
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("atomic rename failed: %v", err)
		return
	}

	now := time.Now()
	task.Status = StatusCompleted
	task.CompletedAt = &now
	task.ProgressPercent = 100.0
	task.SpeedBps = 0
	task.ETASeconds = 0
	if task.TotalBytes > 0 {
		task.DownloadedBytes = task.TotalBytes
	}
}

func (d *Downloader) failTask(task *DownloadTask, err error) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.Status = StatusFailed
	task.Error = err.Error()
	task.SpeedBps = 0
}

func parseContentRangeTotal(header string) int64 {
	// Format: bytes 100-199/500
	parts := strings.Split(header, "/")
	if len(parts) == 2 {
		if total, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			return total
		}
	}
	return 0
}

type progressStreamer struct {
	task            *DownloadTask
	writer          io.Writer
	downloadedBytes int64
	lastUpdate      time.Time
	lastBytes       int64
}

func (ps *progressStreamer) Write(p []byte) (int, error) {
	n, err := ps.writer.Write(p)
	if n > 0 {
		ps.downloadedBytes += int64(n)
		atomic.StoreInt64(&ps.task.DownloadedBytes, ps.downloadedBytes)

		now := time.Now()
		elapsed := now.Sub(ps.lastUpdate)
		if elapsed >= 200*time.Millisecond {
			deltaBytes := ps.downloadedBytes - ps.lastBytes
			speed := float64(deltaBytes) / elapsed.Seconds()

			ps.task.mu.Lock()
			ps.task.SpeedBps = int64(speed)
			if ps.task.TotalBytes > 0 {
				ps.task.ProgressPercent = (float64(ps.downloadedBytes) / float64(ps.task.TotalBytes)) * 100.0
				remaining := ps.task.TotalBytes - ps.downloadedBytes
				if speed > 0 && remaining > 0 {
					ps.task.ETASeconds = int64(float64(remaining) / speed)
				} else {
					ps.task.ETASeconds = 0
				}
			}
			ps.task.mu.Unlock()

			ps.lastUpdate = now
			ps.lastBytes = ps.downloadedBytes
		}
	}
	return n, err
}
