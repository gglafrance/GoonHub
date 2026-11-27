package backoff

import (
	"testing"
	"time"
)

func TestBackoffDoublesAndCaps(t *testing.T) {
	b := Backoff{Min: 100 * time.Millisecond, Max: time.Second, Jitter: 0.01}
	want := []time.Duration{100, 200, 400, 800, 1000, 1000}
	for i, w := range want {
		d := b.Next()
		lo, hi := w*time.Millisecond*99/100, w*time.Millisecond*101/100
		if d < lo || d > hi {
			t.Fatalf("step %d: %v, want about %vms", i, d, w)
		}
	}
	b.Reset()
	if d := b.Next(); d > 102*time.Millisecond {
		t.Fatalf("after reset: %v", d)
	}
}
