package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type importHandlerDeps struct {
	importRepo *mocks.MockImportRepository
	sceneRepo  *mocks.MockSceneRepository
	tagRepo    *mocks.MockTagRepository
}

func newTestImportRouter(t *testing.T) (*gin.Engine, *importHandlerDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := &importHandlerDeps{
		importRepo: mocks.NewMockImportRepository(ctrl),
		sceneRepo:  mocks.NewMockSceneRepository(ctrl),
		tagRepo:    mocks.NewMockTagRepository(ctrl),
	}
	svc := core.NewImportService(d.importRepo, d.sceneRepo, d.tagRepo,
		mocks.NewMockActorRepository(ctrl), mocks.NewMockStudioRepository(ctrl),
		mocks.NewMockStoragePathRepository(ctrl), mocks.NewMockUserRepository(ctrl),
		mocks.NewMockInteractionRepository(ctrl), nil, zap.NewNop())
	h := NewImportHandler(svc, zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "admin"})
		c.Next()
	})
	r.GET("/import/capabilities", h.Capabilities)
	r.POST("/import/scenes", h.ImportScene)
	r.POST("/import/scenes/summary", h.SceneSummary)
	r.POST("/import/markers", h.ImportMarker)
	return r, d
}

func (d *importHandlerDeps) expectTx() {
	d.importRepo.EXPECT().RunInTransaction(gomock.Any()).DoAndReturn(func(fn func(data.ImportRepository) error) error {
		return fn(d.importRepo)
	})
}

func doImportRequest(t *testing.T, r *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("invalid JSON response %q: %v", w.Body.String(), err)
		}
	}
	return w, out
}

func TestImportCapabilities(t *testing.T) {
	r, _ := newTestImportRouter(t)
	w, out := doImportRequest(t, r, http.MethodGet, "/import/capabilities", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if out["version"] != float64(2) {
		t.Fatalf("expected version 2, got %v", out["version"])
	}
	features, _ := out["features"].([]any)
	want := []string{"scene_associations", "scene_interactions", "scene_update_existing", "marker_dedup", "marker_tags", "marker_end", "scene_summary", "jizz_history", "skip_index", "source_url"}
	if len(features) != len(want) {
		t.Fatalf("expected %d features, got %v", len(want), features)
	}
	for i, f := range want {
		if features[i] != f {
			t.Fatalf("expected feature %q at %d, got %v", f, i, features[i])
		}
	}
}

func TestImportSceneHandler_Created(t *testing.T) {
	r, d := newTestImportRouter(t)
	d.sceneRepo.EXPECT().GetByStoredPath("/m/a.mp4").Return(nil, gorm.ErrRecordNotFound)
	d.expectTx()
	d.importRepo.EXPECT().CreateScene(gomock.Any()).DoAndReturn(func(s *data.Scene) error { s.ID = 42; return nil })

	w, out := doImportRequest(t, r, http.MethodPost, "/import/scenes",
		`{"title":"A","stored_path":"/m/a.mp4","skip_file_check":true,"tag_ids":[],"skip_index":true}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if out["id"] != float64(42) || out["title"] != "A" || out["created"] != true {
		t.Fatalf("unexpected body: %v", out)
	}
}

func TestImportSceneHandler_Conflict(t *testing.T) {
	r, d := newTestImportRouter(t)
	d.sceneRepo.EXPECT().GetByStoredPath("/m/a.mp4").Return(&data.Scene{ID: 7, Title: "Existing"}, nil)

	w, out := doImportRequest(t, r, http.MethodPost, "/import/scenes",
		`{"title":"A","stored_path":"/m/a.mp4","skip_file_check":true}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if out["id"] != float64(7) || out["title"] != "Existing" {
		t.Fatalf("unexpected body: %v", out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "already exists") {
		t.Fatalf("unexpected error message: %v", out["error"])
	}
}

func TestImportSceneHandler_UpdateExisting(t *testing.T) {
	r, d := newTestImportRouter(t)
	d.sceneRepo.EXPECT().GetByStoredPath("/m/a.mp4").Return(&data.Scene{ID: 7, Title: "Existing"}, nil)
	d.expectTx()
	d.importRepo.EXPECT().UpdateSceneFields(uint(7), gomock.Any()).Return(nil)
	d.importRepo.EXPECT().SetSceneTags(uint(7), []uint{}).Return(nil) // [] = clear

	w, out := doImportRequest(t, r, http.MethodPost, "/import/scenes",
		`{"title":"A","stored_path":"/m/a.mp4","skip_file_check":true,"update_existing":true,"tag_ids":[],"actor_ids":null,"skip_index":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if out["id"] != float64(7) || out["created"] != false {
		t.Fatalf("unexpected body: %v", out)
	}
}

func TestImportSceneHandler_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed json", `{`},
		{"missing title", `{"stored_path":"/m/a.mp4"}`},
		{"bad release date", `{"title":"A","stored_path":"/m/a.mp4","skip_file_check":true,"release_date":"01/02/2020"}`},
		{"bad rating", `{"title":"A","stored_path":"/m/a.mp4","skip_file_check":true,"interactions":{"rating":7}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newTestImportRouter(t)
			w, out := doImportRequest(t, r, http.MethodPost, "/import/scenes", tt.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if out["error"] == "" || out["error"] == nil {
				t.Fatalf("expected an error message, got %v", out)
			}
		})
	}
}

func TestImportSceneHandler_UnknownTagIsBadRequest(t *testing.T) {
	r, d := newTestImportRouter(t)
	d.tagRepo.EXPECT().GetByIDs([]uint{5}).Return([]data.Tag{}, nil)

	w, out := doImportRequest(t, r, http.MethodPost, "/import/scenes",
		`{"title":"A","stored_path":"/m/a.mp4","skip_file_check":true,"tag_ids":[5]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "unknown tag_ids: [5]") {
		t.Fatalf("unexpected error: %v", out)
	}
}

func TestImportMarkerHandler(t *testing.T) {
	t.Run("label too long", func(t *testing.T) {
		r, _ := newTestImportRouter(t)
		body := `{"scene_id":1,"timestamp":5,"label":"` + strings.Repeat("a", 101) + `"}`
		w, _ := doImportRequest(t, r, http.MethodPost, "/import/markers", body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("created", func(t *testing.T) {
		r, d := newTestImportRouter(t)
		d.sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
		d.expectTx()
		d.importRepo.EXPECT().FindMarker(uint(1), uint(1), 5, "Intro").Return(nil, nil)
		d.importRepo.EXPECT().CreateMarker(gomock.Any()).DoAndReturn(func(m *data.UserSceneMarker) error { m.ID = 9; return nil })
		d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(1), uint(9), "Intro").Return(nil)

		w, out := doImportRequest(t, r, http.MethodPost, "/import/markers", `{"scene_id":1,"timestamp":5,"label":"Intro"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
		if out["id"] != float64(9) || out["scene_id"] != float64(1) || out["created"] != true {
			t.Fatalf("unexpected body: %v", out)
		}
	})
	t.Run("conflict", func(t *testing.T) {
		r, d := newTestImportRouter(t)
		d.sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
		d.expectTx()
		d.importRepo.EXPECT().FindMarker(uint(1), uint(1), 5, "Intro").Return(&data.UserSceneMarker{ID: 9, SceneID: 1}, nil)

		w, out := doImportRequest(t, r, http.MethodPost, "/import/markers", `{"scene_id":1,"timestamp":5,"label":"Intro"}`)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
		if out["id"] != float64(9) || out["scene_id"] != float64(1) || out["error"] == nil {
			t.Fatalf("unexpected body: %v", out)
		}
	})
	t.Run("update existing", func(t *testing.T) {
		r, d := newTestImportRouter(t)
		d.sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
		d.expectTx()
		d.importRepo.EXPECT().FindMarker(uint(1), uint(1), 5, "Intro").Return(&data.UserSceneMarker{ID: 9, SceneID: 1, Color: "#FFFFFF"}, nil)
		d.importRepo.EXPECT().ApplyLabelTagsToMarker(uint(1), uint(9), "Intro").Return(nil)

		w, out := doImportRequest(t, r, http.MethodPost, "/import/markers",
			`{"scene_id":1,"timestamp":5,"label":"Intro","color":"#FFFFFF","update_existing":true}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if out["id"] != float64(9) || out["created"] != false {
			t.Fatalf("unexpected body: %v", out)
		}
	})
}

func TestImportSceneSummaryHandler(t *testing.T) {
	t.Run("too many ids", func(t *testing.T) {
		r, _ := newTestImportRouter(t)
		ids := make([]string, core.MaxImportSummaryIDs+1)
		for i := range ids {
			ids[i] = "1"
		}
		w, _ := doImportRequest(t, r, http.MethodPost, "/import/scenes/summary", `{"ids":[`+strings.Join(ids, ",")+`]}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
	t.Run("all missing", func(t *testing.T) {
		r, d := newTestImportRouter(t)
		d.importRepo.EXPECT().GetScenesForSummary([]uint{4, 5}).Return([]data.Scene{}, nil)
		w, out := doImportRequest(t, r, http.MethodPost, "/import/scenes/summary", `{"ids":[4,5]}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		dataItems, ok := out["data"].([]any)
		if !ok || len(dataItems) != 0 {
			t.Fatalf("expected empty data array, got %v", out["data"])
		}
		missing, _ := out["missing"].([]any)
		if len(missing) != 2 || missing[0] != float64(4) || missing[1] != float64(5) {
			t.Fatalf("expected missing [4 5], got %v", out["missing"])
		}
	})
}
