package streaming

import (
	"sync"
	"testing"
)

func TestNewBufferPool(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		wantSize   int
	}{
		{"default size for zero", 0, 262144},
		{"default size for negative", -1, 262144},
		{"custom size", 1024, 1024},
		{"large size", 1048576, 1048576},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := NewBufferPool(tt.size)
			if bp.BufferSize() != tt.wantSize {
				t.Fatalf("expected buffer size %d, got %d", tt.wantSize, bp.BufferSize())
			}
		})
	}
}

func TestBufferPoolGetPut(t *testing.T) {
	bp := NewBufferPool(1024)

	buf := bp.Get()
	if len(buf) != 1024 {
		t.Fatalf("expected buffer length 1024, got %d", len(buf))
	}
	if cap(buf) != 1024 {
		t.Fatalf("expected buffer capacity 1024, got %d", cap(buf))
	}

	// Modify buffer and put it back
	for i := range buf {
		buf[i] = 0xFF
	}
	bp.Put(buf)

	// Get another buffer - it might be the same one (reused)
	buf2 := bp.Get()
	if len(buf2) != 1024 {
		t.Fatalf("expected buffer length 1024, got %d", len(buf2))
	}
}

func TestBufferPoolPutWrongCapacity(t *testing.T) {
	bp := NewBufferPool(1024)

	// Create a buffer with wrong capacity
	wrongBuf := make([]byte, 512)

	// This should not panic and should not add the buffer to the pool
	bp.Put(wrongBuf)

	// Pool should still return correct size buffers
	buf := bp.Get()
	if cap(buf) != 1024 {
		t.Fatalf("expected buffer capacity 1024, got %d", cap(buf))
	}
}

func TestBufferPoolConcurrent(t *testing.T) {
	bp := NewBufferPool(4096)
	const numGoroutines = 100
	const iterationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				buf := bp.Get()
				if len(buf) != 4096 {
					t.Errorf("expected buffer length 4096, got %d", len(buf))
					return
				}
				// Simulate some work
				buf[0] = byte(j)
				buf[len(buf)-1] = byte(j)
				bp.Put(buf)
			}
		}()
	}

	wg.Wait()
}

func BenchmarkBufferPoolGetPut(b *testing.B) {
	bp := NewBufferPool(262144) // 256KB

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bp.Get()
			bp.Put(buf)
		}
	})
}

func BenchmarkNewBufferEveryTime(b *testing.B) {
	const bufSize = 262144 // 256KB

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := make([]byte, bufSize)
			_ = buf
		}
	})
}
