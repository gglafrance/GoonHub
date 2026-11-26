package data

import (
	"time"

	"github.com/google/uuid"
)

// Conversation kinds. Only direct (one-to-one) conversations exist for now; the participants
// table lets group conversations come later (docs/MESSAGES.md).
const ConversationKindDirect = "direct"

// Attachment kinds: what a message shares, resolved at read time (docs/MESSAGES.md).
const (
	AttachmentScene    = "scene"
	AttachmentMarker   = "marker"
	AttachmentPlaylist = "playlist"
	AttachmentLobby    = "lobby"
)

// ValidAttachmentKind reports whether kind is a known attachment kind.
func ValidAttachmentKind(kind string) bool {
	switch kind {
	case AttachmentScene, AttachmentMarker, AttachmentPlaylist, AttachmentLobby:
		return true
	}
	return false
}

// Conversation is a conversation between users. DirectLowID/DirectHighID are the pair of a
// direct conversation (smaller id first), NULL once that user is deleted.
type Conversation struct {
	ID            uint       `gorm:"primarykey"`
	Kind          string     `gorm:"size:10;not null;default:'direct'"`
	DirectLowID   *uint      `gorm:"column:direct_low_id"`
	DirectHighID  *uint      `gorm:"column:direct_high_id"`
	LastMessageAt *time.Time `gorm:"column:last_message_at"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Conversation) TableName() string { return "conversations" }

// ConversationParticipant is a user taking part in a conversation, with their read state.
type ConversationParticipant struct {
	ConversationID    uint      `gorm:"primaryKey;autoIncrement:false"`
	UserID            uint      `gorm:"primaryKey;autoIncrement:false"`
	LastReadMessageID *uint     `gorm:"column:last_read_message_id"`
	Muted             bool      `gorm:"not null;default:false"`
	JoinedAt          time.Time `gorm:"not null;default:now()"`
}

func (ConversationParticipant) TableName() string { return "conversation_participants" }

// Message is a message of a conversation. SenderID is nil once the sender is deleted. A
// deleted message (DeletedAt) keeps its row, without body nor attachment. The attachment
// columns reference what is shared: the row going away leaves the kind with a nil id.
type Message struct {
	ID                   uint       `gorm:"primarykey"`
	ConversationID       uint       `gorm:"not null"`
	SenderID             *uint      `gorm:"column:sender_id"`
	Body                 string     `gorm:"type:text;not null;default:''"`
	AttachmentKind       *string    `gorm:"column:attachment_kind;size:16"`
	AttachmentSceneID    *uint      `gorm:"column:attachment_scene_id"`
	AttachmentMarkerID   *uint      `gorm:"column:attachment_marker_id"`
	AttachmentPlaylistID *uint      `gorm:"column:attachment_playlist_id"`
	AttachmentLobbyUUID  *uuid.UUID `gorm:"column:attachment_lobby_uuid;type:uuid"`
	AttachmentTimeS      *float64   `gorm:"column:attachment_time_s"`
	EditedAt             *time.Time `gorm:"column:edited_at"`
	DeletedAt            *time.Time `gorm:"column:deleted_at"`
	CreatedAt            time.Time  `gorm:"not null;default:now()"`
}

func (Message) TableName() string { return "messages" }

// ConversationRow is a conversation as one participant (the viewer) lists it: their read
// state, the other participant of a direct conversation (nil fields once deleted) and the
// last message.
type ConversationRow struct {
	ID                uint       `gorm:"column:id"`
	Kind              string     `gorm:"column:kind"`
	LastMessageAt     *time.Time `gorm:"column:last_message_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	Muted             bool       `gorm:"column:muted"`
	LastReadMessageID *uint      `gorm:"column:last_read_message_id"`
	UnreadCount       int64      `gorm:"column:unread_count"`

	OtherUserID            *uint  `gorm:"column:other_user_id"`
	OtherUsername          string `gorm:"column:other_username"`
	OtherDisplayName       string `gorm:"column:other_display_name"`
	OtherAvatarVersion     int64  `gorm:"column:other_avatar_version"`
	OtherLastReadMessageID *uint  `gorm:"column:other_last_read_message_id"`

	LastID             *uint      `gorm:"column:last_id"`
	LastSenderID       *uint      `gorm:"column:last_sender_id"`
	LastBody           string     `gorm:"column:last_body"`
	LastAttachmentKind *string    `gorm:"column:last_attachment_kind"`
	LastDeletedAt      *time.Time `gorm:"column:last_deleted_at"`
	LastCreatedAt      *time.Time `gorm:"column:last_created_at"`
}

// MessageSettings are the message preferences of a user (user_social_settings).
type MessageSettings struct {
	Policy string `gorm:"column:message_policy"`
	Toasts bool   `gorm:"column:message_toasts"`
}

// UnreadSummary counts what a user has not read, muted conversations excluded.
type UnreadSummary struct {
	Messages      int64 `gorm:"column:messages"`
	Conversations int64 `gorm:"column:conversations"`
}

// ShareSceneRow is what a scene card needs (no file information).
type ShareSceneRow struct {
	ID            uint       `gorm:"column:id"`
	Title         string     `gorm:"column:title"`
	Duration      int        `gorm:"column:duration"`
	ThumbnailPath string     `gorm:"column:thumbnail_path"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
	TrashedAt     *time.Time `gorm:"column:trashed_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at"`
}

// ShareMarkerRow is what a marker card needs, with its scene.
type ShareMarkerRow struct {
	ID             uint       `gorm:"column:id"`
	UserID         uint       `gorm:"column:user_id"`
	OwnerUsername  string     `gorm:"column:owner_username"`
	SceneID        uint       `gorm:"column:scene_id"`
	Timestamp      int        `gorm:"column:timestamp"`
	EndTimestamp   *int       `gorm:"column:end_timestamp"`
	Label          string     `gorm:"column:label"`
	Color          string     `gorm:"column:color"`
	ThumbnailPath  string     `gorm:"column:thumbnail_path"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	SceneTitle     string     `gorm:"column:scene_title"`
	SceneDuration  int        `gorm:"column:scene_duration"`
	SceneTrashedAt *time.Time `gorm:"column:scene_trashed_at"`
	SceneDeletedAt *time.Time `gorm:"column:scene_deleted_at"`
}

// SharePlaylistRow is what a playlist card needs: the playable item count and the first
// thumbnail (a scene id, or a marker id for a marker playlist).
type SharePlaylistRow struct {
	ID                uint       `gorm:"column:id"`
	UUID              uuid.UUID  `gorm:"column:uuid"`
	Name              string     `gorm:"column:name"`
	Kind              string     `gorm:"column:kind"`
	Visibility        string     `gorm:"column:visibility"`
	UserID            uint       `gorm:"column:user_id"`
	OwnerUsername     string     `gorm:"column:owner_username"`
	ItemCount         int64      `gorm:"column:item_count"`
	ThumbnailSceneID  *uint      `gorm:"column:thumbnail_scene_id"`
	ThumbnailSceneAt  *time.Time `gorm:"column:thumbnail_scene_at"`
	ThumbnailMarkerID *uint      `gorm:"column:thumbnail_marker_id"`
	ThumbnailMarkerAt *time.Time `gorm:"column:thumbnail_marker_at"`
}

// MessageReaction is a reaction of a user to a message: a Unicode emoji, or ":name:" of a
// server custom emote (docs/MESSAGES.md, "Reactions").
type MessageReaction struct {
	MessageID uint      `gorm:"primaryKey;autoIncrement:false"`
	UserID    uint      `gorm:"primaryKey;autoIncrement:false"`
	Emoji     string    `gorm:"primaryKey;size:64"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (MessageReaction) TableName() string { return "message_reactions" }

// MessageReactionRow is a reaction with the name of its user, as the history lists them.
type MessageReactionRow struct {
	MessageID   uint      `gorm:"column:message_id"`
	UserID      uint      `gorm:"column:user_id"`
	Emoji       string    `gorm:"column:emoji"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	Username    string    `gorm:"column:username"`
	DisplayName string    `gorm:"column:display_name"`
}

// ReactionAddResult is the outcome of MessageRepository.AddReaction.
type ReactionAddResult int

const (
	// ReactionAdded: the reaction was saved.
	ReactionAdded ReactionAddResult = iota
	// ReactionExists: the user already reacted with this key (nothing changed).
	ReactionExists
	// ReactionMessageGone: the message does not exist or is deleted.
	ReactionMessageGone
	// ReactionTooManyKinds: the message already has the most distinct keys.
	ReactionTooManyKinds
	// ReactionTooManyForUser: the user already has the most reactions on the message.
	ReactionTooManyForUser
)
