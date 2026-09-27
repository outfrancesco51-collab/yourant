package api

import (
	"net/http"
	"os"
	"time"

	"yourant/internal/party"
)

// Handler coordinates REST API requests across application subsystems.
type Handler struct {
	cfg        RouterConfig
	anilist    AniListService
	storage    StorageService
	downloader DownloadService
	partyHub   *party.Hub
	startTime  time.Time
}

// NewRouter constructs the net/http.ServeMux router and applies the middleware chain.
func NewRouter(cfg RouterConfig, anilist AniListService, storage StorageService, downloader DownloadService) http.Handler {
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}

	partyHub := cfg.PartyHub
	if partyHub == nil {
		partyHub = party.NewHub()
	}

	h := &Handler{
		cfg:        cfg,
		anilist:    anilist,
		storage:    storage,
		downloader: downloader,
		partyHub:   partyHub,
		startTime:  time.Now(),
	}

	mux := http.NewServeMux()

	// 1. Health
	mux.HandleFunc("GET /api/health", h.HandleHealth)

	// 2. Anime Metadata (AniList)
	mux.HandleFunc("GET /api/anime/trending", h.HandleGetTrending)
	mux.HandleFunc("GET /api/anime/popular", h.HandleGetPopular)
	mux.HandleFunc("GET /api/anime/search", h.HandleSearchAnime)
	mux.HandleFunc("GET /api/anime/{id}", h.HandleGetAnimeDetail)

	// 3. User Tracking Library
	mux.HandleFunc("GET /api/user/tracking/list", h.HandleGetTrackingList)
	mux.HandleFunc("POST /api/user/tracking/update", h.HandleUpdateTracking)

	// 4. Downloads & Controlled Storage
	mux.HandleFunc("GET /api/downloads", h.HandleGetDownloads)
	mux.HandleFunc("POST /api/downloads", h.HandleStartDownload)
	mux.HandleFunc("DELETE /api/downloads/{id}", h.HandleCancelDownload)
	mux.HandleFunc("GET /api/downloads/files", h.HandleListFiles)
	mux.HandleFunc("DELETE /api/downloads/files/{name...}", h.HandleDeleteFile)

	// 5. RFC 7233 HTTP 206 Partial Content Video Streaming
	mux.HandleFunc("GET /api/downloads/stream/{name...}", h.HandleStreamVideo)

	// 6. Watch Party WebSocket & Code Generation
	mux.HandleFunc("/api/watchparty/ws", h.HandleWatchPartyWS)
	mux.HandleFunc("GET /api/watchparty/code", h.HandleCreateRoomCode)
	mux.HandleFunc("POST /api/watchparty/code", h.HandleCreateRoomCode)

	// 7. Optional static frontend file server
	if cfg.StaticDir != "" {
		if fi, err := os.Stat(cfg.StaticDir); err == nil && fi.IsDir() {
			fs := http.FileServer(http.Dir(cfg.StaticDir))
			mux.Handle("/", fs)
		}
	}

	// Middleware chain: Recovery -> Logging -> CORS
	return RecoveryMiddleware(LoggingMiddleware(CORSMiddleware(mux)))
}
