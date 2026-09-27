package anilist

import (
	"testing"
)

func TestSeedCatalog_Count(t *testing.T) {
	if len(SeedCatalog) != 10 {
		t.Fatalf("expected 10 seed catalog items, got %d", len(SeedCatalog))
	}
}

func TestFallbackTrending(t *testing.T) {
	res := FallbackTrending(1, 5)
	if len(res.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(res.Items))
	}
	if res.Items[0].Trending < res.Items[1].Trending {
		t.Errorf("expected trending descending order")
	}
}

func TestFallbackPopular(t *testing.T) {
	res := FallbackPopular(1, 5)
	if len(res.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(res.Items))
	}
	if res.Items[0].Popularity < res.Items[1].Popularity {
		t.Errorf("expected popularity descending order")
	}
}

func TestFallbackSearch(t *testing.T) {
	res := FallbackSearch("frieren", 1, 10)
	if len(res.Items) == 0 {
		t.Fatalf("expected at least 1 item matching 'frieren'")
	}
	if res.Items[0].ID != 154587 {
		t.Errorf("expected ID 154587, got %d", res.Items[0].ID)
	}

	genreRes := FallbackSearch("Sci-Fi", 1, 10)
	if len(genreRes.Items) == 0 {
		t.Fatalf("expected at least 1 item matching 'Sci-Fi'")
	}
}

func TestFallbackDetail(t *testing.T) {
	media, err := FallbackDetail(154587)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if media.Title.English != "Frieren: Beyond Journey's End" {
		t.Errorf("unexpected title: %s", media.Title.English)
	}

	_, errNotFound := FallbackDetail(9999999)
	if errNotFound != ErrAnimeNotFound {
		t.Errorf("expected ErrAnimeNotFound, got %v", errNotFound)
	}
}
