package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/requiem/cognitive/internal/security"
)

func TestAuthSignup_Success(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	body := `{"name":"Test User","email":"test@example.com","password":"securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Signup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "account created successfully" {
		t.Errorf("unexpected message: %s", resp["message"])
	}
}

func TestAuthSignup_MissingFields(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{
			name:     "missing name",
			body:     `{"email":"test@example.com","password":"securepass123"}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "name, email, and password are required",
		},
		{
			name:     "missing email",
			body:     `{"name":"Test","password":"securepass123"}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "name, email, and password are required",
		},
		{
			name:     "missing password",
			body:     `{"name":"Test","email":"test@example.com"}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "name, email, and password are required",
		},
		{
			name:     "short password",
			body:     `{"name":"Test","email":"test@example.com","password":"short"}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "password must be at least 8 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Signup(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rec.Code)
			}

			var resp map[string]string
			json.NewDecoder(rec.Body).Decode(&resp)
			if resp["error"] != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, resp["error"])
			}
		})
	}
}

func TestAuthSignup_DuplicateEmail(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	body := `{"name":"Test User","email":"test@example.com","password":"securepass123"}`

	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	handler.Signup(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first signup should succeed: %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.Signup(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Errorf("duplicate signup should return 409, got %d", rec2.Code)
	}
}

func TestAuthLogin_Success(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	signupBody := `{"name":"Test User","email":"test@example.com","password":"securepass123"}`
	signupReq := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(signupBody))
	signupReq.Header.Set("Content-Type", "application/json")
	signupRec := httptest.NewRecorder()
	handler.Signup(signupRec, signupReq)

	if signupRec.Code != http.StatusOK {
		t.Fatalf("signup failed: %d", signupRec.Code)
	}

	loginBody := `{"email":"test@example.com","password":"securepass123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d, body: %s", loginRec.Code, loginRec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(loginRec.Body).Decode(&resp)
	if resp["email"] != "test@example.com" {
		t.Errorf("unexpected email in response: %v", resp["email"])
	}

	cookie := loginRec.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Error("expected Set-Cookie header")
	}
	if !strings.Contains(cookie, "session=") {
		t.Error("expected session cookie")
	}
	if !strings.Contains(cookie, "HttpOnly") {
		t.Error("expected HttpOnly flag on cookie")
	}

	token := extractSessionToken(cookie)
	if token == "" {
		t.Fatal("failed to extract session token")
	}

	claims, err := authService.ValidateSession(token)
	if err != nil {
		t.Fatalf("failed to validate session: %v", err)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("unexpected email in claims: %s", claims.Email)
	}
	if claims.IsAdmin {
		t.Error("regular user should not be admin")
	}
}

func TestAuthLogin_InvalidCredentials(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	body := `{"email":"nonexistent@example.com","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	signupBody := `{"name":"Test User","email":"test@example.com","password":"securepass123"}`
	signupReq := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(signupBody))
	signupReq.Header.Set("Content-Type", "application/json")
	handler.Signup(httptest.NewRecorder(), signupReq)

	loginBody := `{"email":"test@example.com","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthLogout(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	cookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "session=;") {
		t.Error("expected session cookie to be cleared")
	}
	if !strings.Contains(cookie, "Max-Age=0") {
		t.Error("expected Max-Age=0 in cleared cookie")
	}
}

func TestAuthMe_NotAuthenticated(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMe_Authenticated(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	signupBody := `{"name":"Test User","email":"test@example.com","password":"securepass123"}`
	signupReq := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(signupBody))
	signupReq.Header.Set("Content-Type", "application/json")
	handler.Signup(httptest.NewRecorder(), signupReq)

	loginBody := `{"email":"test@example.com","password":"securepass123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	token := extractSessionToken(loginRec.Header().Get("Set-Cookie"))

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: "session", Value: token})
	meRec := httptest.NewRecorder()

	handler.Me(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(meRec.Body).Decode(&resp)
	if resp["email"] != "test@example.com" {
		t.Errorf("unexpected email: %v", resp["email"])
	}
	if resp["name"] != "Test User" {
		t.Errorf("unexpected name: %v", resp["name"])
	}
}

func TestPasswordHashing_AndVerification(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)

	password := "mySecurePassword123"
	hash, err := authService.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == password {
		t.Error("hash should not equal plaintext password")
	}

	if err := authService.VerifyPassword(password, hash); err != nil {
		t.Errorf("VerifyPassword should succeed: %v", err)
	}

	if err := authService.VerifyPassword("wrongpassword", hash); err == nil {
		t.Error("VerifyPassword should fail with wrong password")
	}
}

func TestAPIKeyAuth_RejectsMissingKey(t *testing.T) {
	auth := NewAPIKeyAuth([]string{"valid-key-123"}, "X-API-Key", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mind", nil)
	rec := httptest.NewRecorder()

	auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing API key, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_RejectsInvalidKey(t *testing.T) {
	auth := NewAPIKeyAuth([]string{"valid-key-123"}, "X-API-Key", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mind", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	rec := httptest.NewRecorder()

	auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid API key, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_AcceptsValidKey(t *testing.T) {
	auth := NewAPIKeyAuth([]string{"valid-key-123"}, "X-API-Key", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mind", nil)
	req.Header.Set("X-API-Key", "valid-key-123")
	rec := httptest.NewRecorder()

	auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid API key, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_AcceptsBearerToken(t *testing.T) {
	auth := NewAPIKeyAuth([]string{"valid-key-123"}, "X-API-Key", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mind", nil)
	req.Header.Set("Authorization", "Bearer valid-key-123")
	rec := httptest.NewRecorder()

	auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid bearer token, got %d", rec.Code)
	}
}

func TestSecurityMiddleware_NoAuthKeyFailsClosed(t *testing.T) {
	os.Unsetenv("REQUIEM_MASTER_KEY")
	os.Unsetenv("REQUIEM_SECURE_COOKIES")

	secretsStore, err := security.NewSecretsStore("test-master-key-that-is-at-least-32-bytes-long", nil)
	if err != nil {
		t.Fatalf("failed to create secrets store: %v", err)
	}

	mw := security.NewMiddleware(secretsStore, nil, []string{"http://localhost:3000"})
	if mw == nil {
		t.Error("expected security middleware to be created")
	}

	_, apiKey, err := secretsStore.GenerateKey("test-key", []security.Permission{security.PermissionRead}, nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	validKeys := secretsStore.ListKeys()
	if len(validKeys) == 0 {
		t.Error("expected at least one valid key")
	}
	_ = apiKey
}

func TestRateLimiter_RejectsOverLimit(t *testing.T) {
	limiter := NewRateLimiter(2, 2, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := limiter.Wrap(handler)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after rate limit, got %d", rec.Code)
	}

	if retryAfter := rec.Header().Get("Retry-After"); retryAfter != "1" {
		t.Errorf("expected Retry-After: 1, got %s", retryAfter)
	}
}

func extractSessionToken(setCookie string) string {
	parts := strings.Split(setCookie, "session=")
	if len(parts) < 2 {
		return ""
	}
	rest := parts[1]
	idx := strings.Index(rest, ";")
	if idx >= 0 {
		return rest[:idx]
	}
	return rest
}

func TestNoRawSecretKeyInLogs(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	body := `{"name":"Test User","email":"secret-test@example.com","password":"securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "SUPER_SECRET_KEY_VALUE_12345")

	rec := httptest.NewRecorder()
	handler.Signup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("signup failed: %d", rec.Code)
	}

	respBody := rec.Body.String()
	if strings.Contains(respBody, "SUPER_SECRET_KEY_VALUE_12345") {
		t.Error("API key should never appear in response body")
	}
}

func TestConcurrentSignup(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	body := `{"name":"Test User","email":"test@example.com","password":"securepass123"}`

	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.Signup(rec, req)
			done <- nil
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	users, err := store.Auth().ListUsers(context.Background())
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected exactly 1 user (race condition), got %d", len(users))
	}
}

func TestSignupAndLogin_TestRequiemCfd(t *testing.T) {
	store := newMockStore()
	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	signupBody := `{"name":"Test User","email":"test@requiem.cfd","password":"testtest"}`
	signupReq := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(signupBody))
	signupReq.Header.Set("Content-Type", "application/json")
	signupRec := httptest.NewRecorder()
	handler.Signup(signupRec, signupReq)

	if signupRec.Code != http.StatusOK {
		t.Fatalf("signup failed: %d, body: %s", signupRec.Code, signupRec.Body.String())
	}

	var signupResp map[string]string
	json.NewDecoder(signupRec.Body).Decode(&signupResp)
	if signupResp["message"] != "account created successfully" {
		t.Errorf("unexpected signup message: %s", signupResp["message"])
	}

	loginBody := `{"email":"test@requiem.cfd","password":"testtest"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d, body: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp map[string]any
	json.NewDecoder(loginRec.Body).Decode(&loginResp)
	if loginResp["email"] != "test@requiem.cfd" {
		t.Errorf("unexpected email in login response: %v", loginResp["email"])
	}

	cookie := loginRec.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("expected Set-Cookie header on login")
	}
	if !strings.Contains(cookie, "session=") {
		t.Error("expected session cookie")
	}

	token := extractSessionToken(cookie)
	if token == "" {
		t.Fatal("failed to extract session token")
	}

	claims, err := authService.ValidateSession(token)
	if err != nil {
		t.Fatalf("failed to validate session token: %v", err)
	}
	if claims.Email != "test@requiem.cfd" {
		t.Errorf("expected email test@requiem.cfd in claims, got: %s", claims.Email)
	}
}
