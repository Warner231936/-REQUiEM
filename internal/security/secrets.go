package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

type Permission string

const (
	PermissionRead     Permission = "read"
	PermissionWrite    Permission = "write"
	PermissionAdmin    Permission = "admin"
	PermissionPipeline Permission = "pipeline"
)

type APIKey struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	KeyHash     string       `json:"key_hash"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Revoked     bool         `json:"revoked"`
	LastUsedAt  *time.Time   `json:"last_used_at,omitempty"`
}

func (k *APIKey) HasPermission(perm Permission) bool {
	for _, p := range k.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

func (k *APIKey) IsValid() bool {
	if k.Revoked {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}

type KeyHasher struct {
	salt []byte
}

func NewKeyHasher(salt []byte) *KeyHasher {
	return &KeyHasher{salt: salt}
}

func (h *KeyHasher) HashKey(key string) string {
	hash := argon2.IDKey([]byte(key), h.salt, 1, 64*1024, 4, 32)
	return base64.StdEncoding.EncodeToString(hash)
}

func (h *KeyHasher) VerifyKey(key, hash string) (bool, error) {
	expectedHash, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	computedHash := argon2.IDKey([]byte(key), h.salt, 1, 64*1024, 4, 32)
	if len(expectedHash) != len(computedHash) {
		return false, nil
	}
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1, nil
}

type SecretsStore struct {
	mu         sync.RWMutex
	keys       map[string]*APIKey
	hasher     *KeyHasher
	encryption []byte
	logger     *slog.Logger
}

type EncryptedSecrets struct {
	Version   string `json:"version"`
	Encrypted string `json:"encrypted"`
	Salt      string `json:"salt"`
	UpdatedAt string `json:"updated_at"`
}

func NewSecretsStore(masterKey string, logger *slog.Logger) (*SecretsStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if len(masterKey) < 32 {
		return nil, fmt.Errorf("master key must be at least 32 bytes")
	}

	salt := sha256.Sum256([]byte(masterKey + "salt"))
	hasher := NewKeyHasher(salt[:16])

	return &SecretsStore{
		keys:       make(map[string]*APIKey),
		hasher:     hasher,
		encryption: sha256.New().Sum([]byte(masterKey)),
		logger:     logger,
	}, nil
}

func (s *SecretsStore) GenerateKey(name string, permissions []Permission, ttl *time.Duration) (string, *APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyID := generateKeyID()
	rawKey := generateAPIKey()
	keyHash := s.hasher.HashKey(rawKey)

	var expiresAt *time.Time
	if ttl != nil {
		exp := time.Now().Add(*ttl)
		expiresAt = &exp
	}

	apiKey := &APIKey{
		ID:          keyID,
		Name:        name,
		KeyHash:     keyHash,
		Permissions: permissions,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
		Revoked:     false,
	}

	s.keys[keyID] = apiKey

	s.logger.Info("API key generated",
		"key_id", keyID,
		"name", name,
		"permissions", permissions,
	)

	return rawKey, apiKey, nil
}

func (s *SecretsStore) VerifyKey(rawKey string) (*APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range s.keys {
		if !key.IsValid() {
			continue
		}
		valid, err := s.hasher.VerifyKey(rawKey, key.KeyHash)
		if err != nil {
			continue
		}
		if valid {
			now := time.Now().UTC()
			key.LastUsedAt = &now
			return key, true
		}
	}
	return nil, false
}

func (s *SecretsStore) RevokeKey(keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key, ok := s.keys[keyID]; ok {
		key.Revoked = true
		s.logger.Info("API key revoked", "key_id", keyID, "name", key.Name)
		return nil
	}
	return fmt.Errorf("key not found: %s", keyID)
}

func (s *SecretsStore) DeleteKey(keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[keyID]; ok {
		delete(s.keys, keyID)
		s.logger.Info("API key deleted", "key_id", keyID)
		return nil
	}
	return fmt.Errorf("key not found: %s", keyID)
}

func (s *SecretsStore) ListKeys() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*APIKey, 0, len(s.keys))
	for _, k := range s.keys {
		keys = append(keys, k)
	}
	return keys
}

func (s *SecretsStore) ExportEncrypted() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.Marshal(s.keys)
	if err != nil {
		return "", fmt.Errorf("marshal keys: %w", err)
	}

	block, err := aes.NewCipher(s.encryption)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *SecretsStore) ImportEncrypted(encryptedData string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(s.encryption)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	var keys map[string]*APIKey
	if err := json.Unmarshal(plaintext, &keys); err != nil {
		return fmt.Errorf("unmarshal keys: %w", err)
	}

	s.keys = keys
	return nil
}

func (s *SecretsStore) SaveToFile(path string) error {
	encrypted, err := s.ExportEncrypted()
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	secrets := EncryptedSecrets{
		Version:   "1.0",
		Encrypted: encrypted,
		Salt:      base64.StdEncoding.EncodeToString(s.hasher.salt),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *SecretsStore) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var secrets EncryptedSecrets
	if err := json.Unmarshal(data, &secrets); err != nil {
		return fmt.Errorf("unmarshal secrets: %w", err)
	}

	return s.ImportEncrypted(secrets.Encrypted)
}

func generateKeyID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("key_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("key_%x", b[:8])
}

func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback_%d", time.Now().UnixNano())
	}
	return "rk_" + base64.URLEncoding.EncodeToString(b)
}
