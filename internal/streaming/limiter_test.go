package streaming

import (
	"sync"
	"testing"
	"time"
)

func TestNewStreamLimiter(t *testing.T) {
	tests := []struct {
		name       string
		maxGlobal  int
		maxPerIP   int
		wantGlobal int
		wantPerIP  int
	}{
		{"defaults for zero", 0, 0, 100, 10},
		{"defaults for negative", -1, -1, 100, 10},
		{"custom values", 50, 5, 50, 5},
		{"large values", 1000, 100, 1000, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := NewStreamLimiter(tt.maxGlobal, tt.maxPerIP)
			defer sl.Stop()

			stats := sl.Stats()
			if stats.MaxGlobal != tt.wantGlobal {
				t.Fatalf("expected MaxGlobal %d, got %d", tt.wantGlobal, stats.MaxGlobal)
			}
			if stats.MaxPerIP != tt.wantPerIP {
				t.Fatalf("expected MaxPerIP %d, got %d", tt.wantPerIP, stats.MaxPerIP)
			}
		})
	}
}

func TestStreamLimiterAcquireRelease(t *testing.T) {
	sl := NewStreamLimiter(10, 3)
	defer sl.Stop()

	ip := "192.168.1.1"
	var sceneID uint = 1

	if !sl.Acquire(ip, sceneID) {
		t.Fatal("expected Acquire to succeed")
	}

	if sl.GlobalCount() != 1 {
		t.Fatalf("expected global count 1, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 1 {
		t.Fatalf("expected IP count 1, got %d", sl.IPCount(ip))
	}

	sl.Release(ip, sceneID)

	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 0 {
		t.Fatalf("expected IP count 0, got %d", sl.IPCount(ip))
	}
}

func TestStreamLimiterRefCounting(t *testing.T) {
	sl := NewStreamLimiter(100, 10)
	defer sl.Stop()

	ip := "192.168.1.1"
	var sceneID uint = 42

	// Multiple concurrent requests for the same IP+scene should share one slot
	if !sl.Acquire(ip, sceneID) {
		t.Fatal("expected first Acquire to succeed")
	}
	if !sl.Acquire(ip, sceneID) {
		t.Fatal("expected second Acquire (same scene) to succeed")
	}
	if !sl.Acquire(ip, sceneID) {
		t.Fatal("expected third Acquire (same scene) to succeed")
	}

	// Should still only count as 1 global stream and 1 per-IP stream
	if sl.GlobalCount() != 1 {
		t.Fatalf("expected global count 1, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 1 {
		t.Fatalf("expected IP count 1, got %d", sl.IPCount(ip))
	}

	// Release two of the three refs — slot should still be held
	sl.Release(ip, sceneID)
	sl.Release(ip, sceneID)

	if sl.GlobalCount() != 1 {
		t.Fatalf("expected global count 1 after partial release, got %d", sl.GlobalCount())
	}

	// Release the last ref — slot should be freed
	sl.Release(ip, sceneID)

	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0 after full release, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 0 {
		t.Fatalf("expected IP count 0 after full release, got %d", sl.IPCount(ip))
	}
}

func TestStreamLimiterGlobalLimit(t *testing.T) {
	sl := NewStreamLimiter(3, 10)
	defer sl.Stop()

	// Acquire up to limit with different IPs and scenes
	if !sl.Acquire("192.168.1.1", 1) {
		t.Fatal("expected Acquire 1 to succeed")
	}
	if !sl.Acquire("192.168.1.2", 2) {
		t.Fatal("expected Acquire 2 to succeed")
	}
	if !sl.Acquire("192.168.1.3", 3) {
		t.Fatal("expected Acquire 3 to succeed")
	}

	// Next acquire should fail (global limit reached)
	if sl.Acquire("192.168.1.100", 4) {
		t.Fatal("expected Acquire to fail at global limit")
	}

	// Release one and try again
	sl.Release("192.168.1.1", 1)
	if !sl.Acquire("192.168.1.100", 4) {
		t.Fatal("expected Acquire to succeed after release")
	}
}

func TestStreamLimiterPerIPLimit(t *testing.T) {
	sl := NewStreamLimiter(100, 2)
	defer sl.Stop()

	ip := "192.168.1.1"

	// Acquire two different scenes for the same IP
	if !sl.Acquire(ip, 1) {
		t.Fatal("expected first Acquire to succeed")
	}
	if !sl.Acquire(ip, 2) {
		t.Fatal("expected second Acquire to succeed")
	}

	// Third different scene should fail (per-IP limit of 2)
	if sl.Acquire(ip, 3) {
		t.Fatal("expected third Acquire to fail (per-IP limit)")
	}

	// Global count should be 2
	if sl.GlobalCount() != 2 {
		t.Fatalf("expected global count 2, got %d", sl.GlobalCount())
	}

	// Same scene should still succeed (refcount, not a new slot)
	if !sl.Acquire(ip, 1) {
		t.Fatal("expected same-scene Acquire to succeed via refcount")
	}
	if sl.GlobalCount() != 2 {
		t.Fatalf("expected global count still 2, got %d", sl.GlobalCount())
	}

	// Different IP should succeed
	if !sl.Acquire("192.168.1.2", 1) {
		t.Fatal("expected different IP to succeed")
	}
}

func TestStreamLimiterStats(t *testing.T) {
	sl := NewStreamLimiter(100, 10)
	defer sl.Stop()

	sl.Acquire("192.168.1.1", 1)
	sl.Acquire("192.168.1.1", 2)
	sl.Acquire("192.168.1.2", 1)

	stats := sl.Stats()
	if stats.GlobalCount != 3 {
		t.Fatalf("expected GlobalCount 3, got %d", stats.GlobalCount)
	}
	if stats.ActiveIPs != 2 {
		t.Fatalf("expected ActiveIPs 2, got %d", stats.ActiveIPs)
	}
}

func TestStreamLimiterConcurrent(t *testing.T) {
	sl := NewStreamLimiter(100, 10)
	defer sl.Stop()

	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ip := "192.168." + string(rune('0'+id%10)) + ".1"
			sceneID := uint(id%5 + 1)

			if sl.Acquire(ip, sceneID) {
				time.Sleep(time.Millisecond)
				sl.Release(ip, sceneID)
			}
		}(i)
	}

	wg.Wait()

	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0, got %d", sl.GlobalCount())
	}
}

func TestStreamLimiterRaceCondition(t *testing.T) {
	sl := NewStreamLimiter(10, 5)
	defer sl.Stop()

	ip := "192.168.1.1"
	var sceneID uint = 1
	const iterations = 10000

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Acquire and release
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if sl.Acquire(ip, sceneID) {
				sl.Release(ip, sceneID)
			}
		}
	}()

	// Goroutine 2: Stats
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = sl.Stats()
			_ = sl.GlobalCount()
			_ = sl.IPCount(ip)
		}
	}()

	wg.Wait()
}

func TestStreamLimiterIPCountNonexistent(t *testing.T) {
	sl := NewStreamLimiter(10, 5)
	defer sl.Stop()

	if sl.IPCount("10.0.0.1") != 0 {
		t.Fatal("expected IPCount for nonexistent IP to be 0")
	}
}

func TestStreamLimiterReleaseNonexistent(t *testing.T) {
	sl := NewStreamLimiter(10, 5)
	defer sl.Stop()

	// Release on IP+scene that was never acquired should not panic
	sl.Release("10.0.0.1", 1)

	// Counts should remain zero
	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0, got %d", sl.GlobalCount())
	}
}

func TestStreamLimiterConcurrentRefCounting(t *testing.T) {
	sl := NewStreamLimiter(100, 10)
	defer sl.Stop()

	ip := "192.168.1.1"
	var sceneID uint = 42
	const numGoroutines = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Simulate many concurrent range requests for the same video
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if sl.Acquire(ip, sceneID) {
				time.Sleep(time.Millisecond)
				sl.Release(ip, sceneID)
			}
		}()
	}

	wg.Wait()

	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0 after all releases, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 0 {
		t.Fatalf("expected IP count 0 after all releases, got %d", sl.IPCount(ip))
	}
}

func BenchmarkStreamLimiterAcquireRelease(b *testing.B) {
	sl := NewStreamLimiter(1000, 100)
	defer sl.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ip := "192.168.1.1"
		var sceneID uint = 1
		for pb.Next() {
			if sl.Acquire(ip, sceneID) {
				sl.Release(ip, sceneID)
			}
		}
	})
}
