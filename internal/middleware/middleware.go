package middleware

import (
	"github.com/example/ozmap-user-manager/internal/auth"
	"github.com/example/ozmap-user-manager/internal/config"
	"net/http"
	"sync"
	"time"
)

func RequireAuth(s *auth.SessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.Valid(r); !ok {
			if r.URL.Path != "/" && len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
func CSRF(s *auth.SessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && !s.CheckCSRF(r) {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

type limiter struct {
	mu     sync.Mutex
	m      map[string][]time.Time
	window time.Duration
	max    int
}

func RateLimit(next http.Handler, c config.Config) http.Handler {
	l := &limiter{m: map[string][]time.Time{}, window: time.Minute, max: 60}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && (r.URL.Path == "/login" || r.URL.Path == "/api/upload" || r.URL.Path == "/api/create" || r.URL.Path == "/api/roles") {
			ip := r.RemoteAddr
			l.mu.Lock()
			now := time.Now()
			old := l.m[ip][:0]
			for _, t := range l.m[ip] {
				if now.Sub(t) < l.window {
					old = append(old, t)
				}
			}
			if len(old) >= l.max {
				l.mu.Unlock()
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			l.m[ip] = append(old, now)
			l.mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}
