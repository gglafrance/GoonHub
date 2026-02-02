package middleware

import (
	"fmt"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Setup(r *gin.Engine, logger *logging.Logger, allowedOrigins []string, environment string) {
	// Panic Recovery
	r.Use(gin.Recovery())

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

func AuthMiddleware(authService *core.AuthService) gin.HandlerFunc {
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

		c.Set("user", payload)
		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userPayload, ok := user.(*core.UserPayload)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
			c.Abort()
			return
		}

		if userPayload.Role != role {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequirePermission(rbac *core.RBACService, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userPayload, ok := user.(*core.UserPayload)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
			c.Abort()
			return
		}

		if !rbac.HasPermission(userPayload.Role, permission) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
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
