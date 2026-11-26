// Package togethertest provides in-memory fakes of the Goon2gether repositories for the
// tests of the hub and of the handlers.
package togethertest

import (
	"sort"
	"strings"
	"sync"
	"time"

	"goonhub/internal/data"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Store is an in-memory data.LobbyRepository.
type Store struct {
	mu      sync.Mutex
	nextID  uint
	lobbies map[uint]*data.Lobby
	members map[uint]map[uint]*data.LobbyMember
	items   map[uint]*data.LobbyQueueItem
	// Users maps user IDs to usernames (member queries); Scenes gives titles and durations.
	Users  map[uint]string
	Scenes map[uint]data.Scene
	// DeleteIdleCalls records the DeleteIdle cutoffs.
	DeleteIdleCalls []time.Time
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{
		lobbies: make(map[uint]*data.Lobby),
		members: make(map[uint]map[uint]*data.LobbyMember),
		items:   make(map[uint]*data.LobbyQueueItem),
		Users:   make(map[uint]string),
		Scenes:  make(map[uint]data.Scene),
	}
}

func (s *Store) id() uint {
	s.nextID++
	return s.nextID
}

// Lobby gives a copy of a stored lobby (nil if none).
func (s *Store) Lobby(id uint) *data.Lobby {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.lobbies[id]; ok {
		c := *l
		return &c
	}
	return nil
}

func (s *Store) Create(lobby *data.Lobby, host *data.LobbyMember, items []data.LobbyQueueItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.lobbies {
		if l.InviteCode == lobby.InviteCode {
			return data.ErrInviteCodeTaken
		}
	}
	lobby.ID = s.id()
	if lobby.UUID == uuid.Nil {
		lobby.UUID = uuid.New()
	}
	lobby.CreatedAt = time.Now()
	c := *lobby
	s.lobbies[lobby.ID] = &c
	s.members[lobby.ID] = make(map[uint]*data.LobbyMember)
	if host != nil {
		host.LobbyID = lobby.ID
		m := *host
		s.members[lobby.ID][host.UserID] = &m
	}
	for i := range items {
		items[i].LobbyID = lobby.ID
		items[i].ID = s.id()
		it := items[i]
		s.items[it.ID] = &it
	}
	return nil
}

func (s *Store) GetByUUID(id uuid.UUID) (*data.Lobby, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.lobbies {
		if l.UUID == id {
			c := *l
			return &c, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *Store) GetByInviteCode(code string) (*data.Lobby, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.lobbies {
		if l.InviteCode == code {
			c := *l
			return &c, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *Store) List(userID uint, all bool) ([]data.LobbyListEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []data.LobbyListEntry
	for _, l := range s.lobbies {
		m := s.members[l.ID][userID]
		isMember := m != nil && m.BannedAt == nil
		if !all && l.Visibility != data.LobbyVisibilityPublic && !isMember {
			continue
		}
		e := data.LobbyListEntry{Lobby: *l, IsMember: isMember}
		if m != nil {
			e.MemberBannedAt = m.BannedAt
		}
		if l.HostUserID != nil {
			e.HostUsername = s.Users[*l.HostUserID]
		}
		for _, it := range s.items {
			if it.LobbyID == l.ID && it.Status == data.QueueItemQueued {
				e.QueueLength++
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastActiveAt.After(out[j].LastActiveAt) })
	return out, nil
}

func (s *Store) CountHostedBy(userID uint) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, l := range s.lobbies {
		if l.HostUserID != nil && *l.HostUserID == userID {
			n++
		}
	}
	return n, nil
}

func (s *Store) update(id uint, fn func(l *data.Lobby)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.lobbies[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	fn(l)
	return nil
}

func (s *Store) UpdateSettings(lobby *data.Lobby) error {
	return s.update(lobby.ID, func(l *data.Lobby) {
		l.Name, l.Visibility = lobby.Name, lobby.Visibility
		l.ControlOpen, l.QueueOpen, l.WaitForAll, l.MaxMembers = lobby.ControlOpen, lobby.QueueOpen, lobby.WaitForAll, lobby.MaxMembers
	})
}

func (s *Store) SetHost(lobbyID uint, userID *uint) error {
	return s.update(lobbyID, func(l *data.Lobby) { l.HostUserID = userID })
}

func (s *Store) SetInviteCode(lobbyID uint, code string) error {
	return s.update(lobbyID, func(l *data.Lobby) { l.InviteCode = code })
}

func (s *Store) SavePlayback(lobbyID uint, itemID *uint, position, rate float64) error {
	return s.update(lobbyID, func(l *data.Lobby) {
		l.CurrentItemID, l.PositionS, l.PlaybackRate = itemID, position, rate
	})
}

func (s *Store) Touch(lobbyIDs []uint, at time.Time) error {
	for _, id := range lobbyIDs {
		_ = s.update(id, func(l *data.Lobby) { l.LastActiveAt = at })
	}
	return nil
}

func (s *Store) Delete(lobbyID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.lobbies, lobbyID)
	delete(s.members, lobbyID)
	for id, it := range s.items {
		if it.LobbyID == lobbyID {
			delete(s.items, id)
		}
	}
	return nil
}

func (s *Store) DeleteIdle(before time.Time, keep []uint) (int64, error) {
	s.mu.Lock()
	s.DeleteIdleCalls = append(s.DeleteIdleCalls, before)
	kept := make(map[uint]bool, len(keep))
	for _, id := range keep {
		kept[id] = true
	}
	var del []uint
	for id, l := range s.lobbies {
		if !kept[id] && l.LastActiveAt.Before(before) {
			del = append(del, id)
		}
	}
	s.mu.Unlock()
	for _, id := range del {
		_ = s.Delete(id)
	}
	return int64(len(del)), nil
}

func (s *Store) ListMembers(lobbyID uint) ([]data.LobbyMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []data.LobbyMember
	for _, m := range s.members[lobbyID] {
		c := *m
		c.Username = s.Users[m.UserID]
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

func (s *Store) GetMember(lobbyID, userID uint) (*data.LobbyMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.members[lobbyID][userID]; ok {
		c := *m
		c.Username = s.Users[userID]
		return &c, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *Store) AddMember(member *data.LobbyMember) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.members[member.LobbyID] == nil {
		s.members[member.LobbyID] = make(map[uint]*data.LobbyMember)
	}
	if _, ok := s.members[member.LobbyID][member.UserID]; !ok {
		c := *member
		s.members[member.LobbyID][member.UserID] = &c
	}
	return nil
}

func (s *Store) SetMemberControl(lobbyID, userID uint, canControl bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.members[lobbyID][userID]; ok {
		m.CanControl = canControl
	}
	return nil
}

func (s *Store) BanMember(lobbyID, userID uint, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.members[lobbyID] == nil {
		s.members[lobbyID] = make(map[uint]*data.LobbyMember)
	}
	m, ok := s.members[lobbyID][userID]
	if !ok {
		m = &data.LobbyMember{LobbyID: lobbyID, UserID: userID, JoinedAt: at}
		s.members[lobbyID][userID] = m
	}
	m.BannedAt, m.CanControl = &at, false
	return nil
}

func (s *Store) RemoveMember(lobbyID, userID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.members[lobbyID], userID)
	return nil
}

func (s *Store) ListQueue(lobbyID uint) ([]data.LobbyQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []data.LobbyQueueItem
	for _, it := range s.items {
		if it.LobbyID != lobbyID {
			continue
		}
		c := *it
		c.SceneTitle = s.Scenes[it.SceneID].Title
		c.SceneDuration = s.Scenes[it.SceneID].Duration
		if it.AddedBy != nil {
			c.AddedByName = s.Users[*it.AddedBy]
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == data.QueueItemQueued
		}
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Queue gives the stored queued item IDs in order (assertions).
func (s *Store) Queue(lobbyID uint) []uint {
	items, _ := s.ListQueue(lobbyID)
	var out []uint
	for _, it := range items {
		if it.Status == data.QueueItemQueued {
			out = append(out, it.ID)
		}
	}
	return out
}

func (s *Store) AddQueueItems(items []data.LobbyQueueItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range items {
		items[i].ID = s.id()
		it := items[i]
		s.items[it.ID] = &it
	}
	return nil
}

func (s *Store) DeleteQueueItem(lobbyID, itemID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.items[itemID]; ok && it.LobbyID == lobbyID {
		delete(s.items, itemID)
		for _, l := range s.lobbies {
			if l.CurrentItemID != nil && *l.CurrentItemID == itemID {
				l.CurrentItemID = nil
			}
		}
	}
	return nil
}

func (s *Store) SetQueueOrder(lobbyID uint, itemIDs []uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, id := range itemIDs {
		if it, ok := s.items[id]; ok && it.LobbyID == lobbyID {
			it.Status, it.Position = data.QueueItemQueued, i
		}
	}
	return nil
}

// Scenes is an in-memory scene lookup.
type Scenes map[uint]data.Scene

func (m Scenes) GetByIDs(ids []uint) ([]data.Scene, error) {
	out := make([]data.Scene, 0, len(ids))
	for _, id := range ids {
		if s, ok := m[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// Emotes is an in-memory data.EmoteRepository.
type Emotes struct {
	mu     sync.Mutex
	nextID uint
	rows   map[uint]data.Emote
}

func NewEmotes() *Emotes { return &Emotes{rows: make(map[uint]data.Emote)} }

func (e *Emotes) List() ([]data.Emote, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]data.Emote, 0, len(e.rows))
	for _, r := range e.rows {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (e *Emotes) GetByID(id uint) (*data.Emote, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.rows[id]; ok {
		return &r, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (e *Emotes) Count() (int64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return int64(len(e.rows)), nil
}

func (e *Emotes) taken(name string, except uint) bool {
	for id, r := range e.rows {
		if id != except && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

func (e *Emotes) Create(emote *data.Emote) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.taken(emote.Name, 0) {
		return data.ErrEmoteNameTaken
	}
	e.nextID++
	emote.ID = e.nextID
	e.rows[emote.ID] = *emote
	return nil
}

func (e *Emotes) Rename(id uint, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.rows[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if e.taken(name, id) {
		return data.ErrEmoteNameTaken
	}
	r.Name = name
	e.rows[id] = r
	return nil
}

func (e *Emotes) Delete(id uint) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.rows[id]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(e.rows, id)
	return nil
}

var (
	_ data.LobbyRepository = (*Store)(nil)
	_ data.EmoteRepository = (*Emotes)(nil)
)
