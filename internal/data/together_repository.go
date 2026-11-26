package data

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrEmoteNameTaken is returned when another emote already has the name (case-insensitive).
var ErrEmoteNameTaken = errors.New("emote name already taken")

// ErrInviteCodeTaken is returned when a generated invite code collides with another lobby's.
var ErrInviteCodeTaken = errors.New("invite code already taken")

// LobbyRepository persists the Goon2gether lobbies, their members and their queue
// (docs/GOON2GETHER.md). The live state is in memory (together.Hub).
type LobbyRepository interface {
	// Create inserts the lobby, its host member and its first queue items (IDs filled).
	Create(lobby *Lobby, host *LobbyMember, items []LobbyQueueItem) error
	GetByUUID(id uuid.UUID) (*Lobby, error)
	GetByInviteCode(code string) (*Lobby, error)
	// List gives the public lobbies and those userID is a member of (every lobby with all),
	// most recently active first.
	List(userID uint, all bool) ([]LobbyListEntry, error)
	CountHostedBy(userID uint) (int64, error)
	// UpdateSettings saves name, visibility, control_open, queue_open, wait_for_all,
	// max_members and auto_next.
	UpdateSettings(lobby *Lobby) error
	SetHost(lobbyID uint, userID *uint) error
	SetInviteCode(lobbyID uint, code string) error
	SavePlayback(lobbyID uint, itemID *uint, position, rate float64) error
	Touch(lobbyIDs []uint, at time.Time) error
	Delete(lobbyID uint) error
	// DeleteIdle deletes the lobbies last active before the cutoff, except keep.
	DeleteIdle(before time.Time, keep []uint) (int64, error)

	ListMembers(lobbyID uint) ([]LobbyMember, error)
	GetMember(lobbyID, userID uint) (*LobbyMember, error)
	// AddMember inserts the member, or does nothing when the user already has a row.
	AddMember(member *LobbyMember) error
	// AddAnonymousMember inserts an anonymous member with the next anonymous number of the
	// lobby (lobbies.anon_seq), set in member.AnonNumber.
	AddAnonymousMember(member *LobbyMember) error
	SetMemberControl(lobbyID, userID uint, canControl bool) error
	BanMember(lobbyID, userID uint, at time.Time) error
	RemoveMember(lobbyID, userID uint) error

	// ListQueue gives the queued items (by position) then the suggestions (by position).
	ListQueue(lobbyID uint) ([]LobbyQueueItem, error)
	AddQueueItems(items []LobbyQueueItem) error
	DeleteQueueItem(lobbyID, itemID uint) error
	// SetQueueOrder makes itemIDs the queued items of the lobby, in that order.
	SetQueueOrder(lobbyID uint, itemIDs []uint) error
}

// EmoteRepository persists the server custom emotes.
type EmoteRepository interface {
	List() ([]Emote, error)
	GetByID(id uint) (*Emote, error)
	Count() (int64, error)
	Create(emote *Emote) error
	Rename(id uint, name string) error
	Delete(id uint) error
}

type LobbyRepositoryImpl struct {
	DB *gorm.DB
}

func NewLobbyRepository(db *gorm.DB) *LobbyRepositoryImpl {
	return &LobbyRepositoryImpl{DB: db}
}

func (r *LobbyRepositoryImpl) Create(lobby *Lobby, host *LobbyMember, items []LobbyQueueItem) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if lobby.UUID == uuid.Nil {
			lobby.UUID = uuid.New()
		}
		if err := tx.Create(lobby).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrInviteCodeTaken
			}
			return fmt.Errorf("create lobby: %w", err)
		}
		if host != nil {
			host.LobbyID = lobby.ID
			if err := tx.Create(host).Error; err != nil {
				return fmt.Errorf("create lobby host member: %w", err)
			}
		}
		if len(items) > 0 {
			for i := range items {
				items[i].LobbyID = lobby.ID
			}
			if err := tx.Create(&items).Error; err != nil {
				return fmt.Errorf("create lobby queue: %w", err)
			}
		}
		return nil
	})
}

func (r *LobbyRepositoryImpl) GetByUUID(id uuid.UUID) (*Lobby, error) {
	var lobby Lobby
	if err := r.DB.Where("uuid = ?", id).First(&lobby).Error; err != nil {
		return nil, err
	}
	return &lobby, nil
}

func (r *LobbyRepositoryImpl) GetByInviteCode(code string) (*Lobby, error) {
	var lobby Lobby
	if err := r.DB.Where("invite_code = ?", code).First(&lobby).Error; err != nil {
		return nil, err
	}
	return &lobby, nil
}

func (r *LobbyRepositoryImpl) List(userID uint, all bool) ([]LobbyListEntry, error) {
	var out []LobbyListEntry
	q := r.DB.Table("lobbies l").
		Select(`l.*, COALESCE(h.username, '') AS host_username,
			(SELECT COUNT(*) FROM lobby_queue_items q WHERE q.lobby_id = l.id AND q.status = 'queued') AS queue_length,
			ci.scene_id AS now_scene_id, COALESCE(s.title, '') AS now_scene_title, ci.label AS now_label,
			(m.user_id IS NOT NULL AND m.banned_at IS NULL) AS is_member,
			COALESCE(m.anonymous, FALSE) AS member_anonymous, m.banned_at AS member_banned_at`).
		Joins("LEFT JOIN users h ON h.id = l.host_user_id").
		Joins("LEFT JOIN lobby_queue_items ci ON ci.id = l.current_item_id").
		Joins("LEFT JOIN scenes s ON s.id = ci.scene_id").
		Joins("LEFT JOIN lobby_members m ON m.lobby_id = l.id AND m.user_id = ?", userID)
	if !all {
		q = q.Where("l.visibility = ? OR (m.user_id IS NOT NULL AND m.banned_at IS NULL)", LobbyVisibilityPublic)
	}
	if err := q.Order("l.last_active_at DESC, l.id DESC").Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("list lobbies: %w", err)
	}
	return out, nil
}

func (r *LobbyRepositoryImpl) CountHostedBy(userID uint) (int64, error) {
	var n int64
	err := r.DB.Model(&Lobby{}).Where("host_user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *LobbyRepositoryImpl) UpdateSettings(lobby *Lobby) error {
	res := r.DB.Model(&Lobby{}).Where("id = ?", lobby.ID).Updates(map[string]any{
		"name":         lobby.Name,
		"visibility":   lobby.Visibility,
		"control_open": lobby.ControlOpen,
		"queue_open":   lobby.QueueOpen,
		"wait_for_all": lobby.WaitForAll,
		"max_members":  lobby.MaxMembers,
		"auto_next":    lobby.AutoNext,
		"updated_at":   time.Now().UTC(),
	})
	if res.Error != nil {
		return fmt.Errorf("update lobby settings: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *LobbyRepositoryImpl) SetHost(lobbyID uint, userID *uint) error {
	return r.DB.Model(&Lobby{}).Where("id = ?", lobbyID).
		Updates(map[string]any{"host_user_id": userID, "updated_at": time.Now().UTC()}).Error
}

func (r *LobbyRepositoryImpl) SetInviteCode(lobbyID uint, code string) error {
	err := r.DB.Model(&Lobby{}).Where("id = ?", lobbyID).
		Updates(map[string]any{"invite_code": code, "updated_at": time.Now().UTC()}).Error
	if isUniqueViolation(err) {
		return ErrInviteCodeTaken
	}
	return err
}

func (r *LobbyRepositoryImpl) SavePlayback(lobbyID uint, itemID *uint, position, rate float64) error {
	return r.DB.Model(&Lobby{}).Where("id = ?", lobbyID).Updates(map[string]any{
		"current_item_id": itemID,
		"position_s":      position,
		"playback_rate":   rate,
	}).Error
}

func (r *LobbyRepositoryImpl) Touch(lobbyIDs []uint, at time.Time) error {
	if len(lobbyIDs) == 0 {
		return nil
	}
	return r.DB.Model(&Lobby{}).Where("id IN ?", lobbyIDs).Update("last_active_at", at).Error
}

func (r *LobbyRepositoryImpl) Delete(lobbyID uint) error {
	return r.DB.Delete(&Lobby{}, lobbyID).Error
}

func (r *LobbyRepositoryImpl) DeleteIdle(before time.Time, keep []uint) (int64, error) {
	q := r.DB.Where("last_active_at < ?", before)
	if len(keep) > 0 {
		q = q.Where("id NOT IN ?", keep)
	}
	res := q.Delete(&Lobby{})
	return res.RowsAffected, res.Error
}

func (r *LobbyRepositoryImpl) ListMembers(lobbyID uint) ([]LobbyMember, error) {
	var members []LobbyMember
	err := r.DB.Table("lobby_members m").
		Select("m.*, u.username").
		Joins("JOIN users u ON u.id = m.user_id").
		Where("m.lobby_id = ?", lobbyID).
		Order("m.joined_at, m.user_id").
		Scan(&members).Error
	if err != nil {
		return nil, fmt.Errorf("list lobby members: %w", err)
	}
	return members, nil
}

func (r *LobbyRepositoryImpl) GetMember(lobbyID, userID uint) (*LobbyMember, error) {
	var members []LobbyMember
	err := r.DB.Table("lobby_members m").
		Select("m.*, u.username").
		Joins("JOIN users u ON u.id = m.user_id").
		Where("m.lobby_id = ? AND m.user_id = ?", lobbyID, userID).
		Limit(1).
		Scan(&members).Error
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &members[0], nil
}

func (r *LobbyRepositoryImpl) AddMember(member *LobbyMember) error {
	if member.JoinedAt.IsZero() {
		member.JoinedAt = time.Now().UTC()
	}
	return r.DB.Clauses(clause.OnConflict{DoNothing: true}).Omit("Username").Create(member).Error
}

func (r *LobbyRepositoryImpl) AddAnonymousMember(member *LobbyMember) error {
	if member.JoinedAt.IsZero() {
		member.JoinedAt = time.Now().UTC()
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		var seq []int
		if err := tx.Raw(`UPDATE lobbies SET anon_seq = anon_seq + 1 WHERE id = ? RETURNING anon_seq`, member.LobbyID).
			Scan(&seq).Error; err != nil {
			return fmt.Errorf("next anonymous number: %w", err)
		}
		if len(seq) == 0 {
			return gorm.ErrRecordNotFound
		}
		n := seq[0]
		member.Anonymous = true
		member.AnonNumber = &n
		if err := tx.Omit("Username").Create(member).Error; err != nil {
			return fmt.Errorf("add anonymous lobby member: %w", err)
		}
		return nil
	})
}

func (r *LobbyRepositoryImpl) SetMemberControl(lobbyID, userID uint, canControl bool) error {
	return r.DB.Model(&LobbyMember{}).Where("lobby_id = ? AND user_id = ?", lobbyID, userID).
		Update("can_control", canControl).Error
}

func (r *LobbyRepositoryImpl) BanMember(lobbyID, userID uint, at time.Time) error {
	m := LobbyMember{LobbyID: lobbyID, UserID: userID, BannedAt: &at, JoinedAt: at}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "lobby_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{"banned_at": at, "can_control": false}),
	}).Omit("Username").Create(&m).Error
}

func (r *LobbyRepositoryImpl) RemoveMember(lobbyID, userID uint) error {
	return r.DB.Where("lobby_id = ? AND user_id = ?", lobbyID, userID).Delete(&LobbyMember{}).Error
}

func (r *LobbyRepositoryImpl) ListQueue(lobbyID uint) ([]LobbyQueueItem, error) {
	var items []LobbyQueueItem
	err := r.DB.Table("lobby_queue_items q").
		Select("q.*, COALESCE(s.title, '') AS scene_title, COALESCE(s.duration, 0) AS scene_duration, COALESCE(u.username, '') AS added_by_name").
		Joins("JOIN scenes s ON s.id = q.scene_id").
		Joins("LEFT JOIN users u ON u.id = q.added_by").
		Where("q.lobby_id = ?", lobbyID).
		Order("CASE q.status WHEN 'queued' THEN 0 ELSE 1 END, q.position, q.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list lobby queue: %w", err)
	}
	return items, nil
}

func (r *LobbyRepositoryImpl) AddQueueItems(items []LobbyQueueItem) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.DB.Create(&items).Error; err != nil {
		return fmt.Errorf("add lobby queue items: %w", err)
	}
	return nil
}

func (r *LobbyRepositoryImpl) DeleteQueueItem(lobbyID, itemID uint) error {
	return r.DB.Where("lobby_id = ? AND id = ?", lobbyID, itemID).Delete(&LobbyQueueItem{}).Error
}

func (r *LobbyRepositoryImpl) SetQueueOrder(lobbyID uint, itemIDs []uint) error {
	if len(itemIDs) == 0 {
		return nil
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range itemIDs {
			res := tx.Model(&LobbyQueueItem{}).Where("lobby_id = ? AND id = ?", lobbyID, id).
				Updates(map[string]any{"status": QueueItemQueued, "position": i})
			if res.Error != nil {
				return fmt.Errorf("set queue order: %w", res.Error)
			}
		}
		return nil
	})
}

type EmoteRepositoryImpl struct {
	DB *gorm.DB
}

func NewEmoteRepository(db *gorm.DB) *EmoteRepositoryImpl {
	return &EmoteRepositoryImpl{DB: db}
}

func (r *EmoteRepositoryImpl) List() ([]Emote, error) {
	var emotes []Emote
	if err := r.DB.Order("LOWER(name)").Find(&emotes).Error; err != nil {
		return nil, fmt.Errorf("list emotes: %w", err)
	}
	return emotes, nil
}

func (r *EmoteRepositoryImpl) GetByID(id uint) (*Emote, error) {
	var e Emote
	if err := r.DB.First(&e, id).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EmoteRepositoryImpl) Count() (int64, error) {
	var n int64
	err := r.DB.Model(&Emote{}).Count(&n).Error
	return n, err
}

func (r *EmoteRepositoryImpl) Create(emote *Emote) error {
	if err := r.DB.Create(emote).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrEmoteNameTaken
		}
		return fmt.Errorf("create emote: %w", err)
	}
	return nil
}

func (r *EmoteRepositoryImpl) Rename(id uint, name string) error {
	res := r.DB.Model(&Emote{}).Where("id = ?", id).Updates(map[string]any{"name": name, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return ErrEmoteNameTaken
		}
		return fmt.Errorf("rename emote: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *EmoteRepositoryImpl) Delete(id uint) error {
	res := r.DB.Delete(&Emote{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete emote: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
