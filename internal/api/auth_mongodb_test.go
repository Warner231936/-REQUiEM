package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"github.com/requiem/cognitive/internal/security"
	"github.com/requiem/cognitive/internal/storage"
)

func TestSignupAndLogin_RealMongoDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB integration test in short mode")
	}

	ctx := context.Background()

	store, err := storage.NewMongoStore(ctx, storage.MongoConfig{
		URI:      "mongodb://127.0.0.1:27017",
		Database: "requiem_auth_test",
	}, nil)
	if err != nil {
		t.Skipf("Skipping - MongoDB not available: %v", err)
	}
	defer store.Close(ctx)

	if err := store.Database().RunCommand(ctx, bson.D{{Key: "dropDatabase", Value: 1}}).Err(); err != nil {
		t.Skipf("Skipping - cannot drop database: %v", err)
	}
	defer store.Database().RunCommand(ctx, bson.D{{Key: "dropDatabase", Value: 1}})

	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	testEmail := "test@requiem.cfd"
	testPassword := "testtest"
	testName := "Test User"

	signupBody := `{"name":"` + testName + `","email":"` + testEmail + `","password":"` + testPassword + `"}`
	signupReq := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(signupBody))
	signupReq.Header.Set("Content-Type", "application/json")
	signupRec := httptest.NewRecorder()
	handler.Signup(signupRec, signupReq)

	if signupRec.Code != 200 {
		t.Fatalf("signup with test@requiem.cfd failed: %d, body: %s", signupRec.Code, signupRec.Body.String())
	}

	loginBody := `{"email":"` + testEmail + `","password":"` + testPassword + `"}`
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	if loginRec.Code != 200 {
		t.Fatalf("login with test@requiem.cfd failed: %d, body: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp map[string]any
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if loginResp["email"] != testEmail {
		t.Errorf("expected email %s in login response, got: %v", testEmail, loginResp["email"])
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
		t.Fatalf("failed to validate session: %v", err)
	}
	if claims.Email != testEmail {
		t.Errorf("expected email %s in claims, got: %s", testEmail, claims.Email)
	}

	t.Logf("Successfully signed up and logged in user: %s", testEmail)
}

func TestSignupAndLogin_RealMongoDB_WrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB integration test in short mode")
	}

	ctx := context.Background()

	store, err := storage.NewMongoStore(ctx, storage.MongoConfig{
		URI:      "mongodb://127.0.0.1:27017",
		Database: "requiem_auth_test2",
	}, nil)
	if err != nil {
		t.Skipf("Skipping - MongoDB not available: %v", err)
	}
	defer store.Close(ctx)

	if err := store.Database().RunCommand(ctx, bson.D{{Key: "dropDatabase", Value: 1}}).Err(); err != nil {
		t.Skipf("Skipping - cannot drop database: %v", err)
	}
	defer store.Database().RunCommand(ctx, bson.D{{Key: "dropDatabase", Value: 1}})

	mw := security.NewMiddleware(nil, nil, []string{"http://localhost:3000"})
	secretsStore, err := security.NewSecretsStore("test-master-key-1234567890123456", nil)
	if err != nil {
		t.Fatalf("failed to create secrets store: %v", err)
	}
	_ = mw
	_ = secretsStore

	authService := NewAuthService(store, "test-jwt-secret", "", nil)
	handler := NewAuthHandler(store, authService, nil)

	signupBody := `{"name":"Test User","email":"test@requiem.cfd","password":"testtest"}`
	signupReq := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(signupBody))
	signupReq.Header.Set("Content-Type", "application/json")
	handler.Signup(httptest.NewRecorder(), signupReq)

	loginBody := `{"email":"test@requiem.cfd","password":"wrongpassword"}`
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	if loginRec.Code != 401 {
		t.Errorf("expected 401 for wrong password, got %d", loginRec.Code)
	}
}
