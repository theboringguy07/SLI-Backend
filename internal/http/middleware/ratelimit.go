package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/errors"
)

// rateLimiter represents a simple token bucket rate limiter per IP.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
}

type visitor struct {
	tokens    int
	lastSeen  time.Time
}

// RateLimit returns a middleware that limits requests per minute per IP.
func RateLimit(limit int) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
	}

	// Background cleanup goroutine to prevent memory leaks
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			rl.mu.Lock()
			v, exists := rl.visitors[ip]
			if !exists || time.Since(v.lastSeen) > time.Minute {
				rl.visitors[ip] = &visitor{tokens: rl.limit, lastSeen: time.Now()}
				v = rl.visitors[ip]
			}
			
			v.lastSeen = time.Now()
			
			if v.tokens <= 0 {
				rl.mu.Unlock()
				appErr := errors.New(errors.CodeBadRequest, "Rate limit exceeded")
				response.Error(w, r, appErr)
				return
			}
			
			v.tokens--
			rl.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// clientIP returns a stable per-client key for rate limiting.
//
// r.RemoteAddr includes an ephemeral source port that changes on every new
// TCP connection, so using it directly (as this middleware used to) meant
// the same client almost never hit the same map key twice and the limiter
// was effectively a no-op. This strips the port and, if the app is running
// behind a trusted reverse proxy, prefers the original client IP forwarded
// in X-Forwarded-For/X-Real-IP over the proxy's own address.
//
// NOTE: these headers are trivially spoofable by a direct client if there is
// NOT a reverse proxy in front of this service stripping/overwriting them.
// Only rely on them when deployed behind a proxy/load balancer you control.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
