package core

import (
	"time"

	"goonhub/internal/config"
)

// Presence (docs/PRESENCE.md): who is online, idle or offline, and what they are doing.

// EventPresenceUpdate is the presence SSE event: the presence of one user as the receiving
// viewer may see it (PresenceView). Always targeted (EventBus.PublishToUser).
const EventPresenceUpdate = "presence:update"

// Presence statuses.
const (
	PresenceOnline  = "online"
	PresenceIdle    = "idle"
	PresenceOffline = "offline"
)

// Activity types. A feature adds its type here, a branch in validateActivity and, when its
// details need it, a check in activityView (docs/PRESENCE.md, "Adding an activity type").
const (
	// ActivityWatchingScene: playing a scene (watch page, playlist playback).
	ActivityWatchingScene = "watching_scene"
)

// PresenceActivityInput is an activity reported by a client tab.
type PresenceActivityInput struct {
	// TabID identifies the reporting tab (several tabs keep one activity each).
	TabID    string
	Type     string
	SceneID  uint
	Position float64
	Paused   bool
}

// PresenceUserRef identifies a user in a presence view.
type PresenceUserRef struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// PresenceScene is the scene of a watching_scene activity: title and thumbnail only, never
// file information.
type PresenceScene struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Duration     int    `json:"duration"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// PresenceActivityView is an activity as a viewer may see it. Scene, Position, PositionAt and
// Paused are absent when the viewer cannot see the scene ("watching something").
type PresenceActivityView struct {
	Type      string         `json:"type"`
	StartedAt time.Time      `json:"started_at"`
	Scene     *PresenceScene `json:"scene,omitempty"`
	// Position (seconds) at PositionAt; while not paused the client extrapolates it.
	Position   *float64   `json:"position,omitempty"`
	PositionAt *time.Time `json:"position_at,omitempty"`
	Paused     *bool      `json:"paused,omitempty"`
}

// PresenceView is the presence of a user as a viewer may see it (snapshot entries and
// presence:update events). Visible false: the viewer may not (or no longer) see the presence
// of that user, every other field is absent. Seq orders the views of a user: a client keeps the
// highest.
type PresenceView struct {
	User       PresenceUserRef       `json:"user"`
	Visible    bool                  `json:"visible"`
	Status     string                `json:"status,omitempty"`
	LastSeenAt *time.Time            `json:"last_seen_at,omitempty"`
	Activity   *PresenceActivityView `json:"activity,omitempty"`
	Seq        uint64                `json:"seq"`
}

// PresenceSnapshot is the initial presence state of a viewer (GET /social/presence).
type PresenceSnapshot struct {
	// Users: the friends and followed users whose presence the viewer may see.
	Users []PresenceView
	// Invisible: the viewer's own invisible mode.
	Invisible bool
	// HeartbeatInterval, ActivityInterval: how often the client reports input and playback.
	HeartbeatInterval time.Duration
	ActivityInterval  time.Duration
}

// normalizePresenceConfig fills the unset timings with the defaults.
func normalizePresenceConfig(cfg config.PresenceConfig) config.PresenceConfig {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&cfg.DisconnectGrace, 30*time.Second)
	def(&cfg.IdleAfter, 5*time.Minute)
	def(&cfg.HeartbeatInterval, 60*time.Second)
	def(&cfg.ActivityInterval, 15*time.Second)
	def(&cfg.ActivityTTL, 60*time.Second)
	def(&cfg.PausedAfter, 20*time.Second)
	def(&cfg.SeekThreshold, 15*time.Second)
	def(&cfg.FlushInterval, 2*time.Second)
	def(&cfg.LastSeenInterval, 5*time.Minute)
	if cfg.MaxTabsPerUser <= 0 {
		cfg.MaxTabsPerUser = 8
	}
	return cfg
}
