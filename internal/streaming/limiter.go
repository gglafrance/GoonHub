package streaming

import (
	"sync"
	"sync/atomic"
	"time"
)

// StreamLimiter limits concurrent streams globally and per-IP.
// Thread-safe for concurrent access.
type StreamLimiter struct {
	maxGlobal  int
	maxPerIP   int
	globalUsed atomic.Int64

	mu       sync.RWMutex
	ipCounts map[string]*ipEntry

	// Cleanup configuration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupDone     chan struct{}
}

type ipEntry struct {
	count    atomic.Int64
	lastUsed atomic.Int64 // Unix timestamp
}

// StreamStats provides statistics about current stream usage.
type StreamStats struct {
	GlobalCount int64
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
		ipCounts:        make(map[string]*ipEntry),
		cleanupInterval: 5 * time.Minute,
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go sl.cleanupLoop()

	return sl
}

// Acquire attempts to acquire a stream slot for the given IP.
// Returns true if successful, false if limits exceeded.
func (sl *StreamLimiter) Acquire(ip string) bool {
	// Check and increment global counter atomically
	for {
		current := sl.globalUsed.Load()
		if current >= int64(sl.maxGlobal) {
			return false
		}
		if sl.globalUsed.CompareAndSwap(current, current+1) {
			break
		}
	}

	// Get or create IP entry
	sl.mu.Lock()
	entry, exists := sl.ipCounts[ip]
	if !exists {
		entry = &ipEntry{}
		sl.ipCounts[ip] = entry
	}
	sl.mu.Unlock()

	// Check and increment per-IP counter atomically
	for {
		current := entry.count.Load()
		if current >= int64(sl.maxPerIP) {
			// Rollback global increment
			sl.globalUsed.Add(-1)
			return false
		}
		if entry.count.CompareAndSwap(current, current+1) {
			break
		}
	}

	// Update last used timestamp
	entry.lastUsed.Store(time.Now().Unix())

	return true
}

// Release releases a stream slot for the given IP.
// Should be called when a stream ends (typically via defer).
func (sl *StreamLimiter) Release(ip string) {
	sl.globalUsed.Add(-1)

	sl.mu.RLock()
	entry, exists := sl.ipCounts[ip]
	sl.mu.RUnlock()

	if exists {
		entry.count.Add(-1)
		entry.lastUsed.Store(time.Now().Unix())
	}
}

// Stats returns current stream statistics.
func (sl *StreamLimiter) Stats() StreamStats {
	sl.mu.RLock()
	activeIPs := 0
	for _, entry := range sl.ipCounts {
		if entry.count.Load() > 0 {
			activeIPs++
		}
	}
	sl.mu.RUnlock()

	return StreamStats{
		GlobalCount: sl.globalUsed.Load(),
		MaxGlobal:   sl.maxGlobal,
		MaxPerIP:    sl.maxPerIP,
		ActiveIPs:   activeIPs,
	}
}

// Stop stops the background cleanup goroutine.
func (sl *StreamLimiter) Stop() {
	close(sl.stopCleanup)
	<-sl.cleanupDone
}

// cleanupLoop periodically removes stale IP entries to prevent memory leaks.
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

// cleanup removes IP entries that have been inactive for longer than cleanupInterval
// and have zero active streams.
func (sl *StreamLimiter) cleanup() {
	cutoff := time.Now().Add(-sl.cleanupInterval).Unix()

	sl.mu.Lock()
	defer sl.mu.Unlock()

	for ip, entry := range sl.ipCounts {
		if entry.count.Load() == 0 && entry.lastUsed.Load() < cutoff {
			delete(sl.ipCounts, ip)
		}
	}
}

// GlobalCount returns the current number of active global streams.
func (sl *StreamLimiter) GlobalCount() int64 {
	return sl.globalUsed.Load()
}

// IPCount returns the current number of active streams for a given IP.
func (sl *StreamLimiter) IPCount(ip string) int64 {
	sl.mu.RLock()
	entry, exists := sl.ipCounts[ip]
	sl.mu.RUnlock()

	if !exists {
		return 0
	}
	return entry.count.Load()
}
