package anilist

// AnimeTitle encapsulates multi-language titles.
type AnimeTitle struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
	Native  string `json:"native"`
}

// AnimeCoverImage contains cover image variations and average color.
type AnimeCoverImage struct {
	ExtraLarge string `json:"extraLarge"`
	Large      string `json:"large"`
	Medium     string `json:"medium,omitempty"`
	Color      string `json:"color,omitempty"`
}

// AnimeDate represents year/month/day dates.
type AnimeDate struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month,omitempty"`
	Day   int `json:"day,omitempty"`
}

// CharacterNode represents a character in the cast.
type CharacterNode struct {
	ID   int `json:"id"`
	Name struct {
		Full   string `json:"full"`
		Native string `json:"native,omitempty"`
	} `json:"name"`
	Image struct {
		Large  string `json:"large,omitempty"`
		Medium string `json:"medium,omitempty"`
	} `json:"image,omitempty"`
}

// CharacterEdge connects role to character.
type CharacterEdge struct {
	Role string        `json:"role"` // MAIN, SUPPORTING
	Node CharacterNode `json:"node"`
}

// CharacterConnection holds characters edges.
type CharacterConnection struct {
	Edges []CharacterEdge `json:"edges"`
}

// RelationNode represents related anime/manga media.
type RelationNode struct {
	ID           int             `json:"id"`
	Title        AnimeTitle      `json:"title"`
	Type         string          `json:"type,omitempty"`   // ANIME, MANGA
	Format       string          `json:"format,omitempty"` // TV, MOVIE, OVA, SPECIAL
	Status       string          `json:"status,omitempty"` // FINISHED, RELEASING
	CoverImage   AnimeCoverImage `json:"coverImage,omitempty"`
	AverageScore int             `json:"averageScore,omitempty"`
}

// RelationEdge connects relation type to target media.
type RelationEdge struct {
	RelationType string       `json:"relationType"` // PREQUEL, SEQUEL, SIDE_STORY, SPIN_OFF
	Node         RelationNode `json:"node"`
}

// RelationConnection holds media relation edges.
type RelationConnection struct {
	Edges []RelationEdge `json:"edges"`
}

// StudioNode represents producing animation studios.
type StudioNode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// StudioEdge connects studio with main status.
type StudioEdge struct {
	IsMain bool       `json:"isMain"`
	Node   StudioNode `json:"node"`
}

// StudioConnection holds studio edges.
type StudioConnection struct {
	Edges []StudioEdge `json:"edges"`
}

// AiringEpisode describes the next upcoming broadcast episode.
type AiringEpisode struct {
	AiringAt        int64 `json:"airingAt"`
	TimeUntilAiring int64 `json:"timeUntilAiring"`
	Episode         int   `json:"episode"`
}

// AnimeMedia represents the comprehensive anime media object.
type AnimeMedia struct {
	ID                int                  `json:"id"`
	IDMal             *int                 `json:"idMal,omitempty"`
	Title             AnimeTitle           `json:"title"`
	Format            string               `json:"format"` // TV, MOVIE, OVA, SPECIAL, ONA
	Status            string               `json:"status"` // FINISHED, RELEASING, NOT_YET_RELEASED
	Description       string               `json:"description,omitempty"`
	Season            string               `json:"season,omitempty"` // WINTER, SPRING, SUMMER, FALL
	SeasonYear        int                  `json:"seasonYear,omitempty"`
	Episodes          int                  `json:"episodes,omitempty"`
	Duration          int                  `json:"duration,omitempty"` // Runtime in minutes
	CoverImage        AnimeCoverImage      `json:"coverImage"`
	BannerImage       string               `json:"bannerImage,omitempty"`
	Genres            []string             `json:"genres"`
	AverageScore      int                  `json:"averageScore"` // Score out of 100
	MeanScore         int                  `json:"meanScore,omitempty"`
	Popularity        int                  `json:"popularity"`
	Trending          int                  `json:"trending"`
	Favourites        int                  `json:"favourites,omitempty"`
	StartDate         *AnimeDate           `json:"startDate,omitempty"`
	EndDate           *AnimeDate           `json:"endDate,omitempty"`
	Studios           *StudioConnection    `json:"studios,omitempty"`
	Characters        *CharacterConnection `json:"characters,omitempty"`
	Relations         *RelationConnection  `json:"relations,omitempty"`
	NextAiringEpisode *AiringEpisode       `json:"nextAiringEpisode,omitempty"`
}

// PageInfo holds GraphQL pagination metadata.
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
