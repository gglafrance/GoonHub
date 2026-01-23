package middleware

import (
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
	svc := core.NewAuthService(userRepo, revokedRepo, key, 24*time.Hour, zap.NewNop())
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
	router.Use(AuthMiddleware(authSvc))
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
	router.Use(AuthMiddleware(authSvc))
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
	router.Use(AuthMiddleware(authSvc))
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

	var capturedPayload *core.UserPayload
	router := gin.New()
	router.Use(AuthMiddleware(authSvc))
	router.GET("/protected", func(c *gin.Context) {
		user, _ := c.Get("user")
		capturedPayload = user.(*core.UserPayload)
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
	if capturedPayload.Role != "admin" {
		t.Fatalf("expected role admin, got %s", capturedPayload.Role)
	}
}

func TestRequireRole_Correct(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "admin"})
		c.Next()
	})
	router.Use(RequireRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for correct role, got %d", w.Code)
	}
}

func TestRequireRole_Wrong(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "viewer"})
		c.Next()
	})
	router.Use(RequireRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("expected 403 for wrong role, got %d", w.Code)
	}
}

func TestRequirePermission_Has(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"admin": {"videos.upload"},
	}, nil)
	rbac, _ := core.NewRBACService(roleRepo, permRepo, zap.NewNop())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "admin"})
		c.Next()
	})
	router.Use(RequirePermission(rbac, "videos.upload"))
	router.GET("/upload", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/upload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 when permission exists, got %d", w.Code)
	}
}

func TestRequirePermission_Lacks(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"viewer": {"videos.view"},
	}, nil)
	rbac, _ := core.NewRBACService(roleRepo, permRepo, zap.NewNop())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "viewer"})
		c.Next()
	})
	router.Use(RequirePermission(rbac, "videos.delete"))
	router.GET("/delete", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/delete", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("expected 403 when permission lacking, got %d", w.Code)
	}
}
