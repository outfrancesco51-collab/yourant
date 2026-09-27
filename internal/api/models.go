package api

import (
	"context"
	"yourant/internal/anilist"
	"yourant/internal/downloader"
	"yourant/internal/party"
	"yourant/internal/storage"
)

// RouterConfig holds configuration parameters for the HTTP router.
type RouterConfig struct {
	DownloadsDir string
	StaticDir    string
	Version      string
	PartyHub     *party.Hub
}

// AniListService defines required operations for anime metadata.
type AniListService interface {
	GetTrending(ctx context.Context, page, perPage int) (*anilist.PageResult, error)
	GetPopular(ctx context.Context, page, perPage int) (*anilist.PageResult, error)
	Search(ctx context.Context, query string, page, perPage int) (*anilist.PageResult, error)
	GetDetail(ctx context.Context, id int) (*anilist.AnimeMedia, error)
}

// StorageService defines tracking operations for user library.
type StorageService interface {
	GetList(statusFilter string) []*storage.UserLibraryEntry
	GetEntry(mediaID int) (*storage.UserLibraryEntry, bool)
	Update(req storage.TrackingUpdateRequest) (*storage.UserLibraryEntry, error)
	Upsert(entry *storage.UserLibraryEntry) (*storage.UserLibraryEntry, error)
	Delete(mediaID int) bool
	Save() error
}

// DownloadService defines operations for file downloads and sandboxed filesystem.
type DownloadService interface {
	Enqueue(req downloader.DownloadRequest) (*downloader.DownloadTask, error)
	GetTask(id string) (*downloader.DownloadTask, bool)
	ListTasks() []*downloader.DownloadTask
	Cancel(id string) error
	ListFiles() ([]downloader.FileInfo, error)
	DeleteFile(fileName string) error
	ResolveSafePath(fileName string) (string, error)
}
