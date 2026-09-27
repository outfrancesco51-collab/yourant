package anilist

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	TTLTrending = 1 * time.Hour
	TTLPopular  = 1 * time.Hour
	TTLDetail   = 6 * time.Hour
	TTLSearch   = 15 * time.Minute
)

// TrendingKey generates a cache key for trending queries.
func TrendingKey(page, perPage int) string {
	return fmt.Sprintf("trending:%d:%d", page, perPage)
}

// PopularKey generates a cache key for popular queries.
func PopularKey(page, perPage int) string {
	return fmt.Sprintf("popular:%d:%d", page, perPage)
}

// SearchKey generates a cache key for search queries.
func SearchKey(query string, page, perPage int) string {
	return fmt.Sprintf("search:%s:%d:%d", strings.ToLower(strings.TrimSpace(query)), page, perPage)
}

// DetailKey generates a cache key for detail queries.
func DetailKey(id int) string {
	return fmt.Sprintf("detail:%d", id)
}

type cacheEntry struct {
	value      any
	expiration time.Time
}

// Cache provides a thread-safe in-memory key-value store with time-to-live expiration.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	stopCh  chan struct{}
}

// NewCache instantiates an in-memory TTL cache with an automatic background cleanup worker.
func NewCache(cleanupInterval time.Duration) *Cache {
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	c := &Cache{
		entries: make(map[string]cacheEntry),
		stopCh:  make(chan struct{}),
	}
	go c.startJanitor(cleanupInterval)
	return c
}

// Get retrieves an item from cache if present and unexpired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiration) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	return entry.value, true
}

// Set stores a value with an expiration duration.
func (c *Cache) Set(key string, val any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		value:      val,
		expiration: time.Now().Add(ttl),
	}
}

// Delete removes a specific key.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Flush removes all cached entries.
func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cacheEntry)
}

// Close terminates the background janitor goroutine.
func (c *Cache) Close() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

func (c *Cache) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case now := <-ticker.C:
			c.mu.Lock()
			for k, entry := range c.entries {
				if now.After(entry.expiration) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}
}
