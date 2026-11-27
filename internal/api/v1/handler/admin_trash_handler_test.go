package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTrashTestRouter(t *testing.T) (*gin.Engine, *mocks.MockSceneRepository) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSceneRepository(ctrl)
	svc := core.NewSceneService(repo, t.TempDir(), t.TempDir(), nil, nil, zap.NewNop(), nil, nil, nil, nil, nil)
	h := &AdminHandler{SceneService: svc}
	r := gin.New()
	r.GET("/admin/trash", h.ListTrash)
	r.GET("/admin/trash/ids", h.ListTrashIDs)
	r.GET("/admin/trash/facets", h.TrashFacets)
	r.POST("/admin/trash/restore-bulk", h.RestoreTrashBulk)
	r.POST("/admin/trash/delete-bulk", h.DeleteTrashBulk)
	return r, repo
}

func serve(r *gin.Engine, method, url, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

func TestListTrash_ParsesFiltersAndKeepsOldFields(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	trashedAt := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	repo.EXPECT().ListTrash(gomock.Any()).DoAndReturn(func(q data.TrashQuery) ([]data.TrashedSceneRow, data.TrashTotals, error) {
		f := q.Filter
		from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		before := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
		switch {
		case q.Sort != data.TrashSortSize || q.Desc || q.Page != 2 || q.Limit != 50:
			t.Fatalf("sort/page: %+v", q)
		case f.Query != "beach" || f.StoragePathID == nil || *f.StoragePathID != 4:
			t.Fatalf("query/storage path: %+v", f)
		case f.MinSize != 100 || f.MaxSize != 900 || f.MinDuration != 60 || f.MaxDuration != 600:
			t.Fatalf("ranges: %+v", f)
		case f.TrashedFrom == nil || !f.TrashedFrom.Equal(from) || f.TrashedBefore == nil || !f.TrashedBefore.Equal(before):
			t.Fatalf("dates: %+v", f)
		case f.MinHeight != 1080 || f.MaxHeight != 1439 || f.Codec != "hevc" || f.Source != data.TrashSourceDuplicates:
			t.Fatalf("resolution/codec/source: %+v", f)
		}
		return []data.TrashedSceneRow{{ID: 7, Title: "Beach", ThumbnailPath: "/t/7.webp", Size: 300, TrashedAt: trashedAt, PurgeAt: trashedAt.AddDate(0, 0, 7)}},
			data.TrashTotals{Count: 51, Size: 4000}, nil
	})
	repo.EXPECT().TrashTotals(data.TrashFilter{}).Return(data.TrashTotals{Count: 80, Size: 9000}, nil)

	w := serve(r, "GET", "/admin/trash?page=2&limit=50&sort=size&order=asc&q=%20beach%20&storage_path_id=4"+
		"&min_size=100&max_size=900&min_duration=60&max_duration=600&trashed_from=2026-11-01&trashed_to=2026-11-02"+
		"&resolution=1080p&codec=hevc&source=duplicates", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Data []struct {
			ID            uint   `json:"id"`
			Title         string `json:"title"`
			ThumbnailPath string `json:"thumbnail_path"`
			TrashedAt     string `json:"trashed_at"`
			ExpiresAt     string `json:"expires_at"`
			Size          int64  `json:"size"`
		} `json:"data"`
		Total         int64 `json:"total"`
		Page          int   `json:"page"`
		Limit         int   `json:"limit"`
		RetentionDays int   `json:"retention_days"`
		Pagination    struct {
			TotalItems int64 `json:"total_items"`
			TotalPages int   `json:"total_pages"`
		} `json:"pagination"`
		Totals struct {
			Filtered data.TrashTotals `json:"filtered"`
			All      data.TrashTotals `json:"all"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != 7 || got.Data[0].ThumbnailPath != "/t/7.webp" || got.Data[0].Size != 300 ||
		got.Data[0].TrashedAt != "2026-11-20T10:00:00Z" || got.Data[0].ExpiresAt != "2026-11-27T10:00:00Z" {
		t.Fatalf("data: %s", w.Body.String())
	}
	if got.Total != 51 || got.Page != 2 || got.Limit != 50 || got.RetentionDays != 7 {
		t.Fatalf("old fields: %s", w.Body.String())
	}
	if got.Pagination.TotalItems != 51 || got.Pagination.TotalPages != 2 {
		t.Fatalf("pagination: %s", w.Body.String())
	}
	if got.Totals.Filtered.Count != 51 || got.Totals.Filtered.Size != 4000 || got.Totals.All.Count != 80 || got.Totals.All.Size != 9000 {
		t.Fatalf("totals: %s", w.Body.String())
	}
}

func TestListTrash_Defaults(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	repo.EXPECT().ListTrash(gomock.Any()).DoAndReturn(func(q data.TrashQuery) ([]data.TrashedSceneRow, data.TrashTotals, error) {
		if q.Sort != data.TrashSortTrashedAt || !q.Desc || q.Page != 1 || q.Limit != 20 || q.Filter != (data.TrashFilter{}) {
			t.Fatalf("defaults: %+v", q)
		}
		return nil, data.TrashTotals{}, nil
	})
	if w := serve(r, "GET", "/admin/trash?limit=1000", ""); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTrash_StoragePathNone(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	repo.EXPECT().ListTrash(gomock.Any()).DoAndReturn(func(q data.TrashQuery) ([]data.TrashedSceneRow, data.TrashTotals, error) {
		if q.Filter.StoragePathID == nil || *q.Filter.StoragePathID != 0 || q.Desc || q.Sort != data.TrashSortTitle {
			t.Fatalf("unexpected query: %+v", q)
		}
		return nil, data.TrashTotals{}, nil
	})
	repo.EXPECT().TrashTotals(data.TrashFilter{}).Return(data.TrashTotals{}, nil)
	if w := serve(r, "GET", "/admin/trash?storage_path_id=none&sort=title", ""); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTrash_InvalidParams(t *testing.T) {
	r, _ := newTrashTestRouter(t)
	for _, q := range []string{
		"sort=name", "order=up", "storage_path_id=x", "min_size=-1", "max_duration=abc",
		"trashed_from=yesterday", "trashed_to=2026-13-01", "resolution=8k", "source=web",
	} {
		w := serve(r, "GET", "/admin/trash?"+q, "")
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", q, w.Code)
		}
	}
}

func TestListTrashIDs(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	repo.EXPECT().ListTrashIDs(gomock.Any(), core.MaxTrashBulk).DoAndReturn(func(q data.TrashQuery, limit int) ([]data.TrashIDSize, int64, error) {
		if q.Filter.Codec != "h264" || q.Sort != data.TrashSortPurgeAt {
			t.Fatalf("unexpected query: %+v", q)
		}
		return []data.TrashIDSize{{ID: 4, Size: 40}, {ID: 2, Size: 20}}, 900, nil
	})
	w := serve(r, "GET", "/admin/trash/ids?codec=h264&sort=purge_at", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"ids":[4,2],"items":[{"id":4,"size":40},{"id":2,"size":20}],"limit":500,"total":900}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestTrashFacets(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	repo.EXPECT().TrashFacets().Return(data.TrashFacets{Codecs: []data.TrashCodecCount{{Codec: "hevc", Count: 2}}, StoragePaths: []data.TrashStoragePathCount{}, Duplicates: 1}, nil)
	w := serve(r, "GET", "/admin/trash/facets", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"codec":"hevc"`) {
		t.Fatalf("unexpected response %d: %s", w.Code, w.Body.String())
	}
}

func TestTrashBulk_Validation(t *testing.T) {
	r, _ := newTrashTestRouter(t)
	ids := make([]string, core.MaxTrashBulk+1)
	for i := range ids {
		ids[i] = fmt.Sprint(i + 1)
	}
	tooMany := `{"scene_ids":[` + strings.Join(ids, ",") + `]}`
	for _, path := range []string{"/admin/trash/restore-bulk", "/admin/trash/delete-bulk"} {
		for _, body := range []string{`{}`, `{"scene_ids":[]}`, tooMany} {
			if w := serve(r, "POST", path, body); w.Code != http.StatusBadRequest {
				t.Errorf("%s %.30s: expected 400, got %d", path, body, w.Code)
			}
		}
	}
}

func TestDeleteTrashBulk_PerItemResults(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	now := time.Now()
	scene := &data.Scene{ID: 1, Size: 500, TrashedAt: &now}
	repo.EXPECT().GetByIDIncludingTrashed(uint(1)).Return(scene, nil)
	repo.EXPECT().HardDelete(uint(1)).Return(scene, nil)
	repo.EXPECT().GetByIDIncludingTrashed(uint(2)).Return(nil, gorm.ErrRecordNotFound)

	w := serve(r, "POST", "/admin/trash/delete-bulk", `{"scene_ids":[1,2]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	want := `{"succeeded":[1],"failed":[{"scene_id":2,"error":"scene not found"}],"size":500}`
	if w.Body.String() != want {
		t.Fatalf("got %s, want %s", w.Body.String(), want)
	}
}

func TestRestoreTrashBulk_PerItemResults(t *testing.T) {
	r, repo := newTrashTestRouter(t)
	repo.EXPECT().GetByIDIncludingTrashed(uint(3)).Return(&data.Scene{ID: 3, Size: 10}, nil).Times(2)

	w := serve(r, "POST", "/admin/trash/restore-bulk", `{"scene_ids":[3]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"succeeded":[]`) || !strings.Contains(w.Body.String(), `"scene_id":3`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}
