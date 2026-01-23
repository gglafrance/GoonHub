package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRateLimitedRouter(r rate.Limit, burst int) *gin.Engine {
	limiter := NewIPRateLimiter(r, burst)
	router := gin.New()
	router.Use(RateLimitMiddleware(limiter, zap.NewNop()))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	return router
}

func TestRateLimiter_AllowsWithinBurst(t *testing.T) {
	router := setupRateLimitedRouter(1, 5) // 1 req/s, burst 5

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimiter_BlocksOverBurst(t *testing.T) {
	router := setupRateLimitedRouter(1, 3) // 1 req/s, burst 3

	// Exhaust burst
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("burst request %d: expected 200, got %d", i, w.Code)
		}
	}

	// This should be blocked
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 429 {
		t.Fatalf("expected 429 after burst exhausted, got %d", w.Code)
	}
}

func TestRateLimiter_DifferentIPs_Independent(t *testing.T) {
	router := setupRateLimitedRouter(1, 2) // 1 req/s, burst 2

	// Exhaust IP1's burst
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// IP1 should be blocked
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Fatalf("expected IP1 blocked (429), got %d", w.Code)
	}

	// IP2 should still work
	req, _ = http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:5678"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected IP2 allowed (200), got %d", w.Code)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewIPRateLimiter(1000, 1000) // High limits to avoid blocking
	router := gin.New()
	router.Use(RateLimitMiddleware(limiter, zap.NewNop()))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// Just verifying no race/panic
		}(i)
	}
	wg.Wait()
}

func TestRateLimiter_RecoverAfterWait(t *testing.T) {
	// Use a very high rate so token refills within 1ms
	limiter := NewIPRateLimiter(10000, 1) // 10000 req/s, burst 1
	router := gin.New()
	router.Use(RateLimitMiddleware(limiter, zap.NewNop()))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request succeeds (burst=1)
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("first request: expected 200, got %d", w.Code)
	}

	// At 10000 req/s, a token refills every 0.1ms.
	// Sleep 5ms to guarantee at least 1 token is available.
	time.Sleep(5 * time.Millisecond)

	// After waiting, the limiter should have refilled and allow a new request
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("after recovery: expected 200, got %d", w2.Code)
	}
}
