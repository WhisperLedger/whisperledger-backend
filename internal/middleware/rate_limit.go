package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

// RateLimitByIP is deliberately small and dependency-free. It is a first line
// of defence for public write endpoints; Cloud Run/WAF limits should still be
// configured at the edge for cross-instance protection.
func RateLimitByIP(limit int, window time.Duration) func(http.Handler) http.Handler {
	var (
		mu      sync.Mutex
		entries = make(map[string]rateLimitEntry)
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || ip == "" {
				ip = r.RemoteAddr
			}
			now := time.Now()

			mu.Lock()
			entry := entries[ip]
			if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= window {
				entry = rateLimitEntry{windowStart: now}
			}
			entry.count++
			entries[ip] = entry
			allowed := entry.count <= limit
			if len(entries) > 4096 {
				for key, candidate := range entries {
					if now.Sub(candidate.windowStart) >= window {
						delete(entries, key)
					}
				}
			}
			mu.Unlock()

			if !allowed {
				w.Header().Set("Retry-After", "3600")
				response := `{"success":false,"error":{"code":"RATE_LIMITED","message":"please try again later"}}`
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(response))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
