package data

import (
	"context"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// UserPresence is what outlives the in-memory presence of a user (docs/PRESENCE.md): the last
// time they were seen online and the invisible mode. No row = never seen, visible.
type UserPresence struct {
	UserID     uint       `gorm:"primaryKey;autoIncrement:false"`
	LastSeenAt *time.Time `gorm:"column:last_seen_at"`
	Invisible  bool       `gorm:"not null;default:false"`
	UpdatedAt  time.Time  `gorm:"not null;default:now()"`
}

func (UserPresence) TableName() string { return "user_presence" }

// PresenceContact is a user whose presence another user may follow (a friend or a followed
// user), with their stored presence.
type PresenceContact struct {
	ID         uint       `gorm:"column:id"`
	Username   string     `gorm:"column:username"`
	LastSeenAt *time.Time `gorm:"column:last_seen_at"`
	Invisible  bool       `gorm:"column:invisible"`
}

// PresenceRepository stores the persistent part of the presence (user_presence).
type PresenceRepository interface {
	// GetUser returns the card and stored presence of a user (gorm.ErrRecordNotFound when the
	// user does not exist).
	GetUser(ctx context.Context, userID uint) (*PresenceContact, error)
	// Contacts lists the friends of a user and the users they follow (each once), with their
	// stored presence, by username.
	Contacts(ctx context.Context, userID uint) ([]PresenceContact, error)
	// SaveLastSeen sets the last seen time of each user (one statement).
	SaveLastSeen(ctx context.Context, lastSeen map[uint]time.Time) error
	// SetInvisible sets the invisible mode of a user; lastSeen, when given, is saved too.
	SetInvisible(ctx context.Context, userID uint, invisible bool, lastSeen *time.Time) error
}

type PresenceRepositoryImpl struct {
	DB *gorm.DB
}

func NewPresenceRepository(db *gorm.DB) *PresenceRepositoryImpl {
	return &PresenceRepositoryImpl{DB: db}
}

const presenceContactColumns = `u.id, u.username, p.last_seen_at, COALESCE(p.invisible, false) AS invisible`

func (r *PresenceRepositoryImpl) GetUser(ctx context.Context, userID uint) (*PresenceContact, error) {
	var rows []PresenceContact
	if err := r.DB.WithContext(ctx).Raw(`SELECT `+presenceContactColumns+` FROM users u
		LEFT JOIN user_presence p ON p.user_id = u.id WHERE u.id = ?`, userID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read presence: %w", err)
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

func (r *PresenceRepositoryImpl) Contacts(ctx context.Context, userID uint) ([]PresenceContact, error) {
	var rows []PresenceContact
	if err := r.DB.WithContext(ctx).Raw(`SELECT `+presenceContactColumns+` FROM users u
		LEFT JOIN user_presence p ON p.user_id = u.id
		WHERE u.id IN (
			SELECT friend_id FROM user_friends WHERE user_id = ?
			UNION
			SELECT followed_id FROM user_follows WHERE follower_id = ?
		)
		ORDER BY LOWER(u.username), u.id`, userID, userID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list presence contacts: %w", err)
	}
	return rows, nil
}

func (r *PresenceRepositoryImpl) SaveLastSeen(ctx context.Context, lastSeen map[uint]time.Time) error {
	if len(lastSeen) == 0 {
		return nil
	}
	ids := make(pq.Int64Array, 0, len(lastSeen))
	times := make(pq.StringArray, 0, len(lastSeen))
	for id, at := range lastSeen {
		ids = append(ids, int64(id))
		times = append(times, at.UTC().Format(time.RFC3339Nano))
	}
	// users deleted meanwhile are skipped (the join), not failed on the foreign key
	if err := r.DB.WithContext(ctx).Exec(`INSERT INTO user_presence (user_id, last_seen_at, updated_at)
		SELECT v.user_id, v.at, NOW() FROM unnest(?::bigint[], ?::timestamptz[]) AS v(user_id, at)
		JOIN users u ON u.id = v.user_id
		ON CONFLICT (user_id) DO UPDATE SET last_seen_at = GREATEST(COALESCE(user_presence.last_seen_at, EXCLUDED.last_seen_at), EXCLUDED.last_seen_at), updated_at = NOW()`,
		ids, times).Error; err != nil {
		return fmt.Errorf("failed to save last seen: %w", err)
	}
	return nil
}

func (r *PresenceRepositoryImpl) SetInvisible(ctx context.Context, userID uint, invisible bool, lastSeen *time.Time) error {
	var at any
	if lastSeen != nil {
		at = lastSeen.UTC()
	}
	if err := r.DB.WithContext(ctx).Exec(`INSERT INTO user_presence (user_id, invisible, last_seen_at, updated_at)
		VALUES (?, ?, ?, NOW())
		ON CONFLICT (user_id) DO UPDATE SET invisible = EXCLUDED.invisible,
			last_seen_at = COALESCE(EXCLUDED.last_seen_at, user_presence.last_seen_at), updated_at = NOW()`,
		userID, invisible, at).Error; err != nil {
		return fmt.Errorf("failed to save the invisible mode: %w", err)
	}
	return nil
}
