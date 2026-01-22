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
		if limiter.AllowN(time.Now(), i.b) {
			delete(i.ips, ip)
		}
	}
}

func RateLimitMiddleware(limiter *IPRateLimiter, logger *zap.Logger) gin.HandlerFunc {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			limiter.CleanupOldEntries()
		}
	}()

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
