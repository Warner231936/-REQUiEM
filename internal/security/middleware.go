package security

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	apiKeyKey    contextKey = "api_key"
	sessionKey   contextKey = "session"
)

type Middleware struct {
	secrets         *SecretsStore
	logger          *slog.Logger
	corsOrigins     map[string]bool
	validateSession func(token string) bool
}

func (m *Middleware) SetSessionValidator(fn func(token string) bool) {
	m.validateSession = fn
}

func NewMiddleware(secrets *SecretsStore, logger *slog.Logger, allowedOrigins []string) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	origins := make(map[string]bool)
	if len(allowedOrigins) == 0 {
		origins["http://localhost:3000"] = true
		origins["http://127.0.0.1:3000"] = true
	} else {
		for _, o := range allowedOrigins {
			origins[o] = true
		}
	}

	return &Middleware{
		secrets:     secrets,
		logger:      logger,
		corsOrigins: origins,
	}
}

func (m *Middleware) RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		w.Header().Set(RequestIDHeader, requestID)

		next.ServeHTTP(w, r)
	})
}

func GetRequestID(r *http.Request) string {
	if v, ok := r.Context().Value(requestIDKey).(string); ok {
		return v
	}
	return r.Header.Get(RequestIDHeader)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	requestID  string
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (m *Middleware) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := GetRequestID(r)

		ww := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			requestID:      requestID,
		}

		next.ServeHTTP(ww, r)

		m.logger.Info("request completed",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", getClientIP(r),
			"user_agent", r.UserAgent(),
		)
	})
}

func (m *Middleware) HardenedCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && m.corsOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Requested-With, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline'; connect-src 'self' https://api.requiem.cfd; img-src 'self' data:; font-src 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) RequireAPIKey(requiredPerms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m.secrets == nil {
				http.Error(w, `{"error": "authentication not configured"}`, http.StatusInternalServerError)
				return
			}

			rawKey := extractAPIKey(r)
			if rawKey == "" {
				http.Error(w, `{"error": "missing API key"}`, http.StatusUnauthorized)
				return
			}

			key, ok := m.secrets.VerifyKey(rawKey)
			if !ok {
				m.logger.Warn("invalid API key attempt",
					"request_id", GetRequestID(r),
					"remote_addr", getClientIP(r),
					"path", r.URL.Path,
				)
				http.Error(w, `{"error": "invalid API key"}`, http.StatusUnauthorized)
				return
			}

			if !key.IsValid() {
				http.Error(w, `{"error": "API key expired or revoked"}`, http.StatusUnauthorized)
				return
			}

			for _, perm := range requiredPerms {
				if !key.HasPermission(perm) {
					m.logger.Warn("insufficient permissions",
						"request_id", GetRequestID(r),
						"key_id", key.ID,
						"required_perm", perm,
						"path", r.URL.Path,
					)
					http.Error(w, `{"error": "insufficient permissions"}`, http.StatusForbidden)
					return
				}
			}

			ctx := context.WithValue(r.Context(), apiKeyKey, key)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func GetAPIKeyFromContext(r *http.Request) *APIKey {
	if v, ok := r.Context().Value(apiKeyKey).(*APIKey); ok {
		return v
	}
	return nil
}

func extractAPIKey(r *http.Request) string {
	key := r.Header.Get("X-API-Key")
	if key != "" {
		return key
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return r.URL.Query().Get("api_key")
}

func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.secrets == nil {
			next.ServeHTTP(w, r)
			return
		}

		rawKey := extractAPIKey(r)
		if rawKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		if key, ok := m.secrets.VerifyKey(rawKey); ok && key.IsValid() {
			ctx := context.WithValue(r.Context(), apiKeyKey, key)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) RequireAuthOrSession(requiredPerms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m.secrets == nil {
				next.ServeHTTP(w, r)
				return
			}

			if key := extractAPIKey(r); key != "" {
				if validKey, ok := m.secrets.VerifyKey(key); ok && validKey.IsValid() {
					for _, perm := range requiredPerms {
						if !validKey.HasPermission(perm) {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusForbidden)
							json.NewEncoder(w).Encode(map[string]string{"error": "insufficient permissions"})
							return
						}
					}
					ctx := context.WithValue(r.Context(), apiKeyKey, validKey)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			sessionCookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, `{"error": "authentication required"}`, http.StatusUnauthorized)
				return
			}

			if m.validateSession == nil {
				m.logger.Warn("session validation not configured in RequireAuthOrSession")
				http.Error(w, `{"error": "session validation not configured"}`, http.StatusUnauthorized)
				return
			}

			m.logger.Info("session auth attempt", "cookie_present", sessionCookie.Value != "", "token_len", len(sessionCookie.Value))
			valid := m.validateSession(sessionCookie.Value)
			m.logger.Info("session validation result", "valid", valid)
			if !valid {
				http.Error(w, `{"error": "invalid or expired session"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

var ipCache = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

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

func CompareConstantTime(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
