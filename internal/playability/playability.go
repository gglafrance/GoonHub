// Package playability decides whether a browser can play a video file directly, from its
// container (file extension) and its codecs as reported by ffprobe. The rule lives here
// alone: Check is the Go form and SQLUnplayable the equivalent SQL condition, both built from
// the same lists, so the library health page, a future transcoding decision or any other
// caller agree on what "unplayable" means.
package playability

import (
	"path/filepath"
	"strings"
)

// Status is the outcome of a playability check.
type Status string

const (
	// StatusPlayable means the container and every codec are supported by mainstream browsers.
	StatusPlayable Status = "playable"
	// StatusUnplayable means at least one of the container, video codec or audio codec is not.
	StatusUnplayable Status = "unplayable"
	// StatusUnknown means the video codec is not known yet (metadata not extracted).
	StatusUnknown Status = "unknown"
)

// Reasons reported for an unplayable file.
const (
	ReasonContainer  = "container"
	ReasonVideoCodec = "video_codec"
	ReasonAudioCodec = "audio_codec"
)

// Containers (lower-case file extensions, without the dot) that Chrome, Firefox and Safari
// play natively. MKV, AVI, WMV, MPEG-TS and the like are not in it.
var playableContainers = []string{"mp4", "m4v", "mov", "webm"}

// Video codecs (ffprobe codec_name) that every mainstream browser decodes. HEVC is left out
// on purpose: Firefox does not play it and Chrome only with hardware support.
var playableVideoCodecs = []string{"h264", "vp8", "vp9", "av1"}

// Audio codecs (ffprobe codec_name) that browsers decode. An empty audio codec means the file
// has no audio track, which is fine.
var playableAudioCodecs = []string{"aac", "mp3", "opus", "vorbis", "flac"}

// Result is the detailed outcome of Check.
type Result struct {
	Status  Status   `json:"status"`
	Reasons []string `json:"reasons,omitempty"`
}

// Container returns the normalized container of a file path: its lower-case extension
// without the dot ("" when there is none).
func Container(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

func normalizeCodec(codec string) string {
	return strings.ToLower(strings.TrimSpace(codec))
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// Check tells whether a browser can play the file at path with the given ffprobe codecs.
func Check(path, videoCodec, audioCodec string) Result {
	video := normalizeCodec(videoCodec)
	if video == "" {
		return Result{Status: StatusUnknown}
	}
	var reasons []string
	if !contains(playableContainers, Container(path)) {
		reasons = append(reasons, ReasonContainer)
	}
	if !contains(playableVideoCodecs, video) {
		reasons = append(reasons, ReasonVideoCodec)
	}
	if audio := normalizeCodec(audioCodec); audio != "" && !contains(playableAudioCodecs, audio) {
		reasons = append(reasons, ReasonAudioCodec)
	}
	if len(reasons) > 0 {
		return Result{Status: StatusUnplayable, Reasons: reasons}
	}
	return Result{Status: StatusPlayable}
}

// IsBrowserPlayable is a shortcut for Check(...).Status == StatusPlayable.
func IsBrowserPlayable(path, videoCodec, audioCodec string) bool {
	return Check(path, videoCodec, audioCodec).Status == StatusPlayable
}

// SQLUnplayable returns a PostgreSQL condition (with "?" placeholders, as used by GORM) that
// is true exactly when Check returns StatusUnplayable for the given columns.
func SQLUnplayable(pathCol, videoCol, audioCol string) (string, []any) {
	container := "COALESCE(lower(substring(" + pathCol + ` from '\.([^./]+)$')), '')`
	video := "lower(btrim(COALESCE(" + videoCol + ", '')))"
	audio := "lower(btrim(COALESCE(" + audioCol + ", '')))"
	cond := "(" + video + " <> '' AND (" +
		container + " NOT IN ? OR " +
		video + " NOT IN ? OR " +
		"(" + audio + " <> '' AND " + audio + " NOT IN ?)))"
	return cond, []any{playableContainers, playableVideoCodecs, playableAudioCodecs}
}
