// Package cache provides in-memory caching for video metadata.
package cache

import (
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/domain"
	gocache "github.com/patrickmn/go-cache"
)

// VideoCache caches video metadata to avoid repeated yt-dlp calls.
type VideoCache struct {
	cache *gocache.Cache
}

// NewVideoCache creates a new VideoCache with the given TTL and cleanup interval.
func NewVideoCache(ttl, cleanupInterval time.Duration) *VideoCache {
	return &VideoCache{
		cache: gocache.New(ttl, cleanupInterval),
	}
}

// DefaultVideoCache creates a VideoCache with default settings.
// TTL: 1 hour, Cleanup: 10 minutes
func DefaultVideoCache() *VideoCache {
	return NewVideoCache(time.Hour, 10*time.Minute)
}

// Get retrieves video info from cache.
func (c *VideoCache) Get(url string) (*domain.VideoInfo, bool) {
	if item, found := c.cache.Get(url); found {
		if info, ok := item.(*domain.VideoInfo); ok {
			return info, true
		}
	}
	return nil, false
}

// Set stores video info in cache.
func (c *VideoCache) Set(url string, info *domain.VideoInfo) {
	c.cache.Set(url, info, gocache.DefaultExpiration)
}

// SetWithTTL stores video info in cache with a custom TTL.
func (c *VideoCache) SetWithTTL(url string, info *domain.VideoInfo, ttl time.Duration) {
	c.cache.Set(url, info, ttl)
}

// Delete removes video info from cache.
func (c *VideoCache) Delete(url string) {
	c.cache.Delete(url)
}

// Flush removes all items from cache.
func (c *VideoCache) Flush() {
	c.cache.Flush()
}

// ItemCount returns the number of items in cache.
func (c *VideoCache) ItemCount() int {
	return c.cache.ItemCount()
}
