package middleware

// Rate limiter fixed-window per alamat IP (temuan DAST: brute-force login
// tidak dibatasi). In-memory per-instance — untuk deployment multi-instance
// ganti backend-nya dengan Redis (terlihat pola yang sama dengan events bus).

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"caupocket-api/pkg/response"
)

type rateWindow struct {
	count int
	reset time.Time
}

type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateWindow
	limit   int
	window  time.Duration
}

func (l *ipRateLimiter) allow(key string) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.entries[key]
	if !ok || now.After(w.reset) {
		l.entries[key] = &rateWindow{count: 1, reset: now.Add(l.window)}
		return true, 0
	}
	if w.count >= l.limit {
		return false, time.Until(w.reset)
	}
	w.count++
	return true, 0
}

// RateLimit membatasi max request per IP dalam window. Melebihi kuota → 429
// RATE_LIMITED + header Retry-After.
func RateLimit(max int, window time.Duration) gin.HandlerFunc {
	limiter := &ipRateLimiter{entries: make(map[string]*rateWindow), limit: max, window: window}
	return func(c *gin.Context) {
		allowed, retryAfter := limiter.allow(c.ClientIP())
		if !allowed {
			c.Header("Retry-After", ceilSeconds(retryAfter))
			response.Error(c, http.StatusTooManyRequests,
				"Terlalu banyak permintaan, coba lagi beberapa saat.", response.ErrCodeRateLimited)
			c.Abort()
			return
		}
		c.Next()
	}
}

func ceilSeconds(d time.Duration) string {
	if d <= 0 {
		return "1"
	}
	return strconv.Itoa(int((d + time.Second - 1) / time.Second))
}
