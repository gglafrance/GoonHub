package together

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/lifecycle"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// maxNameLength is the longest lobby name (characters).
const maxNameLength = 100

// inviteAlphabet has no ambiguous characters (0/O, 1/I/L).
const inviteAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

const inviteCodeLength = 8

// SceneLookup finds scenes (not in the trash) by ID.
type SceneLookup interface {
	GetByIDs(ids []uint) ([]data.Scene, error)
}

// AccessResolver resolves the live permissions of a user (core.RBACService).
type AccessResolver interface {
	ResolveUser(userID uint) (*core.UserAccess, error)
}

// EmoteNames tells whether a server emote exists (EmoteService).
type EmoteNames interface {
	HasEmote(name string) bool
}

// ErrLobbyFull is returned by Connect when the lobby has no room left for the user.
var ErrLobbyFull = errors.New("lobby full")

// Hub holds the live lobbies, in one process. A lobby is loaded from the database when it is
// used and unloaded when nobody is connected anymore. One goroutine (Start) runs the ticks.
type Hub struct {
	cfg    config.Goon2getherConfig
	store  data.LobbyRepository
	scenes SceneLookup
	access AccessResolver
	emotes EmoteNames
	logger *zap.Logger
	now    func() time.Time

	mu          sync.Mutex
	lobbies     map[uuid.UUID]*lobby
	lastCleanup time.Time

	lc       *lifecycle.Manager
	stopOnce sync.Once
}

// NewHub creates the hub (Start runs its loop).
func NewHub(cfg config.Goon2getherConfig, store data.LobbyRepository, scenes SceneLookup, access AccessResolver, logger *zap.Logger) *Hub {
	return &Hub{
		cfg:     cfg,
		store:   store,
		scenes:  scenes,
		access:  access,
		logger:  logger.With(zap.String("component", "goon2gether")),
		now:     func() time.Time { return time.Now() },
		lobbies: make(map[uuid.UUID]*lobby),
	}
}

// SetEmotes wires the emote names used to validate reactions.
func (h *Hub) SetEmotes(e EmoteNames) { h.emotes = e }

// Config is the Goon2gether configuration.
func (h *Hub) Config() config.Goon2getherConfig { return h.cfg }

// Start runs the tick loop (auto-advance, holds, host grace, unloading, idle cleanup).
func (h *Hub) Start() {
	if h.lc != nil {
		return
	}
	h.lc = lifecycle.NewManager(h.logger)
	interval := h.cfg.TickInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	h.lc.Go("goon2gether-hub", func(done <-chan struct{}) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				h.Tick()
			}
		}
	})
}

// Stop ends the loop, saves the playback of the loaded lobbies and closes every connection.
func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		if h.lc != nil {
			if err := h.lc.Shutdown(5 * time.Second); err != nil {
				h.logger.Warn("Goon2gether loop did not stop in time", zap.Error(err))
			}
		}
		h.mu.Lock()
		lobbies := make([]*lobby, 0, len(h.lobbies))
		for _, l := range h.lobbies {
			lobbies = append(lobbies, l)
		}
		h.mu.Unlock()
		for _, l := range lobbies {
			l.mu.Lock()
			l.savePlayback()
			for c := range l.clients {
				c.close(1001, "server shutdown")
			}
			l.mu.Unlock()
		}
	})
}

func (h *Hub) hasPermission(userID uint, perm string) bool {
	if h.access == nil {
		return false
	}
	a, err := h.access.ResolveUser(userID)
	if err != nil {
		return false
	}
	return a.Has(perm)
}

// newInviteCode draws a random invite code.
func newInviteCode() (string, error) {
	b := make([]byte, inviteCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to draw an invite code: %w", err)
	}
	for i := range b {
		b[i] = inviteAlphabet[int(b[i])%len(inviteAlphabet)]
	}
	return string(b), nil
}

// NormalizeInviteCode upper-cases a typed code and drops spaces and dashes.
func NormalizeInviteCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.NewReplacer(" ", "", "-", "").Replace(code)
}

// --- loading ---

// get returns the loaded lobby, loading it if needed.
func (h *Hub) get(id uuid.UUID) (*lobby, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if l, ok := h.lobbies[id]; ok {
		return l, nil
	}
	row, err := h.store.GetByUUID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("lobby", id.String())
		}
		return nil, apperrors.NewInternalError("failed to load the lobby", err)
	}
	members, err := h.store.ListMembers(row.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load the lobby members", err)
	}
	items, err := h.store.ListQueue(row.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load the lobby queue", err)
	}
	l := newLobby(h, *row, members, items)
	h.lobbies[id] = l
	return l, nil
}

// withLobby runs fn on the loaded lobby, its lock held. A lobby unloaded meanwhile is loaded
// again; a closed lobby is forgotten afterwards.
func (h *Hub) withLobby(id uuid.UUID, fn func(l *lobby) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		l, err := h.get(id)
		if err != nil {
			return err
		}
		l.mu.Lock()
		if l.unloaded {
			l.mu.Unlock()
			continue
		}
		if l.closed {
			l.mu.Unlock()
			return apperrors.NewNotFoundError("lobby", id.String())
		}
		err = fn(l)
		closed := l.closed
		l.mu.Unlock()
		if closed {
			h.forget(l)
		}
		return err
	}
	return apperrors.NewInternalError("the lobby is busy, try again", nil)
}

// forget drops a lobby from memory.
func (h *Hub) forget(l *lobby) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.lobbies[l.row.UUID]; ok && cur == l {
		delete(h.lobbies, l.row.UUID)
	}
}

// --- REST operations ---

// Actor is the user calling the hub, with their live access.
type Actor struct {
	ID       uint
	Username string
	Access   *core.UserAccess
}

func (a Actor) moderate() bool { return a.Access.Has(core.PermTogetherModerate) }

// CreateInput are the settings of a new lobby.
type CreateInput struct {
	Name        string      `json:"name"`
	Visibility  string      `json:"visibility"`
	ControlOpen bool        `json:"control_open"`
	QueueOpen   bool        `json:"queue_open"`
	WaitForAll  bool        `json:"wait_for_all"`
	MaxMembers  int         `json:"max_members"`
	Items       []ItemInput `json:"items"`
}

// NowPlaying is the item playing in a lobby summary.
type NowPlaying struct {
	SceneID uint    `json:"scene_id"`
	Title   string  `json:"title,omitempty"`
	Label   *string `json:"label"`
}

// Summary is a lobby as listed (docs/GOON2GETHER.md, "Lobby summary").
type Summary struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Visibility  string      `json:"visibility"`
	Host        *UserRef    `json:"host"`
	OnlineCount int         `json:"online_count"`
	QueueLength int         `json:"queue_length"`
	NowPlaying  *NowPlaying `json:"now_playing"`
	ControlOpen bool        `json:"control_open"`
	QueueOpen   bool        `json:"queue_open"`
	WaitForAll  bool        `json:"wait_for_all"`
	MaxMembers  int         `json:"max_members"`
	IsMember    bool        `json:"is_member"`
	InviteCode  string      `json:"invite_code,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	lastActive  time.Time
}

// summary describes the loaded lobby for a user (lock held).
func (l *lobby) summary(a Actor) Summary {
	s := Summary{
		ID:          l.row.UUID.String(),
		Name:        l.row.Name,
		Visibility:  l.row.Visibility,
		OnlineCount: len(l.onlineUsers()),
		QueueLength: len(l.queue),
		ControlOpen: l.row.ControlOpen,
		QueueOpen:   l.row.QueueOpen,
		WaitForAll:  l.row.WaitForAll,
		MaxMembers:  l.row.MaxMembers,
		IsMember:    l.members[a.ID] != nil,
		CreatedAt:   l.row.CreatedAt,
		lastActive:  l.row.LastActiveAt,
	}
	if len(l.clients) > 0 {
		s.lastActive = l.hub.now()
	}
	if l.row.HostUserID != nil {
		ref := &UserRef{ID: *l.row.HostUserID}
		if m := l.members[*l.row.HostUserID]; m != nil {
			ref.Username = m.username
		}
		s.Host = ref
	}
	if cur := l.current(); cur != nil {
		s.NowPlaying = &NowPlaying{SceneID: cur.sceneID, Label: cur.label}
		if a.Access.Has(core.PermScenesView) {
			s.NowPlaying.Title = cur.title
		}
	}
	if s.IsMember || a.moderate() {
		s.InviteCode = l.row.InviteCode
	}
	return s
}

func summaryFromEntry(e data.LobbyListEntry, a Actor) Summary {
	s := Summary{
		ID:          e.UUID.String(),
		Name:        e.Name,
		Visibility:  e.Visibility,
		QueueLength: e.QueueLength,
		ControlOpen: e.ControlOpen,
		QueueOpen:   e.QueueOpen,
		WaitForAll:  e.WaitForAll,
		MaxMembers:  e.MaxMembers,
		IsMember:    e.IsMember,
		CreatedAt:   e.CreatedAt,
		lastActive:  e.LastActiveAt,
	}
	if e.HostUserID != nil {
		s.Host = &UserRef{ID: *e.HostUserID, Username: e.HostUsername}
	}
	if e.NowSceneID != nil {
		s.NowPlaying = &NowPlaying{SceneID: *e.NowSceneID, Label: e.NowLabel}
		if a.Access.Has(core.PermScenesView) {
			s.NowPlaying.Title = e.NowSceneTitle
		}
	}
	if s.IsMember || a.moderate() {
		s.InviteCode = e.InviteCode
	}
	return s
}

// visible tells whether the user may see the lobby (lock held).
func (l *lobby) visible(a Actor) bool {
	return l.row.Visibility == data.LobbyVisibilityPublic || l.members[a.ID] != nil || a.moderate()
}

// CreateLobby creates a lobby hosted by the actor, with its first queue items.
func (h *Hub) CreateLobby(a Actor, in CreateInput) (*Summary, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = a.Username + "'s lobby"
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return nil, apperrors.NewValidationErrorWithField("name", "name is too long")
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = data.LobbyVisibilityPrivate
	}
	if visibility != data.LobbyVisibilityPublic && visibility != data.LobbyVisibilityPrivate {
		return nil, apperrors.NewValidationErrorWithField("visibility", "visibility must be public or private")
	}
	if in.MaxMembers < 0 || (h.cfg.MaxMembers > 0 && in.MaxMembers > h.cfg.MaxMembers) {
		return nil, apperrors.NewValidationErrorWithField("max_members", "max_members is out of range")
	}
	if limit := h.cfg.MaxLobbiesPerUser; limit > 0 {
		n, err := h.store.CountHostedBy(a.ID)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to count the lobbies", err)
		}
		if n >= int64(limit) {
			return nil, apperrors.NewValidationError(fmt.Sprintf("you already host %d lobbies, close one first", n))
		}
	}
	if len(in.Items) > 0 && !a.Access.Has(core.PermScenesView) {
		return nil, apperrors.NewForbiddenError("you cannot view scenes")
	}
	now := h.now().UTC()
	hostID := a.ID
	row := &data.Lobby{
		Name:         name,
		Visibility:   visibility,
		HostUserID:   &hostID,
		CreatedBy:    &hostID,
		ControlOpen:  in.ControlOpen,
		QueueOpen:    in.QueueOpen,
		WaitForAll:   in.WaitForAll,
		MaxMembers:   in.MaxMembers,
		PlaybackRate: 1,
		LastActiveAt: now,
	}
	host := &data.LobbyMember{UserID: a.ID, JoinedAt: now}
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if row.InviteCode, err = newInviteCode(); err != nil {
			return nil, apperrors.NewInternalError("failed to create the lobby", err)
		}
		row.ID, row.UUID = 0, uuid.Nil
		if err = h.store.Create(row, host, nil); !errors.Is(err, data.ErrInviteCodeTaken) {
			break
		}
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create the lobby", err)
	}
	var summary Summary
	err = h.withLobby(row.UUID, func(l *lobby) error {
		if len(in.Items) > 0 {
			if _, _, err := l.addItems(a.ID, a.Username, in.Items, h.now()); err != nil {
				return cmdToAppError(err)
			}
		}
		summary = l.summary(a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// cmdToAppError maps a refused command to an API error.
func cmdToAppError(err error) error {
	var ce *cmdError
	if !errors.As(err, &ce) {
		return err
	}
	switch ce.code {
	case ErrCodeNotAllowed:
		return apperrors.NewForbiddenError(ce.message)
	case ErrCodeNotFound:
		return apperrors.NewNotFoundError("scene", ce.message)
	case ErrCodeInternal:
		return apperrors.NewInternalError(ce.message, nil)
	default:
		return apperrors.NewValidationError(ce.message)
	}
}

// ListLobbies gives the public lobbies and the actor's lobbies (every lobby with all, for
// moderators), most recently active first.
func (h *Hub) ListLobbies(a Actor, all bool) ([]Summary, error) {
	all = all && a.moderate()
	entries, err := h.store.List(a.ID, all)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list the lobbies", err)
	}
	h.mu.Lock()
	loaded := make(map[uuid.UUID]*lobby, len(h.lobbies))
	for id, l := range h.lobbies {
		loaded[id] = l
	}
	h.mu.Unlock()
	out := make([]Summary, 0, len(entries))
	for _, e := range entries {
		if e.MemberBannedAt != nil && !all {
			continue
		}
		if l, ok := loaded[e.UUID]; ok {
			l.mu.Lock()
			if !l.closed && !l.unloaded {
				out = append(out, l.summary(a))
				l.mu.Unlock()
				continue
			}
			l.mu.Unlock()
		}
		out = append(out, summaryFromEntry(e, a))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].lastActive.After(out[j].lastActive) })
	return out, nil
}

// GetLobby gives a lobby the actor may see (a private lobby: members and moderators).
func (h *Hub) GetLobby(a Actor, id uuid.UUID) (*Summary, error) {
	var s Summary
	err := h.withLobby(id, func(l *lobby) error {
		if l.banned[a.ID] && !a.moderate() {
			return apperrors.NewForbiddenErrorWithCode("you are banned from this lobby", apperrors.CodeLobbyBanned)
		}
		if !l.visible(a) {
			return apperrors.NewNotFoundError("lobby", id.String())
		}
		s = l.summary(a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// join adds the actor as a member (lock held).
func (l *lobby) join(userID uint, username string) error {
	if l.banned[userID] {
		return apperrors.NewForbiddenErrorWithCode("you are banned from this lobby", apperrors.CodeLobbyBanned)
	}
	if l.members[userID] != nil {
		return nil
	}
	m := &data.LobbyMember{LobbyID: l.row.ID, UserID: userID, JoinedAt: l.hub.now().UTC()}
	if err := l.hub.store.AddMember(m); err != nil {
		return apperrors.NewInternalError("failed to join the lobby", err)
	}
	l.members[userID] = &member{userID: userID, username: username, joinedAt: m.JoinedAt}
	l.broadcastMembers()
	return nil
}

// JoinLobby joins a public lobby (moderators: any lobby).
func (h *Hub) JoinLobby(a Actor, id uuid.UUID) (*Summary, error) {
	var s Summary
	err := h.withLobby(id, func(l *lobby) error {
		if l.banned[a.ID] {
			return apperrors.NewForbiddenErrorWithCode("you are banned from this lobby", apperrors.CodeLobbyBanned)
		}
		if !l.visible(a) {
			return apperrors.NewNotFoundError("lobby", id.String())
		}
		if err := l.join(a.ID, a.Username); err != nil {
			return err
		}
		s = l.summary(a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// JoinByCode joins the lobby of an invite code.
func (h *Hub) JoinByCode(a Actor, code string) (*Summary, error) {
	code = NormalizeInviteCode(code)
	if code == "" || len(code) > 16 {
		return nil, apperrors.NewNotFoundError("lobby", code)
	}
	row, err := h.store.GetByInviteCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("lobby", code)
		}
		return nil, apperrors.NewInternalError("failed to find the lobby", err)
	}
	var s Summary
	err = h.withLobby(row.UUID, func(l *lobby) error {
		if l.row.InviteCode != code {
			return apperrors.NewNotFoundError("lobby", code) // rotated meanwhile
		}
		if err := l.join(a.ID, a.Username); err != nil {
			return err
		}
		s = l.summary(a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// InviteUser adds a user to a lobby as a member, as if they had opened the invite link (hook
// for the future "invite a friend": the inviter must be a member).
func (h *Hub) InviteUser(id uuid.UUID, inviterID, userID uint, username string) error {
	return h.withLobby(id, func(l *lobby) error {
		if l.members[inviterID] == nil {
			return apperrors.NewForbiddenError("only members can invite")
		}
		return l.join(userID, username)
	})
}

// LeaveLobby removes the actor from a lobby. A leaving host hands the role to the next
// member; a lobby nobody is left in is closed.
func (h *Hub) LeaveLobby(a Actor, id uuid.UUID) error {
	return h.withLobby(id, func(l *lobby) error {
		if l.members[a.ID] == nil {
			return nil
		}
		if err := l.hub.store.RemoveMember(l.row.ID, a.ID); err != nil {
			return apperrors.NewInternalError("failed to leave the lobby", err)
		}
		wasHost := l.isHost(a.ID)
		delete(l.members, a.ID)
		l.dropUser(a.ID, ByeLeft, CloseLeft)
		if len(l.members) == 0 {
			return l.close()
		}
		l.broadcastMembers()
		l.system(a.Username + " left the lobby")
		if wasHost {
			next := l.nextHost(false)
			if next == nil {
				return l.close()
			}
			if err := l.setHost(next, h.now()); err != nil {
				return apperrors.NewInternalError("failed to hand over the host role", err)
			}
		}
		return nil
	})
}

// CloseLobby deletes a lobby (its host, or a moderator).
func (h *Hub) CloseLobby(a Actor, id uuid.UUID) error {
	return h.withLobby(id, func(l *lobby) error {
		if !l.isHost(a.ID) && !a.moderate() {
			if !l.visible(a) {
				return apperrors.NewNotFoundError("lobby", id.String())
			}
			return apperrors.NewForbiddenError("only the host or a moderator can close the lobby")
		}
		return l.close()
	})
}

// close deletes the lobby and closes every connection (lock held).
func (l *lobby) close() error {
	if err := l.hub.store.Delete(l.row.ID); err != nil {
		return apperrors.NewInternalError("failed to close the lobby", err)
	}
	bye := l.marshal(byeMsg{T: msgBye, Reason: ByeClosed})
	for c := range l.clients {
		if bye != nil {
			c.enqueue(bye)
		}
		c.close(CloseClosed, ByeClosed)
		delete(l.clients, c)
	}
	l.closed = true
	return nil
}

// RotateInviteCode gives a lobby a new invite code (host only).
func (h *Hub) RotateInviteCode(a Actor, id uuid.UUID) (string, error) {
	var code string
	err := h.withLobby(id, func(l *lobby) error {
		if !l.isHost(a.ID) {
			if !l.visible(a) {
				return apperrors.NewNotFoundError("lobby", id.String())
			}
			return apperrors.NewForbiddenError("only the host can change the invite code")
		}
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			if code, err = newInviteCode(); err != nil {
				return apperrors.NewInternalError("failed to change the invite code", err)
			}
			if err = l.hub.store.SetInviteCode(l.row.ID, code); !errors.Is(err, data.ErrInviteCodeTaken) {
				break
			}
		}
		if err != nil {
			return apperrors.NewInternalError("failed to change the invite code", err)
		}
		l.row.InviteCode = code
		l.broadcastLobby()
		return nil
	})
	return code, err
}

// AddToQueue adds items to a lobby queue (as suggestions when the actor may not queue).
func (h *Hub) AddToQueue(a Actor, id uuid.UUID, items []ItemInput) (queued, suggested int, err error) {
	if !a.Access.Has(core.PermScenesView) {
		return 0, 0, apperrors.NewForbiddenError("you cannot view scenes")
	}
	err = h.withLobby(id, func(l *lobby) error {
		if l.members[a.ID] == nil {
			if !l.visible(a) {
				return apperrors.NewNotFoundError("lobby", id.String())
			}
			return apperrors.NewForbiddenError("join the lobby first")
		}
		var cmdErr error
		queued, suggested, cmdErr = l.addItems(a.ID, a.Username, items, h.now())
		return cmdToAppError(cmdErr)
	})
	return queued, suggested, err
}

// ActiveLobby is a lobby a user is connected to.
type ActiveLobby struct {
	ID         string
	Name       string
	Visibility string
	SceneID    uint
}

// ActiveLobbiesForUser lists the lobbies where the user has an open connection (hook for the
// future friends presence: "watching in lobby X").
func (h *Hub) ActiveLobbiesForUser(userID uint) []ActiveLobby {
	h.mu.Lock()
	lobbies := make([]*lobby, 0, len(h.lobbies))
	for _, l := range h.lobbies {
		lobbies = append(lobbies, l)
	}
	h.mu.Unlock()
	var out []ActiveLobby
	for _, l := range lobbies {
		l.mu.Lock()
		if !l.closed && !l.unloaded && l.connections(userID) > 0 {
			a := ActiveLobby{ID: l.row.UUID.String(), Name: l.row.Name, Visibility: l.row.Visibility}
			if cur := l.current(); cur != nil {
				a.SceneID = cur.sceneID
			}
			out = append(out, a)
		}
		l.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// BroadcastEmotesChanged tells every connected client to reload the emotes.
func (h *Hub) BroadcastEmotesChanged() {
	h.mu.Lock()
	lobbies := make([]*lobby, 0, len(h.lobbies))
	for _, l := range h.lobbies {
		lobbies = append(lobbies, l)
	}
	h.mu.Unlock()
	for _, l := range lobbies {
		l.mu.Lock()
		l.broadcast(simpleMsg{T: msgEmotes})
		l.mu.Unlock()
	}
}

// --- connections ---

// Connect attaches a connection of the actor to a lobby. A public lobby is joined on connect
// (moderators: any lobby). The welcome snapshot is queued first.
func (h *Hub) Connect(a Actor, id uuid.UUID) (*Client, error) {
	var c *Client
	err := h.withLobby(id, func(l *lobby) error {
		if l.banned[a.ID] {
			return apperrors.NewForbiddenErrorWithCode("you are banned from this lobby", apperrors.CodeLobbyBanned)
		}
		if l.members[a.ID] == nil {
			if !l.visible(a) {
				return apperrors.NewNotFoundError("lobby", id.String())
			}
			if err := l.join(a.ID, a.Username); err != nil {
				return err
			}
		}
		existing := l.connections(a.ID)
		if limit := h.cfg.MaxConnectionsPerUser; limit > 0 && existing >= limit {
			return ErrLobbyFull
		}
		if limit := l.maxMembers(); existing == 0 && !l.isHost(a.ID) && limit > 0 && len(l.onlineUsers()) >= limit {
			return ErrLobbyFull
		}
		now := h.now()
		c = newClient(a.ID, a.Username, h.cfg.SendBuffer)
		c.lobby = l
		c.moderate = a.moderate()
		c.joinedSync = l.syncSeq
		l.clients[c] = struct{}{}
		l.emptySince = time.Time{}
		if l.isHost(a.ID) {
			l.hostAwaySince = time.Time{}
		}
		l.sendTo(c, l.welcome(c))
		if existing == 0 {
			l.broadcastMembers()
			l.system(a.Username + " joined")
		}
		if err := h.store.Touch([]uint{l.row.ID}, now.UTC()); err != nil {
			h.logger.Warn("Failed to touch the lobby", zap.Error(err))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Disconnect detaches a connection (its reader ended).
func (h *Hub) Disconnect(c *Client) {
	l := c.lobby
	if l == nil {
		return
	}
	c.close(1000, "disconnected")
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.clients[c]; !ok {
		return // already dropped (kick, lobby closed)
	}
	delete(l.clients, c)
	now := h.now()
	if l.connections(c.userID) == 0 {
		if l.isHost(c.userID) {
			l.hostAwaySince = now
		}
		delete(l.endedReports, c.userID)
		l.broadcastMembers()
		if l.members[c.userID] != nil {
			l.system(c.username + " left")
		}
	}
	if len(l.clients) == 0 {
		l.emptySince = now
		l.savePlayback()
		if err := h.store.Touch([]uint{l.row.ID}, now.UTC()); err != nil {
			h.logger.Warn("Failed to touch the lobby", zap.Error(err))
		}
	} else if l.applyHold(now) {
		l.changed()
	}
}

// Receive handles one message of a connection. Refused commands answer an error message.
func (h *Hub) Receive(c *Client, raw []byte) {
	l := c.lobby
	if l == nil {
		return
	}
	var in inbound
	if err := json.Unmarshal(raw, &in); err != nil {
		l.mu.Lock()
		l.sendTo(c, errorMsg{T: msgError, Code: ErrCodeInvalid, Message: "invalid message"})
		l.mu.Unlock()
		return
	}
	if len(in.Ref) > 64 {
		in.Ref = in.Ref[:64]
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.clients[c]; !ok || l.closed {
		return
	}
	if err := l.handle(c, &in, h.now()); err != nil {
		var ce *cmdError
		if !errors.As(err, &ce) {
			ce = refuse(ErrCodeInternal, "internal error")
		}
		l.sendTo(c, errorMsg{T: msgError, Code: ce.code, Message: ce.message, Ref: in.Ref})
	}
}

// --- ticks ---

// Tick runs the periodic work of every loaded lobby: auto-advance, hold timeout, host grace,
// unloading, and the idle lobby cleanup.
func (h *Hub) Tick() {
	now := h.now()
	h.mu.Lock()
	lobbies := make([]*lobby, 0, len(h.lobbies))
	for _, l := range h.lobbies {
		lobbies = append(lobbies, l)
	}
	h.mu.Unlock()
	for _, l := range lobbies {
		if l.tick(now) {
			h.unload(l, now)
		}
	}
	interval := h.cfg.CleanupInterval
	if interval <= 0 {
		interval = time.Minute
	}
	if now.Sub(h.lastCleanup) >= interval {
		h.lastCleanup = now
		h.cleanup(now)
	}
}

// tick runs the periodic work of a lobby and tells whether it can be unloaded.
func (l *lobby) tick(now time.Time) (unload bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.unloaded {
		return false
	}
	cfg := l.hub.cfg

	// End of the item playing
	if cur := l.current(); cur != nil && l.state.moving() {
		if end, known, fromDuration := itemEnd(cur); known {
			limit := end
			if fromDuration {
				limit += cfg.EndGrace.Seconds()
			}
			if l.state.at(now) >= limit {
				if err := l.advance(now); err != nil {
					l.hub.logger.Error("Failed to advance the lobby queue", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
				}
			}
		}
	}

	// "Wait for everyone": a hold lasts at most hold_timeout
	if l.state.hold && cfg.HoldTimeout > 0 && now.Sub(l.holdSince) >= cfg.HoldTimeout {
		l.holdTimedOutAt = l.syncSeq
		l.state.hold = false
		l.state.ref = now
		l.changed()
	}

	// Host away: the role goes to the next online member
	if len(l.clients) > 0 {
		hostOnline := l.row.HostUserID != nil && l.connections(*l.row.HostUserID) > 0
		if !hostOnline {
			if l.hostAwaySince.IsZero() {
				l.hostAwaySince = now
			}
			if l.row.HostUserID == nil || now.Sub(l.hostAwaySince) >= cfg.HostGrace {
				if next := l.nextHost(true); next != nil {
					if err := l.setHost(next, now); err != nil {
						l.hub.logger.Error("Failed to hand over the host role", zap.Uint("lobby_id", l.row.ID), zap.Error(err))
					}
				}
			}
		}
		return false
	}
	if l.emptySince.IsZero() {
		l.emptySince = now
	}
	return now.Sub(l.emptySince) >= cfg.UnloadAfter
}

// nextHost picks the next host: members with control first, then by join date (online only
// when asked). The current host is skipped.
func (l *lobby) nextHost(onlineOnly bool) *member {
	var best *member
	bestOnline := false
	for _, m := range l.members {
		if l.isHost(m.userID) {
			continue
		}
		online := l.connections(m.userID) > 0
		if onlineOnly && !online {
			continue
		}
		if best == nil || better(m, online, best, bestOnline) {
			best, bestOnline = m, online
		}
	}
	return best
}

func better(m *member, online bool, than *member, thanOnline bool) bool {
	if online != thanOnline {
		return online
	}
	if m.canControl != than.canControl {
		return m.canControl
	}
	if !m.joinedAt.Equal(than.joinedAt) {
		return m.joinedAt.Before(than.joinedAt)
	}
	return m.userID < than.userID
}

// unload drops a lobby without connections from memory, its playback saved.
func (h *Hub) unload(l *lobby, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.unloaded || len(l.clients) > 0 {
		return
	}
	l.savePlayback()
	if err := h.store.Touch([]uint{l.row.ID}, now.UTC()); err != nil {
		h.logger.Warn("Failed to touch the lobby", zap.Error(err))
	}
	l.unloaded = true
	if cur, ok := h.lobbies[l.row.UUID]; ok && cur == l {
		delete(h.lobbies, l.row.UUID)
	}
}

// cleanup marks the lobbies with connections as active and deletes the lobbies nobody
// connected to for idle_timeout.
func (h *Hub) cleanup(now time.Time) {
	h.mu.Lock()
	var active, loaded []uint
	for _, l := range h.lobbies {
		l.mu.Lock()
		loaded = append(loaded, l.row.ID)
		if len(l.clients) > 0 {
			active = append(active, l.row.ID)
		}
		l.mu.Unlock()
	}
	h.mu.Unlock()
	if err := h.store.Touch(active, now.UTC()); err != nil {
		h.logger.Warn("Failed to touch the active lobbies", zap.Error(err))
	}
	if h.cfg.IdleTimeout <= 0 {
		return
	}
	n, err := h.store.DeleteIdle(now.UTC().Add(-h.cfg.IdleTimeout), loaded)
	if err != nil {
		h.logger.Warn("Failed to delete the idle lobbies", zap.Error(err))
		return
	}
	if n > 0 {
		h.logger.Info("Deleted idle lobbies", zap.Int64("count", n))
	}
}
