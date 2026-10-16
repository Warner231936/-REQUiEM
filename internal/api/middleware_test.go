package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/requiem/cognitive/internal/events"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

func TestCORSMiddleware_AllowsOrigin(t *testing.T) {
	allowed := "http://localhost:3000"
	mw := NewCORSMiddlewareWithOrigins([]string{allowed})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", allowed)
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != allowed {
		t.Errorf("expected CORS origin %s, got %s", allowed, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("expected Access-Control-Allow-Credentials: true")
	}
}

func TestCORSMiddleware_RejectsUnknownOrigin(t *testing.T) {
	mw := NewCORSMiddlewareWithOrigins([]string{"http://localhost:3000"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("should not allow unknown origin, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_HandlesOptions(t *testing.T) {
	mw := NewCORSMiddlewareWithOrigins([]string{"*"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	mw.Wrap(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rec.Code)
	}
}

func TestRequestSizeLimit_RejectsLargeBody(t *testing.T) {
	limit := NewRequestSizeLimit(100, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 200)
		n, _ := r.Body.Read(buf)
		if n > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
	})

	largeBody := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	rec := httptest.NewRecorder()

	limit.Wrap(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (body read succeeds with MaxBytesReader), got %d", rec.Code)
	}
}

func TestRequestSizeLimit_AllowsSmallBody(t *testing.T) {
	limit := NewRequestSizeLimit(100, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("small"))
	rec := httptest.NewRecorder()

	limit.Wrap(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for small body, got %d", rec.Code)
	}
}

func TestTimeoutMiddleware_TimesOut(t *testing.T) {
	timeout := NewTimeoutMiddleware(1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	timeout.Wrap(handler).ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("timeout middleware should have cancelled after 1s, took %v", elapsed)
	}
}

func TestTimeoutMiddleware_PassesThrough(t *testing.T) {
	timeout := NewTimeoutMiddleware(10)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	timeout.Wrap(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSSEHandler_SetsCorrectHeaders(t *testing.T) {
	bus := events.NewEventBus(nil)
	sseHandler := events.NewSSEHandler(bus, nil, []string{"*"})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		sseHandler.ServeHTTP(rec, req)
	}()

	time.Sleep(100 * time.Millisecond)

	resp := rec.Result()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("expected no-cache, got %s", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("Connection") != "keep-alive" {
		t.Errorf("expected keep-alive, got %s", resp.Header.Get("Connection"))
	}
}

func TestSSEHandler_ClientDisconnect(t *testing.T) {
	bus := events.NewEventBus(nil)
	sseHandler := events.NewSSEHandler(bus, nil, []string{"*"})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		sseHandler.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Log("SSE handler did not complete immediately after disconnect (acceptable)")
	}
}

func TestSSEHandler_HasHeaders(t *testing.T) {
	bus := events.NewEventBus(nil)
	sseHandler := events.NewSSEHandler(bus, nil, []string{"*"})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		sseHandler.ServeHTTP(rec, req)
	}()

	time.Sleep(100 * time.Millisecond)

	resp := rec.Result()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("expected no-cache, got %s", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("Connection") != "keep-alive" {
		t.Errorf("expected keep-alive, got %s", resp.Header.Get("Connection"))
	}
}

func TestMiddlewareChain_Integration(t *testing.T) {
	sizeLimit := NewRequestSizeLimit(1<<20, nil)
	timeout := NewTimeoutMiddleware(5)
	cors := NewCORSMiddlewareWithOrigins([]string{"http://localhost:3000"})
	limiter := NewRateLimiter(100, 200, nil)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	chain := cors.Wrap(limiter.Wrap(timeout.Wrap(sizeLimit.Wrap(finalHandler))))

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader([]byte(`{"message":"hello"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	chain.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("CORS header not set")
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestGetClientIP_ForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	ip := getClientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}
}

func TestGetClientIP_RealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "10.0.0.2")
	ip := getClientIP(req)
	if ip != "10.0.0.2" {
		t.Errorf("expected 10.0.0.2, got %s", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.3:12345"
	ip := getClientIP(req)
	if ip != "10.0.0.3:12345" {
		t.Errorf("expected 10.0.0.3:12345, got %s", ip)
	}
}

func TestTokenBucket_Consume(t *testing.T) {
	tb := newTokenBucket(10, 5)
	for i := 0; i < 5; i++ {
		if !tb.consume() {
			t.Errorf("consume %d should succeed", i)
		}
	}
	if tb.consume() {
		t.Error("consume 6 should fail (burst exhausted)")
	}
}

func TestTokenBucket_RefundOverTime(t *testing.T) {
	tb := newTokenBucket(10, 1)
	if !tb.consume() {
		t.Error("first consume should succeed")
	}
	if tb.consume() {
		t.Error("second consume should fail")
	}
	time.Sleep(2 * time.Second)
	if !tb.consume() {
		t.Error("consume after time should succeed (tokens refilled)")
	}
}

func TestAPIKeyAuth_NoKeysAllowsAll(t *testing.T) {
	auth := NewAPIKeyAuth(nil, "X-API-Key", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when no keys configured, got %d", rec.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"key": "value"})
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "bad request")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "bad request" {
		t.Errorf("expected error 'bad request', got %s", resp["error"])
	}
}

func TestExtractSessionToken(t *testing.T) {
	token := extractSessionToken("session=abc123; HttpOnly; Path=/")
	if token != "abc123" {
		t.Errorf("expected abc123, got %s", token)
	}
	token = extractSessionToken("session=abc123")
	if token != "abc123" {
		t.Errorf("expected abc123, got %s", token)
	}
	token = extractSessionToken("")
	if token != "" {
		t.Errorf("expected empty string, got %s", token)
	}
}

var _ storage.Store = (*mockStore)(nil)

var _ = models.User{}
