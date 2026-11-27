package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageRepository stores the conversations, their participants and the messages
// (docs/MESSAGES.md), the message settings of the users, and reads what the share cards need.
// It knows nothing of the policies and permissions: the service checks them.
type MessageRepository interface {
	// FindOrCreateDirect returns the direct conversation of the pair, created (with its two
	// participants) when missing; created tells which.
	FindOrCreateDirect(ctx context.Context, a, b uint) (conv *Conversation, created bool, err error)
	// FindDirect returns the direct conversation of the pair (gorm.ErrRecordNotFound when none).
	FindDirect(ctx context.Context, a, b uint) (*Conversation, error)
	// GetConversation returns a conversation (gorm.ErrRecordNotFound when unknown).
	GetConversation(ctx context.Context, id uint) (*Conversation, error)
	// ParticipantIDs returns the users taking part in a conversation, sorted.
	ParticipantIDs(ctx context.Context, conversationID uint) ([]uint, error)
	// ListConversations lists the conversations of a user, most recent message first, with
	// their read state, the other participant and the last message.
	ListConversations(ctx context.Context, userID uint, page, limit int) ([]ConversationRow, int64, error)
	// GetConversationRow is one conversation of ListConversations (gorm.ErrRecordNotFound when
	// unknown or when the user does not take part in it).
	GetConversationRow(ctx context.Context, userID, conversationID uint) (*ConversationRow, error)
	// UnreadSummary counts the unread messages of a user (sent by others, not deleted, after
	// their last read message), muted conversations excluded.
	UnreadSummary(ctx context.Context, userID uint) (UnreadSummary, error)

	// InsertMessage adds a message (ID and CreatedAt are set), moves the conversation's last
	// message date and marks it read for its sender, in one transaction.
	InsertMessage(ctx context.Context, msg *Message) error
	// GetMessage returns a message (gorm.ErrRecordNotFound when unknown).
	GetMessage(ctx context.Context, id uint) (*Message, error)
	// ListMessages returns up to limit messages of a conversation older than beforeID (0 = the
	// latest), newest first.
	ListMessages(ctx context.Context, conversationID, beforeID uint, limit int) ([]Message, error)
	// UpdateMessageBody changes the body of a message that is not deleted; updated is false
	// otherwise.
	UpdateMessageBody(ctx context.Context, id uint, body string, at time.Time) (updated bool, err error)
	// SoftDeleteMessage clears the body and attachment of a message, removes its reactions and
	// marks it deleted; deleted is false when it already was.
	SoftDeleteMessage(ctx context.Context, id uint, at time.Time) (deleted bool, err error)
	// ListReactions returns the reactions of the messages, with the name of their user, oldest
	// first.
	ListReactions(ctx context.Context, messageIDs []uint) ([]MessageReactionRow, error)
	// AddReaction saves a reaction of a user to a message that is not deleted, unless the
	// message already has maxKinds distinct keys (a new key) or the user maxPerUser reactions
	// on it (0: no limit). The message row is locked, so concurrent reactions respect the caps.
	AddReaction(ctx context.Context, messageID, userID uint, emoji string, maxKinds, maxPerUser int) (ReactionAddResult, error)
	// RemoveReaction removes a reaction of a user; removed is false when there was none.
	RemoveReaction(ctx context.Context, messageID, userID uint, emoji string) (removed bool, err error)
	// MarkRead moves the last read message of a participant forward to messageID (a message of
	// the conversation); changed is false when it was already there or past it.
	MarkRead(ctx context.Context, conversationID, userID, messageID uint) (changed bool, err error)
	// SetMuted mutes or unmutes a conversation for a participant.
	SetMuted(ctx context.Context, conversationID, userID uint, muted bool) error

	// GetSettings returns the message settings of a user (the defaults without a row).
	GetSettings(ctx context.Context, userID uint) (MessageSettings, error)
	// SaveSettings changes the given settings (nil = unchanged).
	SaveSettings(ctx context.Context, userID uint, policy *string, toasts *bool) error

	// SceneCards, MarkerCards and PlaylistCards read the cards of shared items (unknown ids
	// are left out). Trashed and deleted scenes are returned with their dates.
	SceneCards(ctx context.Context, ids []uint) ([]ShareSceneRow, error)
	MarkerCards(ctx context.Context, ids []uint) ([]ShareMarkerRow, error)
	PlaylistCards(ctx context.Context, ids []uint) ([]SharePlaylistRow, error)
	// PlaylistIDByUUID returns the id of a playlist (gorm.ErrRecordNotFound when unknown).
	PlaylistIDByUUID(ctx context.Context, id uuid.UUID) (uint, error)
	// MarkerVisibleTo reports whether the user sees the marker (their own, or shared by its
	// owner: VisibleMarkersSQL).
	MarkerVisibleTo(ctx context.Context, markerID, userID uint) (bool, error)

	// PurgeBefore deletes the messages created before the date; DeleteOrphanConversations the
	// conversations nobody takes part in anymore (both users deleted).
	PurgeBefore(ctx context.Context, before time.Time) (int64, error)
	DeleteOrphanConversations(ctx context.Context) (int64, error)
}

type MessageRepositoryImpl struct {
	DB *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepositoryImpl {
	return &MessageRepositoryImpl{DB: db}
}

func (r *MessageRepositoryImpl) FindOrCreateDirect(ctx context.Context, a, b uint) (*Conversation, bool, error) {
	if a == b {
		return nil, false, errors.New("a direct conversation needs two different users")
	}
	low, high := orderedPair(a, b)
	var conv Conversation
	created := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Raw(`INSERT INTO conversations (kind, direct_low_id, direct_high_id) VALUES (?, ?, ?)
			ON CONFLICT (direct_low_id, direct_high_id) WHERE kind = 'direct' DO NOTHING RETURNING id`,
			ConversationKindDirect, low, high).Scan(&ids).Error; err != nil {
			return fmt.Errorf("failed to create the conversation: %w", err)
		}
		created = len(ids) > 0
		if err := tx.Where("kind = ? AND direct_low_id = ? AND direct_high_id = ?", ConversationKindDirect, low, high).Take(&conv).Error; err != nil {
			return fmt.Errorf("failed to read the conversation: %w", err)
		}
		if err := tx.Exec(`INSERT INTO conversation_participants (conversation_id, user_id) VALUES (?, ?), (?, ?) ON CONFLICT DO NOTHING`,
			conv.ID, low, conv.ID, high).Error; err != nil {
			return fmt.Errorf("failed to add the participants: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &conv, created, nil
}

func (r *MessageRepositoryImpl) FindDirect(ctx context.Context, a, b uint) (*Conversation, error) {
	low, high := orderedPair(a, b)
	var conv Conversation
	if err := r.DB.WithContext(ctx).Where("kind = ? AND direct_low_id = ? AND direct_high_id = ?", ConversationKindDirect, low, high).Take(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *MessageRepositoryImpl) GetConversation(ctx context.Context, id uint) (*Conversation, error) {
	var conv Conversation
	if err := r.DB.WithContext(ctx).Take(&conv, id).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *MessageRepositoryImpl) ParticipantIDs(ctx context.Context, conversationID uint) ([]uint, error) {
	var ids []uint
	if err := r.DB.WithContext(ctx).Raw(`SELECT user_id FROM conversation_participants WHERE conversation_id = ? ORDER BY user_id`, conversationID).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("failed to list the participants: %w", err)
	}
	return ids, nil
}

// conversationRowSelect reads the ConversationRow of the participant aliased me (bound user
// id first). The other participant is the one of a direct conversation.
const conversationRowSelect = `SELECT c.id, c.kind, c.last_message_at, c.created_at,
	me.muted, me.last_read_message_id,
	(SELECT COUNT(*) FROM messages um WHERE um.conversation_id = c.id AND um.id > COALESCE(me.last_read_message_id, 0)
		AND um.deleted_at IS NULL AND um.sender_id IS DISTINCT FROM me.user_id) AS unread_count,
	o.user_id AS other_user_id, COALESCE(ou.username, '') AS other_username,
	COALESCE(op.display_name, '') AS other_display_name, COALESCE(op.avatar_version, 0) AS other_avatar_version,
	o.last_read_message_id AS other_last_read_message_id,
	lm.id AS last_id, lm.sender_id AS last_sender_id, COALESCE(lm.body, '') AS last_body,
	lm.attachment_kind AS last_attachment_kind, lm.deleted_at AS last_deleted_at, lm.created_at AS last_created_at
	FROM conversation_participants me
	JOIN conversations c ON c.id = me.conversation_id
	LEFT JOIN conversation_participants o ON o.conversation_id = c.id AND o.user_id <> me.user_id
	LEFT JOIN users ou ON ou.id = o.user_id
	LEFT JOIN user_profiles op ON op.user_id = o.user_id
	LEFT JOIN LATERAL (SELECT id, sender_id, LEFT(body, 280) AS body, attachment_kind, deleted_at, created_at
		FROM messages WHERE conversation_id = c.id ORDER BY id DESC LIMIT 1) lm ON true
	WHERE me.user_id = ?`

func (r *MessageRepositoryImpl) ListConversations(ctx context.Context, userID uint, page, limit int) ([]ConversationRow, int64, error) {
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM conversation_participants WHERE user_id = ?`, userID).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count the conversations: %w", err)
	}
	var rows []ConversationRow
	if err := r.DB.WithContext(ctx).Raw(conversationRowSelect+` ORDER BY COALESCE(c.last_message_at, c.created_at) DESC, c.id DESC LIMIT ? OFFSET ?`,
		userID, limit, socialOffset(page, limit)).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list the conversations: %w", err)
	}
	return rows, total, nil
}

func (r *MessageRepositoryImpl) GetConversationRow(ctx context.Context, userID, conversationID uint) (*ConversationRow, error) {
	var rows []ConversationRow
	if err := r.DB.WithContext(ctx).Raw(conversationRowSelect+` AND c.id = ? LIMIT 1`, userID, conversationID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read the conversation: %w", err)
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

func (r *MessageRepositoryImpl) UnreadSummary(ctx context.Context, userID uint) (UnreadSummary, error) {
	var s UnreadSummary
	err := r.DB.WithContext(ctx).Raw(`SELECT COALESCE(SUM(n), 0) AS messages, COUNT(*) FILTER (WHERE n > 0) AS conversations FROM (
		SELECT (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = p.conversation_id AND m.id > COALESCE(p.last_read_message_id, 0)
			AND m.deleted_at IS NULL AND m.sender_id IS DISTINCT FROM p.user_id) AS n
		FROM conversation_participants p WHERE p.user_id = ? AND NOT p.muted) t`, userID).Scan(&s).Error
	if err != nil {
		return UnreadSummary{}, fmt.Errorf("failed to count the unread messages: %w", err)
	}
	return s, nil
}

func (r *MessageRepositoryImpl) InsertMessage(ctx context.Context, msg *Message) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = time.Now().UTC()
		}
		if err := tx.Create(msg).Error; err != nil {
			return fmt.Errorf("failed to save the message: %w", err)
		}
		if err := tx.Exec(`UPDATE conversations SET last_message_at = ?, updated_at = NOW() WHERE id = ?`, msg.CreatedAt, msg.ConversationID).Error; err != nil {
			return fmt.Errorf("failed to update the conversation: %w", err)
		}
		if msg.SenderID != nil {
			if err := tx.Exec(`UPDATE conversation_participants SET last_read_message_id = ? WHERE conversation_id = ? AND user_id = ?
				AND (last_read_message_id IS NULL OR last_read_message_id < ?)`, msg.ID, msg.ConversationID, *msg.SenderID, msg.ID).Error; err != nil {
				return fmt.Errorf("failed to mark the message read: %w", err)
			}
		}
		return nil
	})
}

func (r *MessageRepositoryImpl) GetMessage(ctx context.Context, id uint) (*Message, error) {
	var m Message
	if err := r.DB.WithContext(ctx).Take(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepositoryImpl) ListMessages(ctx context.Context, conversationID, beforeID uint, limit int) ([]Message, error) {
	q := r.DB.WithContext(ctx).Where("conversation_id = ?", conversationID)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var out []Message
	if err := q.Order("id DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("failed to list the messages: %w", err)
	}
	return out, nil
}

func (r *MessageRepositoryImpl) UpdateMessageBody(ctx context.Context, id uint, body string, at time.Time) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`UPDATE messages SET body = ?, edited_at = ? WHERE id = ? AND deleted_at IS NULL`, body, at, id)
	if res.Error != nil {
		return false, fmt.Errorf("failed to edit the message: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *MessageRepositoryImpl) SoftDeleteMessage(ctx context.Context, id uint, at time.Time) (bool, error) {
	deleted := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`UPDATE messages SET body = '', attachment_kind = NULL, attachment_scene_id = NULL,
			attachment_marker_id = NULL, attachment_playlist_id = NULL, attachment_lobby_uuid = NULL, attachment_time_s = NULL,
			deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, at, id)
		if res.Error != nil {
			return fmt.Errorf("failed to delete the message: %w", res.Error)
		}
		deleted = res.RowsAffected > 0
		if !deleted {
			return nil
		}
		if err := tx.Exec(`DELETE FROM message_reactions WHERE message_id = ?`, id).Error; err != nil {
			return fmt.Errorf("failed to delete the reactions of the message: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return deleted, nil
}

func (r *MessageRepositoryImpl) ListReactions(ctx context.Context, messageIDs []uint) ([]MessageReactionRow, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	var rows []MessageReactionRow
	if err := r.DB.WithContext(ctx).Raw(`SELECT r.message_id, r.user_id, r.emoji, r.created_at,
		COALESCE(u.username, '') AS username, COALESCE(p.display_name, '') AS display_name
		FROM message_reactions r
		JOIN users u ON u.id = r.user_id
		LEFT JOIN user_profiles p ON p.user_id = r.user_id
		WHERE r.message_id = ANY(?)
		ORDER BY r.message_id, r.created_at, r.user_id, r.emoji`, uintsToInt64Array(messageIDs)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list the reactions: %w", err)
	}
	return rows, nil
}

// reactionCounts is what AddReaction checks the caps with.
type reactionCounts struct {
	Kinds    int64 `gorm:"column:kinds"`
	Mine     int64 `gorm:"column:mine"`
	KeyUsed  bool  `gorm:"column:key_used"`
	MineSame bool  `gorm:"column:mine_same"`
}

func (r *MessageRepositoryImpl) AddReaction(ctx context.Context, messageID, userID uint, emoji string, maxKinds, maxPerUser int) (ReactionAddResult, error) {
	result := ReactionAdded
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the message: reactions to one message are serialized (caps), and a message
		// deleted meanwhile is seen
		var ids []uint
		if err := tx.Raw(`SELECT id FROM messages WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, messageID).Scan(&ids).Error; err != nil {
			return fmt.Errorf("failed to lock the message: %w", err)
		}
		if len(ids) == 0 {
			result = ReactionMessageGone
			return nil
		}
		var c reactionCounts
		if err := tx.Raw(`SELECT COUNT(DISTINCT emoji) AS kinds,
			COUNT(*) FILTER (WHERE user_id = ?) AS mine,
			COALESCE(BOOL_OR(emoji = ?), false) AS key_used,
			COALESCE(BOOL_OR(user_id = ? AND emoji = ?), false) AS mine_same
			FROM message_reactions WHERE message_id = ?`, userID, emoji, userID, emoji, messageID).Scan(&c).Error; err != nil {
			return fmt.Errorf("failed to count the reactions: %w", err)
		}
		switch {
		case c.MineSame:
			result = ReactionExists
			return nil
		case maxKinds > 0 && !c.KeyUsed && c.Kinds >= int64(maxKinds):
			result = ReactionTooManyKinds
			return nil
		case maxPerUser > 0 && c.Mine >= int64(maxPerUser):
			result = ReactionTooManyForUser
			return nil
		}
		if err := tx.Exec(`INSERT INTO message_reactions (message_id, user_id, emoji, created_at) VALUES (?, ?, ?, NOW())
			ON CONFLICT DO NOTHING`, messageID, userID, emoji).Error; err != nil {
			return fmt.Errorf("failed to save the reaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *MessageRepositoryImpl) RemoveReaction(ctx context.Context, messageID, userID uint, emoji string) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`DELETE FROM message_reactions WHERE message_id = ? AND user_id = ? AND emoji = ?`, messageID, userID, emoji)
	if res.Error != nil {
		return false, fmt.Errorf("failed to remove the reaction: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *MessageRepositoryImpl) MarkRead(ctx context.Context, conversationID, userID, messageID uint) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`UPDATE conversation_participants SET last_read_message_id = ?
		WHERE conversation_id = ? AND user_id = ? AND (last_read_message_id IS NULL OR last_read_message_id < ?)
		AND EXISTS (SELECT 1 FROM messages WHERE id = ? AND conversation_id = ?)`,
		messageID, conversationID, userID, messageID, messageID, conversationID)
	if res.Error != nil {
		return false, fmt.Errorf("failed to mark the conversation read: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *MessageRepositoryImpl) SetMuted(ctx context.Context, conversationID, userID uint, muted bool) error {
	if err := r.DB.WithContext(ctx).Exec(`UPDATE conversation_participants SET muted = ? WHERE conversation_id = ? AND user_id = ?`, muted, conversationID, userID).Error; err != nil {
		return fmt.Errorf("failed to mute the conversation: %w", err)
	}
	return nil
}

func (r *MessageRepositoryImpl) GetSettings(ctx context.Context, userID uint) (MessageSettings, error) {
	var rows []MessageSettings
	if err := r.DB.WithContext(ctx).Raw(`SELECT message_policy, message_toasts FROM user_social_settings WHERE user_id = ?`, userID).Scan(&rows).Error; err != nil {
		return MessageSettings{}, fmt.Errorf("failed to read the message settings: %w", err)
	}
	if len(rows) == 0 {
		return MessageSettings{Policy: MessagePolicyFriends, Toasts: true}, nil
	}
	return rows[0], nil
}

func (r *MessageRepositoryImpl) SaveSettings(ctx context.Context, userID uint, policy *string, toasts *bool) error {
	err := r.DB.WithContext(ctx).Exec(`INSERT INTO user_social_settings (user_id, message_policy, message_toasts, updated_at)
		VALUES (?, COALESCE(?::varchar, 'friends'), COALESCE(?::boolean, true), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			message_policy = COALESCE(?::varchar, user_social_settings.message_policy),
			message_toasts = COALESCE(?::boolean, user_social_settings.message_toasts),
			updated_at = NOW()`, userID, policy, toasts, policy, toasts).Error
	if err != nil {
		return fmt.Errorf("failed to save the message settings: %w", err)
	}
	return nil
}

func (r *MessageRepositoryImpl) SceneCards(ctx context.Context, ids []uint) ([]ShareSceneRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []ShareSceneRow
	if err := r.DB.WithContext(ctx).Raw(`SELECT id, title, COALESCE(duration, 0) AS duration, COALESCE(thumbnail_path, '') AS thumbnail_path,
		updated_at, trashed_at, deleted_at FROM scenes WHERE id = ANY(?)`, uintsToInt64Array(ids)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read the shared scenes: %w", err)
	}
	return rows, nil
}

func (r *MessageRepositoryImpl) MarkerCards(ctx context.Context, ids []uint) ([]ShareMarkerRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []ShareMarkerRow
	if err := r.DB.WithContext(ctx).Raw(`SELECT m.id, m.user_id, COALESCE(u.username, '') AS owner_username, m.scene_id,
		m.timestamp, m.end_timestamp, COALESCE(m.label, '') AS label, COALESCE(m.color, '') AS color,
		COALESCE(m.thumbnail_path, '') AS thumbnail_path, m.updated_at,
		s.title AS scene_title, COALESCE(s.duration, 0) AS scene_duration, s.trashed_at AS scene_trashed_at, s.deleted_at AS scene_deleted_at
		FROM user_scene_markers m JOIN scenes s ON s.id = m.scene_id LEFT JOIN users u ON u.id = m.user_id
		WHERE m.id = ANY(?)`, uintsToInt64Array(ids)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read the shared markers: %w", err)
	}
	return rows, nil
}

func (r *MessageRepositoryImpl) PlaylistCards(ctx context.Context, ids []uint) ([]SharePlaylistRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []SharePlaylistRow
	if err := r.DB.WithContext(ctx).Raw(`SELECT p.id, p.uuid, p.name, p.kind, p.visibility, p.user_id, COALESCE(u.username, '') AS owner_username,
		CASE WHEN p.kind = 'markers' THEN
			(SELECT COUNT(*) FROM playlist_markers pm JOIN user_scene_markers mk ON mk.id = pm.marker_id JOIN scenes s ON s.id = mk.scene_id
				WHERE pm.playlist_id = p.id AND s.trashed_at IS NULL AND s.deleted_at IS NULL)
		ELSE
			(SELECT COUNT(*) FROM playlist_scenes ps JOIN scenes s ON s.id = ps.scene_id
				WHERE ps.playlist_id = p.id AND s.trashed_at IS NULL AND s.deleted_at IS NULL)
		END AS item_count,
		ts.id AS thumbnail_scene_id, ts.updated_at AS thumbnail_scene_at,
		tm.id AS thumbnail_marker_id, tm.updated_at AS thumbnail_marker_at
		FROM playlists p
		LEFT JOIN users u ON u.id = p.user_id
		LEFT JOIN LATERAL (SELECT s.id, s.updated_at FROM playlist_scenes ps JOIN scenes s ON s.id = ps.scene_id
			WHERE ps.playlist_id = p.id AND s.trashed_at IS NULL AND s.deleted_at IS NULL AND COALESCE(s.thumbnail_path, '') <> ''
			ORDER BY ps.position LIMIT 1) ts ON p.kind <> 'markers'
		LEFT JOIN LATERAL (SELECT mk.id, mk.updated_at FROM playlist_markers pm JOIN user_scene_markers mk ON mk.id = pm.marker_id
			JOIN scenes s ON s.id = mk.scene_id
			WHERE pm.playlist_id = p.id AND s.trashed_at IS NULL AND s.deleted_at IS NULL AND COALESCE(mk.thumbnail_path, '') <> ''
			ORDER BY pm.position LIMIT 1) tm ON p.kind = 'markers'
		WHERE p.id = ANY(?)`, uintsToInt64Array(ids)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read the shared playlists: %w", err)
	}
	return rows, nil
}

func (r *MessageRepositoryImpl) PlaylistIDByUUID(ctx context.Context, id uuid.UUID) (uint, error) {
	var ids []uint
	if err := r.DB.WithContext(ctx).Raw(`SELECT id FROM playlists WHERE uuid = ?`, id).Scan(&ids).Error; err != nil {
		return 0, fmt.Errorf("failed to find the playlist: %w", err)
	}
	if len(ids) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return ids[0], nil
}

func (r *MessageRepositoryImpl) MarkerVisibleTo(ctx context.Context, markerID, userID uint) (bool, error) {
	var ok bool
	if err := r.DB.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM user_scene_markers WHERE id = ? AND `+VisibleMarkersSQL("")+`)`, markerID, userID).Scan(&ok).Error; err != nil {
		return false, fmt.Errorf("failed to check the marker: %w", err)
	}
	return ok, nil
}

func (r *MessageRepositoryImpl) PurgeBefore(ctx context.Context, before time.Time) (int64, error) {
	res := r.DB.WithContext(ctx).Exec(`DELETE FROM messages WHERE created_at < ?`, before)
	if res.Error != nil {
		return 0, fmt.Errorf("failed to purge the messages: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *MessageRepositoryImpl) DeleteOrphanConversations(ctx context.Context) (int64, error) {
	res := r.DB.WithContext(ctx).Exec(`DELETE FROM conversations c WHERE NOT EXISTS (SELECT 1 FROM conversation_participants p WHERE p.conversation_id = c.id)`)
	if res.Error != nil {
		return 0, fmt.Errorf("failed to delete the orphan conversations: %w", res.Error)
	}
	return res.RowsAffected, nil
}
