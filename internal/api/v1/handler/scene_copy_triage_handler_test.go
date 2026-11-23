package handler

import (
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

// newCopyTriageRouter serves the copy triage endpoints next to a duplicate group route with a
// parameter at the same level, as in routes.go (the static "copies" segment must win).
func newCopyTriageRouter(t *testing.T) (*gin.Engine, *mocks.MockSceneCopyRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	copies := mocks.NewMockSceneCopyRepository(ctrl)
	svc := core.NewSceneCopyService(copies, nil, nil, nil, nil, zap.NewNop())
	h := NewScanHandler(nil, svc)
	r := gin.New()
	r.GET("/duplicates/copies", h.ListCopies)
	r.GET("/duplicates/copies/ids", h.CopyIDs)
	r.POST("/duplicates/copies/forget-bulk", h.ForgetCopiesBulk)
	r.POST("/duplicates/copies/delete-files-bulk", h.DeleteCopyFilesBulk)
	r.POST("/duplicates/copies/:id/forget", h.ForgetCopy)
	r.POST("/duplicates/copies/:id/delete-file", h.DeleteCopyFile)
	r.GET("/duplicates/:id", func(c *gin.Context) { c.Status(http.StatusTeapot) })
	r.POST("/duplicates/:id/resolve", func(c *gin.Context) { c.Status(http.StatusTeapot) })
	return r, copies
}

func doJSON(r *gin.Engine, method, url, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestListCopies_BadParams(t *testing.T) {
	r, _ := newCopyTriageRouter(t)
	for _, q := range []string{"sort_by=title", "storage_path_id=x", "storage_path_id=0", "min_size=-1", "max_size=abc", "min_size=10&max_size=5"} {
		if w := doJSON(r, http.MethodGet, "/duplicates/copies?"+q, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", q, w.Code)
		}
	}
}

func TestListCopies_FiltersAndShape(t *testing.T) {
	r, copies := newCopyTriageRouter(t)
	sp := uint(3)
	f := data.SceneCopyFilter{StoragePathID: &sp, Search: "stash", MinSize: 100, MaxSize: 900}
	q := data.SceneCopyQuery{SceneCopyFilter: f, Sort: "copies", Page: 2, Limit: 10}
	copies.EXPECT().CopyTotals(f).Return(data.SceneCopyTotals{Scenes: 11, Copies: 30, Size: 5000}, nil)
	copies.EXPECT().ListScenesWithCopies(q).Return([]data.SceneWithCopiesRow{{SceneID: 4, StoredPath: "/m/a.mp4", Size: 500, CopyCount: 1}}, nil)
	copies.EXPECT().LastCompletedScanStart().Return(nil, nil)
	copies.EXPECT().ListCopiesOfScenes([]uint{4}, f).Return([]data.SceneCopy{{ID: 9, SceneID: 4, StoredPath: "/m/stash/a.mp4", Size: 500}}, nil)

	w := doJSON(r, http.MethodGet, "/duplicates/copies?storage_path_id=3&q=stash&min_size=100&max_size=900&sort_by=copies&page=2&limit=10", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []struct {
			SceneID uint `json:"scene_id"`
			Copies  []struct {
				ID                uint   `json:"id"`
				StoredPath        string `json:"stored_path"`
				NotSeenInLastScan bool   `json:"not_seen_in_last_scan"`
			} `json:"copies"`
		} `json:"data"`
		Pagination struct {
			Page       int   `json:"page"`
			TotalItems int64 `json:"total_items"`
			TotalPages int   `json:"total_pages"`
		} `json:"pagination"`
		Totals data.SceneCopyTotals `json:"totals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].SceneID != 4 || len(body.Data[0].Copies) != 1 || body.Data[0].Copies[0].StoredPath != "/m/stash/a.mp4" {
		t.Fatalf("unexpected data: %s", w.Body.String())
	}
	if body.Pagination.Page != 2 || body.Pagination.TotalItems != 11 || body.Pagination.TotalPages != 2 || body.Totals.Size != 5000 {
		t.Fatalf("unexpected pagination/totals: %s", w.Body.String())
	}
}

func TestCopyIDs_ReturnsCap(t *testing.T) {
	r, copies := newCopyTriageRouter(t)
	copies.EXPECT().ListCopyIDs(data.SceneCopyFilter{}, core.MaxBulkSceneCopies).Return([]uint{1, 2}, int64(2), nil)
	w := doJSON(r, http.MethodGet, "/duplicates/copies/ids", "")
	if w.Code != http.StatusOK || w.Body.String() != `{"ids":[1,2],"limit":500,"total":2}` {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

// Deleting files needs confirm=true; nothing is read or deleted otherwise.
func TestDeleteCopyFile_RequiresConfirm(t *testing.T) {
	r, _ := newCopyTriageRouter(t)
	for _, body := range []string{"", "{}", `{"confirm":false}`} {
		if w := doJSON(r, http.MethodPost, "/duplicates/copies/5/delete-file", body); w.Code != http.StatusBadRequest {
			t.Fatalf("body %q: expected 400, got %d", body, w.Code)
		}
	}
	if w := doJSON(r, http.MethodPost, "/duplicates/copies/delete-files-bulk", `{"copy_ids":[1]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bulk without confirm: expected 400, got %d", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/duplicates/copies/abc/delete-file", `{"confirm":true}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: expected 400, got %d", w.Code)
	}
}

func TestCopyBulk_Limits(t *testing.T) {
	r, _ := newCopyTriageRouter(t)
	ids := make([]uint, core.MaxBulkSceneCopies+1)
	for i := range ids {
		ids[i] = uint(i + 1)
	}
	raw, _ := json.Marshal(map[string]any{"copy_ids": ids, "confirm": true})
	for _, url := range []string{"/duplicates/copies/forget-bulk", "/duplicates/copies/delete-files-bulk"} {
		if w := doJSON(r, http.MethodPost, url, string(raw)); w.Code != http.StatusBadRequest {
			t.Fatalf("%s over the cap: expected 400, got %d", url, w.Code)
		}
		if w := doJSON(r, http.MethodPost, url, `{"copy_ids":[]}`); w.Code != http.StatusBadRequest {
			t.Fatalf("%s empty: expected 400, got %d", url, w.Code)
		}
	}
}

func TestForgetCopies(t *testing.T) {
	r, copies := newCopyTriageRouter(t)
	copies.EXPECT().Delete(uint(5)).Return(gorm.ErrRecordNotFound)
	if w := doJSON(r, http.MethodPost, "/duplicates/copies/5/forget", ""); w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	copies.EXPECT().Delete(uint(1)).Return(nil)
	copies.EXPECT().Delete(uint(2)).Return(gorm.ErrRecordNotFound)
	w := doJSON(r, http.MethodPost, "/duplicates/copies/forget-bulk", `{"copy_ids":[1,2]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res core.SceneCopyBulkResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || len(res.Done) != 1 || len(res.Failed) != 1 || res.Failed[0].CopyID != 2 {
		t.Fatalf("unexpected result: %s (%v)", w.Body.String(), err)
	}
	// The group routes with a parameter still answer
	if w := doJSON(r, http.MethodGet, "/duplicates/12", ""); w.Code != http.StatusTeapot {
		t.Fatalf("group route shadowed: %d", w.Code)
	}
}
