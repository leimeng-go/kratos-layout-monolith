package ratelimit

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// Limiter is a simple token bucket rate limiter.
type Limiter struct {
	rate   float64
	burst  int
	tokens float64
	last   float64
	mu     sync.Mutex
}

// NewLimiter creates a new rate limiter.
func NewLimiter(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:  rate,
		burst: burst,
	}
}

// Allow checks if a request is allowed.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.tokens < 1 {
		l.tokens = 0
		return false
	}
	l.tokens--
	return true
}

// AddTokens adds tokens based on time elapsed.
func (l *Limiter) AddTokens(now float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	elapsed := now - l.last
	if elapsed > 0 {
		l.tokens += elapsed * l.rate
		if l.tokens > float64(l.burst) {
			l.tokens = float64(l.burst)
		}
		l.last = now
	}
}

// GlobalLimiter is the global rate limiter instance.
var GlobalLimiter = NewLimiter(100, 200)

// Server returns a rate limiting middleware.
func Server() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if !GlobalLimiter.Allow() {
				if w, ok := khttp.ResponseWriterFromServerContext(ctx); ok {
					w.Header().Set("X-RateLimit-Limit", "100")
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte("rate limit exceeded"))
				}
				return nil, nil
			}
			return next(ctx, req)
		}
	}
}
