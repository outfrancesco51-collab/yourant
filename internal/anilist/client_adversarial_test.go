package anilist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yourant/internal/anilist"
)

// TestAdv_AniList_HTTPErrorResponses verifies fallback behavior when AniList returns
// HTTP 429 (Rate Limited), 500 (Internal Error), 502 (Bad Gateway), and 503 (Unavailable).
func TestAdv_AniList_HTTPErrorResponses(t *testing.T) {
	statusCodes := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(code), code)
			}))
			defer server.Close()

			client := anilist.NewClient(anilist.Config{
				Endpoint:     server.URL,
				RateLimitRPM: 1000.0,
			})
			defer client.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Trending must succeed via fallback
			trending, err := client.GetTrending(ctx, 1, 10)
			if err != nil {
				t.Fatalf("expected fallback trending on %d, got err: %v", code, err)
			}
			if len(trending.Items) == 0 {
				t.Fatalf("expected non-empty trending on %d", code)
			}

			// Popular must succeed via fallback
			popular, err := client.GetPopular(ctx, 1, 10)
			if err != nil {
				t.Fatalf("expected fallback popular on %d, got err: %v", code, err)
			}
			if len(popular.Items) == 0 {
				t.Fatalf("expected non-empty popular on %d", code)
			}

			// Search must succeed via fallback
			search, err := client.Search(ctx, "titan", 1, 5)
			if err != nil {
				t.Fatalf("expected fallback search on %d, got err: %v", code, err)
			}
			if len(search.Items) == 0 {
				t.Fatalf("expected search result on %d", code)
			}

			// Detail for seed anime must succeed via fallback
			detail, err := client.GetDetail(ctx, 154587)
			if err != nil {
				t.Fatalf("expected fallback detail on %d, got err: %v", code, err)
			}
			if detail.ID != 154587 {
				t.Fatalf("expected Frieren detail on %d", code)
			}
		})
	}
}

// TestAdv_AniList_SearchQueryEdgeCases verifies that search query edge cases do not crash
// or return errors.
func TestAdv_AniList_SearchQueryEdgeCases(t *testing.T) {
	// Point to dead server to ensure offline fallback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusInternalServerError)
	}))
	server.Close()

	client := anilist.NewClient(anilist.Config{
		Endpoint:     server.URL,
		RateLimitRPM: 1000.0,
	})
	defer client.Close()

	ctx := context.Background()

	testQueries := []struct {
		name        string
		query       string
		expectMatch bool
	}{
		{"empty_query", "", true},
		{"whitespace_only", "   ", true},
		{"romaji_match", "Sousou no Frieren", true},
		{"english_match", "Attack on Titan", true},
		{"native_japanese_match", "呪術廻戦", true},
		{"genre_match", "Fantasy", true},
		{"special_characters", `!@#$%^&*()`, false},
		{"non_matching_string", "unobtainable_media_xyz_999", false},
		{"partial_case_insensitive", "edGeRuNnEr", true},
	}

	for _, tc := range testQueries {
		t.Run(tc.name, func(t *testing.T) {
			res, err := client.Search(ctx, tc.query, 1, 10)
			if err != nil {
				t.Fatalf("Search failed with error: %v", err)
			}
			if res == nil {
				t.Fatalf("Search returned nil result")
			}
			if tc.expectMatch && len(res.Items) == 0 {
				t.Fatalf("expected matches for query %q, got 0", tc.query)
			}
			if !tc.expectMatch && len(res.Items) != 0 {
				t.Fatalf("expected 0 matches for query %q, got %d", tc.query, len(res.Items))
			}
		})
	}
}

// TestAdv_AniList_PaginationBoundaryScenarios tests bounds: page 0, negative pages,
// and pages way beyond catalog size.
func TestAdv_AniList_PaginationBoundaryScenarios(t *testing.T) {
	// 1. Page 0 and negative perPage should normalize
	p0 := anilist.FallbackTrending(0, -5)
	if p0.PageInfo.CurrentPage != 1 || p0.PageInfo.PerPage != 20 {
		t.Errorf("expected normalized page=1, perPage=20, got page=%d, perPage=%d",
			p0.PageInfo.CurrentPage, p0.PageInfo.PerPage)
	}

	// 2. High page beyond catalog (SeedCatalog has 10 items)
	pHigh := anilist.FallbackPopular(999, 10)
	if len(pHigh.Items) != 0 {
		t.Errorf("expected 0 items on page 999, got %d", len(pHigh.Items))
	}
	if pHigh.PageInfo.HasNextPage {
		t.Errorf("HasNextPage should be false for out of bounds page")
	}

	// 3. Exact last page boundary
	p1 := anilist.FallbackTrending(1, 5)
	if len(p1.Items) != 5 || !p1.PageInfo.HasNextPage {
		t.Errorf("page 1 (perPage 5) should have 5 items and HasNextPage=true")
	}

	p2 := anilist.FallbackTrending(2, 5)
	if len(p2.Items) != 5 || p2.PageInfo.HasNextPage {
		t.Errorf("page 2 (perPage 5) should have 5 items and HasNextPage=false")
	}
}

// TestAdv_AniList_SeedCatalogMetadataIntegrity checks that all 10 seed catalog items
// contain required rich fields (Title, CoverImage, Studios, Score, Description).
func TestAdv_AniList_SeedCatalogMetadataIntegrity(t *testing.T) {
	if len(anilist.SeedCatalog) < 10 {
		t.Fatalf("expected at least 10 seed catalog entries, found %d", len(anilist.SeedCatalog))
	}

	for _, anime := range anilist.SeedCatalog {
		if anime.ID <= 0 {
			t.Errorf("invalid ID: %d", anime.ID)
		}
		if anime.Title.Romaji == "" && anime.Title.English == "" {
			t.Errorf("anime %d missing both Romaji and English titles", anime.ID)
		}
		if anime.CoverImage.ExtraLarge == "" && anime.CoverImage.Large == "" {
			t.Errorf("anime %d missing cover image", anime.ID)
		}
		if anime.Description == "" {
			t.Errorf("anime %d missing description", anime.ID)
		}
		if len(anime.Genres) == 0 {
			t.Errorf("anime %d has no genres", anime.ID)
		}
		if anime.AverageScore <= 0 {
			t.Errorf("anime %d missing average score", anime.ID)
		}
	}
}
