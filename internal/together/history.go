package together

import (
	"time"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

// History of the played items (docs/GOON2GETHER.md "History"). A play starts when an item
// becomes the current one and ends when another item (or none) takes its place: it is then
// recorded once, whatever happened in between (a repeat_one loop, a stop at the end and a play
// again, a restart with play_item). An item that was never played (current but paused, then
// skipped or removed) is not recorded, nor an item skipped after playing less than
// history_min_play (skipping through the queue).

// historyEntry is an item the lobby played.
type historyEntry struct {
	id        uint
	it        item // the queue item as it was (adder kept for the redaction)
	startedAt time.Time
	endedAt   time.Time
	outcome   string
	available bool // the scene can still be queued (not in the trash)
}

func historyFromRow(r data.LobbyHistoryEntry) *historyEntry {
	return &historyEntry{
		id: r.ID,
		it: item{
			sceneID:     r.SceneID,
			title:       r.SceneTitle,
			duration:    r.SceneDuration,
			start:       r.StartS,
			end:         r.EndS,
			label:       r.Label,
			addedBy:     r.AddedBy,
			addedByName: r.AddedByName,
			addedAnon:   derefInt(r.AddedByAnon),
		},
		startedAt: r.StartedAt,
		endedAt:   r.EndedAt,
		outcome:   r.Outcome,
		available: r.SceneAvailable,
	}
}

// dto is the entry as sent: full (moderators) or with an anonymous adder redacted.
func (h *historyEntry) dto(full bool) HistoryEntry {
	q := h.it.dto(full)
	return HistoryEntry{
		ID:        h.id,
		SceneID:   q.SceneID,
		Title:     q.Title,
		Duration:  q.Duration,
		Start:     q.Start,
		End:       q.End,
		Label:     q.Label,
		AddedBy:   q.AddedBy,
		StartedAt: h.startedAt.UnixMilli(),
		EndedAt:   h.endedAt.UnixMilli(),
		Outcome:   h.outcome,
		Available: h.available,
	}
}

// play is the play of the current item, recorded in the history when the item changes.
type play struct {
	it        *item     // nil: no current item
	startedAt time.Time // when it first played (zero: not played yet)
	completed bool      // it reached its end at least once
}

// historyList is the history as sent, newest first.
func (l *lobby) historyList(full bool) []HistoryEntry {
	out := make([]HistoryEntry, 0, len(l.history))
	for i := len(l.history) - 1; i >= 0; i-- {
		out = append(out, l.history[i].dto(full))
	}
	return out
}

// trackPlay follows the current item after a change of the item playing (startItem): the play
// of the previous item is recorded when another item (or none) takes its place, and a playing
// item gets its start time (lock held).
func (l *lobby) trackPlay(now time.Time) {
	cur := l.current()
	if l.play.it != nil && (cur == nil || cur.id != l.play.it.id) {
		l.recordPlay(l.play, now)
		l.play = play{}
	}
	if cur == nil {
		return
	}
	if l.play.it == nil {
		l.play = play{it: cur}
	}
	if l.state.playing && l.play.startedAt.IsZero() {
		l.play.startedAt = now
	}
}

// playCompleted marks the play of the current item as having reached its end (lock held).
func (l *lobby) playCompleted(now time.Time) {
	cur := l.current()
	if cur == nil {
		return
	}
	if l.play.it == nil || l.play.it.id != cur.id {
		l.play = play{it: cur}
	}
	if l.play.startedAt.IsZero() {
		l.play.startedAt = now
	}
	l.play.completed = true
}

// recordPlay adds a finished play to the history (stored, bounded) and broadcasts it (lock
// held). A play that never started, or skipped too soon, is not recorded.
func (l *lobby) recordPlay(p play, now time.Time) {
	limit := l.hub.cfg.HistoryMax
	if limit <= 0 || p.it == nil || (p.startedAt.IsZero() && !p.completed) {
		return
	}
	if !p.completed && now.Sub(p.startedAt) < l.hub.cfg.HistoryMinPlay {
		return
	}
	started := p.startedAt
	if started.IsZero() {
		started = now
	}
	outcome := data.HistorySkipped
	if p.completed {
		outcome = data.HistoryCompleted
	}
	it := *p.it
	row := data.LobbyHistoryEntry{
		LobbyID:   l.row.ID,
		SceneID:   it.sceneID,
		StartS:    it.start,
		EndS:      it.end,
		Label:     it.label,
		AddedBy:   it.addedBy,
		StartedAt: started.UTC(),
		EndedAt:   now.UTC(),
		Outcome:   outcome,
	}
	if it.addedAnon > 0 {
		n := it.addedAnon
		row.AddedByAnon = &n
	}
	if err := l.hub.store.AddHistory(&row, limit); err != nil {
		l.hub.logger.Warn("Failed to record a lobby history entry", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
		return
	}
	it.id = 0
	e := &historyEntry{id: row.ID, it: it, startedAt: started, endedAt: now, outcome: outcome, available: true}
	l.history = append(l.history, e)
	if len(l.history) > limit {
		l.history = append([]*historyEntry(nil), l.history[len(l.history)-limit:]...)
	}
	l.broadcastSplit(func(full bool) any { return historyAddMsg{T: msgHistoryAdd, Entry: e.dto(full)} })
}

// refreshHistory checks again which history scenes can still be queued (trashed or deleted
// since they played), before a snapshot (lock held).
func (l *lobby) refreshHistory() {
	if len(l.history) == 0 {
		return
	}
	seen := make(map[uint]bool, len(l.history))
	ids := make([]uint, 0, len(l.history))
	for _, e := range l.history {
		if !seen[e.it.sceneID] {
			seen[e.it.sceneID] = true
			ids = append(ids, e.it.sceneID)
		}
	}
	scenes, err := l.hub.scenes.GetByIDs(ids)
	if err != nil {
		l.hub.logger.Warn("Failed to check the lobby history scenes", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
		return
	}
	found := make(map[uint]bool, len(scenes))
	for _, s := range scenes {
		found[s.ID] = true
	}
	for _, e := range l.history {
		e.available = found[e.it.sceneID]
	}
}

// savedPlay is the play of the current item as saved with the playback.
func (l *lobby) savedPlay() (*time.Time, bool) {
	cur := l.current()
	if cur == nil || l.play.it == nil || l.play.it.id != cur.id {
		return nil, false
	}
	var started *time.Time
	if !l.play.startedAt.IsZero() {
		t := l.play.startedAt.UTC()
		started = &t
	}
	return started, l.play.completed
}
