package httpkit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	clients map[string]*rateCounter
}
type rateCounter struct {
	started time.Time
	count   int
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, clients: make(map[string]*rateCounter)}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Forwarded-For")
		if key == "" {
			key = r.RemoteAddr
		} else {
			key = strings.TrimSpace(strings.Split(key, ",")[0])
		}
		if host, _, err := net.SplitHostPort(key); err == nil {
			key = host
		}
		l.mu.Lock()
		now := time.Now()
		for client, value := range l.clients {
			if now.Sub(value.started) >= l.window {
				delete(l.clients, client)
			}
		}
		counter := l.clients[key]
		if counter == nil || now.Sub(counter.started) >= l.window {
			counter = &rateCounter{started: now}
			l.clients[key] = counter
		}
		counter.count++
		allowed := counter.count <= l.limit
		l.mu.Unlock()
		if !allowed {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
