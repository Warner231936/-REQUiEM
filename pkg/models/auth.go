package models

import (
	"time"
)

type User struct {
	ID           string     `json:"id" bson:"_id"`
	Email        string     `json:"email" bson:"email"`
	PasswordHash string     `json:"-" bson:"password_hash"`
	Name         string     `json:"name" bson:"name"`
	IsVerified   bool       `json:"is_verified" bson:"is_verified"`
	IsAdmin      bool       `json:"is_admin" bson:"is_admin"`
	IsActive     bool       `json:"is_active" bson:"is_active"`
	Phone        string     `json:"phone,omitempty" bson:"phone,omitempty"`
	AvatarURL    string     `json:"avatar_url,omitempty" bson:"avatar_url,omitempty"`
	Bio          string     `json:"bio,omitempty" bson:"bio,omitempty"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" bson:"last_login_at,omitempty"`
	LoginCount   int        `json:"login_count" bson:"login_count"`
	CreatedAt    time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" bson:"updated_at"`
}

type VerificationToken struct {
	ID        string    `json:"id" bson:"_id"`
	Email     string    `json:"email" bson:"email"`
	Token     string    `json:"token" bson:"token"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type PasswordResetToken struct {
	ID        string    `json:"id" bson:"_id"`
	Email     string    `json:"email" bson:"email"`
	Token     string    `json:"token" bson:"token"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type AdminKeyfile struct {
	Key         string   `json:"key" bson:"key"`
	Secret      string   `json:"secret" bson:"secret"`
	Cert        string   `json:"cert" bson:"cert"`
	Permissions []string `json:"permissions" bson:"permissions"`
}
