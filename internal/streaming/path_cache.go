package streaming

import (
	"sync"
	"time"
)

// PathCache provides TTL-based caching for scene file paths.
// Reduces database queries for frequently accessed scenes during streaming.
type PathCache struct {
	mu      sync.RWMutex
	entries map[uint]*pathEntry
	ttl     time.Duration
	maxSize int

	// Cleanup configuration
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

type pathEntry struct {
	path      string
	expiresAt time.Time
}

// NewPathCache creates a new path cache with the specified TTL and max size.
func NewPathCache(ttl time.Duration, maxSize int) *PathCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if maxSize <= 0 {
		maxSize = 10000
	}

	pc := &PathCache{
		entries:     make(map[uint]*pathEntry),
		ttl:         ttl,
		maxSize:     maxSize,
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go pc.cleanupLoop()

	return pc
}

// Get retrieves a cached path for the given scene ID.
// Returns the path and true if found and not expired, empty string and false otherwise.
func (pc *PathCache) Get(sceneID uint) (string, bool) {
	pc.mu.RLock()
	entry, exists := pc.entries[sceneID]
	pc.mu.RUnlock()

	if !exists {
		return "", false
	}

	if time.Now().After(entry.expiresAt) {
		// Entry expired, remove it
		pc.mu.Lock()
		// Double-check under write lock
		if e, ok := pc.entries[sceneID]; ok && time.Now().After(e.expiresAt) {
			delete(pc.entries, sceneID)
		}
		pc.mu.Unlock()
		return "", false
	}

	return entry.path, true
}

// Set stores a path in the cache for the given scene ID.
func (pc *PathCache) Set(sceneID uint, path string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Check if we need to evict entries
	if len(pc.entries) >= pc.maxSize {
		pc.evictExpired()
		// If still at capacity after evicting expired, evict oldest entries
		if len(pc.entries) >= pc.maxSize {
			pc.evictOldest(pc.maxSize / 10) // Evict 10% of entries
		}
	}

	pc.entries[sceneID] = &pathEntry{
		path:      path,
		expiresAt: time.Now().Add(pc.ttl),
	}
}

// Invalidate removes a specific scene from the cache.
// Use this when a scene's stored path changes.
func (pc *PathCache) Invalidate(sceneID uint) {
	pc.mu.Lock()
	delete(pc.entries, sceneID)
	pc.mu.Unlock()
}

// Clear removes all entries from the cache.
func (pc *PathCache) Clear() {
	pc.mu.Lock()
	pc.entries = make(map[uint]*pathEntry)
	pc.mu.Unlock()
}

// Size returns the current number of entries in the cache.
func (pc *PathCache) Size() int {
	pc.mu.RLock()
	size := len(pc.entries)
	pc.mu.RUnlock()
	return size
}

// Stop stops the background cleanup goroutine.
func (pc *PathCache) Stop() {
	close(pc.stopCleanup)
	<-pc.cleanupDone
}

// cleanupLoop periodically removes expired entries.
func (pc *PathCache) cleanupLoop() {
	defer close(pc.cleanupDone)

	// Run cleanup every TTL/2
	ticker := time.NewTicker(pc.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-pc.stopCleanup:
			return
		case <-ticker.C:
			pc.mu.Lock()
			pc.evictExpired()
			pc.mu.Unlock()
		}
	}
}

// evictExpired removes all expired entries. Must be called with mu held.
func (pc *PathCache) evictExpired() {
	now := time.Now()
	for id, entry := range pc.entries {
		if now.After(entry.expiresAt) {
			delete(pc.entries, id)
		}
	}
}

// evictOldest removes the n oldest entries. Must be called with mu held.
func (pc *PathCache) evictOldest(n int) {
	if n <= 0 || len(pc.entries) == 0 {
		return
	}

	// Simple eviction: just remove first n entries encountered
	// This is O(n) and doesn't require tracking insertion order
	count := 0
	for id := range pc.entries {
		if count >= n {
			break
		}
		delete(pc.entries, id)
		count++
	}
}
