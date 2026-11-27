// Package together is the server side of Goon2gether, the synchronized watch lobbies: the hub
// holding the live lobbies (playback state, queue, members, chat), the WebSocket protocol
// messages and the custom emotes. Contract: docs/GOON2GETHER.md.
package together

import "encoding/json"

// ProtocolVersion is the version of the WebSocket protocol (docs/GOON2GETHER.md).
const ProtocolVersion = 1

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
)

// Bye reasons.
const (
	ByeKicked    = "kicked"
	ByeBanned    = "banned"
	ByeClosed    = "closed"
	ByeForbidden = "forbidden"
	ByeLeft      = "left"
)

// UserRef identifies a user in payloads.
type UserRef struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
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
	InviteCode  string `json:"invite_code"`
}

// Member is a lobby member as sent to the room.
type Member struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
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
	ID       uint     `json:"id"`
	SceneID  uint     `json:"scene_id"`
	Title    string   `json:"title"`
	Duration int      `json:"duration"`
	Start    *float64 `json:"start"`
	End      *float64 `json:"end"`
	Label    *string  `json:"label"`
	AddedBy  *UserRef `json:"added_by"`
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

// ChatMessage is a chat or system message.
type ChatMessage struct {
	ID       int64   `json:"id"`
	UserID   *uint   `json:"user_id"`
	Username *string `json:"username"`
	Text     string  `json:"text"`
	At       int64   `json:"at"`
	System   bool    `json:"system"`
}

// ClientConfig is the part of the configuration the clients need.
type ClientConfig struct {
	DriftThreshold float64 `json:"drift_threshold"`
	DriftNudge     bool    `json:"drift_nudge"`
	HoldTimeoutMs  int64   `json:"hold_timeout_ms"`
	ChatMaxLength  int     `json:"chat_max_length"`
	EndGrace       float64 `json:"end_grace"`
}

// You is the connected user in the welcome message.
type You struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Moderate bool   `json:"moderate"`
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

type chatDeletedMsg struct {
	T  string `json:"t"`
	ID int64  `json:"id"`
}

type reactionMsg struct {
	T        string `json:"t"`
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Emote    string `json:"emote"`
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
	Ban         bool            `json:"ban"`
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
