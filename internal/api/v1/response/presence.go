package response

import "goonhub/internal/core"

// PresenceSnapshotResponse is GET /social/presence: the presence of the friends and followed
// users the viewer may see, the viewer's invisible mode and the client timings.
type PresenceSnapshotResponse struct {
	Users                    []core.PresenceView `json:"users"`
	Invisible                bool                `json:"invisible"`
	HeartbeatIntervalSeconds int                 `json:"heartbeat_interval_seconds"`
	ActivityIntervalSeconds  int                 `json:"activity_interval_seconds"`
	IdleAfterSeconds         int                 `json:"idle_after_seconds"`
	StalledAfterSeconds      int                 `json:"stalled_after_seconds"`
}

func NewPresenceSnapshotResponse(s *core.PresenceSnapshot) PresenceSnapshotResponse {
	users := s.Users
	if users == nil {
		users = []core.PresenceView{}
	}
	return PresenceSnapshotResponse{
		Users:                    users,
		Invisible:                s.Invisible,
		HeartbeatIntervalSeconds: int(s.HeartbeatInterval.Seconds()),
		ActivityIntervalSeconds:  int(s.ActivityInterval.Seconds()),
		IdleAfterSeconds:         int(s.IdleAfter.Seconds()),
		StalledAfterSeconds:      int(s.StalledAfter.Seconds()),
	}
}

// PresenceSettingsResponse is GET/PUT /social/presence/settings.
type PresenceSettingsResponse struct {
	Invisible bool `json:"invisible"`
}
