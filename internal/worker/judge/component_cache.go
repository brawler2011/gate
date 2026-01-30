package judge

import (
	"sync"
	"time"

	"github.com/gate149/core/pkg/sandbox"
)

// CacheEntry represents a cached component
type CacheEntry struct {
	FileID     string
	CachedAt   time.Time
	AccessedAt time.Time
}

// ComponentCache caches compiled problem components
type ComponentCache struct {
	cache         map[string]*CacheEntry
	mu            sync.RWMutex
	sandboxClient *sandbox.Client
	maxSize       int
	ttl           time.Duration
}

// NewComponentCache creates a new component cache
func NewComponentCache(sandboxClient *sandbox.Client) *ComponentCache {
	cache := &ComponentCache{
		cache:         make(map[string]*CacheEntry),
		sandboxClient: sandboxClient,
		maxSize:       1000, // max 1000 cached components
		ttl:           24 * time.Hour, // cache for 24 hours
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached component
func (c *ComponentCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return "", false
	}

	// Check if entry is expired
	if time.Since(entry.CachedAt) > c.ttl {
		return "", false
	}

	// Update access time
	entry.AccessedAt = time.Now()

	return entry.FileID, true
}

// Set stores a compiled component in cache
func (c *ComponentCache) Set(key string, fileID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if cache is full
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	now := time.Now()
	c.cache[key] = &CacheEntry{
		FileID:     fileID,
		CachedAt:   now,
		AccessedAt: now,
	}
}

// evictOldest removes the least recently used entry (must be called with lock held)
func (c *ComponentCache) evictOldest() {
	if len(c.cache) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.cache {
		if first || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
			first = false
		}
	}

	delete(c.cache, oldestKey)
}

// cleanupLoop periodically removes expired entries
func (c *ComponentCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *ComponentCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.Sub(entry.CachedAt) > c.ttl {
			delete(c.cache, key)
		}
	}
}

// Clear removes all cached entries
func (c *ComponentCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)
}

// Size returns the number of cached entries
func (c *ComponentCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// Stats returns cache statistics
func (c *ComponentCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"size":     len(c.cache),
		"max_size": c.maxSize,
		"ttl":      c.ttl.String(),
	}
}
