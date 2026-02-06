package streaming

import (
	"sync"
	"time"
)

// StreamLimiter limits concurrent streams globally and per-IP.
// It tracks streams by IP+SceneID so that multiple concurrent HTTP range requests
// for the same video from the same client count as a single logical stream.
// Thread-safe for concurrent access.
type StreamLimiter struct {
	maxGlobal  int
	maxPerIP   int

	mu          sync.Mutex
	streams     map[streamKey]*streamEntry
	ipCounts    map[string]int
	globalCount int

	// Cleanup configuration
	staleTimeout    time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupDone     chan struct{}
}

// streamKey uniquely identifies a logical stream (one viewer watching one scene).
type streamKey struct {
	ip      string
	sceneID uint
}

// streamEntry tracks the number of concurrent HTTP requests for a single logical stream
// and when it was last active (for stale entry cleanup).
type streamEntry struct {
	refCount int
	lastSeen time.Time
}

// StreamStats provides statistics about current stream usage.
type StreamStats struct {
	GlobalCount int
	MaxGlobal   int
	MaxPerIP    int
	ActiveIPs   int
}

// NewStreamLimiter creates a new stream limiter with the specified limits.
func NewStreamLimiter(maxGlobal, maxPerIP int) *StreamLimiter {
	if maxGlobal <= 0 {
		maxGlobal = 100
	}
	if maxPerIP <= 0 {
		maxPerIP = 10
	}

	sl := &StreamLimiter{
		maxGlobal:       maxGlobal,
		maxPerIP:        maxPerIP,
		streams:         make(map[streamKey]*streamEntry),
		ipCounts:        make(map[string]int),
		staleTimeout:    5 * time.Minute,
		cleanupInterval: 1 * time.Minute,
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}

	go sl.cleanupLoop()

	return sl
}

// Acquire attempts to acquire a stream slot for the given IP and scene.
// Multiple concurrent requests for the same IP+scene pair share a single slot.
// Returns true if successful, false if limits are exceeded.
func (sl *StreamLimiter) Acquire(ip string, sceneID uint) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	key := streamKey{ip: ip, sceneID: sceneID}

	if entry, exists := sl.streams[key]; exists {
		// Same scene from same IP — piggyback on existing slot
		entry.refCount++
		entry.lastSeen = time.Now()
		return true
	}

	// New logical stream — check limits
	if sl.globalCount >= sl.maxGlobal {
		return false
	}
	if sl.ipCounts[ip] >= sl.maxPerIP {
		return false
	}

	// Acquire new slot
	sl.globalCount++
	sl.ipCounts[ip]++
	sl.streams[key] = &streamEntry{refCount: 1, lastSeen: time.Now()}
	return true
}

// Release releases a stream slot for the given IP and scene.
// The slot is only freed when all concurrent requests for this IP+scene pair have released.
func (sl *StreamLimiter) Release(ip string, sceneID uint) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	key := streamKey{ip: ip, sceneID: sceneID}
	entry, exists := sl.streams[key]
	if !exists {
		return
	}

	entry.refCount--
	if entry.refCount <= 0 {
		delete(sl.streams, key)
		sl.globalCount--
		sl.ipCounts[ip]--
		if sl.ipCounts[ip] <= 0 {
			delete(sl.ipCounts, ip)
		}
	}
}

// Stats returns current stream statistics.
func (sl *StreamLimiter) Stats() StreamStats {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	return StreamStats{
		GlobalCount: sl.globalCount,
		MaxGlobal:   sl.maxGlobal,
		MaxPerIP:    sl.maxPerIP,
		ActiveIPs:   len(sl.ipCounts),
	}
}

// GlobalCount returns the current number of active global streams.
func (sl *StreamLimiter) GlobalCount() int {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.globalCount
}

// IPCount returns the current number of active streams for a given IP.
func (sl *StreamLimiter) IPCount(ip string) int {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.ipCounts[ip]
}

// Stop stops the background cleanup goroutine.
func (sl *StreamLimiter) Stop() {
	close(sl.stopCleanup)
	<-sl.cleanupDone
}

// cleanupLoop periodically removes stale stream entries to prevent leaks
// from requests that never called Release (e.g., panics, dropped connections).
func (sl *StreamLimiter) cleanupLoop() {
	defer close(sl.cleanupDone)

	ticker := time.NewTicker(sl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sl.stopCleanup:
			return
		case <-ticker.C:
			sl.cleanup()
		}
	}
}

// cleanup removes stream entries that have been inactive for longer than staleTimeout.
// This handles leaked slots from requests that crashed before calling Release.
func (sl *StreamLimiter) cleanup() {
	cutoff := time.Now().Add(-sl.staleTimeout)

	sl.mu.Lock()
	defer sl.mu.Unlock()

	for key, entry := range sl.streams {
		if entry.lastSeen.Before(cutoff) {
			delete(sl.streams, key)
			sl.globalCount--
			sl.ipCounts[key.ip]--
			if sl.ipCounts[key.ip] <= 0 {
				delete(sl.ipCounts, key.ip)
			}
		}
	}
}
