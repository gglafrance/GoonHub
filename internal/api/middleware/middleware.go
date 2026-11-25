package middleware

import (
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Setup(r *gin.Engine, logger *logging.Logger, allowedOrigins []string, environment string) {
	// Panic Recovery
	r.Use(gin.Recovery())

	// Gzip Compression (exclude video streaming — gzip sets Content-Encoding
	// eagerly which causes http.ServeContent to skip Range request handling,
	// breaking seeking in Firefox)
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPathsRegexs([]string{
		`/api/v1/scenes/\d+/stream`,
		// On-the-fly conversion: streamed and flushed chunk by chunk
		`/api/v1/scenes/\d+/transcode`,
		`/api/v1/shares/[^/]+/transcode`,
		// Remote workers: media served with Range, uploads without a read deadline
		`/api/v1/workers/`,
	})))

	// Security Headers
	r.Use(SecurityHeaders(environment))

	// Request ID
	r.Use(RequestID())

	// Structured Logger
	r.Use(Logger(logger))

	// CORS - validate origins at startup in production
	if environment == "production" {
		for _, origin := range allowedOrigins {
			if origin == "*" {
				logger.Warn("CORS wildcard origin is not recommended in production")
			}
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
}

// SecurityHeaders adds essential security headers to all responses.
func SecurityHeaders(environment string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// XSS protection (legacy browsers)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer policy - don't leak URLs to third parties
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy - disable unnecessary features
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// HSTS - only in production (requires HTTPS)
		if environment == "production" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content Security Policy - restrictive default
		// Allows self, inline styles (for Tailwind), Google Fonts, PornDB CDN, and Iconify API
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline'; " +
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
			"img-src 'self' data: blob: https://cdn.theporndb.net; " +
			"media-src 'self' blob:; " +
			"font-src 'self' https://fonts.gstatic.com data:; " +
			"connect-src 'self' https://api.iconify.design; " +
			"worker-src 'self' blob:; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		c.Header("Content-Security-Policy", csp)

		c.Next()
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.New().String()
		c.Set("RequestID", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func Logger(logger *logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logger.Error(e)
			}
		} else {
			logger.Info("Request",
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.Duration("latency", latency),
				zap.String("request_id", c.GetString("RequestID")),
			)
		}
	}
}

// AuthCookieName is the name of the HTTP-only auth cookie
const AuthCookieName = "goonhub_auth"

// AccessResolver resolves the live access (current role, effective permissions) of a user
// (implemented by core.RBACService).
type AccessResolver interface {
	ResolveUser(userID uint) (*core.UserAccess, error)
}

// contextAccessKey is the gin context key of the authenticated user's *core.UserAccess.
const contextAccessKey = "access"

// SetAccess stores the user's access in the context (done by AuthMiddleware; tests use it
// to stand in for it).
func SetAccess(c *gin.Context, access *core.UserAccess) {
	c.Set(contextAccessKey, access)
}

// GetAccess returns the authenticated user's access, nil when there is none.
func GetAccess(c *gin.Context) *core.UserAccess {
	v, ok := c.Get(contextAccessKey)
	if !ok {
		return nil
	}
	access, _ := v.(*core.UserAccess)
	return access
}

// HasPermission reports whether the authenticated user has the permission.
func HasPermission(c *gin.Context, permission string) bool {
	return GetAccess(c).Has(permission)
}

// AuthMiddleware authenticates the session token, then resolves the user's current role and
// effective permissions: the role stored in the token is not used for authorization. The
// token of a user that no longer exists is refused.
func AuthMiddleware(authService *core.AuthService, resolver AccessResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""

		// Try to get token from HTTP-only cookie first (preferred, more secure)
		if cookie, err := c.Cookie(AuthCookieName); err == nil && cookie != "" {
			token = cookie
		}

		// Fall back to Authorization header for backward compatibility
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				token = strings.TrimPrefix(authHeader, "Bearer ")
				if token == authHeader {
					token = "" // Not a Bearer token
				}
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		payload, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		access, err := resolver.ResolveUser(payload.UserID)
		if err != nil {
			if apperrors.IsNotFound(err) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve permissions"})
			}
			c.Abort()
			return
		}

		live := *payload
		live.Role = access.Role
		c.Set("user", &live)
		SetAccess(c, access)
		c.Next()
	}
}

// RequirePermission lets the request through only if the authenticated user has the
// permission (a superuser has them all).
func RequirePermission(permission string) gin.HandlerFunc {
	return RequireAnyPermission(permission)
}

// RequireAnyPermission lets the request through if the authenticated user has at least one
// of the permissions.
func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		access := GetAccess(c)
		if access == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}
		for _, p := range permissions {
			if access.Has(p) {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

func GetUserFromContext(c *gin.Context) (*core.UserPayload, error) {
	user, exists := c.Get("user")
	if !exists {
		return nil, fmt.Errorf("user not found in context")
	}

	userPayload, ok := user.(*core.UserPayload)
	if !ok {
		return nil, fmt.Errorf("invalid user data in context")
	}

	return userPayload, nil
}
