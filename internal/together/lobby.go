package together

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"goonhub/internal/data"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// member is a (non banned) lobby member.
type member struct {
	userID     uint
	username   string
	canControl bool
	joinedAt   time.Time
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
	}
}

func (it *item) dto() QueueItem {
	q := QueueItem{
		ID:       it.id,
		SceneID:  it.sceneID,
		Title:    it.title,
		Duration: it.duration,
		Start:    it.start,
		End:      it.end,
		Label:    it.label,
	}
	if it.addedBy != nil {
		q.AddedBy = &UserRef{ID: *it.addedBy, Username: it.addedByName}
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

	chatLimiters     map[uint]*rate.Limiter
	reactionLimiters map[uint]*rate.Limiter

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
		syncSeq:          1,
	}
	for _, m := range members {
		if m.BannedAt != nil {
			l.banned[m.UserID] = true
			continue
		}
		l.members[m.UserID] = &member{userID: m.UserID, username: m.Username, canControl: m.CanControl, joinedAt: m.JoinedAt}
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

func (l *lobby) canControl(userID uint) bool {
	if l.isHost(userID) || l.row.ControlOpen {
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
		InviteCode:  l.row.InviteCode,
	}
}

func (l *lobby) memberList() []Member {
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
		out = append(out, Member{
			UserID:      m.userID,
			Username:    m.username,
			Host:        l.isHost(m.userID),
			Control:     l.canControl(m.userID),
			Online:      conns[m.userID] > 0,
			Connections: conns[m.userID],
			Buffering:   buffering[m.userID],
			JoinedAt:    m.joinedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func itemList(items []*item) []QueueItem {
	out := make([]QueueItem, 0, len(items))
	for _, it := range items {
		out = append(out, it.dto())
	}
	return out
}

func (l *lobby) welcome(c *Client) welcomeMsg {
	chat := make([]ChatMessage, len(l.chat))
	copy(chat, l.chat)
	cfg := l.hub.cfg
	return welcomeMsg{
		T:          msgWelcome,
		V:          ProtocolVersion,
		ServerTime: l.hub.now().UnixMilli(),
		You:        You{UserID: c.userID, Username: c.username, Moderate: c.moderate},
		Config: ClientConfig{
			DriftThreshold: cfg.DriftThreshold,
			DriftNudge:     cfg.DriftNudge,
			HoldTimeoutMs:  cfg.HoldTimeout.Milliseconds(),
			ChatMaxLength:  cfg.ChatMaxLength,
			EndGrace:       cfg.EndGrace.Seconds(),
		},
		Lobby:       l.info(),
		Members:     l.memberList(),
		Queue:       itemList(l.queue),
		Suggestions: itemList(l.suggestions),
		State:       l.state.snapshot(),
		Chat:        chat,
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

func (l *lobby) broadcastState() { l.broadcast(stateMsg{T: msgState, State: l.state.snapshot()}) }

func (l *lobby) broadcastQueue() {
	l.broadcast(queueMsg{T: msgQueue, Queue: itemList(l.queue), Suggestions: itemList(l.suggestions)})
}

func (l *lobby) broadcastMembers() { l.broadcast(membersMsg{T: msgMembers, Members: l.memberList()}) }

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
	l.broadcast(chatMsg{T: msgChat, Message: msg})
}

func (l *lobby) system(text string) {
	l.addChat(ChatMessage{Text: text, System: true})
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

// advance ends the item playing: it leaves the queue and the next one starts.
func (l *lobby) advance(now time.Time) error {
	cur := l.current()
	if cur == nil {
		return nil
	}
	if err := l.hub.store.DeleteQueueItem(l.row.ID, cur.id); err != nil {
		return err
	}
	l.queue = l.queue[1:]
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
