package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the storage interface for users.
// Implementations live in internal/repository/.
type Repository interface {
	// Create inserts a new registered user and their default notification_prefs in a single transaction.
	Create(ctx context.Context, u *User) (*User, error)

	// CreateAnonymous inserts an anonymous user placeholder (no notification_prefs row).
	CreateAnonymous(ctx context.Context, displayName string, createdBy uuid.UUID) (*User, error)

	// FindByID returns the user with the given ID. Returns ErrNotFound if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)

	// FindByEmail returns the user with the given email. Returns ErrNotFound if absent.
	FindByEmail(ctx context.Context, email string) (*User, error)

	// FindByOAuth returns the user linked to the given OAuth provider account.
	// Returns ErrNotFound if no link exists.
	FindByOAuth(ctx context.Context, provider, providerUID string) (*User, error)

	// UpsertOAuthAccount links an OAuth account to an existing user, or updates the link if it already exists.
	UpsertOAuthAccount(ctx context.Context, userID uuid.UUID, provider, providerUID string, email *string) error

	// Update stores changes to display_name, avatar_url, currency_pref, timezone.
	Update(ctx context.Context, u *User) (*User, error)

	// UpdateAvatarURL sets the avatar_url for a user. Used to refresh the photo on each OAuth login.
	UpdateAvatarURL(ctx context.Context, userID uuid.UUID, avatarURL string) error

	// UpdatePassword sets a new bcrypt password hash for the user.
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error

	// GetNotificationPrefs returns the caller's notification preferences.
	GetNotificationPrefs(ctx context.Context, userID uuid.UUID) (*NotificationPrefs, error)

	// UpdateNotificationPrefs replaces the caller's notification preferences.
	UpdateNotificationPrefs(ctx context.Context, prefs *NotificationPrefs) (*NotificationPrefs, error)

	// CreateClaimToken inserts a claim_tokens row with a hashed token.
	CreateClaimToken(ctx context.Context, anonUserID, createdBy uuid.UUID, tokenHash string, expiresAt time.Time) (*ClaimToken, error)

	// Claim atomically merges an anonymous user into the registered user in a single transaction.
	// It marks the claim token as used, sets claimed_by/claimed_at on the anon user, and
	// reassigns all expenses, splits, settlements, and team memberships.
	Claim(ctx context.Context, tokenHash string, claimedByID uuid.UUID) (anonUserID uuid.UUID, err error)
}
