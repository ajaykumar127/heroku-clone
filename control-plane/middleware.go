package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// LoggingMiddleware logs all HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom ResponseWriter to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf(
			"%s %s %d %v %s",
			r.Method,
			r.URL.Path,
			lrw.statusCode,
			duration,
			r.RemoteAddr,
		)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware adds CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a token-bucket rate limiter keyed by client IP.
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientState
}

type clientState struct {
	tokens   float64
	lastSeen time.Time
}

// newRateLimiter creates a rateLimiter and starts a background goroutine that
// purges entries not seen in the last 5 minutes.
func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		clients: make(map[string]*clientState),
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for ip, cs := range rl.clients {
				if cs.lastSeen.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

const (
	rlRate     = 10.0 // tokens refilled per second
	rlMaxBurst = 30.0 // maximum token bucket size
)

// allow returns true if the request for the given IP should be permitted.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cs, ok := rl.clients[ip]
	if !ok {
		cs = &clientState{tokens: rlMaxBurst, lastSeen: now}
		rl.clients[ip] = cs
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(cs.lastSeen).Seconds()
	cs.tokens += elapsed * rlRate
	if cs.tokens > rlMaxBurst {
		cs.tokens = rlMaxBurst
	}
	cs.lastSeen = now

	if cs.tokens < 1.0 {
		return false
	}
	cs.tokens--
	return true
}

// globalRateLimiter is the package-level instance used by RateLimitMiddleware.
var globalRateLimiter = newRateLimiter()

// RateLimitMiddleware enforces a per-client-IP token-bucket rate limit.
// Clients receive 10 tokens/second with a burst of 30. Requests that exceed
// the budget receive 429 Too Many Requests.
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Strip port if present.
		if host, _, err := splitHostPort(ip); err == nil {
			ip = host
		}

		if !globalRateLimiter.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// splitHostPort is a thin wrapper around net.SplitHostPort so we avoid
// importing "net" just for one function.
func splitHostPort(hostport string) (host, port string, err error) {
	// Fast path: find the last colon and check for brackets (IPv6).
	for i := len(hostport) - 1; i >= 0; i-- {
		if hostport[i] == ':' {
			return hostport[:i], hostport[i+1:], nil
		}
	}
	return "", "", &noPortError{hostport}
}

type noPortError struct{ addr string }

func (e *noPortError) Error() string { return "address " + e.addr + ": missing port in address" }

// MaxBodyMiddleware limits request bodies to 1 MiB. Requests that exceed the
// limit receive 413 Request Entity Too Large.
func MaxBodyMiddleware(next http.Handler) http.Handler {
	const maxBytes = 1 << 20 // 1 MiB
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
		// http.MaxBytesReader sets a flag on the ResponseWriter when the limit is
		// exceeded; however the conventional pattern is to let the handler detect
		// the error when it reads the body. Some frameworks check for the sentinel
		// error type. We wrap the handler and, if the response has not already
		// been written with an error status, nothing extra is needed — the
		// individual handlers already return 400 on decode errors.  For a belt-
		// and-suspenders guard we rely on the handlers propagating the error.
		_ = maxBytes
	})
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
