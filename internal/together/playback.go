package together

import (
	"math"
	"time"
)

// Rate bounds of the rate command.
const (
	minRate = 0.25
	maxRate = 4.0
)

// endedTolerance: with a known end, an ended report earlier than this before it is ignored.
const endedTolerance = 5.0

// playback is the authoritative playback state of a lobby: position (seconds) at ref.
type playback struct {
	itemID   uint // 0: nothing playing
	playing  bool // wanted state
	hold     bool // waiting for buffering members: the position does not move
	position float64
	ref      time.Time
	rate     float64
	seq      uint64
}

// moving reports whether the position advances with time.
func (p *playback) moving() bool { return p.itemID != 0 && p.playing && !p.hold }

// at is the unclamped position at t.
func (p *playback) at(t time.Time) float64 {
	if !p.moving() {
		return p.position
	}
	return p.position + t.Sub(p.ref).Seconds()*p.rate
}

// clampPosition keeps a position in [0, end] (end <= 0: unknown, no upper bound).
func clampPosition(pos, end float64) float64 {
	if math.IsNaN(pos) || pos < 0 {
		return 0
	}
	if end > 0 && pos > end {
		return end
	}
	return pos
}

// itemEnd is where an item ends: its range end, else the scene duration. fromDuration tells
// the duration was used (rounded to the second: the server waits end_grace more).
func itemEnd(it *item) (end float64, known, fromDuration bool) {
	if it == nil {
		return 0, false, false
	}
	if it.end != nil {
		return *it.end, true, false
	}
	if it.duration > 0 {
		return float64(it.duration), true, true
	}
	return 0, false, false
}

// itemStart is where an item starts playing.
func itemStart(it *item) float64 {
	if it != nil && it.start != nil {
		return *it.start
	}
	return 0
}

// snapshot is the state sent to the clients.
func (p *playback) snapshot() State {
	s := State{
		Playing:    p.playing && p.itemID != 0,
		Hold:       p.hold,
		Position:   p.position,
		ServerTime: p.ref.UnixMilli(),
		Rate:       p.rate,
		Seq:        p.seq,
	}
	if p.itemID != 0 {
		id := p.itemID
		s.ItemID = &id
	}
	return s
}
