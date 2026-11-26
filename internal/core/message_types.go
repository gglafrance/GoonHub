package core

import (
	"time"

	"github.com/google/uuid"
)

// Message SSE events, always targeted at the participants of a conversation
// (EventBus.PublishToUser, docs/MESSAGES.md). The chat: prefix is in sseTargetedPrefixes.
const (
	// EventChatMessage: a new message, to every participant (the sender's other tabs too),
	// data {conversation_id, message} with the attachment resolved for each recipient.
	EventChatMessage = "chat:message"
	// EventChatMessageUpdated: a message was edited or deleted, data {conversation_id, message}.
	EventChatMessageUpdated = "chat:message_updated"
	// EventChatRead: a participant read up to a message, to every participant, data
	// {conversation_id, user_id, last_read_message_id}.
	EventChatRead = "chat:read"
	// EventChatTyping: a participant is typing, to the other participants only, data
	// {conversation_id, user_id}. Never stored.
	EventChatTyping = "chat:typing"
	// EventChatConversationUpdated: something of a conversation changed for the user (muted,
	// the other user's relation, so whether one can still send), data {conversation_id}:
	// reload it.
	EventChatConversationUpdated = "chat:conversation_updated"
)

// Read-only reasons of a conversation (ConversationView.ReadOnlyReason).
const (
	// ReadOnlyNoPermission: the viewer lacks messages:use.
	ReadOnlyNoPermission = "no_permission"
	// ReadOnlyDeletedUser: the other user's account was deleted.
	ReadOnlyDeletedUser = "deleted_user"
	// ReadOnlyPolicy: the other user's message policy no longer lets the viewer write (for
	// example they are no longer friends).
	ReadOnlyPolicy = "policy"
)

// Reasons a share card is not available (AttachmentView.Reason).
const (
	ShareReasonNotFound     = "not_found"     // the item was deleted (or the lobby closed)
	ShareReasonTrashed      = "trashed"       // the scene is in the trash
	ShareReasonNoPermission = "no_permission" // the viewer may not see this kind of item
	ShareReasonPrivate      = "private"       // a private playlist of someone else
	ShareReasonNotJoinable  = "not_joinable"  // a lobby the viewer cannot join (inviter left, banned)
)

// MessageUser is the card of a participant (no role, no login dates).
type MessageUser struct {
	ID            uint   `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	AvatarVersion int64  `json:"avatar_version"`
}

// ShareSceneCard is a shared scene as the viewer may see it: never any file information.
type ShareSceneCard struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Duration     int    `json:"duration"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// ShareMarkerCard is a shared marker: its scene and time range. A shared marker is shown to
// the recipient even when its owner does not share their markers (sharing it is the point).
type ShareMarkerCard struct {
	ID            uint   `json:"id"`
	SceneID       uint   `json:"scene_id"`
	SceneTitle    string `json:"scene_title"`
	SceneDuration int    `json:"scene_duration"`
	Start         int    `json:"start"`
	End           *int   `json:"end"`
	Label         string `json:"label"`
	Color         string `json:"color"`
	OwnerUsername string `json:"owner_username"`
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`
}

// SharePlaylistCard is a shared playlist (scenes or markers).
type SharePlaylistCard struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Visibility    string `json:"visibility"`
	OwnerUsername string `json:"owner_username"`
	ItemCount     int64  `json:"item_count"`
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`
}

// LobbyInviteScene is the item playing in an invited lobby (Title only with scenes:view).
type LobbyInviteScene struct {
	SceneID uint   `json:"scene_id"`
	Title   string `json:"title,omitempty"`
}

// LobbyInviteCard is a Goon2gether lobby invite as the viewer may see it.
type LobbyInviteCard struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Visibility   string            `json:"visibility"`
	OnlineCount  int               `json:"online_count"`
	HostUsername string            `json:"host_username,omitempty"`
	NowPlaying   *LobbyInviteScene `json:"now_playing"`
	// IsMember: the viewer is already a member (Join just opens the room)
	IsMember bool `json:"is_member"`
	// Joinable: the viewer may join through this invite (a public lobby, or the inviter is
	// still a member, and the viewer is not banned)
	Joinable bool `json:"joinable"`
}

// LobbyInvites is what the messages need of Goon2gether (together.Hub), nil when it is
// disabled.
type LobbyInvites interface {
	// CanInvite returns nil when the user may invite others to the lobby (a member of it).
	CanInvite(lobbyID uuid.UUID, inviterID uint) error
	// InviteCard describes the lobby to the viewer (nil when it no longer exists).
	InviteCard(lobbyID uuid.UUID, inviterID uint, viewer *UserAccess) (*LobbyInviteCard, error)
	// AcceptInvite makes the user a member of the lobby: a public lobby, or a private one when
	// the inviter is still a member (as if they had opened the invite link). Bans apply.
	AcceptInvite(lobbyID uuid.UUID, inviterID, userID uint, username string) error
}

// AttachmentView is the share of a message, resolved for the viewer at read time.
type AttachmentView struct {
	Kind string `json:"kind"`
	// Available: the viewer may see the item (else only Kind and Reason are set)
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	// Time: a scene shared at a time (seconds)
	Time     *float64           `json:"time,omitempty"`
	Scene    *ShareSceneCard    `json:"scene,omitempty"`
	Marker   *ShareMarkerCard   `json:"marker,omitempty"`
	Playlist *SharePlaylistCard `json:"playlist,omitempty"`
	Lobby    *LobbyInviteCard   `json:"lobby,omitempty"`
}

// MessageView is a message as a participant sees it. SenderID is nil for a deleted user; a
// deleted message has no body nor attachment.
type MessageView struct {
	ID             uint            `json:"id"`
	ConversationID uint            `json:"conversation_id"`
	SenderID       *uint           `json:"sender_id"`
	Body           string          `json:"body"`
	Attachment     *AttachmentView `json:"attachment"`
	EditedAt       *time.Time      `json:"edited_at"`
	Deleted        bool            `json:"deleted"`
	CreatedAt      time.Time       `json:"created_at"`
}

// MessagePreview is the last message of a conversation in the lists.
type MessagePreview struct {
	ID             uint      `json:"id"`
	SenderID       *uint     `json:"sender_id"`
	Body           string    `json:"body"`
	AttachmentKind *string   `json:"attachment_kind"`
	Deleted        bool      `json:"deleted"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConversationView is a conversation as one participant sees it.
type ConversationView struct {
	ID   uint   `json:"id"`
	Kind string `json:"kind"`
	// Other is the other participant of a direct conversation (nil: deleted user)
	Other                  *MessageUser    `json:"other"`
	LastMessage            *MessagePreview `json:"last_message"`
	UnreadCount            int64           `json:"unread_count"`
	Muted                  bool            `json:"muted"`
	LastReadMessageID      *uint           `json:"last_read_message_id"`
	OtherLastReadMessageID *uint           `json:"other_last_read_message_id"`
	// CanSend: the viewer may write in it now (permission, the other's policy)
	CanSend        bool      `json:"can_send"`
	ReadOnlyReason string    `json:"read_only_reason,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ConversationPage is a page of the conversations of a user.
type ConversationPage struct {
	Conversations []ConversationView
	Total         int64
}

// DirectThread is GET /messages/with/:username: the user, the conversation with them (nil
// before the first message) and whether the viewer may write.
type DirectThread struct {
	User           MessageUser       `json:"user"`
	Conversation   *ConversationView `json:"conversation"`
	CanSend        bool              `json:"can_send"`
	ReadOnlyReason string            `json:"read_only_reason,omitempty"`
}

// MessageHistory is a page of messages, oldest first; HasMore: older ones exist.
type MessageHistory struct {
	Messages []MessageView `json:"messages"`
	HasMore  bool          `json:"has_more"`
}

// AttachmentInput is what a message shares: Kind and the matching reference.
type AttachmentInput struct {
	Kind string `json:"kind"`
	// SceneID: kind scene; Time (seconds, optional): share the scene at that time
	SceneID uint     `json:"scene_id,omitempty"`
	Time    *float64 `json:"time,omitempty"`
	// MarkerID: kind marker
	MarkerID uint `json:"marker_id,omitempty"`
	// PlaylistUUID: kind playlist
	PlaylistUUID string `json:"playlist_uuid,omitempty"`
	// LobbyID: kind lobby (the lobby UUID)
	LobbyID string `json:"lobby_id,omitempty"`
}

// SendMessageInput is a message to send: a body, an attachment, or both.
type SendMessageInput struct {
	Body       string           `json:"body"`
	Attachment *AttachmentInput `json:"attachment"`
}

// ShareInput sends items to several users: one message per item and user, the body on the
// first one.
type ShareInput struct {
	UserIDs []uint            `json:"user_ids"`
	Body    string            `json:"body"`
	Items   []AttachmentInput `json:"items"`
}

// ShareResult is the outcome of a share for one user.
type ShareResult struct {
	UserID         uint   `json:"user_id"`
	OK             bool   `json:"ok"`
	ConversationID uint   `json:"conversation_id,omitempty"`
	Code           string `json:"code,omitempty"`
	Error          string `json:"error,omitempty"`
}

// MessagePolicyInfo describes a message policy for the settings UI.
type MessagePolicyInfo struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// MessagePolicies lists who may send a user messages (data.MessagePolicy*), widest first.
var MessagePolicies = []MessagePolicyInfo{
	{"everyone", "Everyone", "Any signed-in user"},
	{"mutuals", "Friends and mutuals", "Your friends, and users you follow who follow you back"},
	{"friends", "Friends", "Only your friends"},
	{"nobody", "Nobody", "Nobody can write to you (you can still read your conversations)"},
}

func validMessagePolicy(p string) bool {
	for _, info := range MessagePolicies {
		if info.Value == p {
			return true
		}
	}
	return false
}

// MessageSettingsView is GET/PUT /messages/settings.
type MessageSettingsView struct {
	Policy   string              `json:"policy"`
	Toasts   bool                `json:"toasts"`
	Policies []MessagePolicyInfo `json:"policies"`
	// MaxLength is the longest message body (characters)
	MaxLength int `json:"max_length"`
	// TypingIntervalMs: send the typing indicator at most this often
	TypingIntervalMs int64 `json:"typing_interval_ms"`
}

// UnreadView is GET /messages/unread (muted conversations excluded).
type UnreadView struct {
	Messages      int64 `json:"messages"`
	Conversations int64 `json:"conversations"`
}

// MessageActor is the user calling the message service, with their live access.
type MessageActor struct {
	ID       uint
	Username string
	Access   *UserAccess
}
