package storage

import "time"

// User tracking statuses.
const (
	StatusCurrent   = "CURRENT"   // Watching
	StatusPlanning  = "PLANNING"  // Plan to Watch
	StatusCompleted = "COMPLETED" // Completed
	StatusDropped   = "DROPPED"   // Dropped
	StatusPaused    = "PAUSED"    // On Hold / Paused
)

// UserLibraryEntry represents an anime tracked in user's library.
type UserLibraryEntry struct {
	ID            int        `json:"id"`
	MediaID       int        `json:"mediaId"`
	Title         string     `json:"title"`
	Status        string     `json:"status"` // CURRENT, PLANNING, COMPLETED, DROPPED, PAUSED
	Progress      int        `json:"progress"`
	TotalEpisodes int        `json:"totalEpisodes"`
	Score         float64    `json:"score"` // 0.0 to 10.0
	CoverImage    string     `json:"coverImage,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// TrackingUpdateRequest encapsulates user tracking mutation payload.
type TrackingUpdateRequest struct {
	MediaID       int      `json:"mediaId"`
	Status        *string  `json:"status,omitempty"`
	Progress      *int     `json:"progress,omitempty"`
	Score         *float64 `json:"score,omitempty"`
	TotalEpisodes *int     `json:"totalEpisodes,omitempty"`
	Title         *string  `json:"title,omitempty"`
	CoverImage    *string  `json:"coverImage,omitempty"`
	Notes         *string  `json:"notes,omitempty"`
}

// UserLibraryResponse encapsulates grouped or listed library entries.
type UserLibraryResponse struct {
	Lists []*UserLibraryEntry `json:"lists"`
}
