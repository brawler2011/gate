package judge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComponentCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := &ComponentCache{cache: make(map[string]*CacheEntry), maxSize: 2, ttl: time.Hour}
	cache.Set("checker", "checker-file")
	cache.Set("validator", "validator-file")
	cache.cache["checker"].AccessedAt = time.Now().Add(-2 * time.Minute)
	cache.cache["validator"].AccessedAt = time.Now().Add(-time.Minute)
	file, found := cache.Get("checker")
	require.True(t, found)
	require.Equal(t, "checker-file", file)
	cache.Set("generator", "generator-file")
	require.Equal(t, 2, cache.Size())
	_, found = cache.Get("validator")
	require.False(t, found)
	file, found = cache.Get("checker")
	require.True(t, found)
	require.Equal(t, "checker-file", file)
	file, found = cache.Get("generator")
	require.True(t, found)
	require.Equal(t, "generator-file", file)
	cache.Clear()
	require.Zero(t, cache.Size())
	_, found = cache.Get("generator")
	require.False(t, found)
}

func TestComponentCacheRejectsAndRemovesExpiredEntries(t *testing.T) {
	cache := &ComponentCache{cache: make(map[string]*CacheEntry), maxSize: 2, ttl: time.Hour}
	cache.Set("expired", "old-file")
	cache.Set("current", "new-file")
	cache.cache["expired"].CachedAt = time.Now().Add(-2 * time.Hour)
	_, found := cache.Get("expired")
	require.False(t, found)
	cache.cleanup()
	require.Equal(t, 1, cache.Size())
	file, found := cache.Get("current")
	require.True(t, found)
	require.Equal(t, "new-file", file)
}
