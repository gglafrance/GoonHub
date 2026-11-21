package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"goonhub/internal/streaming"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type transcodeFixture struct {
	handler     *SceneHandler
	sceneRepo   *mocks.MockSceneRepository
	appSettings *mocks.MockAppSettingsRepository
	transcoder  *streaming.Transcoder
	dataPath    string
	router      *gin.Engine

	mu       sync.Mutex
	lastArgs []string
}

func newTranscodeFixture(t *testing.T, maxConcurrent int) *transcodeFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	appSettings := mocks.NewMockAppSettingsRepository(ctrl)
	streamManager := streaming.NewManager(streaming.DefaultConfig(), sceneRepo, zap.NewNop())
	t.Cleanup(streamManager.Stop)
	transcoder := streaming.NewTranscoder(config.TranscodeConfig{MaxConcurrent: maxConcurrent}, appSettings, zap.NewNop())
	t.Cleanup(transcoder.Stop)

	f := &transcodeFixture{
		sceneRepo:   sceneRepo,
		appSettings: appSettings,
		transcoder:  transcoder,
		dataPath:    t.TempDir(),
	}
	f.handler = &SceneHandler{
		Service:       &core.SceneService{Repo: sceneRepo},
		StreamManager: streamManager,
		Transcoder:    transcoder,
	}
	// Fake ffmpeg: records the arguments and writes a few bytes
	transcoder.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		f.mu.Lock()
		f.lastArgs = args
		f.mu.Unlock()
		_, err := w.Write([]byte("fMP4"))
		return err
	})
	f.router = gin.New()
	f.router.GET("/api/v1/scenes/:id", f.handler.GetScene)
	f.router.GET("/api/v1/scenes/:id/transcode", f.handler.TranscodeScene)
	f.router.HEAD("/api/v1/scenes/:id/transcode", f.handler.TranscodeScene)
	return f
}

func (f *transcodeFixture) enabled(on bool) {
	f.appSettings.EXPECT().Get().Return(&data.AppSettingsRecord{ID: 1, TranscodingEnabled: on}, nil).AnyTimes()
}

func (f *transcodeFixture) scene(t *testing.T, name, video, audio string, duration int) *data.Scene {
	t.Helper()
	path := createTestFile(t, f.dataPath, name, []byte("original"))
	scene := &data.Scene{ID: 7, StoredPath: path, VideoCodec: video, AudioCodec: audio, Duration: duration}
	f.sceneRepo.EXPECT().GetByID(uint(7)).Return(scene, nil).AnyTimes()
	return scene
}

func (f *transcodeFixture) do(method, url string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body %q: %v", w.Body.String(), err)
	}
	return body.Code
}

func TestTranscodeScene_Disabled(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(false)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)

	w := f.do("GET", "/api/v1/scenes/7/transcode")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if code := errorCode(t, w); code != codeTranscodeDisabled {
		t.Fatalf("code = %q", code)
	}
	if head := f.do("HEAD", "/api/v1/scenes/7/transcode"); head.Code != http.StatusForbidden {
		t.Fatalf("HEAD: expected 403, got %d", head.Code)
	}
}

func TestTranscodeScene_LimitReached(t *testing.T) {
	f := newTranscodeFixture(t, 1)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)

	// Another viewer holds the only slot
	lease, err := f.transcoder.Acquire(context.Background(), "someone-else")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lease.Release()

	w := f.do("GET", "/api/v1/scenes/7/transcode?sid=abc")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if code := errorCode(t, w); code != codeTranscodeLimit {
		t.Fatalf("code = %q", code)
	}
	if !strings.Contains(w.Body.String(), "1/1") {
		t.Fatalf("message should tell the slot usage: %s", w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected a Retry-After header")
	}
	if head := f.do("HEAD", "/api/v1/scenes/7/transcode?sid=abc"); head.Code != http.StatusServiceUnavailable {
		t.Fatalf("HEAD: expected 503, got %d", head.Code)
	}
	// The stream limiter slot was given back
	if got := f.handler.StreamManager.Limiter().GlobalCount(); got != 0 {
		t.Fatalf("stream slots leaked: %d", got)
	}
}

func TestTranscodeScene_Streams(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(true)
	scene := f.scene(t, "a.mpg", "mpeg2video", "mp2", 600)

	w := f.do("GET", "/api/v1/scenes/7/transcode?start=90.5&sid=abc", "Range", "bytes=0-")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fMP4" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if w.Header().Get("X-Transcode-Mode") != "transcode" || w.Header().Get("X-Transcode-Start") != "90.500" {
		t.Fatalf("headers %v", w.Header())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	f.mu.Lock()
	args := f.lastArgs
	f.mu.Unlock()
	if i := slices.Index(args, "-ss"); i < 0 || args[i+1] != "90.500" {
		t.Fatalf("expected -ss 90.500 in %v", args)
	}
	if i := slices.Index(args, "-i"); i < 0 || args[i+1] != scene.StoredPath {
		t.Fatalf("expected the scene file as input in %v", args)
	}
	if got := f.transcoder.Stats().Active; got != 0 {
		t.Fatalf("transcode slot not released: %d", got)
	}
	if got := f.handler.StreamManager.Limiter().GlobalCount(); got != 0 {
		t.Fatalf("stream slot not released: %d", got)
	}
}

func TestTranscodeScene_RemuxAndForcedTranscode(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(true)
	f.scene(t, "a.mkv", "h264", "aac", 600)

	w := f.do("GET", "/api/v1/scenes/7/transcode")
	if w.Code != http.StatusOK || w.Header().Get("X-Transcode-Mode") != "remux" {
		t.Fatalf("expected a remux, got %d %v", w.Code, w.Header())
	}
	f.mu.Lock()
	args := f.lastArgs
	f.mu.Unlock()
	if i := slices.Index(args, "-c:v"); i < 0 || args[i+1] != "copy" {
		t.Fatalf("expected video copy in %v", args)
	}

	w = f.do("GET", "/api/v1/scenes/7/transcode?mode=transcode")
	if w.Code != http.StatusOK || w.Header().Get("X-Transcode-Mode") != "transcode" {
		t.Fatalf("expected a forced transcode, got %d %v", w.Code, w.Header())
	}
}

func TestTranscodeScene_DirectSceneIsReencoded(t *testing.T) {
	// The player only asks for a conversion of a "direct" scene when the browser failed on it
	f := newTranscodeFixture(t, 2)
	f.enabled(true)
	f.scene(t, "a.mp4", "h264", "aac", 600)

	w := f.do("GET", "/api/v1/scenes/7/transcode")
	if w.Code != http.StatusOK || w.Header().Get("X-Transcode-Mode") != "transcode" {
		t.Fatalf("expected a transcode, got %d %v", w.Code, w.Header())
	}
}

func TestTranscodeScene_StartClampedToDuration(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 100)

	w := f.do("GET", "/api/v1/scenes/7/transcode?start=5000")
	if w.Code != http.StatusOK || w.Header().Get("X-Transcode-Start") != "99.000" {
		t.Fatalf("expected start clamped to 99, got %d %v", w.Code, w.Header())
	}
}

func TestTranscodeScene_BadRequests(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)

	for _, start := range []string{"abc", "-1", "NaN", "Inf"} {
		if w := f.do("GET", "/api/v1/scenes/7/transcode?start="+start); w.Code != http.StatusBadRequest {
			t.Fatalf("start=%s: expected 400, got %d", start, w.Code)
		}
	}
	if w := f.do("GET", "/api/v1/scenes/x/transcode"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: expected 400, got %d", w.Code)
	}
	// The stream can't be resumed in the middle
	if w := f.do("GET", "/api/v1/scenes/7/transcode", "Range", "bytes=1000-"); w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("range: expected 416, got %d", w.Code)
	}
}

func TestTranscodeScene_NotFound(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(true)
	f.sceneRepo.EXPECT().GetByID(uint(8)).Return(nil, gorm.ErrRecordNotFound)
	if w := f.do("GET", "/api/v1/scenes/8/transcode"); w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	f.sceneRepo.EXPECT().GetByID(uint(9)).Return(&data.Scene{ID: 9, StoredPath: f.dataPath + "/missing.avi"}, nil)
	if w := f.do("GET", "/api/v1/scenes/9/transcode"); w.Code != http.StatusNotFound {
		t.Fatalf("missing file: expected 404, got %d", w.Code)
	}
}

func TestTranscodeScene_HeadAvailable(t *testing.T) {
	f := newTranscodeFixture(t, 1)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)

	w := f.do("HEAD", "/api/v1/scenes/7/transcode?sid=abc")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatal("HEAD must not stream")
	}
	f.mu.Lock()
	ran := f.lastArgs != nil
	f.mu.Unlock()
	if ran {
		t.Fatal("HEAD must not run ffmpeg")
	}
}

func TestTranscodeScene_HeadRemuxStartsOnKeyframe(t *testing.T) {
	f := newTranscodeFixture(t, 1)
	f.enabled(true)
	f.scene(t, "a.mkv", "h264", "aac", 600)
	f.transcoder.SetProbeFunc(func(ctx context.Context, args []string) ([]byte, error) {
		return []byte(`{"packets":[{"pts_time":"40.000","flags":"K__"},{"pts_time":"50.000","flags":"K__"}],"format":{"start_time":"0"}}`), nil
	})

	w := f.do("HEAD", "/api/v1/scenes/7/transcode?start=45&sid=abc")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-Transcode-Mode") != "remux" || w.Header().Get("X-Transcode-Start") != "40.001" {
		t.Fatalf("headers %v", w.Header())
	}
}

func TestTranscodeScene_FailureBeforeOutput(t *testing.T) {
	f := newTranscodeFixture(t, 1)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)
	f.transcoder.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		return io.ErrUnexpectedEOF
	})

	w := f.do("GET", "/api/v1/scenes/7/transcode")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if code := errorCode(t, w); code != codeTranscodeFailed {
		t.Fatalf("code = %q", code)
	}
}

func TestTranscodeScene_SeekReplacesOwnStream(t *testing.T) {
	f := newTranscodeFixture(t, 1)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)

	started := make(chan struct{}, 2)
	f.transcoder.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		started <- struct{}{}
		if slices.Contains(args, "-ss") {
			return nil // the second stream ends at once
		}
		<-ctx.Done() // the first one runs until it is replaced
		return ctx.Err()
	})

	srv := httptest.NewServer(f.router)
	defer srv.Close()
	firstDone := make(chan int, 1)
	go func() {
		resp, err := http.Get(srv.URL + "/api/v1/scenes/7/transcode?sid=abc")
		if err != nil {
			firstDone <- -1
			return
		}
		resp.Body.Close()
		firstDone <- resp.StatusCode
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("first stream did not start")
	}

	// Same player seeks: it gets the only slot instead of a 503
	resp, err := http.Get(srv.URL + "/api/v1/scenes/7/transcode?sid=abc&start=30")
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("seek: expected 200, got %d", resp.StatusCode)
	}
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatal("first stream was not stopped")
	}

	// Another player of the same client does not take over the slot
	lease, err := f.transcoder.Acquire(context.Background(), transcodeSessionKey("127.0.0.1", "abc", 7))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lease.Release()
	resp, err = http.Get(srv.URL + "/api/v1/scenes/7/transcode?sid=other")
	if err != nil {
		t.Fatalf("other player: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("other player: expected 503, got %d", resp.StatusCode)
	}
}

func TestGetScene_IncludesPlayback(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.enabled(false)
	f.sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{
		ID: 7, Title: "t", StoredPath: "/m/a.wmv", VideoCodec: "wmv3", AudioCodec: "wmav2",
	}, nil)

	w := f.do("GET", "/api/v1/scenes/7")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		ID       uint   `json:"id"`
		Title    string `json:"title"`
		Playback struct {
			Mode             string `json:"mode"`
			Reason           string `json:"reason"`
			TranscodeEnabled bool   `json:"transcode_enabled"`
		} `json:"playback"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != 7 || body.Title != "t" {
		t.Fatalf("scene fields missing: %s", w.Body.String())
	}
	if body.Playback.Mode != "transcode" || body.Playback.TranscodeEnabled || !strings.Contains(body.Playback.Reason, "WMV") {
		t.Fatalf("unexpected playback %+v", body.Playback)
	}
}

func TestStreamScene_Download(t *testing.T) {
	handler, sceneRepo, dataPath := newTestSceneHandler(t)
	filePath := createTestFile(t, dataPath, "My Clip.avi", []byte("original"))
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, StoredPath: filePath}, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/stream/1?download=1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="My Clip.avi"` {
		t.Fatalf("Content-Disposition = %q", cd)
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/stream/1", nil))
	if cd := w.Header().Get("Content-Disposition"); cd != "" {
		t.Fatalf("no attachment expected without download=1, got %q", cd)
	}
}
