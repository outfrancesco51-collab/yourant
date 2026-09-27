package anilist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yourant/internal/anilist"
)

func TestCache_HitMissAndExpiry(t *testing.T) {
	cache := anilist.NewCache(100 * time.Millisecond)
	defer cache.Close()

	key := "test:item"
	val := "hello-world"

	// 1. Initial miss
	if _, ok := cache.Get(key); ok {
		t.Fatalf("expected cache miss for non-existent key")
	}

	// 2. Set with short TTL
	cache.Set(key, val, 60*time.Millisecond)

	// 3. Immediate hit
	got, ok := cache.Get(key)
	if !ok || got != val {
		t.Fatalf("expected cache hit with %v, got %v", val, got)
	}

	// 4. Sleep past TTL
	time.Sleep(80 * time.Millisecond)
	if _, ok := cache.Get(key); ok {
		t.Fatalf("expected cache miss after expiration")
	}
}

func TestLimiter_RateLimiting(t *testing.T) {
	// Burst of 2, 60 requests/minute = 1 token/sec
	limiter := anilist.NewTokenBucketLimiter(60.0, 2)
	ctx := context.Background()

	// First 2 should be immediate
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("unexpected error on first token: %v", err)
	}
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("unexpected error on second token: %v", err)
	}

	// Third token must wait ~1s
	start := time.Now()
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("unexpected error on third token: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 800*time.Millisecond {
		t.Fatalf("expected rate limiter to wait at least ~800ms, waited %v", elapsed)
	}
}

func TestCatalog_Fallbacks(t *testing.T) {
	// Trending fallback
	trending := anilist.FallbackTrending(1, 5)
	if len(trending.Items) != 5 {
		t.Fatalf("expected 5 trending items, got %d", len(trending.Items))
	}
	if trending.PageInfo.Total != 10 {
		t.Fatalf("expected total 10 items in catalog, got %d", trending.PageInfo.Total)
	}

	// Popular fallback
	popular := anilist.FallbackPopular(1, 5)
	if len(popular.Items) != 5 {
		t.Fatalf("expected 5 popular items, got %d", len(popular.Items))
	}

	// Search fallback
	search := anilist.FallbackSearch("frieren", 1, 5)
	if len(search.Items) < 1 {
		t.Fatalf("expected to find frieren in fallback catalog")
	}
	if search.Items[0].ID != 154587 {
		t.Fatalf("expected frieren ID 154587, got %d", search.Items[0].ID)
	}

	// Detail fallback
	detail, err := anilist.FallbackDetail(16498)
	if err != nil {
		t.Fatalf("expected to find Attack on Titan (16498): %v", err)
	}
	if detail.Title.English != "Attack on Titan" {
		t.Fatalf("expected Attack on Titan, got %s", detail.Title.English)
	}
	if len(detail.Characters.Edges) < 3 {
		t.Fatalf("expected character roster in Attack on Titan detail")
	}

	// Non-existent detail
	_, err = anilist.FallbackDetail(9999999)
	if err == nil {
		t.Fatalf("expected error for non-existent anime ID, got nil")
	}
}

func TestClient_OfflineFallbackOnNetworkFailure(t *testing.T) {
	// Point client to an invalid mock endpoint that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	server.Close() // Immediately closed to simulate offline network failure

	client := anilist.NewClient(anilist.Config{
		Endpoint:     server.URL,
		RateLimitRPM: 600.0, // High rate for fast test
	})
	defer client.Close()

	ctx := context.Background()

	// 1. Trending should fall back to offline catalog gracefully
	trending, err := client.GetTrending(ctx, 1, 10)
	if err != nil {
		t.Fatalf("expected fallback result without error, got: %v", err)
	}
	if len(trending.Items) == 0 {
		t.Fatalf("expected non-empty trending items from offline fallback")
	}

	// 2. Search should fall back to offline catalog gracefully
	search, err := client.Search(ctx, "Cyberpunk", 1, 5)
	if err != nil {
		t.Fatalf("expected fallback search without error, got: %v", err)
	}
	if len(search.Items) == 0 {
		t.Fatalf("expected search to find Cyberpunk in offline fallback")
	}

	// 3. Detail should fall back to offline catalog gracefully
	detail, err := client.GetDetail(ctx, 154587)
	if err != nil {
		t.Fatalf("expected fallback detail without error, got: %v", err)
	}
	if detail.Title.English != "Frieren: Beyond Journey's End" {
		t.Fatalf("expected Frieren, got: %s", detail.Title.English)
	}
}
