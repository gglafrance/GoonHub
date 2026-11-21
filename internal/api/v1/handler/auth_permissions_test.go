package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// The RBAC service is the provider wired in the routes.
var _ RolePermissionsProvider = (*core.RBACService)(nil)

func newPermissionsTestRBAC(t *testing.T) *core.RBACService {
	t.Helper()
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)
	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"moderator": {"scenes:view", "scenes:upload", "scenes:trash"},
		"user":      {"scenes:view"},
	}, nil)
	svc, err := core.NewRBACService(roleRepo, permRepo, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create RBAC service: %v", err)
	}
	return svc
}

func performPermissionsRequest(t *testing.T, provider RolePermissionsProvider, payload *core.UserPayload) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if payload != nil {
		r.Use(func(c *gin.Context) {
			c.Set("user", payload)
		})
	}
	r.GET("/auth/permissions", MyPermissions(provider))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/permissions", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestMyPermissions_ReturnsRolePermissions(t *testing.T) {
	rbac := newPermissionsTestRBAC(t)
	w := performPermissionsRequest(t, rbac, &core.UserPayload{UserID: 3, Username: "mod", Role: "moderator"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body MyPermissionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Role != "moderator" {
		t.Fatalf("expected role moderator, got %q", body.Role)
	}
	want := []string{"scenes:trash", "scenes:upload", "scenes:view"}
	if len(body.Permissions) != len(want) {
		t.Fatalf("expected %v, got %v", want, body.Permissions)
	}
	for i := range want {
		if body.Permissions[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, body.Permissions)
		}
	}
}

func TestMyPermissions_UnknownRoleReturnsEmptyList(t *testing.T) {
	rbac := newPermissionsTestRBAC(t)
	w := performPermissionsRequest(t, rbac, &core.UserPayload{UserID: 4, Username: "x", Role: "ghost"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// An empty JSON array, not null, so the client can use it directly
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if string(raw["permissions"]) != "[]" {
		t.Fatalf("expected permissions [], got %s", raw["permissions"])
	}
}

func TestMyPermissions_WithoutUserIsUnauthorized(t *testing.T) {
	rbac := newPermissionsTestRBAC(t)
	w := performPermissionsRequest(t, rbac, nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
