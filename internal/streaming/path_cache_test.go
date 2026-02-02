package streaming

import (
	"sync"
	"testing"
	"time"
)

func TestNewPathCache(t *testing.T) {
	tests := []struct {
		name        string
		ttl         time.Duration
		maxSize     int
	}{
		{"default values", 0, 0},
		{"custom values", time.Minute, 1000},
		{"large values", time.Hour, 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := NewPathCache(tt.ttl, tt.maxSize)
			defer pc.Stop()

			if pc.Size() != 0 {
				t.Fatalf("expected empty cache, got size %d", pc.Size())
			}
		})
	}
}

func TestPathCacheGetSet(t *testing.T) {
	pc := NewPathCache(time.Minute, 100)
	defer pc.Stop()

	sceneID := uint(123)
	path := "/videos/scene123.mp4"

	// Get should return false for non-existent entry
	if _, ok := pc.Get(sceneID); ok {
		t.Fatal("expected Get to return false for non-existent entry")
	}

	// Set and get
	pc.Set(sceneID, path)

	gotPath, ok := pc.Get(sceneID)
	if !ok {
		t.Fatal("expected Get to return true after Set")
	}
	if gotPath != path {
		t.Fatalf("expected path %q, got %q", path, gotPath)
	}

	if pc.Size() != 1 {
		t.Fatalf("expected size 1, got %d", pc.Size())
	}
}

func TestPathCacheExpiration(t *testing.T) {
	// Use very short TTL for testing
	pc := NewPathCache(50*time.Millisecond, 100)
	defer pc.Stop()

	sceneID := uint(1)
	pc.Set(sceneID, "/videos/test.mp4")

	// Should be available immediately
	if _, ok := pc.Get(sceneID); !ok {
		t.Fatal("expected entry to be available immediately after Set")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should be expired now
	if _, ok := pc.Get(sceneID); ok {
		t.Fatal("expected entry to be expired")
	}
}

func TestPathCacheInvalidate(t *testing.T) {
	pc := NewPathCache(time.Minute, 100)
	defer pc.Stop()

	sceneID := uint(1)
	pc.Set(sceneID, "/videos/test.mp4")

	// Verify it's there
	if _, ok := pc.Get(sceneID); !ok {
		t.Fatal("expected entry to exist")
	}

	// Invalidate
	pc.Invalidate(sceneID)

	// Should be gone
	if _, ok := pc.Get(sceneID); ok {
		t.Fatal("expected entry to be invalidated")
	}
}

func TestPathCacheClear(t *testing.T) {
	pc := NewPathCache(time.Minute, 100)
	defer pc.Stop()

	// Add several entries
	for i := uint(1); i <= 10; i++ {
		pc.Set(i, "/videos/scene.mp4")
	}

	if pc.Size() != 10 {
		t.Fatalf("expected size 10, got %d", pc.Size())
	}

	// Clear
	pc.Clear()

	if pc.Size() != 0 {
		t.Fatalf("expected size 0 after clear, got %d", pc.Size())
	}

	// Should not find any entries
	for i := uint(1); i <= 10; i++ {
		if _, ok := pc.Get(i); ok {
			t.Fatalf("expected entry %d to be cleared", i)
		}
	}
}

func TestPathCacheMaxSize(t *testing.T) {
	maxSize := 10
	pc := NewPathCache(time.Minute, maxSize)
	defer pc.Stop()

	// Add more than max entries
	for i := uint(1); i <= 20; i++ {
		pc.Set(i, "/videos/scene.mp4")
	}

	// Size should not exceed max (though it might be less due to eviction)
	if pc.Size() > maxSize {
		t.Fatalf("expected size <= %d, got %d", maxSize, pc.Size())
	}
}

func TestPathCacheConcurrent(t *testing.T) {
	pc := NewPathCache(time.Minute, 1000)
	defer pc.Stop()

	const numGoroutines = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sceneID := uint((id*iterations + j) % 500)
				pc.Set(sceneID, "/videos/scene.mp4")
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sceneID := uint((id*iterations + j) % 500)
				pc.Get(sceneID)
			}
		}(i)
	}

	// Invalidators
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sceneID := uint((id*iterations + j) % 500)
				pc.Invalidate(sceneID)
			}
		}(i)
	}

	wg.Wait()
}

func TestPathCacheRaceCondition(t *testing.T) {
	pc := NewPathCache(50*time.Millisecond, 100)
	defer pc.Stop()

	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(4)

	// Set
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			pc.Set(uint(i%10), "/videos/scene.mp4")
		}
	}()

	// Get
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			pc.Get(uint(i % 10))
		}
	}()

	// Invalidate
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			pc.Invalidate(uint(i % 10))
		}
	}()

	// Size
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = pc.Size()
		}
	}()

	wg.Wait()
}

func BenchmarkPathCacheGetSet(b *testing.B) {
	pc := NewPathCache(time.Minute, 10000)
	defer pc.Stop()

	// Pre-populate
	for i := uint(0); i < 1000; i++ {
		pc.Set(i, "/videos/scene.mp4")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint(0)
		for pb.Next() {
			sceneID := i % 1000
			if _, ok := pc.Get(sceneID); !ok {
				pc.Set(sceneID, "/videos/scene.mp4")
			}
			i++
		}
	})
}
