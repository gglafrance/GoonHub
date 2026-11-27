package together

import (
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"goonhub/internal/data"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// member is a (non banned) lobby member. anon > 0: an anonymous member, "Anonymous #anon"
// for the other members (moderators see who it is).
type member struct {
	userID     uint
	username   string
	canControl bool
	joinedAt   time.Time
	anon       int
}

// anonName is the name of an anonymous member.
func anonName(n int) string { return "Anonymous #" + strconv.Itoa(n) }

// display is the name every member sees.
func (m *member) display() string {
	if m.anon > 0 {
		return anonName(m.anon)
	}
	return m.username
}

// modDisplay is the name moderators see: an anonymous member with the real username.
func (m *member) modDisplay() string {
	if m.anon > 0 {
		return anonName(m.anon) + " (" + m.username + ")"
	}
	return m.username
}

// anonPtr is the anonymous number as sent (nil: named).
func anonPtr(n int) *int {
	if n <= 0 {
		return nil
	}
	return &n
}

// item is a queue item or a suggestion.
type item struct {
	id          uint
	sceneID     uint
	title       string
	duration    int
	start       *float64
	end         *float64
	label       *string
	addedBy     *uint
	addedByName string
	addedAnon   int // the adder was anonymous (their number)
}

func itemFromRow(r data.LobbyQueueItem) *item {
	return &item{
		id:          r.ID,
		sceneID:     r.SceneID,
		title:       r.SceneTitle,
		duration:    r.SceneDuration,
		start:       r.StartS,
		end:         r.EndS,
		label:       r.Label,
		addedBy:     r.AddedBy,
		addedByName: r.AddedByName,
		addedAnon:   derefInt(r.AddedByAnon),
	}
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// dto is the item as sent: full (moderators) or with an anonymous adder redacted.
func (it *item) dto(full bool) QueueItem {
	q := QueueItem{
		ID:       it.id,
		SceneID:  it.sceneID,
		Title:    it.title,
		Duration: it.duration,
		Start:    it.start,
		End:      it.end,
		Label:    it.label,
	}
	switch {
	case it.addedAnon > 0 && !full:
		q.AddedBy = &MemberRef{Username: anonName(it.addedAnon), Anonymous: true, AnonNumber: anonPtr(it.addedAnon)}
	case it.addedBy != nil:
		id := *it.addedBy
		q.AddedBy = &MemberRef{ID: &id, Username: it.addedByName, Anonymous: it.addedAnon > 0, AnonNumber: anonPtr(it.addedAnon)}
	}
	return q
}

// lobby is a loaded lobby. Every field is guarded by mu.
type lobby struct {
	mu  sync.Mutex
	hub *Hub

	row         data.Lobby
	members     map[uint]*member
	banned      map[uint]bool
	queue       []*item // queue[0] is the item playing when state.itemID != 0
	suggestions []*item
	state       playback
	clients     map[*Client]struct{}

	chat      []ChatMessage
	nextMsgID int64

	// wait for everyone: the last sync point (item change, seek) and the one whose hold
	// timed out (no hold until the next sync point)
	syncSeq        uint64
	holdTimedOutAt uint64
	holdSince      time.Time

	endedReports  map[uint]bool
	hostAwaySince time.Time
	emptySince    time.Time
	// a repeat restarted the item: stale ended reports of the previous loop are ignored
	restartedAt time.Time

	// inactivity close: the last activity, and when the lobby closes once announced
	lastActivity time.Time
	closingAt    time.Time

	chatLimiters     map[uint]*rate.Limiter
	reactionLimiters map[uint]*rate.Limiter
	// reactions on chat messages (not the floating reactions)
	chatReactionLimiters map[uint]*rate.Limiter

	closed   bool // deleted: no longer usable
	unloaded bool // dropped from memory: load it again
}

// newLobby builds the live lobby from its stored rows.
func newLobby(h *Hub, row data.Lobby, members []data.LobbyMember, items []data.LobbyQueueItem) *lobby {
	now := h.now()
	l := &lobby{
		hub:              h,
		row:              row,
		members:          make(map[uint]*member),
		banned:           make(map[uint]bool),
		clients:          make(map[*Client]struct{}),
		endedReports:     make(map[uint]bool),
		chatLimiters:     make(map[uint]*rate.Limiter),
		reactionLimiters: make(map[uint]*rate.Limiter),
		emptySince:       now,
		hostAwaySince:    now,
		lastActivity:     now,
		syncSeq:          1,

		chatReactionLimiters: make(map[uint]*rate.Limiter),
	}
	// Nobody connected since last_active_at (restart, reload): the empty close counts from it
	if !row.LastActiveAt.IsZero() && row.LastActiveAt.Before(now) {
		l.emptySince = row.LastActiveAt
	}
	if !data.ValidAutoNext(l.row.AutoNext) {
		l.row.AutoNext = data.AutoNextNext
	}
	for _, m := range members {
		if m.BannedAt != nil {
			l.banned[m.UserID] = true
			continue
		}
		l.members[m.UserID] = &member{userID: m.UserID, username: m.Username, canControl: m.CanControl, joinedAt: m.JoinedAt, anon: derefInt(m.AnonNumber)}
	}
	for _, r := range items {
		if r.Status == data.QueueItemSuggested {
			l.suggestions = append(l.suggestions, itemFromRow(r))
		} else {
			l.queue = append(l.queue, itemFromRow(r))
		}
	}
	playbackRate := row.PlaybackRate
	if playbackRate < minRate || playbackRate > maxRate {
		playbackRate = 1
	}
	l.state = playback{ref: now, rate: playbackRate, seq: 1}
	if len(l.queue) > 0 {
		// The saved current item goes first (it should already be); else the first item
		// starts from its beginning
		pos := itemStart(l.queue[0])
		if row.CurrentItemID != nil {
			if i := l.queueIndex(*row.CurrentItemID); i >= 0 {
				cur := l.queue[i]
				l.queue = append([]*item{cur}, append(l.queue[:i:i], l.queue[i+1:]...)...)
				end, _, _ := itemEnd(cur)
				pos = clampPosition(row.PositionS, end)
			}
		}
		l.state.itemID = l.queue[0].id
		l.state.position = pos
	}
	return l
}

// --- lookups (lock held) ---

func (l *lobby) queueIndex(id uint) int {
	for i, it := range l.queue {
		if it.id == id {
			return i
		}
	}
	return -1
}

func (l *lobby) suggestionIndex(id uint) int {
	for i, it := range l.suggestions {
		if it.id == id {
			return i
		}
	}
	return -1
}

func (l *lobby) current() *item {
	if l.state.itemID == 0 || len(l.queue) == 0 || l.queue[0].id != l.state.itemID {
		return nil
	}
	return l.queue[0]
}

func (l *lobby) isHost(userID uint) bool {
	return l.row.HostUserID != nil && *l.row.HostUserID == userID
}

// canControl tells whether a member controls playback and the queue. A lobby without host
// (only anonymous members left) is controlled by every member.
func (l *lobby) canControl(userID uint) bool {
	if l.isHost(userID) || l.row.ControlOpen || l.row.HostUserID == nil {
		return true
	}
	m := l.members[userID]
	return m != nil && m.canControl
}

func (l *lobby) canQueue(userID uint) bool {
	return l.canControl(userID) || l.row.QueueOpen
}

// connections counts the open connections of a user.
func (l *lobby) connections(userID uint) int {
	n := 0
	for c := range l.clients {
		if c.userID == userID {
			n++
		}
	}
	return n
}

// onlineUsers is the set of users with an open connection.
func (l *lobby) onlineUsers() map[uint]bool {
	out := make(map[uint]bool)
	for c := range l.clients {
		out[c.userID] = true
	}
	return out
}

// maxMembers is the effective online member limit.
func (l *lobby) maxMembers() int {
	limit := l.hub.cfg.MaxMembers
	if l.row.MaxMembers > 0 && (limit <= 0 || l.row.MaxMembers < limit) {
		limit = l.row.MaxMembers
	}
	return limit
}

// posNow is the current position, clamped to the item.
func (l *lobby) posNow(now time.Time) float64 {
	end, _, _ := itemEnd(l.current())
	return clampPosition(l.state.at(now), end)
}

// --- snapshots ---

func (l *lobby) info() LobbyInfo {
	return LobbyInfo{
		ID:          l.row.UUID.String(),
		Name:        l.row.Name,
		Visibility:  l.row.Visibility,
		HostID:      l.row.HostUserID,
		ControlOpen: l.row.ControlOpen,
		QueueOpen:   l.row.QueueOpen,
		WaitForAll:  l.row.WaitForAll,
		MaxMembers:  l.row.MaxMembers,
		AutoNext:    l.row.AutoNext,
		InviteCode:  l.row.InviteCode,
	}
}

// memberDTO is a member as sent: full (moderators) or redacted.
func memberDTO(m *member, full bool) Member {
	id := m.userID
	out := Member{UserID: &id, Username: m.username, Anonymous: m.anon > 0, AnonNumber: anonPtr(m.anon)}
	if m.anon > 0 && !full {
		out.UserID = nil
		out.Username = anonName(m.anon)
	}
	return out
}

// hasAnonymous tells whether some member, chat message or queue item is anonymous: only then
// do the moderators and the other members get different payloads.
func (l *lobby) hasAnonymous() bool {
	for _, m := range l.members {
		if m.anon > 0 {
			return true
		}
	}
	for _, m := range l.chat {
		if m.Anonymous || m.modText != "" {
			return true
		}
		for _, r := range m.reactions {
			for _, u := range r.users {
				if u.anon > 0 {
					return true
				}
			}
		}
	}
	for _, list := range [][]*item{l.queue, l.suggestions} {
		for _, it := range list {
			if it.addedAnon > 0 {
				return true
			}
		}
	}
	return false
}

func (l *lobby) memberList(full bool) []Member {
	conns := make(map[uint]int)
	buffering := make(map[uint]bool)
	for c := range l.clients {
		conns[c.userID]++
		if c.buffering {
			buffering[c.userID] = true
		}
	}
	ms := make([]*member, 0, len(l.members))
	for _, m := range l.members {
		ms = append(ms, m)
	}
	sort.Slice(ms, func(i, j int) bool {
		if !ms[i].joinedAt.Equal(ms[j].joinedAt) {
			return ms[i].joinedAt.Before(ms[j].joinedAt)
		}
		return ms[i].userID < ms[j].userID
	})
	out := make([]Member, 0, len(ms))
	for _, m := range ms {
		dto := memberDTO(m, full)
		dto.Host = l.isHost(m.userID)
		dto.Control = l.canControl(m.userID)
		dto.Online = conns[m.userID] > 0
		dto.Connections = conns[m.userID]
		dto.Buffering = buffering[m.userID]
		dto.JoinedAt = m.joinedAt.UTC().Format(time.RFC3339)
		out = append(out, dto)
	}
	return out
}

func itemList(items []*item, full bool) []QueueItem {
	out := make([]QueueItem, 0, len(items))
	for _, it := range items {
		out = append(out, it.dto(full))
	}
	return out
}

func (l *lobby) welcome(c *Client) welcomeMsg {
	full := c.moderate
	chat := make([]ChatMessage, len(l.chat))
	for i, m := range l.chat {
		chat[i] = m.view(full)
	}
	cfg := l.hub.cfg
	you := You{UserID: c.userID, Username: c.username, Moderate: c.moderate}
	if m := l.members[c.userID]; m != nil && m.anon > 0 {
		you.Anonymous, you.AnonNumber = true, anonPtr(m.anon)
	}
	var closingAt *int64
	if !l.closingAt.IsZero() {
		ms := l.closingAt.UnixMilli()
		closingAt = &ms
	}
	return welcomeMsg{
		T:          msgWelcome,
		V:          ProtocolVersion,
		ServerTime: l.hub.now().UnixMilli(),
		You:        you,
		Config: ClientConfig{
			DriftThreshold: cfg.DriftThreshold,
			DriftNudge:     cfg.DriftNudge,
			HoldTimeoutMs:  cfg.HoldTimeout.Milliseconds(),
			ChatMaxLength:  cfg.ChatMaxLength,
			EndGrace:       cfg.EndGrace.Seconds(),

			ChatMaxReactionKinds:    cfg.ChatMaxReactionKinds,
			ChatMaxReactionsPerUser: cfg.ChatMaxReactionsPerUser,
		},
		Lobby:       l.info(),
		Members:     l.memberList(full),
		Queue:       itemList(l.queue, full),
		Suggestions: itemList(l.suggestions, full),
		State:       l.state.snapshot(),
		Chat:        chat,
		ClosingAt:   closingAt,
	}
}

// --- broadcasting (lock held) ---

func (l *lobby) marshal(msg any) []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		l.hub.logger.Error("Failed to marshal a lobby message", zap.Error(err))
		return nil
	}
	return b
}

func (l *lobby) broadcast(msg any) {
	b := l.marshal(msg)
	if b == nil {
		return
	}
	for c := range l.clients {
		c.enqueue(b)
	}
}

func (l *lobby) sendTo(c *Client, msg any) {
	if b := l.marshal(msg); b != nil {
		c.enqueue(b)
	}
}

// broadcastSplit sends build(true) to the moderators and build(false) to the other members:
// for the payloads naming members (anonymous members are redacted for non-moderators). Each
// variant is marshaled once; without anonymous content everyone gets the same bytes.
func (l *lobby) broadcastSplit(build func(full bool) any) {
	if !l.hasAnonymous() {
		l.broadcast(build(false))
		return
	}
	var pub, full []byte
	for c := range l.clients {
		var b []byte
		if c.moderate {
			if full == nil {
				full = l.marshal(build(true))
			}
			b = full
		} else {
			if pub == nil {
				pub = l.marshal(build(false))
			}
			b = pub
		}
		if b != nil {
			c.enqueue(b)
		}
	}
}

func (l *lobby) broadcastState() { l.broadcast(stateMsg{T: msgState, State: l.state.snapshot()}) }

func (l *lobby) broadcastQueue() {
	l.broadcastSplit(func(full bool) any {
		return queueMsg{T: msgQueue, Queue: itemList(l.queue, full), Suggestions: itemList(l.suggestions, full)}
	})
}

func (l *lobby) broadcastMembers() {
	l.broadcastSplit(func(full bool) any {
		return membersMsg{T: msgMembers, Members: l.memberList(full)}
	})
}

func (l *lobby) broadcastLobby() { l.broadcast(lobbyMsg{T: msgLobby, Lobby: l.info()}) }

// addChat appends a message to the history (bounded) and broadcasts it.
func (l *lobby) addChat(msg ChatMessage) {
	l.nextMsgID++
	msg.ID = l.nextMsgID
	msg.At = l.hub.now().UnixMilli()
	l.chat = append(l.chat, msg)
	if limit := l.hub.cfg.ChatHistory; limit > 0 && len(l.chat) > limit {
		l.chat = append([]ChatMessage(nil), l.chat[len(l.chat)-limit:]...)
	} else if limit <= 0 {
		l.chat = nil
	}
	if !msg.Anonymous && msg.modText == "" {
		l.broadcast(chatMsg{T: msgChat, Message: msg})
		return
	}
	l.broadcastSplit(func(full bool) any { return chatMsg{T: msgChat, Message: msg.view(full)} })
}

func (l *lobby) system(text string) {
	l.addChat(ChatMessage{Text: text, System: true})
}

// systemAbout is a system message naming a member: "Anonymous #N" for everyone, with the
// real username for the moderators.
func (l *lobby) systemAbout(m *member, suffix string) {
	msg := ChatMessage{Text: m.display() + suffix, System: true}
	if m.anon > 0 {
		msg.modText = m.modDisplay() + suffix
	}
	l.addChat(msg)
}

// --- playback changes (lock held) ---

// changed bumps the state sequence and broadcasts the state.
func (l *lobby) changed() {
	l.state.seq++
	l.broadcastState()
}

// savePlayback stores the current item, position and rate (best effort).
func (l *lobby) savePlayback() {
	var itemID *uint
	if l.state.itemID != 0 {
		id := l.state.itemID
		itemID = &id
	}
	pos := l.posNow(l.hub.now())
	if err := l.hub.store.SavePlayback(l.row.ID, itemID, pos, l.state.rate); err != nil {
		l.hub.logger.Warn("Failed to save the lobby playback", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
	}
}

// syncPoint marks a new sync point (item change, seek): every connection must report ready
// again for it. The state sequence is bumped (the caller broadcasts).
func (l *lobby) syncPoint() {
	l.state.seq++
	l.syncSeq = l.state.seq
	l.endedReports = make(map[uint]bool)
}

// wantHold tells whether the lobby must wait for buffering members.
func (l *lobby) wantHold() bool {
	if !l.row.WaitForAll || !l.state.playing || l.state.itemID == 0 || l.holdTimedOutAt == l.syncSeq {
		return false
	}
	for c := range l.clients {
		if c.joinedSync == l.syncSeq {
			continue // joined after the sync point: late joiners catch up, nobody waits
		}
		if c.buffering || c.reportItem != l.state.itemID || c.reportSeq < l.syncSeq {
			return true
		}
	}
	return false
}

// applyHold starts or ends a hold as needed. It returns whether the state changed (the
// caller bumps the sequence and broadcasts).
func (l *lobby) applyHold(now time.Time) bool {
	want := l.wantHold()
	if want == l.state.hold {
		return false
	}
	if want {
		l.state.position = l.posNow(now)
		l.state.ref = now
		l.state.hold = true
		l.holdSince = now
		return true
	}
	l.state.ref = now
	l.state.hold = false
	return true
}

// startItem makes queue[0] the item playing, from its start.
func (l *lobby) startItem(now time.Time, playing bool) {
	if len(l.queue) == 0 {
		l.state.itemID = 0
		l.state.playing = false
		l.state.hold = false
		l.state.position = 0
		l.state.ref = now
		l.syncPoint()
		return
	}
	it := l.queue[0]
	l.state.itemID = it.id
	l.state.playing = playing
	l.state.hold = false
	l.state.position = itemStart(it)
	l.state.ref = now
	l.syncPoint()
	l.applyHold(now)
}

// itemTitle is the name of an item in system messages.
func itemTitle(it *item) string {
	title := it.title
	if title == "" {
		title = "Untitled scene"
	}
	if it.label != nil && *it.label != "" {
		title += " (" + *it.label + ")"
	}
	return title
}

// finishKind is why the item playing ends.
type finishKind int

const (
	finishEnded  finishKind = iota // it reached its end: the auto-next mode decides
	finishSkip                     // skipped: the next item starts (repeat_queue keeps it)
	finishRemove                   // removed from the queue: the next item starts
)

// finish ends the item playing. At its natural end the auto-next mode decides: stop pauses at
// the end, repeat_one starts it over, repeat_queue moves it to the end of the queue, next
// drops it. Skipping drops it too, except in repeat_queue (it goes to the end of the queue).
func (l *lobby) finish(now time.Time, kind finishKind) error {
	cur := l.current()
	if cur == nil {
		return nil
	}
	mode := l.row.AutoNext
	if kind == finishEnded {
		switch mode {
		case data.AutoNextStop:
			if !l.state.playing {
				return nil // already stopped
			}
			l.state.position = l.posNow(now)
			l.state.ref = now
			l.state.playing = false
			l.state.hold = false
			l.changed()
			l.savePlayback()
			return nil
		case data.AutoNextRepeatOne:
			l.restart(now)
			return nil
		}
	}
	if mode == data.AutoNextRepeatQueue && kind != finishRemove {
		if len(l.queue) == 1 {
			l.restart(now)
			return nil
		}
		next := make([]*item, 0, len(l.queue))
		next = append(next, l.queue[1:]...)
		next = append(next, cur)
		if err := l.hub.store.SetQueueOrder(l.row.ID, itemIDs(next)); err != nil {
			return err
		}
		l.queue = next
	} else {
		if err := l.hub.store.DeleteQueueItem(l.row.ID, cur.id); err != nil {
			return err
		}
		l.queue = l.queue[1:]
	}
	l.startItem(now, true)
	l.savePlayback()
	l.broadcastQueue()
	l.broadcastState()
	if next := l.current(); next != nil {
		l.system("Now playing: " + itemTitle(next))
	} else {
		l.system("The queue is finished")
	}
	return nil
}

// restart plays the current item again from its start (repeat).
func (l *lobby) restart(now time.Time) {
	l.startItem(now, true)
	l.restartedAt = now
	l.broadcastState()
	l.savePlayback()
}

// --- inactivity (lock held) ---

// activity records something a member did (playback, chat, queue, control): the inactivity
// close starts over, and an announced close is called off.
func (l *lobby) activity(now time.Time) {
	l.lastActivity = now
	if !l.closingAt.IsZero() {
		l.closingAt = time.Time{}
		l.broadcast(closingMsg{T: msgClosing})
	}
}

// checkInactivity closes a lobby with connected members but nothing happening for
// inactive_close_after, announced close_warning before. It returns whether the lobby closed.
func (l *lobby) checkInactivity(now time.Time) bool {
	cfg := l.hub.cfg
	if l.state.playing && l.state.itemID != 0 {
		l.activity(now) // playback running is activity
		return false
	}
	if cfg.InactiveCloseAfter <= 0 {
		return false
	}
	closeAt := l.lastActivity.Add(cfg.InactiveCloseAfter)
	if !now.Before(closeAt) {
		if err := l.closeWith(ByeInactive); err != nil {
			l.hub.logger.Error("Failed to close an inactive lobby", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
			return false
		}
		l.hub.logger.Info("Closed an inactive lobby", zap.Uint("lobby_id", l.row.ID))
		return true
	}
	if cfg.CloseWarning > 0 && l.closingAt.IsZero() && !now.Before(closeAt.Add(-cfg.CloseWarning)) {
		l.closingAt = closeAt
		at := closeAt.UnixMilli()
		l.broadcast(closingMsg{T: msgClosing, At: &at, Reason: closingInactive})
		l.system("Nothing happened here for a while: the lobby closes in " + humanDuration(closeAt.Sub(now)) +
			". Play something, chat or change the queue to keep it open.")
	}
	return false
}

// humanDuration writes a short duration for system messages ("5 minutes", "30 seconds").
func humanDuration(d time.Duration) string {
	if d >= time.Minute {
		n := int((d + time.Minute - 1) / time.Minute)
		if n == 1 {
			return "1 minute"
		}
		return strconv.Itoa(n) + " minutes"
	}
	n := int((d + time.Second - 1) / time.Second)
	if n <= 1 {
		return "1 second"
	}
	return strconv.Itoa(n) + " seconds"
}

// limiter gives the rate limiter of a user (chat or reactions).
func (l *lobby) limiter(m map[uint]*rate.Limiter, userID uint, perWindow int) *rate.Limiter {
	if lim, ok := m[userID]; ok {
		return lim
	}
	window := l.hub.cfg.RateWindow
	if window <= 0 {
		window = 10 * time.Second
	}
	if perWindow <= 0 {
		perWindow = 1
	}
	lim := rate.NewLimiter(rate.Every(window/time.Duration(perWindow)), perWindow)
	m[userID] = lim
	return lim
}
