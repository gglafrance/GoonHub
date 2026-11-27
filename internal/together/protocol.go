// Package together is the server side of Goon2gether, the synchronized watch lobbies: the hub
// holding the live lobbies (playback state, queue, members, chat), the WebSocket protocol
// messages and the custom emotes. Contract: docs/GOON2GETHER.md.
package together

import "encoding/json"

// ProtocolVersion is the version of the WebSocket protocol (docs/GOON2GETHER.md). Version 2:
// anonymous members (user_id may be null), auto-next mode, inactivity close. Version 3:
// reactions on chat messages (chat_react, chat_unreact, chat_reactions, ChatMessage.reactions).
// Version 4: history of the played items (welcome.history, history_add, config.history_max).
const ProtocolVersion = 4

// MinProtocolVersion is the oldest version the server still accepts: versions 3 and 4 only
// add fields and message types, which older clients ignore.
const MinProtocolVersion = 2

// WebSocket close codes sent by the server.
const (
	CloseKicked    = 4001
	CloseForbidden = 4003
	CloseClosed    = 4004
	CloseLeft      = 4005
	ClosePolicy    = 4008
	CloseFull      = 4009
)

// Error codes of the error message.
const (
	ErrCodeNotAllowed  = "not_allowed"
	ErrCodeInvalid     = "invalid"
	ErrCodeNotFound    = "not_found"
	ErrCodeRateLimited = "rate_limited"
	ErrCodeTooLong     = "too_long"
	ErrCodeQueueFull   = "queue_full"
	ErrCodeUnknownType = "unknown_type"
	ErrCodeInternal    = "internal"

	// ErrCodeReactionLimit: the chat message has the most distinct reactions, or the member
	// the most reactions on it
	ErrCodeReactionLimit = "reaction_limit"
)

// Server to client message types.
const (
	msgWelcome     = "welcome"
	msgState       = "state"
	msgQueue       = "queue"
	msgMembers     = "members"
	msgLobby       = "lobby"
	msgChat        = "chat"
	msgChatDeleted = "chat_deleted"
	msgReaction    = "reaction"
	msgEmotes      = "emotes"
	msgPong        = "pong"
	msgError       = "error"
	msgBye         = "bye"
	msgClosing     = "closing"
	// msgChatReactions: the reactions of a chat message changed (the whole list)
	msgChatReactions = "chat_reactions"
	// msgHistoryAdd: an item finished playing, a new history entry (version 4)
	msgHistoryAdd = "history_add"
)

// Client to server message types.
const (
	cmdPing             = "ping"
	cmdPlay             = "play"
	cmdPause            = "pause"
	cmdSeek             = "seek"
	cmdRate             = "rate"
	cmdSkip             = "skip"
	cmdPlayItem         = "play_item"
	cmdEnded            = "ended"
	cmdStatus           = "status"
	cmdQueueAdd         = "queue_add"
	cmdQueueRemove      = "queue_remove"
	cmdQueueMove        = "queue_move"
	cmdSuggestionAccept = "suggestion_accept"
	cmdSuggestionReject = "suggestion_reject"
	cmdChat             = "chat"
	cmdChatDelete       = "chat_delete"
	cmdReaction         = "reaction"
	cmdSettings         = "settings"
	cmdGrantControl     = "grant_control"
	cmdRevokeControl    = "revoke_control"
	cmdTransferHost     = "transfer_host"
	cmdKick             = "kick"
	cmdAutoNext         = "auto_next"
	cmdKeepOpen         = "keep_open"
	cmdChatReact        = "chat_react"
	cmdChatUnreact      = "chat_unreact"
)

// Bye reasons.
const (
	ByeKicked    = "kicked"
	ByeBanned    = "banned"
	ByeClosed    = "closed"
	ByeForbidden = "forbidden"
	ByeLeft      = "left"
	ByeInactive  = "inactive" // closed for inactivity
)

// Reasons of the closing message.
const closingInactive = "inactive"

// UserRef identifies a user in payloads.
type UserRef struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// MemberRef identifies who did something (added a queue item). For an anonymous member, the
// other members get a null ID and "Anonymous #N" (moderators get the real identity, with
// anonymous set).
type MemberRef struct {
	ID         *uint  `json:"id"`
	Username   string `json:"username"`
	Anonymous  bool   `json:"anonymous,omitempty"`
	AnonNumber *int   `json:"anon_number,omitempty"`
}

// LobbyInfo is the lobby settings sent to its members.
type LobbyInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Visibility  string `json:"visibility"`
	HostID      *uint  `json:"host_id"`
	ControlOpen bool   `json:"control_open"`
	QueueOpen   bool   `json:"queue_open"`
	WaitForAll  bool   `json:"wait_for_all"`
	MaxMembers  int    `json:"max_members"`
	AutoNext    string `json:"auto_next"`
	InviteCode  string `json:"invite_code"`
}

// Member is a lobby member as sent to the room. An anonymous member is sent with a null
// UserID and "Anonymous #N" as Username, except to moderators (real identity, Anonymous set).
type Member struct {
	UserID      *uint  `json:"user_id"`
	Username    string `json:"username"`
	Anonymous   bool   `json:"anonymous"`
	AnonNumber  *int   `json:"anon_number"`
	Host        bool   `json:"host"`
	Control     bool   `json:"control"`
	Online      bool   `json:"online"`
	Connections int    `json:"connections"`
	Buffering   bool   `json:"buffering"`
	JoinedAt    string `json:"joined_at"`
}

// QueueItem is a queue item or a suggestion. Only the scene title and duration are sent: no
// file information.
type QueueItem struct {
	ID       uint       `json:"id"`
	SceneID  uint       `json:"scene_id"`
	Title    string     `json:"title"`
	Duration int        `json:"duration"`
	Start    *float64   `json:"start"`
	End      *float64   `json:"end"`
	Label    *string    `json:"label"`
	AddedBy  *MemberRef `json:"added_by"`
}

// HistoryEntry is an item the lobby played (version 4): the queue item fields (scene title
// and duration only, adder redacted like the queue), when its play started and ended (Unix
// ms), how it ended, and whether the scene can still be queued (not in the trash).
type HistoryEntry struct {
	ID        uint       `json:"id"`
	SceneID   uint       `json:"scene_id"`
	Title     string     `json:"title"`
	Duration  int        `json:"duration"`
	Start     *float64   `json:"start"`
	End       *float64   `json:"end"`
	Label     *string    `json:"label"`
	AddedBy   *MemberRef `json:"added_by"`
	StartedAt int64      `json:"started_at"`
	EndedAt   int64      `json:"ended_at"`
	Outcome   string     `json:"outcome"`
	Available bool       `json:"available"`
}

// State is the playback state of a lobby: Position (seconds) at ServerTime (Unix ms).
type State struct {
	ItemID     *uint   `json:"item_id"`
	Playing    bool    `json:"playing"`
	Hold       bool    `json:"hold"`
	Position   float64 `json:"position"`
	ServerTime int64   `json:"server_time"`
	Rate       float64 `json:"rate"`
	Seq        uint64  `json:"seq"`
}

// ChatMessage is a chat or system message. The history keeps the real author; view redacts
// it for the members who may not see it.
type ChatMessage struct {
	ID         int64   `json:"id"`
	UserID     *uint   `json:"user_id"`
	Username   *string `json:"username"`
	Anonymous  bool    `json:"anonymous,omitempty"`
	AnonNumber *int    `json:"anon_number,omitempty"`
	Text       string  `json:"text"`
	At         int64   `json:"at"`
	System     bool    `json:"system"`
	// Reactions of the members (version 3), omitted when there is none
	Reactions []ChatReaction `json:"reactions,omitempty"`

	// modText is the text moderators get (a system message naming an anonymous member)
	modText string
	// reactions are the live reactions, by key in the order of their first reaction
	reactions []*chatReaction
}

// ChatReaction is one reaction key of a chat message: an emoji or ":name:" of a server emote,
// how many members reacted with it and who (anonymous members redacted like everywhere).
type ChatReaction struct {
	Emote string      `json:"emote"`
	Count int         `json:"count"`
	Users []MemberRef `json:"users"`
}

// reactor is a member who reacted (anon > 0: as "Anonymous #anon").
type reactor struct {
	userID   uint
	username string
	anon     int
}

// ref is the reactor as sent: full (moderators) or redacted.
func (r reactor) ref(full bool) MemberRef {
	if r.anon > 0 && !full {
		return MemberRef{Username: anonName(r.anon), Anonymous: true, AnonNumber: anonPtr(r.anon)}
	}
	id := r.userID
	return MemberRef{ID: &id, Username: r.username, Anonymous: r.anon > 0, AnonNumber: anonPtr(r.anon)}
}

// chatReaction is a reaction key of a chat message with its reactors, in reaction order.
type chatReaction struct {
	emote string
	users []reactor
}

// reactionViews is the reaction list as sent (nil when there is none).
func reactionViews(list []*chatReaction, full bool) []ChatReaction {
	if len(list) == 0 {
		return nil
	}
	out := make([]ChatReaction, 0, len(list))
	for _, r := range list {
		users := make([]MemberRef, 0, len(r.users))
		for _, u := range r.users {
			users = append(users, u.ref(full))
		}
		out = append(out, ChatReaction{Emote: r.emote, Count: len(r.users), Users: users})
	}
	return out
}

// view is the message as sent to a member: full (moderators) or redacted.
func (m ChatMessage) view(full bool) ChatMessage {
	m.Reactions = reactionViews(m.reactions, full)
	if full {
		if m.modText != "" {
			m.Text = m.modText
		}
		return m
	}
	if m.Anonymous && m.AnonNumber != nil {
		name := anonName(*m.AnonNumber)
		m.UserID, m.Username = nil, &name
	}
	return m
}

// ClientConfig is the part of the configuration the clients need.
type ClientConfig struct {
	DriftThreshold float64 `json:"drift_threshold"`
	DriftNudge     bool    `json:"drift_nudge"`
	HoldTimeoutMs  int64   `json:"hold_timeout_ms"`
	ChatMaxLength  int     `json:"chat_max_length"`
	EndGrace       float64 `json:"end_grace"`
	// Reactions on chat messages (version 3): distinct keys per message, reactions of one
	// member on one message
	ChatMaxReactionKinds    int `json:"chat_max_reaction_kinds"`
	ChatMaxReactionsPerUser int `json:"chat_max_reactions_per_user"`
	// History entries kept per lobby (version 4, 0: no history)
	HistoryMax int `json:"history_max"`
}

// You is the connected user in the welcome message.
type You struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Moderate   bool   `json:"moderate"`
	Anonymous  bool   `json:"anonymous"`
	AnonNumber *int   `json:"anon_number"`
}

type welcomeMsg struct {
	T           string        `json:"t"`
	V           int           `json:"v"`
	ServerTime  int64         `json:"server_time"`
	You         You           `json:"you"`
	Config      ClientConfig  `json:"config"`
	Lobby       LobbyInfo     `json:"lobby"`
	Members     []Member      `json:"members"`
	Queue       []QueueItem   `json:"queue"`
	Suggestions []QueueItem   `json:"suggestions"`
	State       State         `json:"state"`
	Chat        []ChatMessage `json:"chat"`
	ClosingAt   *int64        `json:"closing_at"`
	// History is the played items, newest first (version 4)
	History []HistoryEntry `json:"history"`
}

type stateMsg struct {
	T     string `json:"t"`
	State State  `json:"state"`
}

type queueMsg struct {
	T           string      `json:"t"`
	Queue       []QueueItem `json:"queue"`
	Suggestions []QueueItem `json:"suggestions"`
}

type membersMsg struct {
	T       string   `json:"t"`
	Members []Member `json:"members"`
}

type lobbyMsg struct {
	T     string    `json:"t"`
	Lobby LobbyInfo `json:"lobby"`
}

type chatMsg struct {
	T       string      `json:"t"`
	Message ChatMessage `json:"message"`
}

// historyAddMsg carries a new history entry (the newest).
type historyAddMsg struct {
	T     string       `json:"t"`
	Entry HistoryEntry `json:"entry"`
}

type chatDeletedMsg struct {
	T  string `json:"t"`
	ID int64  `json:"id"`
}

// chatReactionsMsg carries the whole reaction list of a chat message after a change.
type chatReactionsMsg struct {
	T         string         `json:"t"`
	ID        int64          `json:"id"`
	Reactions []ChatReaction `json:"reactions"`
}

type reactionMsg struct {
	T          string `json:"t"`
	UserID     *uint  `json:"user_id"`
	Username   string `json:"username"`
	Anonymous  bool   `json:"anonymous,omitempty"`
	AnonNumber *int   `json:"anon_number,omitempty"`
	Emote      string `json:"emote"`
}

// closingMsg announces that the lobby closes at At (Unix ms) unless something happens; a null
// At cancels the announce.
type closingMsg struct {
	T      string `json:"t"`
	At     *int64 `json:"at"`
	Reason string `json:"reason,omitempty"`
}

type pongMsg struct {
	T  string          `json:"t"`
	ID json.RawMessage `json:"id"`
	C  json.RawMessage `json:"c"`
	S  int64           `json:"s"`
}

type errorMsg struct {
	T       string `json:"t"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Ref     string `json:"ref,omitempty"`
}

type byeMsg struct {
	T      string `json:"t"`
	Reason string `json:"reason"`
}

type simpleMsg struct {
	T string `json:"t"`
}

// ItemInput is a queue item to add (REST and queue_add).
type ItemInput struct {
	SceneID uint     `json:"scene_id"`
	Start   *float64 `json:"start"`
	End     *float64 `json:"end"`
	Label   *string  `json:"label"`
}

// inbound is a client message: every field of every command (unused ones stay zero).
type inbound struct {
	T           string          `json:"t"`
	Ref         string          `json:"ref"`
	ID          json.RawMessage `json:"id"`
	C           json.RawMessage `json:"c"`
	Position    *float64        `json:"position"`
	Rate        *float64        `json:"rate"`
	ItemID      *uint           `json:"item_id"`
	Index       *int            `json:"index"`
	Buffering   *bool           `json:"buffering"`
	Seq         *uint64         `json:"seq"`
	Items       []ItemInput     `json:"items"`
	PlayNow     bool            `json:"play_now"`
	Text        *string         `json:"text"`
	Emote       *string         `json:"emote"`
	UserID      *uint           `json:"user_id"`
	Anon        *int            `json:"anon"`
	Ban         bool            `json:"ban"`
	Mode        *string         `json:"mode"`
	Name        *string         `json:"name"`
	Visibility  *string         `json:"visibility"`
	ControlOpen *bool           `json:"control_open"`
	QueueOpen   *bool           `json:"queue_open"`
	WaitForAll  *bool           `json:"wait_for_all"`
	MaxMembers  *int            `json:"max_members"`
}

// cmdError is a refused command, sent back as an error message.
type cmdError struct {
	code    string
	message string
}

func (e *cmdError) Error() string { return e.code + ": " + e.message }

func refuse(code, message string) *cmdError { return &cmdError{code: code, message: message} }
