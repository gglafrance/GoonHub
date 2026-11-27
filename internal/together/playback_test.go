package together

import (
	"math"
	"testing"
	"time"
)

func TestPlaybackAt(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		p    playback
		dt   time.Duration
		want float64
	}{
		{"paused does not move", playback{itemID: 1, position: 12, ref: t0, rate: 1}, time.Hour, 12},
		{"playing at rate 1", playback{itemID: 1, playing: true, position: 12, ref: t0, rate: 1}, 3 * time.Second, 15},
		{"playing at rate 1.5", playback{itemID: 1, playing: true, position: 0, ref: t0, rate: 1.5}, 2 * time.Second, 3},
		{"hold does not move", playback{itemID: 1, playing: true, hold: true, position: 7, ref: t0, rate: 1}, time.Minute, 7},
		{"no item does not move", playback{playing: true, position: 7, ref: t0, rate: 1}, time.Minute, 7},
		{"sub-second", playback{itemID: 1, playing: true, position: 1, ref: t0, rate: 1}, 250 * time.Millisecond, 1.25},
	}
	for _, c := range cases {
		if got := c.p.at(t0.Add(c.dt)); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestClampPosition(t *testing.T) {
	cases := []struct{ pos, end, want float64 }{
		{-3, 100, 0},
		{50, 100, 50},
		{150, 100, 100},
		{150, 0, 150}, // unknown end
		{math.NaN(), 100, 0},
	}
	for _, c := range cases {
		if got := clampPosition(c.pos, c.end); got != c.want {
			t.Errorf("clamp(%v, %v) = %v, want %v", c.pos, c.end, got, c.want)
		}
	}
}

func TestItemEnd(t *testing.T) {
	end := 20.0
	if e, known, fromDuration := itemEnd(&item{duration: 100, end: &end}); e != 20 || !known || fromDuration {
		t.Fatalf("range end: %v %v %v", e, known, fromDuration)
	}
	if e, known, fromDuration := itemEnd(&item{duration: 100}); e != 100 || !known || !fromDuration {
		t.Fatalf("duration end: %v %v %v", e, known, fromDuration)
	}
	if _, known, _ := itemEnd(&item{}); known {
		t.Fatal("unknown duration reported as known")
	}
	if _, known, _ := itemEnd(nil); known {
		t.Fatal("nil item has an end")
	}
}

func TestSnapshot(t *testing.T) {
	t0 := time.UnixMilli(1_700_000_000_123)
	s := (&playback{itemID: 4, playing: true, position: 9.5, ref: t0, rate: 1.25, seq: 7}).snapshot()
	if s.ItemID == nil || *s.ItemID != 4 || !s.Playing || s.ServerTime != 1_700_000_000_123 || s.Rate != 1.25 || s.Seq != 7 {
		t.Fatalf("snapshot %+v", s)
	}
	if empty := (&playback{playing: true, ref: t0, rate: 1}).snapshot(); empty.ItemID != nil || empty.Playing {
		t.Fatalf("empty snapshot %+v", empty)
	}
}

func TestInviteCodes(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, err := newInviteCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != inviteCodeLength {
			t.Fatalf("code %q", code)
		}
		for _, r := range code {
			if !containsRune(inviteAlphabet, r) {
				t.Fatalf("code %q has %q", code, r)
			}
		}
		seen[code] = true
	}
	if len(seen) < 195 {
		t.Fatalf("too many duplicate codes: %d distinct", len(seen))
	}
	if got := NormalizeInviteCode(" abcd-efgh "); got != "ABCDEFGH" {
		t.Fatalf("normalize: %q", got)
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
