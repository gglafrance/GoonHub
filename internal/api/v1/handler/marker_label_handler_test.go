package handler

import (
	"net/http"
	"strings"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type markerLabelTestDeps struct {
	markers *mocks.MockMarkerRepository
	labels  *mocks.MockMarkerLabelRepository
	scenes  *mocks.MockSceneRepository
	tags    *mocks.MockTagRepository
}

// newMarkerLabelTestRouter mounts the marker label routes, and the marker creation as user 1
// holding perms (the permission routes are checked by the router, not by the handlers).
func newMarkerLabelTestRouter(t *testing.T, perms ...string) (*gin.Engine, markerLabelTestDeps) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	d := markerLabelTestDeps{
		markers: mocks.NewMockMarkerRepository(ctrl),
		labels:  mocks.NewMockMarkerLabelRepository(ctrl),
		scenes:  mocks.NewMockSceneRepository(ctrl),
		tags:    mocks.NewMockTagRepository(ctrl),
	}
	svc := core.NewMarkerService(d.markers, d.labels, d.scenes, d.tags, &config.Config{}, zap.NewNop())
	h := NewMarkerHandler(svc, 100)
	h.SetLabelService(core.NewMarkerLabelService(d.labels, d.tags, zap.NewNop()))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Username: "u"})
		middleware.SetAccess(c, core.NewUserAccess(1, "user", false, perms...))
		c.Next()
	})
	r.POST("/scenes/:id/markers", h.CreateMarker)
	r.GET("/marker-labels", h.ListMarkerLabels)
	r.POST("/marker-labels", h.CreateMarkerLabel)
	r.PUT("/marker-labels/:labelID", h.UpdateMarkerLabel)
	r.DELETE("/marker-labels/:labelID", h.DeleteMarkerLabel)
	r.POST("/marker-labels/delete", h.DeleteMarkerLabels)
	r.POST("/marker-labels/merge", h.MergeMarkerLabels)
	r.GET("/markers/label-tags", h.GetLabelTags)
	r.PUT("/markers/label-tags", h.SetLabelTags)
	return r, d
}

func TestCreateMarker_LabelPermission(t *testing.T) {
	expectScene := func(d markerLabelTestDeps) {
		d.scenes.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil).AnyTimes()
		d.markers.EXPECT().CountByUserAndScene(uint(1), uint(2)).Return(int64(0), nil)
	}
	t.Run("unknown label without marker_labels:manage", func(t *testing.T) {
		r, d := newMarkerLabelTestRouter(t, core.PermMarkersCreate)
		expectScene(d)
		d.labels.EXPECT().FindByName("Brand new").Return(nil, nil)
		w, body := doAliasRequest(t, r, http.MethodPost, "/scenes/2/markers", `{"timestamp":5,"label":"Brand new"}`)
		if w.Code != http.StatusBadRequest || !strings.Contains(body["error"].(string), "unknown marker label") {
			t.Fatalf("status %d body %v", w.Code, body)
		}
	})
	t.Run("unknown label created with marker_labels:manage", func(t *testing.T) {
		r, d := newMarkerLabelTestRouter(t, core.PermMarkersCreate, core.PermMarkerLabelsManage)
		expectScene(d)
		d.labels.EXPECT().FindByName("Brand new").Return(nil, nil)
		d.labels.EXPECT().Ensure("Brand new").Return(&data.MarkerLabel{ID: 4, Name: "Brand new", Color: "#FFFFFF"}, true, nil)
		d.markers.EXPECT().Create(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error { m.ID = 9; return nil })
		d.markers.EXPECT().ApplyLabelTagsToMarker(uint(9)).Return(nil)
		w, body := doAliasRequest(t, r, http.MethodPost, "/scenes/2/markers", `{"timestamp":5,"label":"Brand new"}`)
		if w.Code != http.StatusCreated || body["label"] != "Brand new" || body["label_id"] != float64(4) {
			t.Fatalf("status %d body %v", w.Code, body)
		}
	})
}

func TestMarkerLabelEndpoints(t *testing.T) {
	r, d := newMarkerLabelTestRouter(t, core.PermMarkerLabelsManage)

	if w, _ := doAliasRequest(t, r, http.MethodPost, "/marker-labels", `{"color":"#FFFFFF"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("create without name: status %d", w.Code)
	}

	// create with a name used ignoring case: 409 with the label to merge into
	d.labels.EXPECT().FindByName("kiss").Return(&data.MarkerLabel{ID: 2, Name: "Kiss"}, nil)
	w, body := doAliasRequest(t, r, http.MethodPost, "/marker-labels", `{"name":"kiss"}`)
	existing, _ := body["existing"].(map[string]any)
	if w.Code != http.StatusConflict || existing["id"] != float64(2) || body["field"] != "name" {
		t.Fatalf("conflict: status %d body %v", w.Code, body)
	}

	d.labels.EXPECT().FindByName("Hug").Return(nil, nil)
	d.labels.EXPECT().Create(gomock.Any()).DoAndReturn(func(l *data.MarkerLabel) error { l.ID = 3; return nil })
	if w, body := doAliasRequest(t, r, http.MethodPost, "/marker-labels", `{"name":"Hug","color":"#ff0000"}`); w.Code != http.StatusCreated || body["color"] != "#FF0000" {
		t.Fatalf("create: status %d body %v", w.Code, body)
	}

	if w, _ := doAliasRequest(t, r, http.MethodPut, "/marker-labels/x", `{"name":"a"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: status %d", w.Code)
	}
	d.labels.EXPECT().GetByID(uint(3)).Return(&data.MarkerLabel{ID: 3, Name: "Hug", Color: "#FF0000"}, nil)
	d.labels.EXPECT().Update(gomock.Any(), "Hug").Return(int64(0), nil)
	if w, body := doAliasRequest(t, r, http.MethodPut, "/marker-labels/3", `{"color":"#00ff00"}`); w.Code != http.StatusOK || body["color"] != "#00FF00" {
		t.Fatalf("recolor: status %d body %v", w.Code, body)
	}

	d.labels.EXPECT().GetByIDs([]uint{3}).Return([]data.MarkerLabel{{ID: 3}}, nil)
	d.labels.EXPECT().Delete([]uint{3}).Return(int64(2), nil)
	if w, body := doAliasRequest(t, r, http.MethodDelete, "/marker-labels/3", ""); w.Code != http.StatusOK || body["markers_cleared"] != float64(2) {
		t.Fatalf("delete: status %d body %v", w.Code, body)
	}

	if w, _ := doAliasRequest(t, r, http.MethodPost, "/marker-labels/merge", `{"target_id":1}`); w.Code != http.StatusBadRequest {
		t.Fatalf("merge without sources: status %d", w.Code)
	}

	d.labels.EXPECT().ListWithUsage().Return([]data.MarkerLabelWithUsage{{MarkerLabel: data.MarkerLabel{ID: 1, Name: "Kiss"}, MarkerCount: 3}}, nil)
	if w, body := doAliasRequest(t, r, http.MethodGet, "/marker-labels", ""); w.Code != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("list: status %d body %v", w.Code, body)
	}

	// default tags by name, ignoring case
	d.labels.EXPECT().FindByName("nope").Return(nil, nil)
	if w, _ := doAliasRequest(t, r, http.MethodGet, "/markers/label-tags?label=nope", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown label tags: status %d", w.Code)
	}
	d.labels.EXPECT().FindByName("KISS").Return(&data.MarkerLabel{ID: 1, Name: "Kiss"}, nil)
	d.labels.EXPECT().GetByID(uint(1)).Return(&data.MarkerLabel{ID: 1, Name: "Kiss"}, nil).Times(2)
	d.tags.EXPECT().GetByIDs([]uint{5}).Return([]data.Tag{{ID: 5}}, nil)
	d.labels.EXPECT().SetDefaultTags(uint(1), []uint{5}).Return(nil)
	d.labels.EXPECT().GetDefaultTags(uint(1)).Return([]data.Tag{{ID: 5, Name: "oral"}}, nil)
	if w, body := doAliasRequest(t, r, http.MethodPut, "/markers/label-tags?label=KISS", `{"tag_ids":[5]}`); w.Code != http.StatusOK || len(body["tags"].([]any)) != 1 {
		t.Fatalf("set label tags: status %d body %v", w.Code, body)
	}
}
