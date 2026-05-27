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
	PasswordHash *string
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

type OAuthAccount struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Provider    string
	ProviderUID string
	Email       *string
	LinkedAt    time.Time
}

// Domain errors — handlers map these to HTTP status codes.
var (
	ErrNotFound            = errors.New("user not found")
	ErrEmailAlreadyExists  = errors.New("email already registered")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAnonAlreadyClaimed  = errors.New("anonymous user already claimed")
	ErrClaimTokenExpired   = errors.New("claim token is expired or already used")
	ErrInvalidResetToken   = errors.New("password reset token is invalid or expired")
)
