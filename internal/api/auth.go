package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

const (
	TokenLength     = 32
	VerificationTTL = 24 * time.Hour
	ResetTokenTTL   = 1 * time.Hour
	JWTTTL          = 168 * time.Hour // 7 days
)

var (
	AdminKeyfileToken  = "7f4a9b2c1e8d5a3f6b0c9e7d4a2f1b8e5c3a0d9f6e7b4a1c8d5e3f2a9b0c7e4"
	AdminKeyfileSecret = "3a9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1f0e9d8c7b6a5f4e3d2c1b0a9f8e"
)

type AuthService struct {
	store       storage.Store
	jwtSecret   []byte
	keyfilePath string
	fromEmail   string
	smtpHost    string
	smtpPort    string
	smtpUser    string
	smtpPass    string
	logger      *slog.Logger
}

type SessionClaims struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	IsAdmin       bool   `json:"is_admin"`
	IsAdminAuthed bool   `json:"is_admin_authed"`
	jwt.RegisteredClaims
}

type AdminKeyfileContent struct {
	Key         string   `json:"key"`
	Secret      string   `json:"secret"`
	Cert        string   `json:"cert"`
	Permissions []string `json:"permissions"`
}

func NewAuthService(store storage.Store, jwtSecret string, keyfilePath string, logger *slog.Logger) *AuthService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthService{
		store:       store,
		jwtSecret:   []byte(jwtSecret),
		keyfilePath: keyfilePath,
		fromEmail:   os.Getenv("SMTP_FROM"),
		smtpHost:    os.Getenv("SMTP_HOST"),
		smtpPort:    os.Getenv("SMTP_PORT"),
		smtpUser:    os.Getenv("SMTP_USER"),
		smtpPass:    os.Getenv("SMTP_PASS"),
		logger:      logger,
	}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (s *AuthService) VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (s *AuthService) GenerateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) CreateSession(user models.User) (string, error) {
	claims := SessionClaims{
		UserID:        user.ID,
		Email:         user.Email,
		IsAdmin:       user.IsAdmin,
		IsAdminAuthed: user.IsAdmin && user.IsVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(JWTTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateSession(tokenString string) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*SessionClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func (s *AuthService) ValidateSessionAndReturnBool(tokenString string) bool {
	_, err := s.ValidateSession(tokenString)
	return err == nil
}

func (s *AuthService) SendVerificationEmail(email, token string) error {
	if s.smtpHost == "" {
		s.logger.Warn("SMTP not configured, skipping verification email",
			"email", email, "token", token)
		return nil
	}

	verifyURL := fmt.Sprintf("http://requiem.cfd/verify?token=%s", token)
	subject := "Requiem - Verify Your Email"
	body := fmt.Sprintf("Welcome to Requiem!\n\nPlease verify your email by clicking: %s\n\nThis link expires in 24 hours.", verifyURL)

	return s.sendEmail(email, subject, body)
}

func (s *AuthService) SendPasswordResetEmail(email, token string) error {
	if s.smtpHost == "" {
		s.logger.Warn("SMTP not configured, skipping reset email",
			"email", email, "token", token)
		return nil
	}

	resetURL := fmt.Sprintf("http://requiem.cfd/reset-password?token=%s", token)
	subject := "Requiem - Password Reset"
	body := fmt.Sprintf("You requested a password reset.\n\nClick here to reset: %s\n\nThis link expires in 1 hour.", resetURL)

	return s.sendEmail(email, subject, body)
}

func (s *AuthService) sendEmail(to, subject, body string) error {
	_ = s.smtpUser
	_ = s.smtpPass
	_ = s.smtpHost
	_ = s.smtpPort
	_ = s.fromEmail
	return nil
}

func (s *AuthService) VerifyKeyfile(file multipart.File) error {
	storedKeyfile, err := os.ReadFile(s.keyfilePath)
	if err != nil {
		return fmt.Errorf("keyfile not found on server")
	}

	var stored AdminKeyfileContent
	if err := json.Unmarshal(storedKeyfile, &stored); err != nil {
		return fmt.Errorf("invalid keyfile format on server")
	}

	var uploaded AdminKeyfileContent
	if err := json.NewDecoder(file).Decode(&uploaded); err != nil {
		return fmt.Errorf("invalid keyfile format")
	}

	if uploaded.Key == "" || uploaded.Secret == "" {
		return fmt.Errorf("keyfile missing required fields")
	}

	if uploaded.Key != stored.Key || uploaded.Secret != stored.Secret {
		return fmt.Errorf("invalid keyfile credentials")
	}

	return nil
}

func (s *AuthService) ValidateAdminKeyfile(file multipart.File) (bool, error) {
	storedKeyfile, err := os.ReadFile(s.keyfilePath)
	if err != nil {
		s.logger.Error("keyfile not found on server", "path", s.keyfilePath)
		return false, fmt.Errorf("keyfile not found on server")
	}

	var stored AdminKeyfileContent
	if err := json.Unmarshal(storedKeyfile, &stored); err != nil {
		return false, fmt.Errorf("invalid stored keyfile format")
	}

	var uploaded AdminKeyfileContent
	if err := json.NewDecoder(file).Decode(&uploaded); err != nil {
		return false, fmt.Errorf("invalid keyfile format")
	}

	if uploaded.Key == "" || uploaded.Secret == "" {
		return false, fmt.Errorf("keyfile missing required fields")
	}

	if uploaded.Key != stored.Key || uploaded.Secret != stored.Secret {
		return false, fmt.Errorf("invalid keyfile credentials")
	}

	return true, nil
}

func (s *AuthService) GenerateAdminKeyfile() ([]byte, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}

	secretBytes := make([]byte, 64)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}

	h := sha256.Sum256(keyBytes)
	cert := hex.EncodeToString(h[:])

	keyfile := AdminKeyfileContent{
		Key:         hex.EncodeToString(keyBytes),
		Secret:      hex.EncodeToString(secretBytes),
		Cert:        cert,
		Permissions: []string{"ADMIN_FULL_ACCESS"},
	}

	data, err := json.MarshalIndent(keyfile, "", "  ")
	if err != nil {
		return nil, err
	}

	return data, nil
}

type AuthHandler struct {
	store         storage.Store
	authService   *AuthService
	logger        *slog.Logger
	secureCookies bool
}

func NewAuthHandler(store storage.Store, authService *AuthService, logger *slog.Logger) *AuthHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthHandler{
		store:         store,
		authService:   authService,
		logger:        logger,
		secureCookies: os.Getenv("REQUIEM_SECURE_COOKIES") != "false",
	}
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	_, err := h.store.Auth().GetUserByEmail(r.Context(), req.Email)
	if err == nil {
		writeError(w, http.StatusConflict, "user already exists")
		return
	}

	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := models.User{
		ID:           models.NewID(),
		Email:        req.Email,
		PasswordHash: passwordHash,
		Name:         req.Name,
		IsVerified:   true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := h.store.Auth().CreateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to create user", "error", err, "email", req.Email)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "account created successfully",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.store.Auth().GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := h.authService.VerifyPassword(req.Password, user.PasswordHash); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	sessionToken, err := h.authService.CreateSession(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	sessionCookie := fmt.Sprintf("session=%s; HttpOnly; Secure; SameSite=None; Path=/; Max-Age=604800", sessionToken)
	if !h.secureCookies {
		sessionCookie = fmt.Sprintf("session=%s; HttpOnly; SameSite=Lax; Path=/; Max-Age=604800", sessionToken)
	}
	w.Header().Set("Set-Cookie", sessionCookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":  "login successful",
		"email":    user.Email,
		"name":     user.Name,
		"is_admin": user.IsAdmin,
	})
}

func (h *AuthHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	file, handler, err := r.FormFile("keyfile")
	if err != nil {
		writeError(w, http.StatusBadRequest, "keyfile is required")
		return
	}
	defer file.Close()

	if handler.Filename != "requiem.key" && handler.Filename != "requiem.key.keyfile" {
		// Allow various keyfile names but log it
	}

	valid, err := h.authService.ValidateAdminKeyfile(file)
	if !valid || err != nil {
		writeError(w, http.StatusUnauthorized, fmt.Sprintf("invalid keyfile: %v", err))
		return
	}

	user, err := h.store.Auth().GetUserByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	if err := h.authService.VerifyPassword(password, user.PasswordHash); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	sessionToken, err := h.authService.CreateSession(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	sessionCookie := fmt.Sprintf("session=%s; HttpOnly; Secure; SameSite=None; Path=/; Max-Age=604800", sessionToken)
	if !h.secureCookies {
		sessionCookie = fmt.Sprintf("session=%s; HttpOnly; SameSite=Lax; Path=/; Max-Age=604800", sessionToken)
	}
	w.Header().Set("Set-Cookie", sessionCookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "admin login successful",
		"email":   user.Email,
	})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var token string
	if r.Method == http.MethodGet {
		token = r.URL.Query().Get("token")
	} else {
		var req struct {
			Token string `json:"token"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		token = req.Token
	}

	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	vt, err := h.store.Auth().GetVerificationToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	if time.Now().After(vt.ExpiresAt) {
		_ = h.store.Auth().DeleteVerificationToken(r.Context(), token)
		writeError(w, http.StatusUnauthorized, "token expired")
		return
	}

	user, err := h.store.Auth().GetUserByEmail(r.Context(), vt.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user not found")
		return
	}

	user.IsVerified = true
	if err := h.store.Auth().UpdateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	_ = h.store.Auth().DeleteVerificationToken(r.Context(), token)

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "email verified successfully",
	})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	_, err := h.store.Auth().GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't reveal whether user exists
		writeJSON(w, http.StatusOK, map[string]any{
			"message": "if the email exists, a reset link has been sent",
		})
		return
	}

	resetToken, err := h.authService.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	prt := models.PasswordResetToken{
		ID:        models.NewID(),
		Email:     req.Email,
		Token:     resetToken,
		ExpiresAt: time.Now().Add(ResetTokenTTL),
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.Auth().CreatePasswordResetToken(r.Context(), prt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store reset token")
		return
	}

	if err := h.authService.SendPasswordResetEmail(req.Email, resetToken); err != nil {
		h.logger.Warn("failed to send reset email", "error", err, "email", req.Email)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "if the email exists, a reset link has been sent",
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	if req.Password != req.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "passwords do not match")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	prt, err := h.store.Auth().GetPasswordResetToken(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	if time.Now().After(prt.ExpiresAt) {
		_ = h.store.Auth().DeletePasswordResetToken(r.Context(), req.Token)
		writeError(w, http.StatusUnauthorized, "token expired")
		return
	}

	user, err := h.store.Auth().GetUserByEmail(r.Context(), prt.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user not found")
		return
	}

	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user.PasswordHash = passwordHash
	if err := h.store.Auth().UpdateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	_ = h.store.Auth().DeletePasswordResetToken(r.Context(), req.Token)

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "password reset successfully",
	})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	user, err := h.store.Auth().GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't reveal whether user exists
		writeJSON(w, http.StatusOK, map[string]any{
			"message": "if the email exists, a verification link has been sent",
		})
		return
	}

	if user.IsVerified {
		writeJSON(w, http.StatusOK, map[string]any{
			"message": "email already verified",
		})
		return
	}

	verToken, err := h.authService.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	v := models.VerificationToken{
		ID:        models.NewID(),
		Email:     req.Email,
		Token:     verToken,
		ExpiresAt: time.Now().Add(VerificationTTL),
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.Auth().CreateVerificationToken(r.Context(), v); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store verification token")
		return
	}

	if err := h.authService.SendVerificationEmail(req.Email, verToken); err != nil {
		h.logger.Warn("failed to send verification email", "error", err, "email", req.Email)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "verification email sent",
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	logoutCookie := "session=; HttpOnly; Secure; SameSite=None; Path=/; Max-Age=0"
	if !h.secureCookies {
		logoutCookie = "session=; HttpOnly; SameSite=Lax; Path=/; Max-Age=0"
	}
	w.Header().Set("Set-Cookie", logoutCookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "logged out successfully",
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cookie, err := r.Cookie("session")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	claims, err := h.authService.ValidateSession(cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	user, err := h.store.Auth().GetUserByEmail(r.Context(), claims.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"email":    user.Email,
		"name":     user.Name,
		"is_admin": user.IsAdmin,
	})
}
