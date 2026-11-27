package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newNavLayoutTestRouter(t *testing.T, access *core.UserAccess) (*gin.Engine, *mocks.MockNavLayoutRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNavLayoutRepository(ctrl)
	h := NewSettingsHandler(core.NewSettingsService(mocks.NewMockUserSettingsRepository(ctrl), mocks.NewMockUserRepository(ctrl), zap.NewNop()), 100)
	h.NavLayout = core.NewNavLayoutService(repo, core.NewEventBus(zap.NewNop()), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if access != nil {
			middleware.SetAccess(c, access)
		}
		c.Next()
	})
	r.GET("/settings/navigation", h.GetNavLayout)
	r.PUT("/settings/navigation", h.UpdateNavLayout)
	r.DELETE("/settings/navigation", h.ResetNavLayout)
	return r, repo
}

func doNavLayoutRequest(t *testing.T, r *gin.Engine, method, body string) (*httptest.ResponseRecorder, core.NavLayoutView) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/settings/navigation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var view core.NavLayoutView
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
			t.Fatalf("decode %s: %v", w.Body.String(), err)
		}
	}
	return w, view
}

func layoutBody(t *testing.T, layout data.NavLayout) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"layout": layout})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGetNavLayout_DefaultForAViewer(t *testing.T) {
	r, repo := newNavLayoutTestRouter(t, core.NewUserAccess(7, "user", false, core.PermScenesView))
	repo.EXPECT().Get(gomock.Any(), uint(7)).Return(nil, nil)

	w, view := doNavLayoutRequest(t, r, http.MethodGet, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if view.Customized || view.Layout.Version != core.NavLayoutVersion {
		t.Fatalf("view = %+v", view)
	}
	if slices.Contains(view.Layout.Bars.Top.Right, "job_status") {
		t.Fatal("job status offered without jobs:view_status")
	}
	// arrays, never null
	if !strings.Contains(w.Body.String(), `"center":[]`) || !strings.Contains(w.Body.String(), `"hidden":[]`) {
		t.Fatalf("empty lists not sent as arrays: %s", w.Body.String())
	}
}

func TestGetNavLayout_Unauthenticated(t *testing.T) {
	r, _ := newNavLayoutTestRouter(t, nil)
	if w, _ := doNavLayoutRequest(t, r, http.MethodGet, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", w.Code)
	}
}

func TestUpdateNavLayout_StripsAdminItemsOfAViewer(t *testing.T) {
	r, repo := newNavLayoutTestRouter(t, core.NewUserAccess(7, "user", false, core.PermScenesView))
	layout := core.DefaultNavLayout()
	layout.Bars.Nav.Right = []string{"history"}
	layout.Bars.Nav.Left = slices.DeleteFunc(slices.Clone(layout.Bars.Nav.Left), func(s string) bool { return s == "history" })
	var saved data.NavLayout
	repo.EXPECT().Save(gomock.Any(), uint(7), gomock.Any()).DoAndReturn(func(_ context.Context, _ uint, l data.NavLayout) error {
		saved = l
		return nil
	})

	w, view := doNavLayoutRequest(t, r, http.MethodPut, layoutBody(t, layout))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !view.Customized || !slices.Equal(view.Layout.Bars.Nav.Right, []string{"history"}) {
		t.Fatalf("view = %+v", view)
	}
	if slices.Contains(saved.Folders[0].Items, "duplicates") || slices.Contains(saved.Folders[0].Items, "explorer") ||
		slices.Contains(saved.Bars.Top.Right, "job_status") || slices.Contains(saved.Bars.Nav.Left, "together") {
		t.Fatalf("admin items stored for a viewer: %+v", saved)
	}
}

func TestUpdateNavLayout_RefusesInvalidLayouts(t *testing.T) {
	r, _ := newNavLayoutTestRouter(t, core.NewUserAccess(1, "admin", true))
	unknown := core.DefaultNavLayout()
	unknown.Bars.Top.Center = []string{"not_an_item"}
	for name, body := range map[string]string{
		"not json":       `{`,
		"missing layout": `{}`,
		"unknown item":   layoutBody(t, unknown),
	} {
		if w, _ := doNavLayoutRequest(t, r, http.MethodPut, body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
}

func TestResetNavLayout(t *testing.T) {
	r, repo := newNavLayoutTestRouter(t, core.NewUserAccess(1, "admin", true))
	repo.EXPECT().Delete(gomock.Any(), uint(1)).Return(nil)
	w, view := doNavLayoutRequest(t, r, http.MethodDelete, "")
	if w.Code != http.StatusOK || view.Customized {
		t.Fatalf("status %d, view %+v", w.Code, view)
	}
	if !slices.Contains(view.Layout.Folders[0].Items, "duplicates") {
		t.Fatal("superuser default lost the admin items")
	}
}
