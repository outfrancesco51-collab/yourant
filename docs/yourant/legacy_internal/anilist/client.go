package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

const DefaultAniListEndpoint = "https://graphql.anilist.co"

// Client defines the operations supported by the AniList subsystem.
type Client interface {
	GetTrending(ctx context.Context, page, perPage int) (*PageResult, error)
	GetPopular(ctx context.Context, page, perPage int) (*PageResult, error)
	Search(ctx context.Context, query string, page, perPage int) (*PageResult, error)
	GetDetail(ctx context.Context, id int) (*AnimeMedia, error)
	Close()
}

// Config configures the AniList client.
type Config struct {
	Endpoint     string
	RateLimitRPM float64
	CacheTTL     time.Duration
	HTTPClient   *http.Client
}

// DefaultClient implements Client with TTL caching, token bucket rate limiting, and fallback catalog.
type DefaultClient struct {
	endpoint   string
	cache      *Cache
	limiter    *TokenBucketLimiter
	httpClient *http.Client
}

// NewClient creates a new AniList client with caching and rate limiting.
func NewClient(cfg Config) *DefaultClient {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultAniListEndpoint
	}

	rateLimit := cfg.RateLimitRPM
	if rateLimit <= 0 {
		rateLimit = 80.0 // Strict 80 req/min
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &DefaultClient{
		endpoint:   endpoint,
		cache:      NewCache(5 * time.Minute),
		limiter:    NewTokenBucketLimiter(rateLimit, 10),
		httpClient: client,
	}
}

// Close releases background resources.
func (c *DefaultClient) Close() {
	if c.cache != nil {
		c.cache.Close()
	}
}

// GetTrending returns currently trending anime with caching and fallback.
func (c *DefaultClient) GetTrending(ctx context.Context, page, perPage int) (*PageResult, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	key := TrendingKey(page, perPage)
	if val, ok := c.cache.Get(key); ok {
		if res, ok := val.(*PageResult); ok {
			return res, nil
		}
	}

	var resp graphQLResponse
	err := c.executeGraphQL(ctx, TrendingQuery, map[string]any{
		"page":    page,
		"perPage": perPage,
	}, &resp)

	if err != nil || len(resp.Data.Page.Media) == 0 {
		log.Printf("[ANILIST] GetTrending network fetch failed (%v), using offline catalog\n", err)
		fallback := FallbackTrending(page, perPage)
		return fallback, nil
	}

	res := &PageResult{
		PageInfo: resp.Data.Page.PageInfo,
		Items:    resp.Data.Page.Media,
	}

	c.cache.Set(key, res, TTLTrending)
	return res, nil
}

// GetPopular returns popular anime with caching and fallback.
func (c *DefaultClient) GetPopular(ctx context.Context, page, perPage int) (*PageResult, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	key := PopularKey(page, perPage)
	if val, ok := c.cache.Get(key); ok {
		if res, ok := val.(*PageResult); ok {
			return res, nil
		}
	}

	var resp graphQLResponse
	err := c.executeGraphQL(ctx, PopularQuery, map[string]any{
		"page":    page,
		"perPage": perPage,
	}, &resp)

	if err != nil || len(resp.Data.Page.Media) == 0 {
		log.Printf("[ANILIST] GetPopular network fetch failed (%v), using offline catalog\n", err)
		fallback := FallbackPopular(page, perPage)
		return fallback, nil
	}

	res := &PageResult{
		PageInfo: resp.Data.Page.PageInfo,
		Items:    resp.Data.Page.Media,
	}

	c.cache.Set(key, res, TTLPopular)
	return res, nil
}

// Search searches anime by query string with caching and fallback.
func (c *DefaultClient) Search(ctx context.Context, query string, page, perPage int) (*PageResult, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	key := SearchKey(query, page, perPage)
	if val, ok := c.cache.Get(key); ok {
		if res, ok := val.(*PageResult); ok {
			return res, nil
		}
	}

	var resp graphQLResponse
	err := c.executeGraphQL(ctx, SearchQuery, map[string]any{
		"search":  query,
		"page":    page,
		"perPage": perPage,
	}, &resp)

	if err != nil || len(resp.Data.Page.Media) == 0 {
		log.Printf("[ANILIST] Search network fetch failed (%v), using offline catalog\n", err)
		fallback := FallbackSearch(query, page, perPage)
		return fallback, nil
	}

	res := &PageResult{
		PageInfo: resp.Data.Page.PageInfo,
		Items:    resp.Data.Page.Media,
	}

	c.cache.Set(key, res, TTLSearch)
	return res, nil
}

// GetDetail retrieves full media detail by ID with caching and fallback.
func (c *DefaultClient) GetDetail(ctx context.Context, id int) (*AnimeMedia, error) {
	if id <= 0 {
		return nil, errors.New("invalid media id")
	}

	key := DetailKey(id)
	if val, ok := c.cache.Get(key); ok {
		if media, ok := val.(*AnimeMedia); ok {
			return media, nil
		}
	}

	var resp graphQLResponse
	err := c.executeGraphQL(ctx, DetailQuery, map[string]any{
		"id": id,
	}, &resp)

	if err != nil || resp.Data.Media == nil {
		log.Printf("[ANILIST] GetDetail network fetch failed (%v), using offline catalog\n", err)
		fallback, fbErr := FallbackDetail(id)
		if fbErr == nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("failed to fetch anime detail: %w", err)
	}

	c.cache.Set(key, resp.Data.Media, TTLDetail)
	return resp.Data.Media, nil
}

func (c *DefaultClient) executeGraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	// Enforce 80 req/min rate limit
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	reqBody, err := json.Marshal(graphQLRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("marshal request error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("anilist api returned http %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response error: %w", err)
	}

	return nil
}
