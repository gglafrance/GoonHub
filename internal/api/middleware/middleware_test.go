package middleware

import (
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuthService(t *testing.T) (*core.AuthService, *mocks.MockUserRepository, *mocks.MockRevokedTokenRepository) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	key := "01234567890123456789012345678901"
	// Lockout: 5 attempts, 15 minute duration
	svc, err := core.NewAuthService(userRepo, revokedRepo, key, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}
	return svc, userRepo, revokedRepo
}

func hashForTest(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	return string(h)
}

func TestAuthMiddleware_NoHeader(t *testing.T) {
	authSvc, _, _ := newTestAuthService(t)

	router := gin.New()
	router.Use(AuthMiddleware(authSvc, fakeResolver{}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 for missing auth header, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoBearerPrefix(t *testing.T) {
	authSvc, _, _ := newTestAuthService(t)

	router := gin.New()
	router.Use(AuthMiddleware(authSvc, fakeResolver{}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 for non-Bearer auth, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	authSvc, _, revokedRepo := newTestAuthService(t)

	revokedRepo.EXPECT().IsRevoked(gomock.Any()).Return(false, nil)

	router := gin.New()
	router.Use(AuthMiddleware(authSvc, fakeResolver{}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-data")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 for invalid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken_SetsContext(t *testing.T) {
	authSvc, userRepo, revokedRepo := newTestAuthService(t)

	hashed := hashForTest(t, "testpass")
	user := &data.User{ID: 42, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(42)).Return(nil)

	token, _, err := authSvc.Login("alice", "testpass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	revokedRepo.EXPECT().IsRevoked(gomock.Any()).Return(false, nil)

	// the role in the token is admin, the user's current role is user
	var capturedPayload *core.UserPayload
	var capturedAccess *core.UserAccess
	router := gin.New()
	router.Use(AuthMiddleware(authSvc, fakeResolver{42: core.NewUserAccess(42, "user", false, "scenes:view")}))
	router.GET("/protected", func(c *gin.Context) {
		user, _ := c.Get("user")
		capturedPayload = user.(*core.UserPayload)
		capturedAccess = GetAccess(c)
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for valid token, got %d", w.Code)
	}
	if capturedPayload == nil {
		t.Fatal("expected user payload in context")
	}
	if capturedPayload.UserID != 42 {
		t.Fatalf("expected UserID 42, got %d", capturedPayload.UserID)
	}
	if capturedPayload.Role != "user" {
		t.Fatalf("expected the live role user, got %s", capturedPayload.Role)
	}
	if capturedAccess == nil || !capturedAccess.Has("scenes:view") || capturedAccess.Has("users:manage") {
		t.Fatalf("expected the resolved access in the context, got %+v", capturedAccess)
	}
}

// fakeResolver resolves the users it knows, NotFound for the others (deleted users).
type fakeResolver map[uint]*core.UserAccess

func (f fakeResolver) ResolveUser(userID uint) (*core.UserAccess, error) {
	if a, ok := f[userID]; ok {
		return a, nil
	}
	return nil, apperrors.NewNotFoundError("user", userID)
}

func TestAuthMiddleware_DeletedUser(t *testing.T) {
	authSvc, userRepo, revokedRepo := newTestAuthService(t)

	user := &data.User{ID: 7, Username: "bob", Password: hashForTest(t, "testpass"), Role: "user"}
	userRepo.EXPECT().GetByUsername("bob").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(7)).Return(nil)
	token, _, err := authSvc.Login("bob", "testpass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	revokedRepo.EXPECT().IsRevoked(gomock.Any()).Return(false, nil)

	router := gin.New()
	router.Use(AuthMiddleware(authSvc, fakeResolver{}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 for the token of a deleted user, got %d", w.Code)
	}
}

func serveWithAccess(access *core.UserAccess, guard gin.HandlerFunc) int {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if access != nil {
			c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
			SetAccess(c, access)
		}
		c.Next()
	})
	router.Use(guard)
	router.GET("/x", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	req, _ := http.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func TestRequirePermission(t *testing.T) {
	viewer := core.NewUserAccess(1, "user", false, "scenes:view", "files:view_size")
	admin := core.NewUserAccess(2, "admin", true)
	tests := []struct {
		name   string
		access *core.UserAccess
		guard  gin.HandlerFunc
		want   int
	}{
		{"has the permission", viewer, RequirePermission("scenes:view"), 200},
		{"lacks the permission", viewer, RequirePermission("scenes:delete"), 403},
		{"superuser passes", admin, RequirePermission("roles:manage"), 200},
		{"not authenticated", nil, RequirePermission("scenes:view"), 401},
		{"any: one of them", viewer, RequireAnyPermission("roles:manage", "files:view_size"), 200},
		{"any: none of them", viewer, RequireAnyPermission("roles:manage", "users:manage"), 403},
		{"any: superuser", admin, RequireAnyPermission("roles:manage", "users:manage"), 200},
		{"any: not authenticated", nil, RequireAnyPermission("roles:manage"), 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := serveWithAccess(tc.access, tc.guard); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestHasPermission_FromContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if HasPermission(c, "scenes:view") {
		t.Fatal("no access in the context: no permission")
	}
	SetAccess(c, core.NewUserAccess(1, "user", false, "scenes:view"))
	if !HasPermission(c, "scenes:view") || HasPermission(c, "scenes:edit") {
		t.Fatal("unexpected permissions from the context")
	}
}
