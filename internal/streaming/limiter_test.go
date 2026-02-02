package streaming

import (
	"sync"
	"testing"
	"time"
)

func TestNewStreamLimiter(t *testing.T) {
	tests := []struct {
		name        string
		maxGlobal   int
		maxPerIP    int
		wantGlobal  int
		wantPerIP   int
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

	// Acquire should succeed
	if !sl.Acquire(ip) {
		t.Fatal("expected Acquire to succeed")
	}

	if sl.GlobalCount() != 1 {
		t.Fatalf("expected global count 1, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 1 {
		t.Fatalf("expected IP count 1, got %d", sl.IPCount(ip))
	}

	// Release
	sl.Release(ip)

	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0, got %d", sl.GlobalCount())
	}
	if sl.IPCount(ip) != 0 {
		t.Fatalf("expected IP count 0, got %d", sl.IPCount(ip))
	}
}

func TestStreamLimiterGlobalLimit(t *testing.T) {
	sl := NewStreamLimiter(3, 10)
	defer sl.Stop()

	// Acquire up to limit
	for i := 0; i < 3; i++ {
		ip := "192.168.1." + string(rune('1'+i))
		if !sl.Acquire(ip) {
			t.Fatalf("expected Acquire %d to succeed", i)
		}
	}

	// Next acquire should fail (global limit reached)
	if sl.Acquire("192.168.1.100") {
		t.Fatal("expected Acquire to fail at global limit")
	}

	// Release one and try again
	sl.Release("192.168.1.1")
	if !sl.Acquire("192.168.1.100") {
		t.Fatal("expected Acquire to succeed after release")
	}
}

func TestStreamLimiterPerIPLimit(t *testing.T) {
	sl := NewStreamLimiter(100, 2)
	defer sl.Stop()

	ip := "192.168.1.1"

	// Acquire up to per-IP limit
	if !sl.Acquire(ip) {
		t.Fatal("expected first Acquire to succeed")
	}
	if !sl.Acquire(ip) {
		t.Fatal("expected second Acquire to succeed")
	}

	// Third should fail
	if sl.Acquire(ip) {
		t.Fatal("expected third Acquire to fail (per-IP limit)")
	}

	// Global count should be 2 (not 3, since the failed acquire should rollback)
	if sl.GlobalCount() != 2 {
		t.Fatalf("expected global count 2, got %d", sl.GlobalCount())
	}

	// Different IP should succeed
	if !sl.Acquire("192.168.1.2") {
		t.Fatal("expected different IP to succeed")
	}
}

func TestStreamLimiterStats(t *testing.T) {
	sl := NewStreamLimiter(100, 10)
	defer sl.Stop()

	sl.Acquire("192.168.1.1")
	sl.Acquire("192.168.1.1")
	sl.Acquire("192.168.1.2")

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
	const ipsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ip := "192.168." + string(rune('0'+id%10)) + ".1"

			// Acquire
			if sl.Acquire(ip) {
				// Hold for a bit
				time.Sleep(time.Millisecond)
				// Release
				sl.Release(ip)
			}
		}(i)
	}

	wg.Wait()

	// All should be released
	if sl.GlobalCount() != 0 {
		t.Fatalf("expected global count 0, got %d", sl.GlobalCount())
	}
}

func TestStreamLimiterRaceCondition(t *testing.T) {
	sl := NewStreamLimiter(10, 5)
	defer sl.Stop()

	ip := "192.168.1.1"
	const iterations = 10000

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Acquire and release
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if sl.Acquire(ip) {
				sl.Release(ip)
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

	// IP that was never used should return 0
	if sl.IPCount("10.0.0.1") != 0 {
		t.Fatal("expected IPCount for nonexistent IP to be 0")
	}
}

func TestStreamLimiterReleaseNonexistent(t *testing.T) {
	sl := NewStreamLimiter(10, 5)
	defer sl.Stop()

	// Release on IP that was never acquired should not panic
	// and should decrement global counter (which would go negative,
	// but that's acceptable behavior for Release called without Acquire)
	sl.Release("10.0.0.1")
}

func BenchmarkStreamLimiterAcquireRelease(b *testing.B) {
	sl := NewStreamLimiter(1000, 100)
	defer sl.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ip := "192.168.1.1"
		for pb.Next() {
			if sl.Acquire(ip) {
				sl.Release(ip)
			}
		}
	})
}
