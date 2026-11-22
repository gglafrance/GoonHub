// Package backoff implements exponential backoff with jitter.
package backoff

import (
	"context"
	"math/rand/v2"
	"time"
)

// Backoff doubles from Min to Max with +-Jitter (fraction). The zero value is 1 s -> 30 s,
// +-20 % (docs/REMOTE_WORKERS.md, section 12).
type Backoff struct {
	Min, Max time.Duration
	Jitter   float64
	attempt  int
}

// Next returns the next delay.
func (b *Backoff) Next() time.Duration {
	lo, hi := b.Min, b.Max
	if lo <= 0 {
		lo = time.Second
	}
	if hi <= 0 {
		hi = 30 * time.Second
	}
	j := b.Jitter
	if j == 0 {
		j = 0.2
	}
	d := lo << min(b.attempt, 30)
	if d > hi || d <= 0 {
		d = hi
	}
	b.attempt++
	f := 1 + j*(2*rand.Float64()-1)
	return time.Duration(float64(d) * f)
}

// Reset starts again from Min.
func (b *Backoff) Reset() { b.attempt = 0 }

// Attempts is the number of delays returned since the last reset.
func (b *Backoff) Attempts() int { return b.attempt }

// Wait sleeps the next delay; it returns ctx.Err() when ctx ends first.
func (b *Backoff) Wait(ctx context.Context) error {
	return Sleep(ctx, b.Next())
}

// Sleep sleeps d or until ctx ends.
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
