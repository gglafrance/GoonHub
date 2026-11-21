package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
)

func newShareTranscodeRouter(t *testing.T, link *data.ShareLink, scene *data.Scene, enabled bool) *gin.Engine {
	t.Helper()
	ctrl := gomock.NewController(t)
	shareRepo := mocks.NewMockShareLinkRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	appSettings := mocks.NewMockAppSettingsRepository(ctrl)

	shareRepo.EXPECT().GetByToken(link.Token).Return(link, nil).AnyTimes()
	shareRepo.EXPECT().IncrementViewCount(gomock.Any()).Return(nil).AnyTimes()
	sceneRepo.EXPECT().GetByID(scene.ID).Return(scene, nil).AnyTimes()
	appSettings.EXPECT().Get().Return(&data.AppSettingsRecord{TranscodingEnabled: enabled}, nil).AnyTimes()

	streamManager := streaming.NewManager(streaming.DefaultConfig(), sceneRepo, zap.NewNop())
	t.Cleanup(streamManager.Stop)
	transcoder := streaming.NewTranscoder(config.TranscodeConfig{MaxConcurrent: 1}, appSettings, zap.NewNop())
	t.Cleanup(transcoder.Stop)
	transcoder.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		_, err := w.Write([]byte("fMP4"))
		return err
	})

	h := NewShareHandler(core.NewShareService(shareRepo, sceneRepo, zap.NewNop()), nil, streamManager, transcoder, "")
	r := gin.New()
	r.GET("/api/v1/shares/:token", h.ResolveShareLink)
	r.GET("/api/v1/shares/:token/transcode", h.TranscodeShareLink)
	return r
}

func shareTestScene(t *testing.T) *data.Scene {
	path := createTestFile(t, t.TempDir(), "a.avi", []byte("original"))
	return &data.Scene{ID: 3, StoredPath: path, VideoCodec: "mpeg4", AudioCodec: "mp3", Duration: 60}
}

func TestTranscodeShareLink(t *testing.T) {
	scene := shareTestScene(t)
	link := &data.ShareLink{ID: 1, Token: "tok", SceneID: scene.ID, ShareType: data.ShareTypePublic}
	r := newShareTranscodeRouter(t, link, scene, true)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/shares/tok/transcode?start=10", nil))
	if w.Code != http.StatusOK || w.Body.String() != "fMP4" {
		t.Fatalf("expected the stream, got %d %q", w.Code, w.Body.String())
	}
}

func TestTranscodeShareLink_Expired(t *testing.T) {
	scene := shareTestScene(t)
	past := time.Now().Add(-time.Hour)
	link := &data.ShareLink{ID: 1, Token: "tok", SceneID: scene.ID, ShareType: data.ShareTypePublic, ExpiresAt: &past}
	r := newShareTranscodeRouter(t, link, scene, true)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/shares/tok/transcode", nil))
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", w.Code)
	}
}

func TestTranscodeShareLink_Disabled(t *testing.T) {
	scene := shareTestScene(t)
	link := &data.ShareLink{ID: 1, Token: "tok", SceneID: scene.ID, ShareType: data.ShareTypePublic}
	r := newShareTranscodeRouter(t, link, scene, false)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/shares/tok/transcode", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	// The resolved share tells the player
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/shares/tok", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"playback":{"mode":"transcode"`)) ||
		!bytes.Contains(w.Body.Bytes(), []byte(`"transcode_enabled":false`)) {
		t.Fatalf("playback missing from %s", w.Body.String())
	}
}

func TestUpdateAppSettings_KeepsMissingFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAppSettingsRepository(ctrl)
	current := &data.AppSettingsRecord{ID: 1, TrashRetentionDays: 7, ServeOGMetadata: true, TranscodingEnabled: true}
	repo.EXPECT().Get().Return(current, nil).Times(2)

	var saved data.AppSettingsRecord
	repo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(r *data.AppSettingsRecord) error {
		saved = *r
		return nil
	})

	h := &AdminHandler{AppSettingsRepo: repo}
	r := gin.New()
	r.PUT("/app-settings", h.UpdateAppSettings)
	body, _ := json.Marshal(map[string]any{"serve_og_metadata": false, "trash_retention_days": 14})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/app-settings", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if saved.ServeOGMetadata || saved.TrashRetentionDays != 14 || !saved.TranscodingEnabled {
		t.Fatalf("unexpected saved settings %+v", saved)
	}
}
