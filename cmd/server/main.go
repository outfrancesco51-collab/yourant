package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"yourant/internal/anilist"
	"yourant/internal/api"
	"yourant/internal/downloader"
	"yourant/internal/storage"
)

type Config struct {
	Port         string
	Host         string
	DownloadsDir string
	StorageFile  string
	RateLimitRPM float64
	StaticDir    string
}

func loadConfig() Config {
	var cfg Config
	flag.StringVar(&cfg.Port, "port", "8080", "HTTP server listening port")
	flag.StringVar(&cfg.Host, "host", "0.0.0.0", "HTTP server listening host")
	flag.StringVar(&cfg.DownloadsDir, "downloads-dir", "downloads", "Media downloads storage directory")
	flag.StringVar(&cfg.StorageFile, "storage-file", filepath.Join("data", "library.json"), "Path to tracking JSON storage")
	flag.Float64Var(&cfg.RateLimitRPM, "rate-limit", 80.0, "AniList client requests per minute limit")
	flag.StringVar(&cfg.StaticDir, "static-dir", filepath.Join("frontend", "dist"), "Optional path to static frontend files")
	flag.Parse()

	// Environment variable overrides
	if envPort := os.Getenv("PORT"); envPort != "" {
		cfg.Port = envPort
	}
	if envHost := os.Getenv("HOST"); envHost != "" {
		cfg.Host = envHost
	}
	if envDownloads := os.Getenv("DOWNLOADS_DIR"); envDownloads != "" {
		cfg.DownloadsDir = envDownloads
	}
	if envStorage := os.Getenv("STORAGE_FILE"); envStorage != "" {
		cfg.StorageFile = envStorage
	}
	return cfg
}

func main() {
	cfg := loadConfig()
	log.Printf("[MAIN] Starting Yourant server on %s:%s\n", cfg.Host, cfg.Port)

	// 1. Resolve canonical absolute paths for safety
	absDownloadsDir, err := filepath.Abs(cfg.DownloadsDir)
	if err != nil {
		log.Fatalf("[FATAL] Cannot resolve downloads directory: %v\n", err)
	}
	if err := os.MkdirAll(absDownloadsDir, 0755); err != nil {
		log.Fatalf("[FATAL] Cannot create downloads directory: %v\n", err)
	}

	absStorageDir := filepath.Dir(cfg.StorageFile)
	if err := os.MkdirAll(absStorageDir, 0755); err != nil {
		log.Fatalf("[FATAL] Cannot create storage directory: %v\n", err)
	}

	// 2. Initialize Subsystems
	anilistClient := anilist.NewClient(anilist.Config{
		RateLimitRPM: cfg.RateLimitRPM,
		CacheTTL:     time.Hour,
	})
	defer anilistClient.Close()

	libraryStore, err := storage.NewLibraryStore(cfg.StorageFile)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize library store: %v\n", err)
	}
	if err := libraryStore.Load(); err != nil {
		log.Printf("[WARN] Error loading library store: %v. Initializing fresh.\n", err)
	}

	downloadManager, err := downloader.NewManager(absDownloadsDir)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize download manager: %v\n", err)
	}

	// 3. Initialize Router with Dependencies
	router := api.NewRouter(api.RouterConfig{
		DownloadsDir: absDownloadsDir,
		StaticDir:    cfg.StaticDir,
		Version:      "1.0.0",
	}, anilistClient, libraryStore, downloadManager)

	// 4. Configure HTTP Server
	// WriteTimeout is left unset (0) to permit continuous video streaming and scrubbing.
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 5. Signal trap for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		log.Printf("[SERVER] HTTP server running on http://%s:%s\n", cfg.Host, cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[SERVER] ListenAndServe error: %v\n", err)
		}
	}()

	// Block waiting for termination signal
	<-ctx.Done()
	log.Println("[SERVER] Shutdown signal received, initiating graceful teardown...")

	// 6. Graceful Shutdown with 10-second timeout context
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Drain active downloads safely
	if err := downloadManager.Close(); err != nil {
		log.Printf("[WARN] Error closing download manager: %v\n", err)
	}

	// Flush library state to disk
	if err := libraryStore.Save(); err != nil {
		log.Printf("[WARN] Error persisting library store on shutdown: %v\n", err)
	}

	// Terminate HTTP listener
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server shutdown error: %v\n", err)
	} else {
		log.Println("[SERVER] Server exited cleanly.")
	}
}
