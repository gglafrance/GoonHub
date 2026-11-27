package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

func performPermissionsRequest(t *testing.T, access *core.UserAccess) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if access != nil {
		r.Use(func(c *gin.Context) {
			c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
			middleware.SetAccess(c, access)
		})
	}
	r.GET("/auth/permissions", MyPermissions())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/permissions", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestMyPermissions_ReturnsEffectivePermissions(t *testing.T) {
	w := performPermissionsRequest(t, core.NewUserAccess(3, "moderator", false, "scenes:view", "scenes:upload", "scenes:trash"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body MyPermissionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Role != "moderator" || body.Superuser {
		t.Fatalf("unexpected role %q superuser %v", body.Role, body.Superuser)
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

func TestMyPermissions_Superuser(t *testing.T) {
	w := performPermissionsRequest(t, core.NewUserAccess(1, "admin", true, "scenes:view"))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if string(raw["superuser"]) != "true" || string(raw["role"]) != `"admin"` {
		t.Fatalf("unexpected body %s", w.Body.String())
	}
}

func TestMyPermissions_NoPermissionReturnsEmptyList(t *testing.T) {
	w := performPermissionsRequest(t, core.NewUserAccess(4, "ghost", false))

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
	w := performPermissionsRequest(t, nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
