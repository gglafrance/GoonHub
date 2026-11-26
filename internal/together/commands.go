package together

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"goonhub/internal/core"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

// maxItemsPerAdd bounds the items of one queue_add (or REST add).
const maxItemsPerAdd = 100

// maxLabelLength is the longest queue item label (characters).
const maxLabelLength = 100

// handle runs one client message (lobby lock held). A refused command returns a cmdError.
func (l *lobby) handle(c *Client, in *inbound, now time.Time) error {
	switch in.T {
	case cmdPing:
		l.sendTo(c, pongMsg{T: msgPong, ID: in.ID, C: in.C, S: now.UnixMilli()})
		return nil
	case cmdStatus:
		return l.cmdStatus(c, in, now)
	case cmdEnded:
		return l.cmdEnded(c, in, now)
	case cmdChat:
		return l.cmdChat(c, in, now)
	case cmdChatDelete:
		return l.cmdChatDelete(c, in)
	case cmdReaction:
		return l.cmdReaction(c, in, now)
	case cmdQueueAdd:
		if !l.hub.hasPermission(c.userID, core.PermScenesView) {
			return refuse(ErrCodeNotAllowed, "you cannot view scenes")
		}
		_, _, err := l.addItems(c.userID, c.username, in.Items, now)
		return err
	case cmdQueueRemove:
		return l.cmdQueueRemove(c, in, now)
	case cmdSuggestionReject:
		return l.cmdSuggestionReject(c, in)
	case cmdKick:
		return l.cmdKick(c, in)
	case cmdSettings, cmdGrantControl, cmdRevokeControl, cmdTransferHost:
		if !l.isHost(c.userID) {
			return refuse(ErrCodeNotAllowed, "only the host can do this")
		}
		return l.handleHost(c, in, now)
	case cmdPlay, cmdPause, cmdSeek, cmdRate, cmdSkip, cmdPlayItem, cmdQueueMove, cmdSuggestionAccept:
		if !l.canControl(c.userID) {
			return refuse(ErrCodeNotAllowed, "you do not have control of the lobby")
		}
		return l.handleControl(in, now)
	default:
		return refuse(ErrCodeUnknownType, "unknown message type")
	}
}

// handleControl runs the commands that need control.
func (l *lobby) handleControl(in *inbound, now time.Time) error {
	switch in.T {
	case cmdPlay:
		cur := l.current()
		if cur == nil {
			return refuse(ErrCodeInvalid, "the queue is empty")
		}
		end, _, _ := itemEnd(cur)
		pos := l.posNow(now)
		if in.Position != nil {
			pos = clampPosition(*in.Position, end)
		}
		l.state.position = pos
		l.state.ref = now
		l.state.playing = true
		l.applyHold(now)
		l.changed()
		return nil
	case cmdPause:
		if l.current() == nil {
			return refuse(ErrCodeInvalid, "the queue is empty")
		}
		end, _, _ := itemEnd(l.current())
		pos := l.posNow(now)
		if in.Position != nil {
			pos = clampPosition(*in.Position, end)
		}
		l.state.position = pos
		l.state.ref = now
		l.state.playing = false
		l.state.hold = false
		l.changed()
		l.savePlayback()
		return nil
	case cmdSeek:
		cur := l.current()
		if cur == nil {
			return refuse(ErrCodeInvalid, "the queue is empty")
		}
		if in.Position == nil {
			return refuse(ErrCodeInvalid, "position is required")
		}
		end, _, _ := itemEnd(cur)
		l.state.position = clampPosition(*in.Position, end)
		l.state.ref = now
		l.state.hold = false
		l.syncPoint()
		l.applyHold(now)
		l.broadcastState()
		l.savePlayback()
		return nil
	case cmdRate:
		if in.Rate == nil || *in.Rate < minRate || *in.Rate > maxRate {
			return refuse(ErrCodeInvalid, "rate must be between 0.25 and 4")
		}
		l.state.position = l.posNow(now)
		l.state.ref = now
		l.state.rate = *in.Rate
		l.changed()
		l.savePlayback()
		return nil
	case cmdSkip:
		if l.current() == nil {
			return refuse(ErrCodeInvalid, "the queue is empty")
		}
		return l.internal(l.advance(now))
	case cmdPlayItem:
		return l.cmdPlayItem(in, now)
	case cmdQueueMove:
		return l.cmdQueueMove(in)
	case cmdSuggestionAccept:
		return l.cmdSuggestionAccept(in, now)
	}
	return nil
}

// internal turns a storage error into a refused command (logged).
func (l *lobby) internal(err error) error {
	if err == nil {
		return nil
	}
	l.hub.logger.Error("Lobby storage error", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
	return refuse(ErrCodeInternal, "the change could not be saved")
}

func (l *lobby) cmdStatus(c *Client, in *inbound, now time.Time) error {
	before := c.buffering
	if in.Buffering != nil {
		c.buffering = *in.Buffering
	}
	if in.Seq != nil {
		c.reportSeq = *in.Seq
	}
	if in.ItemID != nil {
		c.reportItem = *in.ItemID
	}
	if before != c.buffering {
		l.broadcastMembers()
	}
	if l.applyHold(now) {
		l.changed()
	}
	return nil
}

func (l *lobby) cmdEnded(c *Client, in *inbound, now time.Time) error {
	cur := l.current()
	if cur == nil || in.ItemID == nil || *in.ItemID != cur.id {
		return nil // another item: already advanced, or a late report
	}
	end, known, _ := itemEnd(cur)
	if known {
		if l.state.at(now) < end-endedTolerance {
			return nil // too early: a broken stream must not skip for everyone
		}
		return l.internal(l.advance(now))
	}
	if l.canControl(c.userID) {
		return l.internal(l.advance(now))
	}
	l.endedReports[c.userID] = true
	online := l.onlineUsers()
	reported := 0
	for uid := range l.endedReports {
		if online[uid] {
			reported++
		}
	}
	if reported*2 >= len(online) {
		return l.internal(l.advance(now))
	}
	return nil
}

func (l *lobby) cmdPlayItem(in *inbound, now time.Time) error {
	if in.ItemID == nil {
		return refuse(ErrCodeInvalid, "item_id is required")
	}
	i := l.queueIndex(*in.ItemID)
	if i < 0 {
		return refuse(ErrCodeNotFound, "no such queue item")
	}
	if i == 0 {
		// The item playing: start it over
		l.startItem(now, true)
		l.broadcastState()
		l.savePlayback()
		return nil
	}
	target := l.queue[i]
	rest := make([]*item, 0, len(l.queue)-1)
	rest = append(rest, target)
	for j, it := range l.queue {
		if j == 0 || j == i {
			continue
		}
		rest = append(rest, it)
	}
	if err := l.hub.store.DeleteQueueItem(l.row.ID, l.queue[0].id); err != nil {
		return l.internal(err)
	}
	if err := l.hub.store.SetQueueOrder(l.row.ID, itemIDs(rest)); err != nil {
		return l.internal(err)
	}
	l.queue = rest
	l.startItem(now, true)
	l.savePlayback()
	l.broadcastQueue()
	l.broadcastState()
	l.system("Now playing: " + itemTitle(target))
	return nil
}

func itemIDs(items []*item) []uint {
	ids := make([]uint, len(items))
	for i, it := range items {
		ids[i] = it.id
	}
	return ids
}

func (l *lobby) cmdQueueMove(in *inbound) error {
	if in.ItemID == nil || in.Index == nil {
		return refuse(ErrCodeInvalid, "item_id and index are required")
	}
	i := l.queueIndex(*in.ItemID)
	first := 0
	if l.current() != nil {
		first = 1
	}
	if i < first {
		return refuse(ErrCodeNotFound, "no such upcoming item")
	}
	moved := l.queue[i]
	upcoming := make([]*item, 0, len(l.queue))
	for j := first; j < len(l.queue); j++ {
		if j != i {
			upcoming = append(upcoming, l.queue[j])
		}
	}
	idx := *in.Index
	if idx < 0 {
		idx = 0
	}
	if idx > len(upcoming) {
		idx = len(upcoming)
	}
	upcoming = append(upcoming[:idx], append([]*item{moved}, upcoming[idx:]...)...)
	next := append(append([]*item{}, l.queue[:first]...), upcoming...)
	if err := l.hub.store.SetQueueOrder(l.row.ID, itemIDs(next)); err != nil {
		return l.internal(err)
	}
	l.queue = next
	l.broadcastQueue()
	return nil
}

func (l *lobby) cmdSuggestionAccept(in *inbound, now time.Time) error {
	if in.ItemID == nil {
		return refuse(ErrCodeInvalid, "item_id is required")
	}
	i := l.suggestionIndex(*in.ItemID)
	if i < 0 {
		return refuse(ErrCodeNotFound, "no such suggestion")
	}
	if max := l.hub.cfg.MaxQueueItems; max > 0 && len(l.queue) >= max {
		return refuse(ErrCodeQueueFull, "the queue is full")
	}
	accepted := l.suggestions[i]
	next := append(append([]*item{}, l.queue...), accepted)
	if err := l.hub.store.SetQueueOrder(l.row.ID, itemIDs(next)); err != nil {
		return l.internal(err)
	}
	l.queue = next
	l.suggestions = append(l.suggestions[:i:i], l.suggestions[i+1:]...)
	if l.current() == nil {
		l.startItem(now, false)
		l.broadcastState()
		l.savePlayback()
	}
	l.broadcastQueue()
	return nil
}

func (l *lobby) cmdQueueRemove(c *Client, in *inbound, now time.Time) error {
	if in.ItemID == nil {
		return refuse(ErrCodeInvalid, "item_id is required")
	}
	if i := l.suggestionIndex(*in.ItemID); i >= 0 {
		return l.cmdSuggestionReject(c, in)
	}
	i := l.queueIndex(*in.ItemID)
	if i < 0 {
		return refuse(ErrCodeNotFound, "no such queue item")
	}
	if !l.canControl(c.userID) {
		return refuse(ErrCodeNotAllowed, "you do not have control of the lobby")
	}
	if i == 0 && l.current() != nil {
		return l.internal(l.advance(now))
	}
	if err := l.hub.store.DeleteQueueItem(l.row.ID, *in.ItemID); err != nil {
		return l.internal(err)
	}
	l.queue = append(l.queue[:i:i], l.queue[i+1:]...)
	l.broadcastQueue()
	return nil
}

func (l *lobby) cmdSuggestionReject(c *Client, in *inbound) error {
	if in.ItemID == nil {
		return refuse(ErrCodeInvalid, "item_id is required")
	}
	i := l.suggestionIndex(*in.ItemID)
	if i < 0 {
		return refuse(ErrCodeNotFound, "no such suggestion")
	}
	s := l.suggestions[i]
	own := s.addedBy != nil && *s.addedBy == c.userID
	if !own && !l.canControl(c.userID) {
		return refuse(ErrCodeNotAllowed, "you do not have control of the lobby")
	}
	if err := l.hub.store.DeleteQueueItem(l.row.ID, s.id); err != nil {
		return l.internal(err)
	}
	l.suggestions = append(l.suggestions[:i:i], l.suggestions[i+1:]...)
	l.broadcastQueue()
	return nil
}

// validateItems checks the items to add and gives them with their scene.
func (l *lobby) validateItems(inputs []ItemInput) ([]ItemInput, map[uint]data.Scene, error) {
	if len(inputs) == 0 {
		return nil, nil, refuse(ErrCodeInvalid, "no item to add")
	}
	if len(inputs) > maxItemsPerAdd {
		return nil, nil, refuse(ErrCodeInvalid, "too many items at once")
	}
	ids := make([]uint, 0, len(inputs))
	for i, in := range inputs {
		if in.SceneID == 0 {
			return nil, nil, refuse(ErrCodeInvalid, "scene_id is required")
		}
		if in.Start != nil && *in.Start < 0 {
			return nil, nil, refuse(ErrCodeInvalid, "start must not be negative")
		}
		if in.End != nil {
			start := 0.0
			if in.Start != nil {
				start = *in.Start
			}
			if *in.End <= start {
				return nil, nil, refuse(ErrCodeInvalid, "end must be after start")
			}
		}
		if in.Label != nil {
			label := strings.TrimSpace(*in.Label)
			if utf8.RuneCountInString(label) > maxLabelLength {
				return nil, nil, refuse(ErrCodeTooLong, "label is too long")
			}
			if label == "" {
				inputs[i].Label = nil
			} else {
				inputs[i].Label = &label
			}
		}
		ids = append(ids, in.SceneID)
	}
	scenes, err := l.hub.scenes.GetByIDs(ids)
	if err != nil {
		return nil, nil, l.internal(err)
	}
	byID := make(map[uint]data.Scene, len(scenes))
	for _, s := range scenes {
		byID[s.ID] = s
	}
	valid := inputs[:0:0]
	for _, in := range inputs {
		if _, ok := byID[in.SceneID]; ok {
			valid = append(valid, in)
		}
	}
	if len(valid) == 0 {
		return nil, nil, refuse(ErrCodeNotFound, "no such scene")
	}
	return valid, byID, nil
}

// addItems adds items to the queue when the user may, else as suggestions. The first item
// queued into an empty queue becomes the current one, paused at its start.
func (l *lobby) addItems(userID uint, username string, inputs []ItemInput, now time.Time) (queued, suggested int, err error) {
	valid, scenes, err := l.validateItems(inputs)
	if err != nil {
		return 0, 0, err
	}
	toQueue := l.canQueue(userID)
	status := data.QueueItemSuggested
	have, limit := len(l.suggestions), l.hub.cfg.MaxSuggestions
	if toQueue {
		status = data.QueueItemQueued
		have, limit = len(l.queue), l.hub.cfg.MaxQueueItems
	}
	if limit > 0 && have+len(valid) > limit {
		return 0, 0, refuse(ErrCodeQueueFull, "the queue is full")
	}
	adder := userID
	rows := make([]data.LobbyQueueItem, len(valid))
	for i, in := range valid {
		rows[i] = data.LobbyQueueItem{
			LobbyID:  l.row.ID,
			SceneID:  in.SceneID,
			Status:   status,
			Position: have + i,
			StartS:   in.Start,
			EndS:     in.End,
			Label:    in.Label,
			AddedBy:  &adder,
		}
	}
	if err := l.hub.store.AddQueueItems(rows); err != nil {
		return 0, 0, l.internal(err)
	}
	added := make([]*item, len(rows))
	for i, r := range rows {
		s := scenes[r.SceneID]
		r.SceneTitle = s.Title
		r.SceneDuration = s.Duration
		r.AddedByName = username
		added[i] = itemFromRow(r)
	}
	if !toQueue {
		l.suggestions = append(l.suggestions, added...)
		l.broadcastQueue()
		l.system(username + " suggested " + itemTitle(added[0]) + plural(len(added)-1))
		return 0, len(added), nil
	}
	wasEmpty := l.current() == nil
	l.queue = append(l.queue, added...)
	if wasEmpty {
		l.startItem(now, false)
		l.broadcastState()
		l.savePlayback()
	}
	l.broadcastQueue()
	return len(added), 0, nil
}

func plural(more int) string {
	switch {
	case more <= 0:
		return ""
	case more == 1:
		return " and 1 more"
	default:
		return " and " + strconv.Itoa(more) + " more"
	}
}

// --- chat and reactions ---

// cleanChat trims a chat text and refuses control characters (newlines are kept).
func cleanChat(text string, maxLen int) (string, error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return "", refuse(ErrCodeInvalid, "empty message")
	}
	if !utf8.ValidString(text) {
		return "", refuse(ErrCodeInvalid, "invalid text")
	}
	if maxLen > 0 && utf8.RuneCountInString(text) > maxLen {
		return "", refuse(ErrCodeTooLong, "message is too long")
	}
	for _, r := range text {
		if r != '\n' && unicode.IsControl(r) {
			return "", refuse(ErrCodeInvalid, "invalid character in message")
		}
	}
	return text, nil
}

func (l *lobby) cmdChat(c *Client, in *inbound, now time.Time) error {
	if in.Text == nil {
		return refuse(ErrCodeInvalid, "text is required")
	}
	text, err := cleanChat(*in.Text, l.hub.cfg.ChatMaxLength)
	if err != nil {
		return err
	}
	if !l.limiter(l.chatLimiters, c.userID, l.hub.cfg.ChatRateLimit).AllowN(now, 1) {
		return refuse(ErrCodeRateLimited, "slow down")
	}
	uid, name := c.userID, c.username
	l.addChat(ChatMessage{UserID: &uid, Username: &name, Text: text})
	return nil
}

func (l *lobby) cmdChatDelete(c *Client, in *inbound) error {
	var id int64
	if err := json.Unmarshal(in.ID, &id); err != nil || id <= 0 {
		return refuse(ErrCodeInvalid, "id is required")
	}
	for i, m := range l.chat {
		if m.ID != id {
			continue
		}
		own := m.UserID != nil && *m.UserID == c.userID
		if !own && !l.isHost(c.userID) && !l.hub.hasPermission(c.userID, core.PermTogetherModerate) {
			return refuse(ErrCodeNotAllowed, "you cannot delete this message")
		}
		l.chat = append(l.chat[:i:i], l.chat[i+1:]...)
		l.broadcast(chatDeletedMsg{T: msgChatDeleted, ID: id})
		return nil
	}
	return refuse(ErrCodeNotFound, "no such message")
}

// validEmoji accepts a short emoji sequence: non-ASCII, no whitespace nor markup.
func validEmoji(s string) bool {
	if s == "" || len(s) > 32 || !utf8.ValidString(s) {
		return false
	}
	nonASCII := false
	for _, r := range s {
		switch {
		case r >= 0x80:
			nonASCII = true
		case r == '#' || r == '*' || (r >= '0' && r <= '9'):
			// keycap emoji
		default:
			return false
		}
		if unicode.IsSpace(r) || (unicode.IsControl(r) && r != 0x200d) {
			return false
		}
	}
	return nonASCII
}

func (l *lobby) cmdReaction(c *Client, in *inbound, now time.Time) error {
	if in.Emote == nil {
		return refuse(ErrCodeInvalid, "emote is required")
	}
	emote := *in.Emote
	if len(emote) > 2 && strings.HasPrefix(emote, ":") && strings.HasSuffix(emote, ":") {
		if l.hub.emotes == nil || !l.hub.emotes.HasEmote(emote[1:len(emote)-1]) {
			return refuse(ErrCodeNotFound, "no such emote")
		}
	} else if !validEmoji(emote) {
		return refuse(ErrCodeInvalid, "not an emoji")
	}
	if !l.limiter(l.reactionLimiters, c.userID, l.hub.cfg.ReactionRateLimit).AllowN(now, 1) {
		return refuse(ErrCodeRateLimited, "slow down")
	}
	l.broadcast(reactionMsg{T: msgReaction, UserID: c.userID, Username: c.username, Emote: emote})
	return nil
}

// --- host and moderation ---

func (l *lobby) handleHost(c *Client, in *inbound, now time.Time) error {
	switch in.T {
	case cmdSettings:
		return l.cmdSettings(in, now)
	case cmdGrantControl, cmdRevokeControl:
		if in.UserID == nil || l.members[*in.UserID] == nil {
			return refuse(ErrCodeNotFound, "no such member")
		}
		grant := in.T == cmdGrantControl
		m := l.members[*in.UserID]
		if m.canControl == grant {
			return nil
		}
		if err := l.hub.store.SetMemberControl(l.row.ID, m.userID, grant); err != nil {
			return l.internal(err)
		}
		m.canControl = grant
		l.broadcastMembers()
		return nil
	case cmdTransferHost:
		if in.UserID == nil || l.members[*in.UserID] == nil {
			return refuse(ErrCodeNotFound, "no such member")
		}
		if *in.UserID == c.userID {
			return nil
		}
		if l.connections(*in.UserID) == 0 {
			return refuse(ErrCodeInvalid, "the new host must be online")
		}
		return l.internal(l.setHost(l.members[*in.UserID], now))
	}
	return nil
}

// setHost gives the host role to a member.
func (l *lobby) setHost(m *member, now time.Time) error {
	id := m.userID
	if err := l.hub.store.SetHost(l.row.ID, &id); err != nil {
		return err
	}
	l.row.HostUserID = &id
	l.hostAwaySince = now
	if l.connections(id) > 0 {
		l.hostAwaySince = time.Time{}
	}
	l.broadcastLobby()
	l.broadcastMembers()
	l.system(m.username + " is now the host")
	return nil
}

func (l *lobby) cmdSettings(in *inbound, now time.Time) error {
	next := l.row
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return refuse(ErrCodeInvalid, "name is required")
		}
		if utf8.RuneCountInString(name) > maxNameLength {
			return refuse(ErrCodeTooLong, "name is too long")
		}
		next.Name = name
	}
	if in.Visibility != nil {
		if *in.Visibility != data.LobbyVisibilityPublic && *in.Visibility != data.LobbyVisibilityPrivate {
			return refuse(ErrCodeInvalid, "visibility must be public or private")
		}
		next.Visibility = *in.Visibility
	}
	if in.ControlOpen != nil {
		next.ControlOpen = *in.ControlOpen
	}
	if in.QueueOpen != nil {
		next.QueueOpen = *in.QueueOpen
	}
	if in.WaitForAll != nil {
		next.WaitForAll = *in.WaitForAll
	}
	if in.MaxMembers != nil {
		if *in.MaxMembers < 0 || (l.hub.cfg.MaxMembers > 0 && *in.MaxMembers > l.hub.cfg.MaxMembers) {
			return refuse(ErrCodeInvalid, "max_members is out of range")
		}
		next.MaxMembers = *in.MaxMembers
	}
	if err := l.hub.store.UpdateSettings(&next); err != nil {
		return l.internal(err)
	}
	l.row = next
	l.broadcastLobby()
	l.broadcastMembers()
	if l.applyHold(now) {
		l.changed()
	}
	return nil
}

func (l *lobby) cmdKick(c *Client, in *inbound) error {
	if in.UserID == nil {
		return refuse(ErrCodeInvalid, "user_id is required")
	}
	if !l.isHost(c.userID) && !l.hub.hasPermission(c.userID, core.PermTogetherModerate) {
		return refuse(ErrCodeNotAllowed, "only the host or a moderator can do this")
	}
	target := *in.UserID
	if target == c.userID || l.isHost(target) {
		return refuse(ErrCodeNotAllowed, "the host cannot be kicked")
	}
	m := l.members[target]
	if m == nil && !in.Ban {
		return refuse(ErrCodeNotFound, "no such member")
	}
	return l.internal(l.removeMember(target, in.Ban))
}

// removeMember kicks (or bans) a user: membership removed and connections closed.
func (l *lobby) removeMember(userID uint, ban bool) error {
	name := ""
	if m := l.members[userID]; m != nil {
		name = m.username
	}
	if ban {
		if err := l.hub.store.BanMember(l.row.ID, userID, l.hub.now().UTC()); err != nil {
			return err
		}
		l.banned[userID] = true
	} else if err := l.hub.store.RemoveMember(l.row.ID, userID); err != nil {
		return err
	}
	delete(l.members, userID)
	reason, code := ByeKicked, CloseKicked
	if ban {
		reason, code = ByeBanned, CloseForbidden
	}
	l.dropUser(userID, reason, code)
	l.broadcastMembers()
	if name != "" {
		if ban {
			l.system(name + " was banned")
		} else {
			l.system(name + " was kicked")
		}
	}
	return nil
}

// dropUser closes every connection of a user, with a bye message.
func (l *lobby) dropUser(userID uint, reason string, code int) {
	bye := l.marshal(byeMsg{T: msgBye, Reason: reason})
	for cl := range l.clients {
		if cl.userID != userID {
			continue
		}
		if bye != nil {
			cl.enqueue(bye)
		}
		cl.close(code, reason)
		delete(l.clients, cl)
	}
}
