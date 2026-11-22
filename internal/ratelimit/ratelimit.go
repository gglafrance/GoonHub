// Package ratelimit limits the throughput of readers sharing one bandwidth budget.
package ratelimit

import (
	"context"
	"io"
	"sync"
	"time"
)

// Limiter is a token bucket in bytes per second, shared by every reader it wraps. A nil
// Limiter or a rate <= 0 does not limit.
type Limiter struct {
	mu     sync.Mutex
	rate   float64 // bytes per second
	burst  float64
	tokens float64
	last   time.Time
}

// New returns a limiter of bytesPerSec (nil when bytesPerSec <= 0).
func New(bytesPerSec int64) *Limiter {
	if bytesPerSec <= 0 {
		return nil
	}
	burst := float64(bytesPerSec) / 4
	if burst < 32<<10 {
		burst = 32 << 10
	}
	return &Limiter{rate: float64(bytesPerSec), burst: burst, tokens: burst, last: time.Now()}
}

// wait blocks until n bytes may pass (n is capped to the burst by the caller).
func (l *Limiter) wait(ctx context.Context, n int) error {
	for {
		l.mu.Lock()
		now := time.Now()
		l.tokens += now.Sub(l.last).Seconds() * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.last = now
		if l.tokens >= float64(n) {
			l.tokens -= float64(n)
			l.mu.Unlock()
			return nil
		}
		need := (float64(n) - l.tokens) / l.rate
		l.mu.Unlock()
		t := time.NewTimer(time.Duration(need * float64(time.Second)))
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// Reader wraps r so that it reads at most at the limiter's rate.
func (l *Limiter) Reader(ctx context.Context, r io.Reader) io.Reader {
	if l == nil {
		return r
	}
	return &reader{ctx: ctx, r: r, l: l}
}

type reader struct {
	ctx context.Context
	r   io.Reader
	l   *Limiter
}

func (r *reader) Read(p []byte) (int, error) {
	if max := int(r.l.burst); len(p) > max {
		p = p[:max]
	}
	n, err := r.r.Read(p)
	if n > 0 {
		if werr := r.l.wait(r.ctx, n); werr != nil && err == nil {
			err = werr
		}
	}
	return n, err
}
