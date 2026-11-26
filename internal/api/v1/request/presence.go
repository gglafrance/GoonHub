package request

// PresenceHeartbeatRequest is the body of POST /social/presence/heartbeat: active is true when
// the user used the tab (input) since the last heartbeat.
type PresenceHeartbeatRequest struct {
	Active bool `json:"active"`
}

// PresenceActivityRequest is the body of PUT /social/presence/activity: the activity of one tab
// (docs/PRESENCE.md). For watching_scene: scene_id, position (seconds) and paused.
type PresenceActivityRequest struct {
	TabID    string  `json:"tab_id"`
	Type     string  `json:"type" binding:"required"`
	SceneID  uint    `json:"scene_id"`
	Position float64 `json:"position"`
	Paused   bool    `json:"paused"`
}

// PresenceSettingsRequest is the body of PUT /social/presence/settings.
type PresenceSettingsRequest struct {
	Invisible *bool `json:"invisible" binding:"required"`
}
