package workerproto

import (
	"fmt"
	"regexp"
	"strconv"
)

// PhaseParams carries the parameters of the assignment's phase: exactly one field is set,
// the one matching Assignment.Phase.
type PhaseParams struct {
	Sprites            *SpritesParams            `json:"sprites,omitempty"`
	AnimatedThumbnails *AnimatedThumbnailsParams `json:"animated_thumbnails,omitempty"`
	Fingerprint        *FingerprintParams        `json:"fingerprint,omitempty"`
}

// PhaseResult carries the typed result of the phase: exactly one field is set.
type PhaseResult struct {
	Sprites            *SpritesResult            `json:"sprites,omitempty"`
	AnimatedThumbnails *AnimatedThumbnailsResult `json:"animated_thumbnails,omitempty"`
	Fingerprint        *FingerprintResult        `json:"fingerprint,omitempty"`
}

// Validate checks that exactly the field of phase is set.
func (p PhaseParams) Validate(phase Phase) error {
	set := countSet(p.Sprites != nil, p.AnimatedThumbnails != nil, p.Fingerprint != nil)
	if set != 1 || !matchesPhase(phase, p.Sprites != nil, p.AnimatedThumbnails != nil, p.Fingerprint != nil) {
		return fmt.Errorf("params must carry exactly the %q field", phase)
	}
	return nil
}

// Validate checks that exactly the field of phase is set.
func (r PhaseResult) Validate(phase Phase) error {
	set := countSet(r.Sprites != nil, r.AnimatedThumbnails != nil, r.Fingerprint != nil)
	if set != 1 || !matchesPhase(phase, r.Sprites != nil, r.AnimatedThumbnails != nil, r.Fingerprint != nil) {
		return fmt.Errorf("result must carry exactly the %q field", phase)
	}
	return nil
}

// Files lists every artifact name the result references (the files the commit moves).
func (r PhaseResult) Files() []string {
	var files []string
	if r.Sprites != nil {
		files = append(files, r.Sprites.SpriteSheets...)
		if r.Sprites.VTT != "" {
			files = append(files, r.Sprites.VTT)
		}
	}
	if r.AnimatedThumbnails != nil {
		for _, m := range r.AnimatedThumbnails.Markers {
			files = append(files, m.File)
		}
		if r.AnimatedThumbnails.Preview != nil {
			files = append(files, r.AnimatedThumbnails.Preview.File)
		}
	}
	return files
}

func countSet(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

func matchesPhase(phase Phase, sprites, animated, fingerprint bool) bool {
	switch phase {
	case PhaseSprites:
		return sprites
	case PhaseAnimatedThumbnails:
		return animated
	case PhaseFingerprint:
		return fingerprint
	}
	return false
}

// ---------------------------------------------------------------------------------------
// Output names (the names the server serves today)
// ---------------------------------------------------------------------------------------

// SpriteSheetName is the name of the index-th sprite sheet (index starts at 1).
func SpriteSheetName(sceneID uint, index int) string {
	return fmt.Sprintf("%d_sheet_%03d.webp", sceneID, index)
}

// VTTName is the name of the sprite VTT file of a scene.
func VTTName(sceneID uint) string { return fmt.Sprintf("%d_thumbnails.vtt", sceneID) }

// MarkerClipName is the name of the animated thumbnail of a marker.
func MarkerClipName(markerID uint) string { return fmt.Sprintf("marker_%d.mp4", markerID) }

// ScenePreviewName is the name of the preview video of a scene.
func ScenePreviewName(sceneID uint) string { return fmt.Sprintf("%d_preview.mp4", sceneID) }

var spriteSheetRe = regexp.MustCompile(`^(\d+)_sheet_(\d{3,})\.webp$`)

// ---------------------------------------------------------------------------------------
// Sprites
// ---------------------------------------------------------------------------------------

// SpritesParams are the sprite sheet + VTT parameters (server config and scene facts).
type SpritesParams struct {
	SceneID    uint `json:"scene_id"`
	TileWidth  int  `json:"tile_width"`
	TileHeight int  `json:"tile_height"`
	// FrameInterval is the configured interval in seconds; the computation shortens it for
	// short scenes (see EffectiveSpriteInterval) and reports the value used.
	FrameInterval int `json:"frame_interval"`
	FrameQuality  int `json:"frame_quality"` // WebP quality 1-100
	GridCols      int `json:"grid_cols"`
	GridRows      int `json:"grid_rows"`
	// Concurrency is the number of parallel ffmpeg frame extractions (0 = auto). A worker
	// may override it with its own setting.
	Concurrency int `json:"concurrency"`
}

// Validate checks the parameters.
func (p *SpritesParams) Validate() error {
	if p.SceneID == 0 {
		return fmt.Errorf("sprites: scene_id is required")
	}
	if p.TileWidth <= 0 || p.TileHeight <= 0 {
		return fmt.Errorf("sprites: tile size must be positive")
	}
	if p.FrameInterval <= 0 {
		return fmt.Errorf("sprites: frame_interval must be positive")
	}
	if p.GridCols <= 0 || p.GridRows <= 0 {
		return fmt.Errorf("sprites: grid must be positive")
	}
	return nil
}

// SpritesResult lists the sprite sheets and the VTT written in the output directory.
type SpritesResult struct {
	SpriteSheets  []string `json:"sprite_sheets"`  // in order, SpriteSheetName(scene, 1..n)
	VTT           string   `json:"vtt"`            // VTTName(scene)
	SheetCount    int      `json:"sheet_count"`    // len(SpriteSheets)
	FrameInterval int      `json:"frame_interval"` // effective interval used
}

// Validate checks the result names against the scene.
func (r *SpritesResult) Validate(sceneID uint) error {
	if r.SheetCount != len(r.SpriteSheets) {
		return fmt.Errorf("sprites: sheet_count %d does not match %d sheets", r.SheetCount, len(r.SpriteSheets))
	}
	if r.VTT != VTTName(sceneID) {
		return fmt.Errorf("sprites: vtt must be %q", VTTName(sceneID))
	}
	for i, name := range r.SpriteSheets {
		m := spriteSheetRe.FindStringSubmatch(name)
		if m == nil || m[1] != strconv.FormatUint(uint64(sceneID), 10) || name != SpriteSheetName(sceneID, i+1) {
			return fmt.Errorf("sprites: sheet %d must be %q, got %q", i+1, SpriteSheetName(sceneID, i+1), name)
		}
	}
	return nil
}

// EffectiveSpriteInterval shortens the sprite frame interval for short scenes so that they
// still get at least minFrames frames (one sprite row) instead of no sprite sheet at all,
// which the scene card scrubber and the player seek preview need.
func EffectiveSpriteInterval(duration, interval, minFrames int) int {
	if interval <= 0 || duration <= 0 || minFrames <= 0 || duration/interval >= minFrames {
		return interval
	}
	short := duration / minFrames
	if short < 1 {
		short = 1
	}
	return short
}

// ---------------------------------------------------------------------------------------
// Animated thumbnails (marker clips + scene preview)
// ---------------------------------------------------------------------------------------

// AnimatedThumbnailsParams are the marker clips to make and the optional scene preview.
type AnimatedThumbnailsParams struct {
	SceneID uint             `json:"scene_id"`
	Markers []MarkerClipSpec `json:"markers"`
	// MarkerWidth is the clip width (height follows the aspect ratio), MarkerCRF its x264 CRF.
	MarkerWidth int `json:"marker_width"`
	MarkerCRF   int `json:"marker_crf"`
	// MarkerTimeoutSec bounds each clip (0 = DefaultMarkerClipTimeoutSec).
	MarkerTimeoutSec int `json:"marker_timeout_sec"`
	// Preview is nil when no scene preview must be made (disabled, already present...).
	Preview *ScenePreviewSpec `json:"preview,omitempty"`
}

// DefaultMarkerClipTimeoutSec is the time allowed to one marker clip.
const DefaultMarkerClipTimeoutSec = 120

// MarkerClipSpec is one marker animated thumbnail to make.
type MarkerClipSpec struct {
	MarkerID     uint   `json:"marker_id"`
	TimestampSec int    `json:"timestamp_sec"` // clip start
	DurationSec  int    `json:"duration_sec"`  // clip length
	Output       string `json:"output"`        // MarkerClipName(marker_id)
}

// ScenePreviewSpec describes the scene preview video.
type ScenePreviewSpec struct {
	Output          string  `json:"output"`           // ScenePreviewName(scene_id)
	Segments        int     `json:"segments"`         // sampled segments
	SegmentDuration float64 `json:"segment_duration"` // seconds per segment
	Width           int     `json:"width"`
	CRF             int     `json:"crf"`
	// WithAudio keeps the audio of each segment (the scene has an audio track); on failure
	// the preview is retried silent.
	WithAudio bool `json:"with_audio"`
}

// Validate checks the structure of the parameters and the output names.
func (p *AnimatedThumbnailsParams) Validate() error {
	if p.SceneID == 0 {
		return fmt.Errorf("animated_thumbnails: scene_id is required")
	}
	// Only the structure is checked: bad numbers fail the clip or the preview in ffmpeg,
	// a per-item best-effort failure as on the server, not the whole job.
	seen := make(map[uint]bool, len(p.Markers))
	for _, m := range p.Markers {
		if m.Output != MarkerClipName(m.MarkerID) {
			return fmt.Errorf("animated_thumbnails: marker %d output must be %q", m.MarkerID, MarkerClipName(m.MarkerID))
		}
		if seen[m.MarkerID] {
			return fmt.Errorf("animated_thumbnails: marker %d listed twice", m.MarkerID)
		}
		seen[m.MarkerID] = true
		if m.TimestampSec < 0 {
			return fmt.Errorf("animated_thumbnails: marker %d has a negative timestamp", m.MarkerID)
		}
	}
	if p.Preview != nil && p.Preview.Output != ScenePreviewName(p.SceneID) {
		return fmt.Errorf("animated_thumbnails: preview output must be %q", ScenePreviewName(p.SceneID))
	}
	return nil
}

// AnimatedThumbnailsResult lists the clips made, the per-marker failures (best effort, as
// on the server today) and the preview.
type AnimatedThumbnailsResult struct {
	Markers        []MarkerClipResult  `json:"markers"`
	MarkerFailures []MarkerFailure     `json:"marker_failures,omitempty"`
	Preview        *ScenePreviewResult `json:"preview,omitempty"`
	// PreviewError is set when the preview was asked but failed (best effort).
	PreviewError string `json:"preview_error,omitempty"`
}

// MarkerClipResult is one clip written in the output directory.
type MarkerClipResult struct {
	MarkerID uint   `json:"marker_id"`
	File     string `json:"file"`
}

// MarkerFailure is one marker whose clip could not be made.
type MarkerFailure struct {
	MarkerID uint   `json:"marker_id"`
	Error    string `json:"error"`
}

// ScenePreviewResult is the preview written in the output directory.
type ScenePreviewResult struct {
	File      string `json:"file"`
	WithAudio bool   `json:"with_audio"` // false when made silent (no audio or audio retry)
}

// Validate checks the result against the parameters it was computed from: only asked
// markers, with their asked names, and the asked preview name.
func (r *AnimatedThumbnailsResult) Validate(p *AnimatedThumbnailsParams) error {
	asked := make(map[uint]string, len(p.Markers))
	for _, m := range p.Markers {
		asked[m.MarkerID] = m.Output
	}
	seen := make(map[uint]bool, len(r.Markers))
	for _, m := range r.Markers {
		want, ok := asked[m.MarkerID]
		if !ok {
			return fmt.Errorf("animated_thumbnails: marker %d was not asked", m.MarkerID)
		}
		if m.File != want {
			return fmt.Errorf("animated_thumbnails: marker %d file must be %q", m.MarkerID, want)
		}
		if seen[m.MarkerID] {
			return fmt.Errorf("animated_thumbnails: marker %d listed twice", m.MarkerID)
		}
		seen[m.MarkerID] = true
	}
	if r.Preview != nil {
		if p.Preview == nil {
			return fmt.Errorf("animated_thumbnails: preview was not asked")
		}
		if r.Preview.File != p.Preview.Output {
			return fmt.Errorf("animated_thumbnails: preview file must be %q", p.Preview.Output)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------
// Fingerprint
// ---------------------------------------------------------------------------------------

// Fingerprint modes (duplication config).
const (
	FingerprintModeAudioOnly = "audio_only"
	FingerprintModeDual      = "dual"
)

// FingerprintParams are the fingerprint parameters. Whether audio is extracted depends on
// InputSpec.Video.AudioCodec: audio when the scene has an audio track; visual when it has
// none, or always in dual mode.
type FingerprintParams struct {
	Mode string `json:"mode"` // FingerprintModeAudioOnly or FingerprintModeDual
}

// Validate checks the parameters.
func (p *FingerprintParams) Validate() error {
	if p.Mode != FingerprintModeAudioOnly && p.Mode != FingerprintModeDual {
		return fmt.Errorf("fingerprint: unknown mode %q", p.Mode)
	}
	return nil
}

// FingerprintResult holds the extracted fingerprints. In dual mode both may be set. On the
// wire, both arrays are base64 of their little-endian bytes (the PostgreSQL storage format).
type FingerprintResult struct {
	AudioFingerprint  AudioFingerprint  `json:"audio_fingerprint,omitempty"`  // chromaprint raw hashes
	VisualFingerprint VisualFingerprint `json:"visual_fingerprint,omitempty"` // one dHash per 2 s
	AudioDurationSec  float64           `json:"audio_duration_sec,omitempty"`
	// AudioError / VisualError report an extraction that failed while the other one
	// succeeded (dual mode partial success).
	AudioError  string `json:"audio_error,omitempty"`
	VisualError string `json:"visual_error,omitempty"`
}

// FingerprintTypeLabel returns "dual", "audio", "visual" or "unknown" depending on which
// fingerprints are populated (the value stored with the scene).
func (r *FingerprintResult) FingerprintTypeLabel() string {
	hasAudio := len(r.AudioFingerprint) > 0
	hasVisual := len(r.VisualFingerprint) > 0
	switch {
	case hasAudio && hasVisual:
		return "dual"
	case hasAudio:
		return "audio"
	case hasVisual:
		return "visual"
	default:
		return "unknown"
	}
}

// Validate checks that the result carries at least one fingerprint.
func (r *FingerprintResult) Validate() error {
	if len(r.AudioFingerprint) == 0 && len(r.VisualFingerprint) == 0 {
		return fmt.Errorf("fingerprint: no fingerprint in the result")
	}
	return nil
}
