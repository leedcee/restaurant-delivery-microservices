package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type rateBucket struct {
	started time.Time
	count   int
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string]rateBucket), now: time.Now}
}

func (l *rateLimiter) middleware(group string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := l.now()
			key := group + ":" + clientAddress(r)
			allowed, retryAfter := l.allow(key, limit, window, now)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Round(time.Second).Seconds()))))
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"type": "/problems/rate-limit", "title": "Too many requests", "status": http.StatusTooManyRequests,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, exists := l.buckets[key]
	if !exists || now.Sub(bucket.started) >= window {
		l.buckets[key] = rateBucket{started: now, count: 1}
		if len(l.buckets) > 10_000 {
			l.cleanup(now, window)
		}
		return true, 0
	}
	if bucket.count >= limit {
		return false, window - now.Sub(bucket.started)
	}
	bucket.count++
	l.buckets[key] = bucket
	return true, 0
}

func (l *rateLimiter) cleanup(now time.Time, window time.Duration) {
	for key, bucket := range l.buckets {
		if now.Sub(bucket.started) >= window {
			delete(l.buckets, key)
		}
	}
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
