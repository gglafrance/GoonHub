package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/pkg/ffmpeg"

	"go.uber.org/zap"
)

var (
	// ErrTranscodeLimit is returned by Acquire when all conversion slots are in use.
	ErrTranscodeLimit = errors.New("all on-the-fly conversion slots are in use")
	// ErrTranscoderStopped is returned by Acquire after Stop.
	ErrTranscoderStopped = errors.New("transcoder is stopped")
)

// How long Acquire waits for the previous stream of the same viewer to end (it is
// cancelled when the viewer seeks and asks for a new start position).
const replaceWait = 5 * time.Second

var x264Presets = map[string]bool{
	"ultrafast": true, "superfast": true, "veryfast": true, "faster": true, "fast": true,
	"medium": true, "slow": true, "slower": true, "veryslow": true,
}

var audioBitratePattern = regexp.MustCompile(`^[1-9][0-9]{1,3}k$`)

// TranscodeSettings are the validated conversion settings.
type TranscodeSettings struct {
	MaxConcurrent    int
	Preset           string
	CRF              int
	MaxHeight        int
	AudioBitrate     string
	Threads          int
	KeyframeInterval int
}

// NormalizeTranscodeSettings applies the defaults to missing or invalid values.
func NormalizeTranscodeSettings(cfg config.TranscodeConfig) TranscodeSettings {
	s := TranscodeSettings{
		MaxConcurrent:    cfg.MaxConcurrent,
		Preset:           strings.ToLower(strings.TrimSpace(cfg.Preset)),
		CRF:              cfg.CRF,
		MaxHeight:        cfg.MaxHeight,
		AudioBitrate:     strings.ToLower(strings.TrimSpace(cfg.AudioBitrate)),
		Threads:          cfg.Threads,
		KeyframeInterval: cfg.KeyframeInterval,
	}
	if s.MaxConcurrent <= 0 {
		s.MaxConcurrent = 4
	}
	if !x264Presets[s.Preset] {
		s.Preset = "veryfast"
	}
	if s.CRF <= 0 || s.CRF > 51 {
		s.CRF = 23
	}
	if s.MaxHeight < 0 {
		s.MaxHeight = 0
	}
	if !audioBitratePattern.MatchString(s.AudioBitrate) {
		s.AudioBitrate = "160k"
	}
	if s.Threads < 0 {
		s.Threads = 0
	}
	if s.KeyframeInterval <= 0 {
		s.KeyframeInterval = 2
	}
	return s
}

// ProbeFunc runs ffprobe with args and returns its standard output.
type ProbeFunc func(ctx context.Context, args []string) ([]byte, error)

// RunFunc runs ffmpeg with args, writing its standard output to stdout.
type RunFunc func(ctx context.Context, args []string, stdout io.Writer, buf []byte) error

// Transcoder converts scenes browsers can't play into a fragmented MP4 stream, with a
// bounded number of concurrent ffmpeg processes. Each conversion runs in the goroutine of
// the HTTP request that asked for it and ends with it.
type Transcoder struct {
	settings    TranscodeSettings
	appSettings data.AppSettingsRepository
	logger      *zap.Logger
	run         RunFunc
	probe       ProbeFunc

	mu       sync.Mutex
	active   int
	stopped  bool
	sessions map[string]*transcodeSession

	baseCtx context.Context
	stop    context.CancelFunc
}

type transcodeSession struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// TranscodeLease is a reserved conversion slot. Release it when the stream ends.
type TranscodeLease struct {
	ctx     context.Context
	release func()
}

// Context is cancelled when the client goes away, when the same viewer starts another
// stream of the scene, or when the transcoder stops.
func (l *TranscodeLease) Context() context.Context { return l.ctx }

// Release frees the slot. Safe to call more than once.
func (l *TranscodeLease) Release() { l.release() }

// NewTranscoder creates a transcoder. appSettings may be nil (transcoding always enabled).
func NewTranscoder(cfg config.TranscodeConfig, appSettings data.AppSettingsRepository, logger *zap.Logger) *Transcoder {
	ctx, cancel := context.WithCancel(context.Background())
	return &Transcoder{
		settings:    NormalizeTranscodeSettings(cfg),
		appSettings: appSettings,
		logger:      logger,
		run:         runFFmpeg,
		probe:       runFFprobe,
		sessions:    make(map[string]*transcodeSession),
		baseCtx:     ctx,
		stop:        cancel,
	}
}

// SetProbeFunc replaces the ffprobe runner (tests).
func (t *Transcoder) SetProbeFunc(probe ProbeFunc) { t.probe = probe }

// SetRunFunc replaces the ffmpeg runner (tests).
func (t *Transcoder) SetRunFunc(run RunFunc) { t.run = run }

// Logger returns the transcoder logger.
func (t *Transcoder) Logger() *zap.Logger { return t.logger }

// Settings returns the validated conversion settings.
func (t *Transcoder) Settings() TranscodeSettings { return t.settings }

// Enabled tells whether the admin allows on-the-fly transcoding (app setting, on by
// default and when the setting can't be read).
func (t *Transcoder) Enabled() bool {
	if t.appSettings == nil {
		return true
	}
	settings, err := t.appSettings.Get()
	if err != nil {
		t.logger.Warn("Failed to read the transcoding app setting, assuming enabled", zap.Error(err))
		return true
	}
	return settings.TranscodingEnabled
}

// PlaybackInfo returns what the player needs to know about a scene.
func (t *Transcoder) PlaybackInfo(scene *data.Scene) PlaybackInfo {
	plan := DecidePlayback(scene.StoredPath, scene.VideoCodec, scene.AudioCodec)
	return PlaybackInfo{Mode: plan.Mode, Reason: plan.Reason, TranscodeEnabled: t.Enabled()}
}

// Acquire reserves a conversion slot for key (one viewer of one scene). A running stream
// with the same key is cancelled first, so a viewer seeking replaces its own conversion
// instead of using a second slot. Returns ErrTranscodeLimit when all slots are in use.
func (t *Transcoder) Acquire(parent context.Context, key string) (*TranscodeLease, error) {
	t.mu.Lock()
	if old := t.sessions[key]; old != nil {
		old.cancel()
		t.mu.Unlock()
		select {
		case <-old.done:
		case <-time.After(replaceWait):
		case <-parent.Done():
			return nil, parent.Err()
		}
		t.mu.Lock()
	}
	defer t.mu.Unlock()

	if t.stopped {
		return nil, ErrTranscoderStopped
	}
	if t.active >= t.settings.MaxConcurrent {
		return nil, ErrTranscodeLimit
	}

	ctx, cancel := context.WithCancel(parent)
	stopOnShutdown := context.AfterFunc(t.baseCtx, cancel)
	session := &transcodeSession{cancel: cancel, done: make(chan struct{})}
	t.active++
	t.sessions[key] = session

	release := sync.OnceFunc(func() {
		stopOnShutdown()
		cancel()
		t.mu.Lock()
		t.active--
		if t.sessions[key] == session {
			delete(t.sessions, key)
		}
		t.mu.Unlock()
		close(session.done)
	})
	return &TranscodeLease{ctx: ctx, release: release}, nil
}

// Available tells whether Acquire(key) would currently get a slot.
func (t *Transcoder) Available(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return false
	}
	return t.active < t.settings.MaxConcurrent || t.sessions[key] != nil
}

// TranscodeStats is the current slot usage.
type TranscodeStats struct {
	Active        int `json:"active"`
	MaxConcurrent int `json:"max_concurrent"`
}

// Stats returns the current slot usage.
func (t *Transcoder) Stats() TranscodeStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return TranscodeStats{Active: t.active, MaxConcurrent: t.settings.MaxConcurrent}
}

// Stop cancels the running conversions and refuses new ones.
func (t *Transcoder) Stop() {
	t.mu.Lock()
	t.stopped = true
	t.mu.Unlock()
	t.stop()
}

// How far before the requested start KeyframeStart looks for a keyframe.
const keyframeSearchWindow = 60.0

// KeyframeStart returns where a stream copy of input starting at start really begins: the
// last video keyframe at or before start (a copy can only cut there). The player asks for
// it (HEAD) before loading a remux so that the time it shows matches the picture. Returns
// start when no keyframe is found or ffprobe fails.
func (t *Transcoder) KeyframeStart(ctx context.Context, input string, start float64) float64 {
	if start <= 0 {
		return 0
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	from := math.Max(0, start-keyframeSearchWindow)
	out, err := t.probe(ctx, []string{
		"-v", "error", "-select_streams", "V:0",
		"-show_entries", "packet=pts_time,flags:format=start_time",
		"-read_intervals", fmt.Sprintf("%.3f%%%.3f", from, start+0.5),
		"-of", "json", input,
	})
	if err != nil {
		t.logger.Warn("Failed to find the keyframe before the start", zap.String("input", input), zap.Error(err))
		return start
	}
	if keyframe, ok := lastKeyframeBefore(out, start); ok {
		return keyframe
	}
	return start
}

// lastKeyframeBefore parses ffprobe packets (JSON) and returns the position, relative to
// the start of the file as ffmpeg -ss expects it, of the last keyframe at or before start.
func lastKeyframeBefore(probeJSON []byte, start float64) (float64, bool) {
	var res struct {
		Packets []struct {
			PtsTime string `json:"pts_time"`
			Flags   string `json:"flags"`
		} `json:"packets"`
		Format struct {
			StartTime string `json:"start_time"`
		} `json:"format"`
	}
	if err := json.Unmarshal(probeJSON, &res); err != nil {
		return 0, false
	}
	fileStart, _ := strconv.ParseFloat(res.Format.StartTime, 64)
	best, found := 0.0, false
	for _, pkt := range res.Packets {
		if !strings.Contains(pkt.Flags, "K") {
			continue
		}
		pts, err := strconv.ParseFloat(pkt.PtsTime, 64)
		if err != nil {
			continue
		}
		pos := pts - fileStart
		// 1 ms of slack so that the keyframe itself is never skipped by a rounding error
		if pos <= start+0.001 && (!found || pos > best) {
			best, found = pos, true
		}
	}
	if !found {
		return 0, false
	}
	// -ss seeks to the keyframe at or before the position: aim just after this one
	return math.Max(0, best+0.001), true
}

// Args builds the ffmpeg arguments for a scene file.
func (t *Transcoder) Args(input string, plan PlaybackPlan, start float64) []string {
	return BuildTranscodeArgs(input, plan, start, t.settings)
}

// Stream runs the conversion and writes the fragmented MP4 to w until the end of the file,
// an error, or the cancellation of ctx (client gone), which kills ffmpeg.
func (t *Transcoder) Stream(ctx context.Context, w io.Writer, args []string, buf []byte) error {
	return t.run(ctx, args, w, buf)
}

// BuildTranscodeArgs builds the ffmpeg command line producing a fragmented MP4 (H.264 +
// AAC, or the copied streams) on stdout, starting at start seconds into the input.
func BuildTranscodeArgs(input string, plan PlaybackPlan, start float64, s TranscodeSettings) []string {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	if plan.CopyVideo {
		// Some containers (AVI, MPEG-TS) lack timestamps the MP4 muxer needs when copying
		args = append(args, "-fflags", "+genpts")
	}
	if start > 0 {
		args = append(args, "-ss", strconv.FormatFloat(start, 'f', 3, 64))
	}
	args = append(args,
		"-i", input,
		// First real video stream (not cover art) and the first audio stream if any
		"-map", "0:V:0", "-map", "0:a:0?",
		"-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1",
	)

	if plan.CopyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args,
			"-vf", videoFilter(s.MaxHeight),
			"-c:v", "libx264",
			"-preset", s.Preset,
			"-crf", strconv.Itoa(s.CRF),
			"-pix_fmt", "yuv420p",
			"-profile:v", "high",
			// Regular keyframes: one fragment per keyframe, so the stream starts quickly
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", s.KeyframeInterval),
		)
		if s.Threads > 0 {
			args = append(args, "-threads", strconv.Itoa(s.Threads))
		}
	}

	if plan.CopyAudio {
		args = append(args, "-c:a", "copy")
		if plan.AudioCodec == "aac" {
			// ADTS AAC (MPEG-TS, HLS downloads) must be converted for the MP4 muxer, which
			// otherwise stops after the first packets; AAC that is already in MP4 form passes
			// through the filter unchanged
			args = append(args, "-bsf:a", "aac_adtstoasc")
		}
	} else {
		args = append(args, "-c:a", "aac", "-b:a", s.AudioBitrate, "-ac", "2")
	}

	return append(args,
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4", "pipe:1",
	)
}

// videoFilter deinterlaces interlaced frames (DVD and TV captures) and keeps even
// dimensions (required by yuv420p), downscaling so that the smaller side of the frame
// (height in landscape, width in portrait) is at most maxHeight.
func videoFilter(maxHeight int) string {
	scale := "scale=w=trunc(iw/2)*2:h=trunc(ih/2)*2"
	if maxHeight > 0 {
		m := strconv.Itoa(maxHeight)
		scale = "scale=w='if(gt(iw,ih),-2,trunc(min(iw," + m + ")/2)*2)'" +
			":h='if(gt(iw,ih),trunc(min(ih," + m + ")/2)*2,-2)'"
	}
	return "yadif=deint=interlaced," + scale
}

// runFFprobe runs ffprobe and returns its standard output.
func runFFprobe(ctx context.Context, args []string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, ffmpeg.FFprobePath(), args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}
	return out, nil
}

// runFFmpeg runs ffmpeg, copying its stdout to w in the calling goroutine. ffmpeg is
// killed when ctx is cancelled or when writing to w fails (client gone).
func runFFmpeg(ctx context.Context, args []string, w io.Writer, buf []byte) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg.FFMpegPath(), args...)
	cmd.WaitDelay = 5 * time.Second
	stderr := &tailBuffer{max: 4096}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open ffmpeg output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	if len(buf) == 0 {
		buf = make([]byte, 64*1024)
	}
	_, copyErr := io.CopyBuffer(w, stdout, buf)
	if copyErr != nil {
		// Kill ffmpeg before waiting: nobody reads its output anymore
		cancel()
	}
	waitErr := cmd.Wait()

	if ctx.Err() != nil && copyErr == nil {
		return ctx.Err()
	}
	if copyErr != nil {
		return fmt.Errorf("failed to send the stream: %w", copyErr)
	}
	if waitErr != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// tailBuffer keeps the last max bytes written to it (ffmpeg error output).
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf bytes.Buffer
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Write(p)
	if over := b.buf.Len() - b.max; over > 0 {
		b.buf.Next(over)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
