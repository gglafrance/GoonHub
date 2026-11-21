package streaming

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func defaultTestSettings() TranscodeSettings {
	return NormalizeTranscodeSettings(config.TranscodeConfig{
		MaxConcurrent: 2, Preset: "veryfast", CRF: 23, MaxHeight: 1080, AudioBitrate: "160k", KeyframeInterval: 2,
	})
}

// argValue returns the value following flag in args ("" if absent).
func argValue(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

func TestNormalizeTranscodeSettings(t *testing.T) {
	got := NormalizeTranscodeSettings(config.TranscodeConfig{
		MaxConcurrent: 0, Preset: "ludicrous", CRF: 99, MaxHeight: -5, AudioBitrate: "loud", Threads: -1, KeyframeInterval: 0,
	})
	want := TranscodeSettings{MaxConcurrent: 4, Preset: "veryfast", CRF: 23, MaxHeight: 0, AudioBitrate: "160k", Threads: 0, KeyframeInterval: 2}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	kept := NormalizeTranscodeSettings(config.TranscodeConfig{
		MaxConcurrent: 8, Preset: " Faster ", CRF: 20, MaxHeight: 720, AudioBitrate: "128K", Threads: 6, KeyframeInterval: 4,
	})
	wantKept := TranscodeSettings{MaxConcurrent: 8, Preset: "faster", CRF: 20, MaxHeight: 720, AudioBitrate: "128k", Threads: 6, KeyframeInterval: 4}
	if kept != wantKept {
		t.Fatalf("got %+v, want %+v", kept, wantKept)
	}
}

func TestBuildTranscodeArgs_Encode(t *testing.T) {
	plan := DecidePlayback("/media/a.mpg", "mpeg2video", "mp2")
	args := BuildTranscodeArgs("/media/a.mpg", plan, 125.5, defaultTestSettings())

	if argValue(args, "-ss") != "125.500" {
		t.Fatalf("-ss = %q", argValue(args, "-ss"))
	}
	if slices.Index(args, "-ss") > slices.Index(args, "-i") {
		t.Fatal("-ss must come before -i (fast input seeking)")
	}
	if argValue(args, "-i") != "/media/a.mpg" {
		t.Fatalf("-i = %q", argValue(args, "-i"))
	}
	checks := map[string]string{
		"-c:v": "libx264", "-preset": "veryfast", "-crf": "23", "-pix_fmt": "yuv420p",
		"-c:a": "aac", "-b:a": "160k", "-ac": "2",
		"-movflags": "frag_keyframe+empty_moov+default_base_moof", "-f": "mp4",
		"-force_key_frames": "expr:gte(t,n_forced*2)",
	}
	for flag, want := range checks {
		if got := argValue(args, flag); got != want {
			t.Fatalf("%s = %q, want %q", flag, got, want)
		}
	}
	vf := argValue(args, "-vf")
	if !strings.HasPrefix(vf, "yadif=deint=interlaced,") || !strings.Contains(vf, "min(ih,1080)") || !strings.Contains(vf, "min(iw,1080)") {
		t.Fatalf("unexpected -vf %q", vf)
	}
	if args[len(args)-1] != "pipe:1" {
		t.Fatalf("output must be stdout, got %q", args[len(args)-1])
	}
	if slices.Contains(args, "-threads") {
		t.Fatal("-threads must be omitted when 0 (auto)")
	}
	if slices.Contains(args, "+genpts") {
		t.Fatal("+genpts is only needed when copying video")
	}
	if !slices.Contains(args, "0:V:0") || !slices.Contains(args, "0:a:0?") {
		t.Fatalf("missing stream maps: %v", args)
	}
}

func TestBuildTranscodeArgs_Remux(t *testing.T) {
	plan := DecidePlayback("/media/a.mkv", "h264", "aac")
	args := BuildTranscodeArgs("/media/a.mkv", plan, 0, defaultTestSettings())

	if slices.Contains(args, "-ss") {
		t.Fatal("no -ss expected for start 0")
	}
	if argValue(args, "-c:v") != "copy" || argValue(args, "-c:a") != "copy" {
		t.Fatalf("expected stream copy: %v", args)
	}
	for _, flag := range []string{"-vf", "-preset", "-crf", "-b:a"} {
		if slices.Contains(args, flag) {
			t.Fatalf("%s is useless when copying: %v", flag, args)
		}
	}
	if argValue(args, "-fflags") != "+genpts" {
		t.Fatalf("expected +genpts when copying video: %v", args)
	}
	if argValue(args, "-movflags") != "frag_keyframe+empty_moov+default_base_moof" {
		t.Fatalf("missing fragmented mp4 flags: %v", args)
	}
}

func TestBuildTranscodeArgs_AACCopyBitstreamFilter(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		video      string
		audio      string
		wantFilter bool
	}{
		// AAC copied from MPEG-TS / HLS downloads is ADTS: the MP4 muxer needs aac_adtstoasc
		{"aac copied", "/media/a.mkv", "h264", "aac", true},
		{"mp3 copied", "/media/a.mkv", "h264", "mp3", false},
		{"opus copied", "/media/a.mkv", "vp9", "opus", false},
		{"audio re-encoded", "/media/a.mp4", "h264", "ac3", false},
		{"full transcode", "/media/a.avi", "mpeg4", "mp3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DecidePlayback(tt.path, tt.video, tt.audio)
			args := BuildTranscodeArgs(tt.path, plan, 0, defaultTestSettings())
			got := argValue(args, "-bsf:a") == "aac_adtstoasc"
			if got != tt.wantFilter {
				t.Fatalf("aac_adtstoasc filter: got %v, want %v (%v)", got, tt.wantFilter, args)
			}
		})
	}
}

func TestBuildTranscodeArgs_RemuxAudioEncode(t *testing.T) {
	plan := DecidePlayback("/media/a.mp4", "h264", "ac3")
	args := BuildTranscodeArgs("/media/a.mp4", plan, 10, defaultTestSettings())
	if argValue(args, "-c:v") != "copy" || argValue(args, "-c:a") != "aac" {
		t.Fatalf("expected video copy and aac audio: %v", args)
	}
}

func TestBuildTranscodeArgs_NoDownscaleAndThreads(t *testing.T) {
	s := defaultTestSettings()
	s.MaxHeight = 0
	s.Threads = 6
	args := BuildTranscodeArgs("/media/a.avi", FullTranscodePlan(), 0, s)
	if vf := argValue(args, "-vf"); vf != "yadif=deint=interlaced,scale=w=trunc(iw/2)*2:h=trunc(ih/2)*2" {
		t.Fatalf("unexpected -vf %q", vf)
	}
	if argValue(args, "-threads") != "6" {
		t.Fatalf("-threads = %q", argValue(args, "-threads"))
	}
}

func newTestTranscoder(t *testing.T, maxConcurrent int) *Transcoder {
	t.Helper()
	tc := NewTranscoder(config.TranscodeConfig{MaxConcurrent: maxConcurrent}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)
	return tc
}

func TestTranscoderAcquire_Limit(t *testing.T) {
	tc := newTestTranscoder(t, 2)
	ctx := context.Background()

	a, err := tc.Acquire(ctx, "a")
	if err != nil {
		t.Fatalf("acquire a: %v", err)
	}
	b, err := tc.Acquire(ctx, "b")
	if err != nil {
		t.Fatalf("acquire b: %v", err)
	}
	if tc.Available("c") {
		t.Fatal("no slot should be available")
	}
	if !tc.Available("a") {
		t.Fatal("the viewer holding a slot can replace its stream")
	}
	if _, err := tc.Acquire(ctx, "c"); !errors.Is(err, ErrTranscodeLimit) {
		t.Fatalf("expected ErrTranscodeLimit, got %v", err)
	}
	if got := tc.Stats(); got.Active != 2 || got.MaxConcurrent != 2 {
		t.Fatalf("stats %+v", got)
	}

	a.Release()
	a.Release() // idempotent
	if got := tc.Stats().Active; got != 1 {
		t.Fatalf("active = %d after release, want 1", got)
	}
	c, err := tc.Acquire(ctx, "c")
	if err != nil {
		t.Fatalf("acquire c after release: %v", err)
	}
	b.Release()
	c.Release()
	if got := tc.Stats().Active; got != 0 {
		t.Fatalf("active = %d, want 0", got)
	}
}

func TestTranscoderAcquire_ReplacesSameViewer(t *testing.T) {
	tc := newTestTranscoder(t, 1)

	first, err := tc.Acquire(context.Background(), "viewer")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// The handler of the first stream releases its slot when its context ends
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-first.Context().Done()
		first.Release()
	}()

	second, err := tc.Acquire(context.Background(), "viewer")
	if err != nil {
		t.Fatalf("second acquire of the same viewer: %v", err)
	}
	wg.Wait()
	if first.Context().Err() == nil {
		t.Fatal("first stream should be cancelled")
	}
	if second.Context().Err() != nil {
		t.Fatal("second stream should be running")
	}
	if got := tc.Stats().Active; got != 1 {
		t.Fatalf("active = %d, want 1", got)
	}
	second.Release()
}

func TestTranscoderStop(t *testing.T) {
	tc := newTestTranscoder(t, 2)
	lease, err := tc.Acquire(context.Background(), "a")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	tc.Stop()
	select {
	case <-lease.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("running stream not cancelled by Stop")
	}
	lease.Release()
	if _, err := tc.Acquire(context.Background(), "b"); !errors.Is(err, ErrTranscoderStopped) {
		t.Fatalf("expected ErrTranscoderStopped, got %v", err)
	}
	if tc.Available("b") {
		t.Fatal("no slot after Stop")
	}
}

func TestTranscoderAcquire_ParentCancelled(t *testing.T) {
	tc := newTestTranscoder(t, 1)
	lease, err := tc.Acquire(context.Background(), "a")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The previous stream never ends (no release): the cancelled request gives up at once
	if _, err := tc.Acquire(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	lease.Release()
}

func TestTranscoderEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAppSettingsRepository(ctrl)
	tc := NewTranscoder(config.TranscodeConfig{}, repo, zap.NewNop())
	t.Cleanup(tc.Stop)

	repo.EXPECT().Get().Return(&data.AppSettingsRecord{TranscodingEnabled: false}, nil)
	if tc.Enabled() {
		t.Fatal("expected disabled")
	}
	repo.EXPECT().Get().Return(&data.AppSettingsRecord{TranscodingEnabled: true}, nil)
	if !tc.Enabled() {
		t.Fatal("expected enabled")
	}
	repo.EXPECT().Get().Return(nil, errors.New("db down"))
	if !tc.Enabled() {
		t.Fatal("expected enabled when the setting can't be read")
	}

	repo.EXPECT().Get().Return(&data.AppSettingsRecord{TranscodingEnabled: false}, nil)
	info := tc.PlaybackInfo(&data.Scene{StoredPath: "/m/a.avi", VideoCodec: "mpeg4", AudioCodec: "mp3"})
	if info.Mode != PlaybackTranscode || info.TranscodeEnabled || info.Reason == "" {
		t.Fatalf("unexpected info %+v", info)
	}
}

func TestTailBuffer(t *testing.T) {
	b := &tailBuffer{max: 5}
	b.Write([]byte("abc"))
	b.Write([]byte("defgh"))
	if got := b.String(); got != "defgh" {
		t.Fatalf("got %q", got)
	}
}

func TestLastKeyframeBefore(t *testing.T) {
	probeJSON := []byte(`{"packets":[
		{"pts_time":"0.000000","flags":"K__"},
		{"pts_time":"0.040000","flags":"___"},
		{"pts_time":"10.000000","flags":"K__"},
		{"pts_time":"20.000000","flags":"K__"},
		{"pts_time":"20.040000","flags":"___"},
		{"pts_time":"30.000000","flags":"K__"}],
		"format":{"start_time":"-0.023000"}}`)
	cases := []struct {
		start float64
		want  float64
	}{
		{25, 20.024}, // keyframe at 20 is 20.023 from the start of the file
		{20.023, 20.024},
		{9.9, 0.024},
		{100, 30.024},
	}
	for _, c := range cases {
		got, ok := lastKeyframeBefore(probeJSON, c.start)
		if !ok || got < c.want-1e-9 || got > c.want+1e-9 {
			t.Fatalf("start %v: got %v (%v), want %v", c.start, got, ok, c.want)
		}
	}
	if _, ok := lastKeyframeBefore([]byte(`{"packets":[{"pts_time":"50.0","flags":"K__"}]}`), 10); ok {
		t.Fatal("no keyframe before the start expected")
	}
	if _, ok := lastKeyframeBefore([]byte(`not json`), 10); ok {
		t.Fatal("invalid output must not give a keyframe")
	}
}

func TestKeyframeStart_Fallbacks(t *testing.T) {
	tc := newTestTranscoder(t, 1)
	tc.SetProbeFunc(func(ctx context.Context, args []string) ([]byte, error) {
		return nil, errors.New("ffprobe missing")
	})
	if got := tc.KeyframeStart(context.Background(), "/m/a.mkv", 42); got != 42 {
		t.Fatalf("probe failure: got %v, want the requested start", got)
	}
	called := false
	tc.SetProbeFunc(func(ctx context.Context, args []string) ([]byte, error) {
		called = true
		if argValue(args, "-read_intervals") != "0.000%42.500" {
			t.Fatalf("read_intervals %q", argValue(args, "-read_intervals"))
		}
		return []byte(`{"packets":[{"pts_time":"40.0","flags":"K_"}],"format":{"start_time":"0"}}`), nil
	})
	if got := tc.KeyframeStart(context.Background(), "/m/a.mkv", 42); got != 40.001 {
		t.Fatalf("got %v, want 40.001", got)
	}
	if !called {
		t.Fatal("ffprobe not called")
	}
	if got := tc.KeyframeStart(context.Background(), "/m/a.mkv", 0); got != 0 {
		t.Fatalf("start 0: got %v", got)
	}
}
