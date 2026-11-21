package streaming

import "goonhub/internal/playability"

// The playability rule (what browsers play, what a conversion can copy) lives in the
// playability package; these aliases keep the streaming API readable.

// PlaybackMode tells how a scene reaches the browser.
type PlaybackMode = playability.Mode

// PlaybackPlan is the conversion decision for one file.
type PlaybackPlan = playability.Plan

const (
	PlaybackDirect    = playability.ModeDirect
	PlaybackRemux     = playability.ModeRemux
	PlaybackTranscode = playability.ModeTranscode
)

// PlaybackInfo is what the scene API exposes to the player.
type PlaybackInfo struct {
	Mode   PlaybackMode `json:"mode"`
	Reason string       `json:"reason,omitempty"`
	// Whether the server converts files on the fly (admin setting). When false, scenes
	// that are not "direct" can't be played in the browser.
	TranscodeEnabled bool `json:"transcode_enabled"`
}

// DecidePlayback decides from the container and the stored codecs how the browser gets
// the file (see playability.Decide).
func DecidePlayback(storedPath, videoCodec, audioCodec string) PlaybackPlan {
	return playability.Decide(storedPath, videoCodec, audioCodec)
}

// FullTranscodePlan re-encodes both streams (see playability.FullTranscode).
func FullTranscodePlan() PlaybackPlan {
	return playability.FullTranscode()
}
