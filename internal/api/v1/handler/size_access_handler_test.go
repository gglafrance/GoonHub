package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goonhub/internal/api/middleware"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// withTestAccess authenticates the requests as user 1 holding the permissions.
func withTestAccess(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "x"})
		middleware.SetAccess(c, core.NewUserAccess(1, "x", false, perms...))
		c.Next()
	}
}

func sizeTestRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSavedSearch_SizeFiltersNeedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	min := 100
	stored := []data.SavedSearch{{ID: 1, UserID: 1, Name: "big", Filters: data.Filters{Query: "x", MinSize: &min, Sort: "size_desc"}}}

	for _, tc := range []struct {
		name  string
		perms []string
		body  string
		want  int
	}{
		{"min_size refused", nil, `{"name":"a","filters":{"min_size":10}}`, http.StatusForbidden},
		{"size sort refused", nil, `{"name":"a","filters":{"sort":"size_asc"}}`, http.StatusForbidden},
		{"other filters accepted", nil, `{"name":"a","filters":{"sort":"title_asc"}}`, http.StatusCreated},
		{"size accepted with files:view_size", []string{core.PermFilesViewSize}, `{"name":"a","filters":{"min_size":10}}`, http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockSavedSearchRepository(ctrl)
			repo.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
			h := NewSavedSearchHandler(core.NewSavedSearchService(repo, zap.NewNop()))
			r := gin.New()
			r.Use(withTestAccess(tc.perms...))
			r.POST("/saved-searches", h.Create)
			if w := sizeTestRequest(r, http.MethodPost, "/saved-searches", tc.body); w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}

	t.Run("stored size filters are hidden", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockSavedSearchRepository(ctrl)
		repo.EXPECT().ListByUserID(uint(1)).Return(stored, nil)
		h := NewSavedSearchHandler(core.NewSavedSearchService(repo, zap.NewNop()))
		r := gin.New()
		r.Use(withTestAccess())
		r.GET("/saved-searches", h.List)
		w := sizeTestRequest(r, http.MethodGet, "/saved-searches", "")
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "min_size") || strings.Contains(w.Body.String(), "size_desc") {
			t.Fatalf("size filters must be hidden: %s", w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"query":"x"`) {
			t.Fatalf("other filters stay: %s", w.Body.String())
		}
	})
}

func TestSettings_SizeSortsNeedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	h := NewSettingsHandler(core.NewSettingsService(settingsRepo, mocks.NewMockUserRepository(ctrl), zap.NewNop()), 100)
	r := gin.New()
	r.Use(withTestAccess())
	r.GET("/settings", h.GetSettings)
	r.PATCH("/settings", h.PatchSettings)

	for _, body := range []string{
		`{"default_sort_order":"size_desc"}`,
		`{"sort_preferences":{"actors":"name_asc","studios":"name_asc","markers":"label_asc","actor_scenes":"size_asc","studio_scenes":""}}`,
	} {
		if w := sizeTestRequest(r, http.MethodPatch, "/settings", body); w.Code != http.StatusForbidden {
			t.Fatalf("%s: got %d, want 403: %s", body, w.Code, w.Body.String())
		}
	}

	stored := existingSettings()
	stored.DefaultSortOrder = "size_desc"
	stored.SortPreferences.StudioScenes = "size_asc"
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(stored, nil)
	w := sizeTestRequest(r, http.MethodGet, "/settings", "")
	var got data.UserSettings
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if got.DefaultSortOrder != core.DefaultSceneSort || got.SortPreferences.StudioScenes != "" {
		t.Fatalf("stored size sorts must fall back to the default: %s", w.Body.String())
	}
}

// fakeAccessResolver resolves the users it knows, NotFound for the others (deleted users).
type fakeAccessResolver map[uint]*core.UserAccess

func (f fakeAccessResolver) ResolveUser(userID uint) (*core.UserAccess, error) {
	if a, ok := f[userID]; ok {
		return a, nil
	}
	return nil, apperrors.NewNotFoundError("user", userID)
}

func TestShareHandler_DeletedUserIsNotAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)
	auth, err := core.NewAuthService(userRepo, revokedRepo, "01234567890123456789012345678901", time.Hour, 5, time.Minute, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	userRepo.EXPECT().GetByUsername("bob").Return(&data.User{ID: 7, Username: "bob", Password: string(hash), Role: "user"}, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(7)).Return(nil)
	token, _, err := auth.Login("bob", "pw")
	if err != nil {
		t.Fatal(err)
	}
	revokedRepo.EXPECT().IsRevoked(gomock.Any()).Return(false, nil).AnyTimes()

	check := func(resolver fakeAccessResolver) bool {
		h := &ShareHandler{AuthService: auth, Access: resolver}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		return h.isAuthenticated(c)
	}
	if !check(fakeAccessResolver{7: core.NewUserAccess(7, "user", false)}) {
		t.Fatal("an existing user is authenticated")
	}
	if check(fakeAccessResolver{}) {
		t.Fatal("the token of a deleted user must not count as authenticated")
	}
}

func TestSSECurrentAccess_DeletedUser(t *testing.T) {
	h := &SSEHandler{access: fakeAccessResolver{2: core.NewUserAccess(2, "user", false)}}
	if a, gone := h.currentAccess(2); a == nil || gone {
		t.Fatal("existing user resolved")
	}
	if a, gone := h.currentAccess(9); a != nil || !gone {
		t.Fatal("a deleted user closes the stream")
	}
}
