package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 1. DATA MODELS & INTERFACES (Matching PROJECT.md Contracts)
// ============================================================================

// AnimeTitle encapsulates multi-language titles.
type AnimeTitle struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
	Native  string `json:"native"`
}

// AnimeCoverImage contains cover images in various sizes.
type AnimeCoverImage struct {
	ExtraLarge string `json:"extraLarge"`
	Large      string `json:"large"`
	Medium     string `json:"medium"`
	Color      string `json:"color"`
}

// AnimeCharacter represents a character in the cast.
type AnimeCharacter struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // MAIN, SUPPORTING
}

// AnimeRelation represents related anime media.
type AnimeRelation struct {
	ID           int        `json:"id"`
	Title        AnimeTitle `json:"title"`
	RelationType string     `json:"relationType"` // PREQUEL, SEQUEL, SIDE_STORY
	Format       string     `json:"format"`
}

// AnimeMedia represents an anime media record.
type AnimeMedia struct {
	ID           int              `json:"id"`
	IDMal        *int             `json:"idMal,omitempty"`
	Title        AnimeTitle       `json:"title"`
	CoverImage   AnimeCoverImage  `json:"coverImage"`
	BannerImage  string           `json:"bannerImage,omitempty"`
	Description  string           `json:"description,omitempty"`
	Format       string           `json:"format"` // TV, MOVIE, OVA
	Status       string           `json:"status"` // RELEASING, FINISHED
	Episodes     int              `json:"episodes"`
	Duration     int              `json:"duration"` // In minutes
	Season       string           `json:"season,omitempty"`
	SeasonYear   int              `json:"seasonYear,omitempty"`
	AverageScore int              `json:"averageScore"`
	Popularity   int              `json:"popularity"`
	Trending     int              `json:"trending"`
	Genres       []string         `json:"genres"`
	Characters   []AnimeCharacter `json:"characters,omitempty"`
	Relations    []AnimeRelation  `json:"relations,omitempty"`
}

// PageInfo holds pagination metadata.
type PageInfo struct {
	Total       int  `json:"total"`
	PerPage     int  `json:"perPage"`
	CurrentPage int  `json:"currentPage"`
	LastPage    int  `json:"lastPage"`
	HasNextPage bool `json:"hasNextPage"`
}

// PageResult represents paginated media output.
type PageResult struct {
	PageInfo PageInfo     `json:"pageInfo"`
	Items    []AnimeMedia `json:"items"`
}

// UserLibraryEntry represents tracked user anime in local library.
type UserLibraryEntry struct {
	ID            int       `json:"id"`
	MediaID       int       `json:"mediaId"`
	Title         string    `json:"title"`
	Status        string    `json:"status"` // CURRENT, PLANNING, COMPLETED, DROPPED, PAUSED
	Progress      int       `json:"progress"`
	TotalEpisodes int       `json:"totalEpisodes"`
	Score         float64   `json:"score"` // 0.0 to 10.0
	UpdatedAt     time.Time `json:"updatedAt"`
}

// DownloadTask represents an offline media download job.
type DownloadTask struct {
	ID              string             `json:"id"`
	AnimeID         int                `json:"animeId"`
	EpisodeNumber   int                `json:"episodeNumber"`
	Title           string             `json:"title"`
	URL             string             `json:"url"`
	FileName        string             `json:"fileName"`
	TotalBytes      int64              `json:"totalBytes"`
	DownloadedBytes int64              `json:"downloadedBytes"`
	SpeedBps        int64              `json:"speedBps"`
	ProgressPercent float64            `json:"progressPercent"`
	ETASeconds      int64              `json:"etaSeconds"`
	Status          string             `json:"status"` // QUEUED, DOWNLOADING, PAUSED, COMPLETED, FAILED, CANCELLED
	Error           string             `json:"error,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
	CancelFunc      context.CancelFunc `json:"-"`
}

// WatchPartyRoom represents an active watch party session.
type WatchPartyRoom struct {
	RoomID        string             `json:"roomId"`
	HostID        string             `json:"hostId"`
	HostName      string             `json:"hostName"`
	MediaID       int                `json:"mediaId"`
	EpisodeNumber int                `json:"episodeNumber"`
	MediaTitle    string             `json:"mediaTitle"`
	IsPlaying     bool               `json:"isPlaying"`
	CurrentTime   float64            `json:"currentTime"`
	PlaybackRate  float64            `json:"playbackRate"`
	MemberCount   int                `json:"memberCount"`
	CreatedAt     time.Time          `json:"createdAt"`
	Members       map[string]string  `json:"-"` // id -> name
}

// WatchPartyMessage represents the JSON protocol message.
type WatchPartyMessage struct {
	Type       string                 `json:"type"`
	RoomID     string                 `json:"roomId"`
	SenderID   string                 `json:"senderId"`
	SenderName string                 `json:"senderName"`
	IsHost     bool                   `json:"isHost"`
	ServerTime int64                  `json:"serverTime"`
	ClientTime int64                  `json:"clientTime,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// ============================================================================
// 2. MOCK ANILIST GRAPHQL SERVER & CATALOG
// ============================================================================

// SampleSeededCatalog returns 10 pre-seeded rich anime entries.
func SampleSeededCatalog() []AnimeMedia {
	return []AnimeMedia{
		{
			ID:       154587,
			Title:    AnimeTitle{Romaji: "Sousou no Frieren", English: "Frieren: Beyond Journey's End", Native: "葬送のフリーレン"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/frieren.jpg", Large: "https://example.com/frieren_lg.jpg", Color: "#38bdf8"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 28,
			Duration: 24,
			AverageScore: 92,
			Popularity:   280000,
			Trending:     100,
			Genres:       []string{"Adventure", "Drama", "Fantasy"},
			Characters: []AnimeCharacter{
				{ID: 1, Name: "Frieren", Role: "MAIN"},
				{ID: 2, Name: "Fern", Role: "MAIN"},
				{ID: 3, Name: "Stark", Role: "MAIN"},
			},
			Relations: []AnimeRelation{
				{ID: 154588, Title: AnimeTitle{English: "Frieren Mini Anime"}, RelationType: "SIDE_STORY", Format: "SPECIAL"},
			},
		},
		{
			ID:       16498,
			Title:    AnimeTitle{Romaji: "Shingeki no Kyojin", English: "Attack on Titan", Native: "進撃の巨人"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/aot.jpg", Large: "https://example.com/aot_lg.jpg", Color: "#e11d48"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 25,
			Duration: 24,
			AverageScore: 89,
			Popularity:   650000,
			Trending:     95,
			Genres:       []string{"Action", "Drama", "Fantasy", "Mystery"},
			Characters: []AnimeCharacter{
				{ID: 10, Name: "Eren Yeager", Role: "MAIN"},
				{ID: 11, Name: "Mikasa Ackerman", Role: "MAIN"},
				{ID: 12, Name: "Levi", Role: "MAIN"},
			},
		},
		{
			ID:       113415,
			Title:    AnimeTitle{Romaji: "Jujutsu Kaisen", English: "Jujutsu Kaisen", Native: "呪術廻戦"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/jjk.jpg", Large: "https://example.com/jjk_lg.jpg", Color: "#0284c7"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 24,
			Duration: 24,
			AverageScore: 86,
			Popularity:   500000,
			Trending:     90,
			Genres:       []string{"Action", "Fantasy", "Supernatural"},
		},
		{
			ID:       101922,
			Title:    AnimeTitle{Romaji: "Kimetsu no Yaiba", English: "Demon Slayer: Kimetsu no Yaiba", Native: "鬼滅の刃"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/ds.jpg", Color: "#881337"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 26,
			Duration: 24,
			AverageScore: 85,
			Popularity:   600000,
			Trending:     88,
			Genres:       []string{"Action", "Adventure", "Fantasy"},
		},
		{
			ID:       120377,
			Title:    AnimeTitle{Romaji: "Cyberpunk: Edgerunners", English: "Cyberpunk: Edgerunners", Native: "サイバーパンク エッジランナーズ"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/cyberpunk.jpg", Color: "#facc15"},
			Format:   "ONA",
			Status:   "FINISHED",
			Episodes: 10,
			Duration: 25,
			AverageScore: 86,
			Popularity:   310000,
			Trending:     85,
			Genres:       []string{"Action", "Sci-Fi"},
		},
		{
			ID:       130003,
			Title:    AnimeTitle{Romaji: "Bocchi the Rock!", English: "Bocchi the Rock!", Native: "ぼっち・ざ・ろっく！"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/bocchi.jpg", Color: "#f43f5e"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 12,
			Duration: 24,
			AverageScore: 88,
			Popularity:   220000,
			Trending:     82,
			Genres:       []string{"Comedy", "Music", "Slice of Life"},
		},
		{
			ID:       9253,
			Title:    AnimeTitle{Romaji: "Steins;Gate", English: "Steins;Gate", Native: "STEINS;GATE"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/steins.jpg", Color: "#64748b"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 24,
			Duration: 24,
			AverageScore: 90,
			Popularity:   420000,
			Trending:     80,
			Genres:       []string{"Drama", "Sci-Fi", "Suspense"},
		},
		{
			ID:       1535,
			Title:    AnimeTitle{Romaji: "Death Note", English: "Death Note", Native: "DEATH NOTE"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/deathnote.jpg", Color: "#070a13"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 37,
			Duration: 23,
			AverageScore: 84,
			Popularity:   700000,
			Trending:     75,
			Genres:       []string{"Mystery", "Psychological", "Supernatural", "Suspense"},
		},
		{
			ID:       5114,
			Title:    AnimeTitle{Romaji: "Hagane no Renkinjutsushi", English: "Fullmetal Alchemist: Brotherhood", Native: "鋼の錬金術師"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/fmab.jpg", Color: "#ea580c"},
			Format:   "TV",
			Status:   "FINISHED",
			Episodes: 64,
			Duration: 24,
			AverageScore: 91,
			Popularity:   620000,
			Trending:     78,
			Genres:       []string{"Action", "Adventure", "Drama", "Fantasy"},
		},
		{
			ID:       199,
			Title:    AnimeTitle{Romaji: "Sen to Chihiro no Kamikakushi", English: "Spirited Away", Native: "千と千尋の神隠し"},
			CoverImage: AnimeCoverImage{ExtraLarge: "https://example.com/spirited.jpg", Color: "#10b981"},
			Format:   "MOVIE",
			Status:   "FINISHED",
			Episodes: 1,
			Duration: 125,
			AverageScore: 87,
			Popularity:   480000,
			Trending:     72,
			Genres:       []string{"Adventure", "Supernatural"},
		},
	}
}

// MockAniListServer creates a test HTTP server emulating AniList GraphQL.
type MockAniListServer struct {
	Server        *httptest.Server
	Catalog       []AnimeMedia
	RequestCount  int64
	Simulate429   bool
	RateLimitMax  int
	RateLimitCurr int
	mu            sync.Mutex
}

func NewMockAniListServer() *MockAniListServer {
	m := &MockAniListServer{
		Catalog:      SampleSeededCatalog(),
		RateLimitMax: 90,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.RequestCount++
		if m.Simulate429 || (m.RateLimitCurr >= m.RateLimitMax) {
			m.mu.Unlock()
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "Too Many Requests"})
			return
		}
		m.RateLimitCurr++
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "90")
		w.Header().Set("X-RateLimit-Remaining", "85")

		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		json.Unmarshal(body, &req)

		// Check query type
		if strings.Contains(req.Query, "TRENDING_DESC") || strings.Contains(req.Query, "GetTrendingAnime") {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"Page": map[string]interface{}{
						"pageInfo": PageInfo{
							Total:       len(m.Catalog),
							PerPage:     20,
							CurrentPage: 1,
							LastPage:    1,
							HasNextPage: false,
						},
						"media": m.Catalog,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if strings.Contains(req.Query, "search") || strings.Contains(req.Query, "SearchAnime") {
			searchVal := ""
			if req.Variables != nil {
				if s, ok := req.Variables["search"].(string); ok {
					searchVal = strings.ToLower(strings.TrimSpace(s))
				}
			}
			var filtered []AnimeMedia
			for _, item := range m.Catalog {
				if searchVal == "" ||
					strings.Contains(strings.ToLower(item.Title.English), searchVal) ||
					strings.Contains(strings.ToLower(item.Title.Romaji), searchVal) ||
					strings.Contains(strings.ToLower(item.Title.Native), searchVal) {
					filtered = append(filtered, item)
				}
			}
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"Page": map[string]interface{}{
						"pageInfo": PageInfo{
							Total:       len(filtered),
							PerPage:     20,
							CurrentPage: 1,
							LastPage:    1,
							HasNextPage: false,
						},
						"media": filtered,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Detail query
		if strings.Contains(req.Query, "GetAnimeDetails") || strings.Contains(req.Query, "Media(id:") || strings.Contains(req.Query, "$id") {
			targetID := 154587
			if req.Variables != nil {
				if idFloat, ok := req.Variables["id"].(float64); ok {
					targetID = int(idFloat)
				}
			}
			var found *AnimeMedia
			for _, item := range m.Catalog {
				if item.ID == targetID {
					found = &item
					break
				}
			}
			if found == nil {
				found = &m.Catalog[0]
			}
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"Media": found,
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Fallback
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"Page": map[string]interface{}{
					"media": m.Catalog,
				},
			},
		})
	})

	m.Server = httptest.NewServer(mux)
	return m
}

func (m *MockAniListServer) Close() {
	m.Server.Close()
}

// ============================================================================
// 3. CONTROLLED STORAGE GUARD & PATH TRAVERSAL DEFENSE
// ============================================================================

var (
	ErrPathTraversal   = errors.New("access denied: path traversal detected")
	ErrInvalidFileName = errors.New("invalid file name: contains forbidden characters")
	unsafeCharsRegex   = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
)

type ControlledStorageGuard struct {
	BaseDir string
}

func NewControlledStorageGuard(baseDir string) (*ControlledStorageGuard, error) {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base directory: %w", err)
	}
	if err := os.MkdirAll(absBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &ControlledStorageGuard{BaseDir: absBase}, nil
}

// SanitizeFileName strips Windows reserved device names and illegal characters.
func SanitizeFileName(name string) string {
	// First check for URL encoded representations
	if unescaped, err := url.QueryUnescape(name); err == nil {
		name = unescaped
	}

	clean := unsafeCharsRegex.ReplaceAllString(name, "_")
	clean = strings.TrimSpace(clean)
	clean = strings.Trim(clean, ". ")
	if clean == "" {
		clean = "download_file"
	}
	upper := strings.ToUpper(clean)
	reserved := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}
	for _, r := range reserved {
		if upper == r || strings.HasPrefix(upper, r+".") {
			clean = "_" + clean
			break
		}
	}
	return clean
}

// ResolveSafePath strictly asserts that the target path resides inside BaseDir.
func (g *ControlledStorageGuard) ResolveSafePath(fileName string) (string, error) {
	// Detect raw directory traversal attempt in original name
	rawUnescaped := fileName
	for i := 0; i < 5; i++ {
		unescaped, err := url.QueryUnescape(rawUnescaped)
		if err != nil || unescaped == rawUnescaped {
			break
		}
		rawUnescaped = unescaped
	}

	if strings.Contains(rawUnescaped, "..") || strings.Contains(rawUnescaped, "\x00") ||
		strings.Contains(rawUnescaped, "/") || strings.Contains(rawUnescaped, "\\") ||
		filepath.IsAbs(rawUnescaped) || (len(rawUnescaped) > 1 && rawUnescaped[1] == ':') {
		return "", ErrPathTraversal
	}

	cleanName := SanitizeFileName(fileName)
	targetPath := filepath.Clean(filepath.Join(g.BaseDir, cleanName))

	rel, err := filepath.Rel(g.BaseDir, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." && cleanName != "" {
		return "", ErrPathTraversal
	}

	return targetPath, nil
}

// ============================================================================
// 4. RESUMABLE STREAMING DOWNLOADER SIMULATOR
// ============================================================================

type ResumableDownloaderSimulator struct {
	Guard       *ControlledStorageGuard
	ActiveTasks map[string]*DownloadTask
	mu          sync.Mutex
}

func NewResumableDownloaderSimulator(guard *ControlledStorageGuard) *ResumableDownloaderSimulator {
	return &ResumableDownloaderSimulator{
		Guard:       guard,
		ActiveTasks: make(map[string]*DownloadTask),
	}
}

func (d *ResumableDownloaderSimulator) StartDownload(ctx context.Context, animeID, ep int, title, srcURL, filename string, totalSize int64) (*DownloadTask, error) {
	safePath, err := d.Guard.ResolveSafePath(filename)
	if err != nil {
		return nil, err
	}

	taskID := fmt.Sprintf("dl-%d-%d", animeID, ep)
	cCtx, cancel := context.WithCancel(ctx)

	task := &DownloadTask{
		ID:            taskID,
		AnimeID:       animeID,
		EpisodeNumber: ep,
		Title:         title,
		URL:           srcURL,
		FileName:      filepath.Base(safePath),
		TotalBytes:    totalSize,
		Status:        "DOWNLOADING",
		CreatedAt:     time.Now(),
		CancelFunc:    cancel,
	}

	d.mu.Lock()
	d.ActiveTasks[taskID] = task
	d.mu.Unlock()

	// Write simulation: write to .part file
	partPath := safePath + ".part"
	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		task.Status = "FAILED"
		task.Error = err.Error()
		return task, err
	}

	chunkSize := int64(1024)
	if totalSize == 0 {
		partFile.Close()
		os.Rename(partPath, safePath)
		task.Status = "COMPLETED"
		task.ProgressPercent = 100.0
		return task, nil
	}

	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = byte(i % 256)
	}

	written := int64(0)
	for written < totalSize {
		select {
		case <-cCtx.Done():
			partFile.Close()
			task.Status = "CANCELLED"
			return task, nil
		default:
		}

		toWrite := chunkSize
		if written+toWrite > totalSize {
			toWrite = totalSize - written
		}
		n, _ := partFile.Write(chunk[:toWrite])
		written += int64(n)
		task.DownloadedBytes = written
		task.ProgressPercent = (float64(written) / float64(totalSize)) * 100.0
	}

	partFile.Close()
	// Atomic rename to destination
	if err := os.Rename(partPath, safePath); err != nil {
		task.Status = "FAILED"
		task.Error = err.Error()
		return task, err
	}

	task.Status = "COMPLETED"
	task.ProgressPercent = 100.0
	return task, nil
}

// ============================================================================
// 5. HTTP 206 VIDEO STREAMING HANDLER
// ============================================================================

func HandleVideoStream(w http.ResponseWriter, r *http.Request, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Could not stat file", http.StatusInternalServerError)
		return
	}
	fileSize := stat.Size()

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Content-Type", "video/mp4")

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, file)
		return
	}

	// Parse bytes=start-end
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		http.Error(w, "Invalid range prefix", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		http.Error(w, "Invalid range format", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var start, end int64
	if parts[0] == "" {
		// Suffix range: -500 means last 500 bytes
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLen <= 0 {
			http.Error(w, "Invalid suffix range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if suffixLen > fileSize {
			suffixLen = fileSize
		}
		start = fileSize - suffixLen
		end = fileSize - 1
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			http.Error(w, "Invalid start range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if parts[1] == "" {
			end = fileSize - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil || end < start {
				http.Error(w, "Invalid end range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}
	}

	if start >= fileSize {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if end >= fileSize {
		end = fileSize - 1
	}

	contentLength := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.WriteHeader(http.StatusPartialContent)

	file.Seek(start, io.SeekStart)
	io.CopyN(w, file, contentLength)
}

// ============================================================================
// 6. WATCH PARTY HUB SIMULATOR & 3-TIER DRIFT COMPENSATION
// ============================================================================

type WatchPartyHubSimulator struct {
	Rooms map[string]*WatchPartyRoom
	mu    sync.RWMutex
}

func NewWatchPartyHubSimulator() *WatchPartyHubSimulator {
	return &WatchPartyHubSimulator{
		Rooms: make(map[string]*WatchPartyRoom),
	}
}

func (h *WatchPartyHubSimulator) CreateRoom(roomID, hostID, hostName string, mediaID int, title string) *WatchPartyRoom {
	h.mu.Lock()
	defer h.mu.Unlock()

	room := &WatchPartyRoom{
		RoomID:        roomID,
		HostID:        hostID,
		HostName:      hostName,
		MediaID:       mediaID,
		MediaTitle:    title,
		PlaybackRate:  1.0,
		CurrentTime:   0.0,
		IsPlaying:     false,
		MemberCount:   1,
		CreatedAt:     time.Now(),
		Members:       map[string]string{hostID: hostName},
	}
	h.Rooms[roomID] = room
	return room
}

func (h *WatchPartyHubSimulator) JoinRoom(roomID, userID, userName string) (*WatchPartyRoom, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.Rooms[roomID]
	if !exists {
		return nil, errors.New("room not found")
	}
	if len(room.Members) >= 100 {
		return nil, errors.New("room capacity reached")
	}

	room.Members[userID] = userName
	room.MemberCount = len(room.Members)
	return room, nil
}

func (h *WatchPartyHubSimulator) LeaveRoom(roomID, userID string) (*WatchPartyRoom, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.Rooms[roomID]
	if !exists {
		return nil, errors.New("room not found")
	}

	delete(room.Members, userID)
	room.MemberCount = len(room.Members)

	// If host left, elect oldest remaining client
	if room.HostID == userID && room.MemberCount > 0 {
		for id, name := range room.Members {
			room.HostID = id
			room.HostName = name
			break
		}
	}
	return room, nil
}

// CalculateDriftCompensation computes client sync action from host timestamp.
// Drift thresholds:
//   absDrift < 0.300s (300ms)  -> In Sync (Tier 1: Rate = 1.0, Action = "IN_SYNC", Pill = "green")
//   0.300s <= absDrift <= 1.5s -> Micro Speed Adjust (Tier 2: Rate = 1.05 or 0.95, Action = "SPEED_ADJUST", Pill = "amber")
//   absDrift > 1.5s            -> Hard Resync (Tier 3: Target Seek, Rate = 1.0, Action = "HARD_SEEK", Pill = "red")
func CalculateDriftCompensation(clientCurrentTime, hostTimestamp float64, isPlaying bool, transitDelaySec float64, playbackRate float64) (targetTime float64, rate float64, action string, pillColor string) {
	if isPlaying {
		targetTime = hostTimestamp + (transitDelaySec * playbackRate)
	} else {
		targetTime = hostTimestamp
	}

	drift := clientCurrentTime - targetTime
	absDrift := math.Abs(drift)

	if absDrift < 0.300 {
		return targetTime, 1.0, "IN_SYNC", "green"
	} else if absDrift <= 1.500 {
		if drift > 0 {
			// Client ahead: slow down slightly
			return targetTime, 0.95, "SPEED_ADJUST", "amber"
		}
		// Client behind: speed up slightly
		return targetTime, 1.05, "SPEED_ADJUST", "amber"
	}
	// Hard resync
	return targetTime, 1.0, "HARD_SEEK", "red"
}

// ============================================================================
// 7. WEB AUDIO BOOSTER SIMULATOR & MATHEMATICAL CLAMPING
// ============================================================================

type AudioBoosterSimulator struct {
	singletonMap map[string]float64 // simulated pointer/ID -> gain value
	mu           sync.Mutex
}

func NewAudioBoosterSimulator() *AudioBoosterSimulator {
	return &AudioBoosterSimulator{
		singletonMap: make(map[string]float64),
	}
}

// CalculateVolumeBoost maps percentage [0, 1000] to gain [0.0, 10.0].
func CalculateVolumeBoost(percentage float64) (gain float64, isBoostZone bool, warningBadge bool) {
	clampedPercent := math.Max(0.0, math.Min(1000.0, percentage))
	gain = clampedPercent / 100.0
	isBoostZone = clampedPercent > 100.0
	warningBadge = clampedPercent > 200.0
	return gain, isBoostZone, warningBadge
}

// AttachOrGetBooster enforces the singleton invariant per media element.
func (a *AudioBoosterSimulator) AttachOrGetBooster(elementID string, initialGain float64) (float64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if val, ok := a.singletonMap[elementID]; ok {
		return val, false // already existing singleton
	}
	a.singletonMap[elementID] = initialGain
	return initialGain, true // newly created
}

// SimulateLimiterClamping models DynamicsCompressorNode threshold -2.0dBFS.
func SimulateLimiterClamping(inputDbfs float64, gainFactor float64) float64 {
	// Gain in dB = 20 * log10(gainFactor)
	gainDb := 20.0 * math.Log10(math.Max(0.0001, gainFactor))
	boostedLevel := inputDbfs + gainDb

	threshold := -2.0
	if boostedLevel > threshold {
		// Strict limiting: compression ratio 16:1
		excess := boostedLevel - threshold
		compressedExcess := excess / 16.0
		return threshold + compressedExcess
	}
	return boostedLevel
}

// ============================================================================
// 8. THEME TOKEN VALIDATOR & WCAG CONTRAST CALCULATOR
// ============================================================================

type ThemeTokenValidator struct{}

func (v *ThemeTokenValidator) GetCoreTokens() map[string]string {
	return map[string]string{
		"--bg-void":        "#070a13",
		"--bg-surface":     "#0f172a",
		"--bg-card":        "#1e293b",
		"--metallic-cyan":  "#38bdf8",
		"--metallic-cobalt": "#0284c7",
		"--blood-base":     "#4c0519",
		"--blood-crimson":  "#881337",
		"--blood-vivid":    "#991b1b",
		"--blood-bright":   "#e11d48",
		"--blood-ruby":     "#f43f5e",
		"--text-primary":   "#f8fafc",
	}
}

func HexToRGB(hexStr string) (r, g, b float64, err error) {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex string length %d", len(hexStr))
	}
	val, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	r = float64((val >> 16) & 0xFF)
	g = float64((val >> 8) & 0xFF)
	b = float64(val & 0xFF)
	return r, g, b, nil
}

func RelativeLuminance(r, g, b float64) float64 {
	transform := func(c float64) float64 {
		c = c / 255.0
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*transform(r) + 0.7152*transform(g) + 0.0722*transform(b)
}

func CalculateContrastRatio(hex1, hex2 string) (float64, error) {
	r1, g1, b1, err := HexToRGB(hex1)
	if err != nil {
		return 0, err
	}
	r2, g2, b2, err := HexToRGB(hex2)
	if err != nil {
		return 0, err
	}

	lum1 := RelativeLuminance(r1, g1, b1)
	lum2 := RelativeLuminance(r2, g2, b2)

	lMax := math.Max(lum1, lum2)
	lMin := math.Min(lum1, lum2)
	return (lMax + 0.05) / (lMin + 0.05), nil
}

// ============================================================================
// 9. GLSL SHADER SYNTAX & UNIFORM VALIDATOR
// ============================================================================

type GLSLShaderValidator struct{}

func (s *GLSLShaderValidator) ValidateFragmentShader(code string) []string {
	var violations []string

	requiredUniforms := []string{"u_resolution", "u_time", "u_mouse", "u_dim"}
	for _, u := range requiredUniforms {
		pattern := fmt.Sprintf(`uniform\s+[a-zA-Z0-9_]+\s+%s;`, u)
		re := regexp.MustCompile(pattern)
		if !re.MatchString(code) {
			violations = append(violations, fmt.Sprintf("Missing required uniform: %s", u))
		}
	}

	if !strings.Contains(code, "precision highp float;") {
		violations = append(violations, "Missing 'precision highp float;' declaration")
	}

	if !strings.Contains(code, "fbm") {
		violations = append(violations, "Missing Fractal Brownian Motion (fbm) implementation")
	}

	if !strings.Contains(code, "u_dim") {
		violations = append(violations, "Missing Cinema Mode dimming factor application with u_dim")
	}

	return violations
}

// ============================================================================
// 10. CINEMA MODE STATE MACHINE SIMULATOR
// ============================================================================

type CinemaModeStateMachine struct {
	IsActive   bool
	DimFactor  float64 // 1.0 (normal) to 0.15 (cinema)
	HotkeyLog  []string
	TargetDim  float64
}

func NewCinemaModeStateMachine() *CinemaModeStateMachine {
	return &CinemaModeStateMachine{
		IsActive:  false,
		DimFactor: 1.0,
		TargetDim: 1.0,
	}
}

func (c *CinemaModeStateMachine) Toggle() bool {
	c.IsActive = !c.IsActive
	if c.IsActive {
		c.TargetDim = 0.15
	} else {
		c.TargetDim = 1.0
	}
	c.DimFactor = c.TargetDim
	return c.IsActive
}

func (c *CinemaModeStateMachine) HandleKeyPress(key string, targetTagName string) bool {
	c.HotkeyLog = append(c.HotkeyLog, key)
	// Ignore hotkeys if user is typing in form inputs
	if targetTagName == "INPUT" || targetTagName == "TEXTAREA" {
		return c.IsActive
	}

	if key == "c" || key == "C" {
		return c.Toggle()
	}
	if key == "Escape" && c.IsActive {
		return c.Toggle()
	}
	return c.IsActive
}
