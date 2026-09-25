package user

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	IdentityTypeRegistered = "registered"
	IdentityTypeAnonymous  = "anonymous"
)

type User struct {
	ID           uuid.UUID
	IdentityType string
	DisplayName  string
	Email        *string
	PasswordHash *string `json:"-"`
	AvatarURL    *string
	CurrencyPref string
	Timezone     string
	ClaimedBy    *uuid.UUID
	ClaimedAt    *time.Time
	CreatedAt    time.Time
	DeletedAt    *time.Time
	DeletedBy    *uuid.UUID
}

func (u *User) IsAnonymous() bool  { return u.IdentityType == IdentityTypeAnonymous }
func (u *User) IsRegistered() bool { return u.IdentityType == IdentityTypeRegistered }
func (u *User) IsClaimed() bool    { return u.ClaimedBy != nil }
func (u *User) IsDeleted() bool    { return u.DeletedAt != nil }

type AuditUser struct {
	ID           uuid.UUID `json:"id"`
	IdentityType string    `json:"identity_type"`
	DisplayName  string    `json:"display_name"`
	Email        *string   `json:"email,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	CurrencyPref string    `json:"currency_pref"`
	Timezone     string    `json:"timezone"`
	CreatedAt    time.Time `json:"created_at"`
}

func NewAuditUser(u *User) AuditUser {
	return AuditUser{
		ID: u.ID, IdentityType: u.IdentityType, DisplayName: u.DisplayName,
		Email: u.Email, AvatarURL: u.AvatarURL, CurrencyPref: u.CurrencyPref,
		Timezone: u.Timezone, CreatedAt: u.CreatedAt,
	}
}

type OAuthAccount struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Provider    string
	ProviderUID string
	Email       *string
	LinkedAt    time.Time
}

type NotificationPrefs struct {
	UserID        uuid.UUID
	EmailEnabled  bool
	DigestMode    bool
	DisabledTypes []string
	UpdatedAt     time.Time
}

type ClaimToken struct {
	ID         uuid.UUID
	AnonUserID uuid.UUID
	CreatedBy  uuid.UUID
	TokenHash  string
	ExpiresAt  time.Time
	UsedAt     *time.Time
	CreatedAt  time.Time
}

// Domain errors — handlers map these to HTTP status codes.
var (
	ErrNotFound           = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrOAuthOnly          = errors.New("account uses OAuth login, no password set")
	ErrAnonAlreadyClaimed = errors.New("anonymous user already claimed")
	ErrClaimTokenExpired  = errors.New("claim token is expired or already used")
	ErrInvalidResetToken  = errors.New("password reset token is invalid or expired")
	ErrNotAnonymous       = errors.New("user is not anonymous")
	ErrNotAnonymousOwner  = errors.New("caller does not own this anonymous user")
	ErrClaimConflict      = errors.New("claim failed: membership conflict")
)
