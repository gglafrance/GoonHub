package streaming

import (
	"sync"
)

// BufferPool provides reusable byte buffers for efficient streaming.
// Uses sync.Pool to reduce allocations during high-concurrency scenarios.
type BufferPool struct {
	pool       sync.Pool
	bufferSize int
}

// NewBufferPool creates a new buffer pool with the specified buffer size.
// Recommended size for video streaming is 256KB (262144 bytes).
func NewBufferPool(bufferSize int) *BufferPool {
	if bufferSize <= 0 {
		bufferSize = 262144 // 256KB default
	}

	bp := &BufferPool{
		bufferSize: bufferSize,
	}

	bp.pool = sync.Pool{
		New: func() any {
			buf := make([]byte, bp.bufferSize)
			return &buf
		},
	}

	return bp
}

// Get retrieves a buffer from the pool.
// The returned buffer is ready to use and has capacity of bufferSize.
func (bp *BufferPool) Get() []byte {
	bufPtr := bp.pool.Get().(*[]byte)
	return *bufPtr
}

// Put returns a buffer to the pool for reuse.
// The buffer should not be used after calling Put.
func (bp *BufferPool) Put(buf []byte) {
	// Only return buffers with expected capacity to prevent memory leaks
	// from accidentally returning smaller slices
	if cap(buf) == bp.bufferSize {
		bp.pool.Put(&buf)
	}
}

// BufferSize returns the size of buffers managed by this pool.
func (bp *BufferPool) BufferSize() int {
	return bp.bufferSize
}
