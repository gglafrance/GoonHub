package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
)

func newPlaylistMarkerHandlerForTest(t *testing.T) (*gin.Engine, *mocks.MockPlaylistRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockPlaylistRepository(ctrl)
	svc := core.NewPlaylistService(repo, mocks.NewMockSceneRepository(ctrl), mocks.NewMockTagRepository(ctrl),
		config.PlaylistsConfig{MaxMarkers: 100, MarkerDefaultDuration: 20}, zap.NewNop())
	h := NewPlaylistHandler(svc, 100)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Username: "u", Role: "user"})
		c.Next()
	})
	r.GET("/playlists/:uuid", h.GetByUUID)
	r.POST("/playlists/:uuid/markers", h.AddMarkers)
	r.POST("/playlists/:uuid/markers/from-filter", h.AddMarkersFromFilter)
	r.DELETE("/playlists/:uuid/markers/:markerId", h.RemoveMarker)
	r.POST("/playlists/:uuid/markers/remove", h.RemoveMarkers)
	r.PUT("/playlists/:uuid/markers/reorder", h.ReorderMarkers)
	r.PUT("/playlists/:uuid/progress", h.UpdateProgress)
	return r, repo
}

func doPlaylistRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func ownMarkerPlaylist() *data.Playlist {
	return &data.Playlist{ID: 3, UUID: uuid.New(), UserID: 1, Name: "Moments", Visibility: "private",
		Kind: data.PlaylistKindMarkers, User: data.User{ID: 1, Username: "u"}}
}

func TestPlaylistMarkerHandler_BadRequests(t *testing.T) {
	r, _ := newPlaylistMarkerHandlerForTest(t)
	id := uuid.NewString()
	tests := []struct{ method, path, body string }{
		{http.MethodPost, "/playlists/not-a-uuid/markers", `{"marker_ids":[1]}`},
		{http.MethodPost, "/playlists/" + id + "/markers", `{}`},
		{http.MethodPost, "/playlists/" + id + "/markers", `not json`},
		{http.MethodDelete, "/playlists/" + id + "/markers/abc", ``},
		{http.MethodDelete, "/playlists/" + id + "/markers/0", ``},
		{http.MethodPost, "/playlists/" + id + "/markers/remove", `{}`},
		{http.MethodPut, "/playlists/" + id + "/markers/reorder", `{}`},
	}
	for _, tt := range tests {
		if w := doPlaylistRequest(r, tt.method, tt.path, tt.body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s %s %s: expected 400, got %d: %s", tt.method, tt.path, tt.body, w.Code, w.Body.String())
		}
	}
}

func TestPlaylistMarkerHandler_AddMarkers(t *testing.T) {
	r, repo := newPlaylistMarkerHandlerForTest(t)
	p := ownMarkerPlaylist()
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
	repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{5, 6}).Return([]uint{5, 6}, nil)
	repo.EXPECT().AddMarkers(uint(3), []uint{5, 6}, 100).Return(1, nil)

	w := doPlaylistRequest(r, http.MethodPost, "/playlists/"+p.UUID.String()+"/markers", `{"marker_ids":[5,6]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res core.AddPlaylistMarkersResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Added != 1 || res.Skipped != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestPlaylistMarkerHandler_AddHiddenMarker404(t *testing.T) {
	r, repo := newPlaylistMarkerHandlerForTest(t)
	p := ownMarkerPlaylist()
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
	repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{9}).Return([]uint{}, nil)

	w := doPlaylistRequest(r, http.MethodPost, "/playlists/"+p.UUID.String()+"/markers", `{"marker_ids":[9]}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlaylistMarkerHandler_AddFromFilter(t *testing.T) {
	r, repo := newPlaylistMarkerHandlerForTest(t)
	p := ownMarkerPlaylist()
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
	want := data.MarkerListOptions{SortBy: "random", Seed: 7, RangesOnly: true, Labels: []string{"Kiss", "Hug"}, Tags: []string{"t1"}, Studio: "S"}
	repo.EXPECT().ResolveVisibleMarkerIDs(uint(1), "", want, 101).Return([]uint{4}, nil)
	repo.EXPECT().AddMarkers(uint(3), []uint{4}, 100).Return(1, nil)

	body := `{"labels":"Kiss, Hug","tags":"t1","studio":" S ","ranges_only":true,"sort":"random","seed":7}`
	w := doPlaylistRequest(r, http.MethodPost, "/playlists/"+p.UUID.String()+"/markers/from-filter", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlaylistMarkerHandler_RemoveAndReorder(t *testing.T) {
	r, repo := newPlaylistMarkerHandlerForTest(t)
	p := ownMarkerPlaylist()
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil).Times(3)
	repo.EXPECT().RemoveMarkers(uint(3), []uint{5}).Return(int64(1), nil)
	repo.EXPECT().RemoveMarkers(uint(3), []uint{6, 7}).Return(int64(2), nil)
	repo.EXPECT().ReorderMarkers(uint(3), []uint{7, 6}).Return(nil)

	base := "/playlists/" + p.UUID.String() + "/markers"
	if w := doPlaylistRequest(r, http.MethodDelete, base+"/5", ``); w.Code != http.StatusNoContent {
		t.Fatalf("remove: got %d: %s", w.Code, w.Body.String())
	}
	if w := doPlaylistRequest(r, http.MethodPost, base+"/remove", `{"marker_ids":[6,7]}`); w.Code != http.StatusNoContent {
		t.Fatalf("remove many: got %d: %s", w.Code, w.Body.String())
	}
	if w := doPlaylistRequest(r, http.MethodPut, base+"/reorder", `{"marker_ids":[7,6]}`); w.Code != http.StatusNoContent {
		t.Fatalf("reorder: got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlaylistMarkerHandler_DetailHasNoFileInfo(t *testing.T) {
	r, repo := newPlaylistMarkerHandlerForTest(t)
	p := ownMarkerPlaylist()
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
	repo.EXPECT().GetMarkerStats(uint(3), 20).Return(data.PlaylistMarkerStats{MarkerCount: 1, SceneCount: 1, TotalDuration: 20}, nil)
	repo.EXPECT().GetThumbnailMarkers(uint(3), 4).Return(nil, nil)
	repo.EXPECT().GetPlaylistTags(uint(3)).Return(nil, nil)
	repo.EXPECT().GetLikeStatus(uint(1), uint(3)).Return(false, nil)
	repo.EXPECT().GetLikeCount(uint(3)).Return(int64(0), nil)
	repo.EXPECT().GetPlaylistMarkers(uint(3)).Return([]data.PlaylistMarkerRow{
		{Position: 0, MarkerID: 8, MarkerUserID: 1, SceneID: 40, Timestamp: 10, Label: "Kiss", Color: "#FFFFFF",
			ThumbnailPath: "/data/marker_8.webp", UpdatedAt: updated, SceneTitle: "Scene", SceneDuration: 25},
	}, nil)
	repo.EXPECT().GetProgress(uint(1), uint(3)).Return(nil, nil)

	w := doPlaylistRequest(r, http.MethodGet, "/playlists/"+p.UUID.String(), ``)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"thumbnail_path", "stored_path", "\"size\"", "/data/"} {
		if strings.Contains(body, leak) {
			t.Fatalf("detail leaks %s: %s", leak, body)
		}
	}
	var detail struct {
		Kind    string `json:"kind"`
		Markers []struct {
			Start  int `json:"start"`
			End    int `json:"end"`
			Marker struct {
				ID               uint  `json:"id"`
				HasThumbnail     bool  `json:"has_thumbnail"`
				ThumbnailVersion int64 `json:"thumbnail_version"`
			} `json:"marker"`
		} `json:"markers"`
		Scenes                []any `json:"scenes"`
		MarkerDefaultDuration int   `json:"marker_default_duration"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.Kind != "markers" || len(detail.Markers) != 1 || detail.Scenes == nil || detail.MarkerDefaultDuration != 20 {
		t.Fatalf("detail = %s", body)
	}
	m := detail.Markers[0]
	// no end: 20 s from 10, cut at the scene end (25)
	if m.Start != 10 || m.End != 25 || !m.Marker.HasThumbnail || m.Marker.ThumbnailVersion != updated.UnixMilli() {
		t.Fatalf("marker entry = %+v", m)
	}
}

func TestPlaylistMarkerHandler_UpdateProgress(t *testing.T) {
	r, repo := newPlaylistMarkerHandlerForTest(t)
	p := ownMarkerPlaylist()
	markerID := uint(8)
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
	repo.EXPECT().HasMarker(uint(3), markerID).Return(true, uint(40), nil)
	repo.EXPECT().UpsertProgress(uint(1), uint(3), uint(40), &markerID, 3.5).Return(nil)

	w := doPlaylistRequest(r, http.MethodPut, "/playlists/"+p.UUID.String()+"/progress", `{"marker_id":8,"position_s":3.5}`)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}
