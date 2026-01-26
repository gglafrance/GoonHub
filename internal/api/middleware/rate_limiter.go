package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

func (i *IPRateLimiter) CleanupOldEntries() {
	i.mu.Lock()
	defer i.mu.Unlock()

	for ip, limiter := range i.ips {
		// Check if limiter has recovered to full capacity (idle long enough)
		// Using Tokens() checks without consuming any tokens
		if limiter.Tokens() >= float64(i.b) {
			delete(i.ips, ip)
		}
	}
}

// cleanupRegistry tracks which limiters have cleanup goroutines running
// to prevent multiple goroutines per limiter instance
var (
	cleanupRegistry   = make(map[*IPRateLimiter]struct{})
	cleanupRegistryMu sync.Mutex
)

// startCleanup starts a single cleanup goroutine for this limiter if not already running
func startCleanup(limiter *IPRateLimiter, interval time.Duration) {
	cleanupRegistryMu.Lock()
	defer cleanupRegistryMu.Unlock()

	if _, exists := cleanupRegistry[limiter]; exists {
		return // Already has a cleanup goroutine
	}

	cleanupRegistry[limiter] = struct{}{}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			limiter.CleanupOldEntries()
		}
	}()
}

func RateLimitMiddleware(limiter *IPRateLimiter, logger *zap.Logger) gin.HandlerFunc {
	// Start cleanup goroutine only once per limiter instance
	startCleanup(limiter, 1*time.Minute)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.GetLimiter(ip).Allow() {
			logger.Warn("Rate limit exceeded", zap.String("ip", ip), zap.String("path", c.Request.URL.Path))
			c.JSON(429, gin.H{"error": "Too many requests"})
			c.Abort()
			return
		}
		c.Next()
	}
}
