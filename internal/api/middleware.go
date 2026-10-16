package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CORSMiddleware struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
}

func NewCORSMiddleware() *CORSMiddleware {
	return &CORSMiddleware{
		allowedOrigins: []string{},
		allowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		allowedHeaders: []string{"Content-Type", "Authorization", "X-API-Key", "X-Requested-With"},
	}
}

func NewCORSMiddlewareWithOrigins(origins []string) *CORSMiddleware {
	return &CORSMiddleware{
		allowedOrigins: origins,
		allowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		allowedHeaders: []string{"Content-Type", "Authorization", "X-API-Key", "X-Requested-With"},
	}
}

func (m *CORSMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false

		for _, o := range m.allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", joinStrings(m.allowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", joinStrings(m.allowedHeaders, ", "))
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func joinStrings(strs []string, sep string) string {
	var sb strings.Builder
	for i, s := range strs {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(s)
	}
	return sb.String()
}

type TimeoutMiddleware struct {
	timeout time.Duration
}

func NewTimeoutMiddleware(timeoutSeconds int) *TimeoutMiddleware {
	return &TimeoutMiddleware{timeout: time.Duration(timeoutSeconds) * time.Second}
}

func (m *TimeoutMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type RateLimiter struct {
	requestsPerSecond int
	burst             int
	buckets           map[string]*tokenBucket
	mu                sync.RWMutex
	logger            *slog.Logger
}

type tokenBucket struct {
	capacity  int
	tokens    int
	lastCheck time.Time
	mu        sync.Mutex
}

func newTokenBucket(capacity, burst int) *tokenBucket {
	return &tokenBucket{
		capacity:  capacity,
		tokens:    burst,
		lastCheck: time.Now(),
	}
}

func (tb *tokenBucket) consume() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastCheck)
	tb.lastCheck = now

	refill := int(elapsed.Seconds() * float64(tb.capacity))
	if refill > 0 {
		tb.tokens += refill
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func NewRateLimiter(requestsPerSecond, burst int, logger *slog.Logger) *RateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RateLimiter{
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
		buckets:           make(map[string]*tokenBucket),
		logger:            logger,
	}
}

func (rl *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl.requestsPerSecond <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := getClientIP(r)
		bucket := rl.getBucket(clientIP)

		if !bucket.consume() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getBucket(ip string) *tokenBucket {
	rl.mu.RLock()
	bucket, ok := rl.buckets[ip]
	rl.mu.RUnlock()

	if ok {
		return bucket
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, ok := rl.buckets[ip]; ok {
		return bucket
	}

	bucket = newTokenBucket(rl.requestsPerSecond, rl.burst)
	rl.buckets[ip] = bucket
	return bucket
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return strings.TrimSpace(realIP)
	}
	return r.RemoteAddr
}

type APIKeyAuth struct {
	validKeys  map[string]bool
	headerName string
	logger     *slog.Logger
}

func NewAPIKeyAuth(apiKeys []string, headerName string, logger *slog.Logger) *APIKeyAuth {
	if logger == nil {
		logger = slog.Default()
	}
	if headerName == "" {
		headerName = "X-API-Key"
	}

	validKeys := make(map[string]bool)
	for _, key := range apiKeys {
		validKeys[key] = true
	}

	return &APIKeyAuth{
		validKeys:  validKeys,
		headerName: headerName,
		logger:     logger,
	}
}

func (a *APIKeyAuth) Wrap(next http.Handler) http.Handler {
	if len(a.validKeys) == 0 {
		a.logger.Warn("API key auth enabled but no keys configured, allowing all requests")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(a.headerName)
		if key == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if key == "" || !a.validKeys[key] {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid or missing API key"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

type RequestSizeLimit struct {
	maxBytes int64
	logger   *slog.Logger
}

func NewRequestSizeLimit(maxBytes int64, logger *slog.Logger) *RequestSizeLimit {
	if logger == nil {
		logger = slog.Default()
	}
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024
	}
	return &RequestSizeLimit{
		maxBytes: maxBytes,
		logger:   logger,
	}
}

func (rsl *RequestSizeLimit) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, rsl.maxBytes)
		next.ServeHTTP(w, r)
	})
}
