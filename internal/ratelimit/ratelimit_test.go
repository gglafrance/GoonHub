package ratelimit

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestLimiterRate(t *testing.T) {
	l := New(1 << 20) // 1 MiB/s, burst 256 KiB
	start := time.Now()
	n, err := io.Copy(io.Discard, l.Reader(context.Background(), bytes.NewReader(make([]byte, 768<<10))))
	if err != nil || n != 768<<10 {
		t.Fatal(n, err)
	}
	// 768 KiB minus the 256 KiB burst at 1 MiB/s: about 0.5 s.
	if el := time.Since(start); el < 400*time.Millisecond || el > 2*time.Second {
		t.Fatalf("took %v", el)
	}
	if New(0) != nil || New(0).Reader(context.Background(), bytes.NewReader(nil)) == nil {
		t.Fatal("unlimited limiter")
	}
}
