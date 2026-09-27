package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidMediaID = errors.New("media ID must be positive")
	ErrInvalidScore   = errors.New("score must be between 0.0 and 10.0")
	ErrInvalidStatus  = errors.New("invalid status value")
)

// LibraryStore defines tracking operations.
type LibraryStore interface {
	GetList(statusFilter string) []*UserLibraryEntry
	GetEntry(mediaID int) (*UserLibraryEntry, bool)
	Upsert(entry *UserLibraryEntry) (*UserLibraryEntry, error)
	Update(req TrackingUpdateRequest) (*UserLibraryEntry, error)
	Delete(mediaID int) bool
	Save() error
	Load() error
}

// FileLibraryStore implements LibraryStore with in-memory map and atomic JSON persistence.
type FileLibraryStore struct {
	filePath string
	mu       sync.RWMutex
	entries  map[int]*UserLibraryEntry
}

// NewLibraryStore creates a new storage instance backed by filePath.
func NewLibraryStore(filePath string) (*FileLibraryStore, error) {
	if strings.TrimSpace(filePath) == "" {
		filePath = filepath.Join("data", "library.json")
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve storage path error: %w", err)
	}

	return &FileLibraryStore{
		filePath: absPath,
		entries:  make(map[int]*UserLibraryEntry),
	}, nil
}

// FilePath returns the configured storage path.
func (s *FileLibraryStore) FilePath() string {
	return s.filePath
}

// GetList retrieves entries, optionally filtered by status (case-insensitive).
func (s *FileLibraryStore) GetList(statusFilter string) []*UserLibraryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filter := strings.ToUpper(strings.TrimSpace(statusFilter))
	res := make([]*UserLibraryEntry, 0, len(s.entries))

	for _, entry := range s.entries {
		if filter == "" || strings.ToUpper(entry.Status) == filter {
			cp := *entry
			res = append(res, &cp)
		}
	}
	return res
}

// GetEntry retrieves a single anime entry by mediaID.
func (s *FileLibraryStore) GetEntry(mediaID int) (*UserLibraryEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[mediaID]
	if !ok {
		return nil, false
	}
	cp := *entry
	return &cp, true
}

// Upsert inserts or replaces an entry, applying business logic and timestamps.
func (s *FileLibraryStore) Upsert(entry *UserLibraryEntry) (*UserLibraryEntry, error) {
	if entry == nil || entry.MediaID <= 0 {
		return nil, ErrInvalidMediaID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cloned := *entry

	if cloned.ID <= 0 {
		cloned.ID = cloned.MediaID
	}

	// Validate & normalize status
	if cloned.Status == "" {
		cloned.Status = StatusPlanning
	} else {
		cloned.Status = strings.ToUpper(strings.TrimSpace(cloned.Status))
	}

	// Clamp score
	if cloned.Score < 0 {
		cloned.Score = 0
	} else if cloned.Score > 10.0 {
		cloned.Score = 10.0
	}

	// Clamp progress
	if cloned.Progress < 0 {
		cloned.Progress = 0
	}
	if cloned.TotalEpisodes > 0 && cloned.Progress > cloned.TotalEpisodes {
		cloned.Progress = cloned.TotalEpisodes
	}

	// Auto-completion logic
	if cloned.TotalEpisodes > 0 && cloned.Progress >= cloned.TotalEpisodes {
		cloned.Status = StatusCompleted
		if cloned.CompletedAt == nil {
			cloned.CompletedAt = &now
		}
	}

	if existing, exists := s.entries[cloned.MediaID]; exists {
		cloned.CreatedAt = existing.CreatedAt
		cloned.UpdatedAt = now
		if existing.StartedAt != nil {
			cloned.StartedAt = existing.StartedAt
		}
	} else {
		cloned.CreatedAt = now
		cloned.UpdatedAt = now
	}

	if cloned.Status == StatusCurrent && cloned.StartedAt == nil {
		cloned.StartedAt = &now
	}

	s.entries[cloned.MediaID] = &cloned
	ret := cloned
	return &ret, nil
}

// Update mutates an entry's fields according to TrackingUpdateRequest.
func (s *FileLibraryStore) Update(req TrackingUpdateRequest) (*UserLibraryEntry, error) {
	if req.MediaID <= 0 {
		return nil, ErrInvalidMediaID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, exists := s.entries[req.MediaID]
	if !exists {
		// Auto-create new entry
		title := "Anime"
		if req.Title != nil && *req.Title != "" {
			title = *req.Title
		}
		status := StatusCurrent
		if req.Status != nil && *req.Status != "" {
			status = strings.ToUpper(strings.TrimSpace(*req.Status))
		}
		progress := 0
		if req.Progress != nil {
			progress = *req.Progress
		}
		total := 0
		if req.TotalEpisodes != nil {
			total = *req.TotalEpisodes
		}
		score := 0.0
		if req.Score != nil {
			score = *req.Score
		}

		entry = &UserLibraryEntry{
			ID:            req.MediaID,
			MediaID:       req.MediaID,
			Title:         title,
			Status:        status,
			Progress:      progress,
			TotalEpisodes: total,
			Score:         score,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if status == StatusCurrent {
			entry.StartedAt = &now
		}
	}

	// Update supplied fields
	if req.Title != nil && *req.Title != "" {
		entry.Title = *req.Title
	}
	if req.CoverImage != nil {
		entry.CoverImage = *req.CoverImage
	}
	if req.Notes != nil {
		entry.Notes = *req.Notes
	}
	if req.TotalEpisodes != nil && *req.TotalEpisodes >= 0 {
		entry.TotalEpisodes = *req.TotalEpisodes
	}
	if req.Score != nil {
		score := *req.Score
		if score < 0.0 {
			score = 0.0
		} else if score > 10.0 {
			score = 10.0
		}
		entry.Score = score
	}
	if req.Progress != nil {
		p := *req.Progress
		if p < 0 {
			p = 0
		}
		entry.Progress = p
		// If progress > 0 and was planning, auto-move to current
		if entry.Progress > 0 && entry.Status == StatusPlanning {
			entry.Status = StatusCurrent
			if entry.StartedAt == nil {
				entry.StartedAt = &now
			}
		}
	}
	if req.Status != nil {
		st := strings.ToUpper(strings.TrimSpace(*req.Status))
		switch st {
		case StatusCurrent, StatusPlanning, StatusCompleted, StatusDropped, StatusPaused:
			entry.Status = st
		}
	}

	// Auto-complete check
	if entry.TotalEpisodes > 0 && entry.Progress >= entry.TotalEpisodes {
		entry.Progress = entry.TotalEpisodes
		entry.Status = StatusCompleted
		if entry.CompletedAt == nil {
			entry.CompletedAt = &now
		}
	}

	entry.UpdatedAt = now
	s.entries[req.MediaID] = entry

	ret := *entry
	return &ret, nil
}

// Delete removes an entry by mediaID.
func (s *FileLibraryStore) Delete(mediaID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[mediaID]; !exists {
		return false
	}
	delete(s.entries, mediaID)
	return true
}

// Load reads storage from JSON file. If the file does not exist, it pre-seeds demo data and saves.
func (s *FileLibraryStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		// Pre-seed demo entries
		s.seedDemoEntries()
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("read storage file error: %w", err)
	}

	var list []*UserLibraryEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("unmarshal storage error: %w", err)
	}

	s.entries = make(map[int]*UserLibraryEntry, len(list))
	for _, item := range list {
		if item != nil && item.MediaID > 0 {
			if item.ID <= 0 {
				item.ID = item.MediaID
			}
			s.entries[item.MediaID] = item
		}
	}

	return nil
}

// Save atomically writes the current entries to disk using a temporary file.
func (s *FileLibraryStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

func (s *FileLibraryStore) saveLocked() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create storage dir error: %w", err)
	}

	list := make([]*UserLibraryEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		list = append(list, entry)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal storage error: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "library.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp storage file error: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("write temp storage file error: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp storage file error: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp storage file error: %w", err)
	}

	// Remove target file first to prevent Windows ERROR_ALREADY_EXISTS on rename
	if _, err := os.Stat(s.filePath); err == nil {
		_ = os.Remove(s.filePath)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("atomic rename storage file error: %w", err)
	}

	return nil
}

func (s *FileLibraryStore) seedDemoEntries() {
	now := time.Now()
	startTime := now.Add(-14 * 24 * time.Hour)
	completeTime := now.Add(-2 * 24 * time.Hour)

	s.entries = map[int]*UserLibraryEntry{
		154587: {
			ID:            154587,
			MediaID:       154587,
			Title:         "Frieren: Beyond Journey's End",
			Status:        StatusCurrent,
			Progress:      14,
			TotalEpisodes: 28,
			Score:         9.5,
			CoverImage:    "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx154587-n2bOfvZihioS.png",
			StartedAt:     &startTime,
			CreatedAt:     startTime,
			UpdatedAt:     now,
		},
		16498: {
			ID:            16498,
			MediaID:       16498,
			Title:         "Attack on Titan",
			Status:        StatusCompleted,
			Progress:      25,
			TotalEpisodes: 25,
			Score:         9.0,
			CoverImage:    "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx16498-C6FPmWm59CyP.jpg",
			StartedAt:     &startTime,
			CompletedAt:   &completeTime,
			CreatedAt:     startTime,
			UpdatedAt:     now,
		},
	}
}
