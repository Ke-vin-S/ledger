package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
)

type Service struct {
	repo   Repository
	auditor audit.Logger
}

func NewService(repo Repository, auditor audit.Logger) *Service {
	return &Service{repo: repo, auditor: auditor}
}

// Register creates a new registered user with an email and password.
func (s *Service) Register(ctx context.Context, displayName, email, password string) (*User, error) {
	if err := validateDisplayName(displayName); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	email = strings.ToLower(strings.TrimSpace(email))
	hashStr := string(hash)
	u := &User{
		IdentityType: IdentityTypeRegistered,
		DisplayName:  strings.TrimSpace(displayName),
		Email:        &email,
		PasswordHash: &hashStr,
		CurrencyPref: "LKR",
		Timezone:     "Asia/Colombo",
	}

	created, err := s.repo.Create(ctx, u)
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionUserCreated,
		ActorID:    &created.ID,
		EntityType: "user",
		EntityID:   created.ID,
		After:      created,
	})
	return created, nil
}

// Login authenticates a user by email and password, returning the user on success.
func (s *Service) Login(ctx context.Context, email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if u.PasswordHash == nil {
		return nil, ErrOAuthOnly
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// FindOrCreateByOAuth finds an existing user by OAuth provider, or creates a new one.
// Returns the user and whether it was newly created.
func (s *Service) FindOrCreateByOAuth(
	ctx context.Context,
	provider, providerUID, email, displayName, avatarURL string,
) (*User, bool, error) {
	// Try to find by OAuth link first.
	u, err := s.repo.FindByOAuth(ctx, provider, providerUID)
	if err == nil {
		if avatarURL != "" {
			_ = s.repo.UpdateAvatarURL(ctx, u.ID, avatarURL)
		}
		return u, false, nil
	}
	if err != ErrNotFound {
		return nil, false, err
	}

	// Try to find by email — link if registered user exists.
	if email != "" {
		existing, err := s.repo.FindByEmail(ctx, strings.ToLower(email))
		if err == nil {
			emailPtr := &email
			if err := s.repo.UpsertOAuthAccount(ctx, existing.ID, provider, providerUID, emailPtr); err != nil {
				return nil, false, fmt.Errorf("link oauth: %w", err)
			}
			if avatarURL != "" {
				_ = s.repo.UpdateAvatarURL(ctx, existing.ID, avatarURL)
			}
			return existing, false, nil
		}
	}

	// New user.
	emailNorm := strings.ToLower(strings.TrimSpace(email))
	var emailPtr *string
	if emailNorm != "" {
		emailPtr = &emailNorm
	}
	var avatarPtr *string
	if avatarURL != "" {
		avatarPtr = &avatarURL
	}
	newUser := &User{
		IdentityType: IdentityTypeRegistered,
		DisplayName:  displayName,
		Email:        emailPtr,
		AvatarURL:    avatarPtr,
		CurrencyPref: "LKR",
		Timezone:     "Asia/Colombo",
	}
	created, err := s.repo.Create(ctx, newUser)
	if err != nil {
		return nil, false, err
	}
	if err := s.repo.UpsertOAuthAccount(ctx, created.ID, provider, providerUID, emailPtr); err != nil {
		return nil, false, fmt.Errorf("link oauth after create: %w", err)
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionUserCreated,
		ActorID:    &created.ID,
		EntityType: "user",
		EntityID:   created.ID,
		After:      created,
	})
	return created, true, nil
}

// GetByID returns a user by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.FindByID(ctx, id)
}

// UpdateMe applies partial profile updates for the authenticated user.
func (s *Service) UpdateMe(ctx context.Context, id uuid.UUID, displayName, avatarURL, currencyPref, timezone *string) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if displayName != nil {
		if err := validateDisplayName(*displayName); err != nil {
			return nil, err
		}
		u.DisplayName = strings.TrimSpace(*displayName)
	}
	if avatarURL != nil {
		u.AvatarURL = avatarURL
	}
	if currencyPref != nil {
		cp := strings.ToUpper(strings.TrimSpace(*currencyPref))
		if len(cp) != 3 {
			return nil, fmt.Errorf("currency_pref must be a 3-character ISO 4217 code")
		}
		u.CurrencyPref = cp
	}
	if timezone != nil {
		tz := strings.TrimSpace(*timezone)
		if tz == "" {
			return nil, fmt.Errorf("timezone cannot be empty")
		}
		u.Timezone = tz
	}
	updated, err := s.repo.Update(ctx, u)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionUserUpdated,
		ActorID:    &id,
		EntityType: "user",
		EntityID:   id,
		After:      updated,
	})
	return updated, nil
}

// GetNotificationPrefs returns the authenticated user's notification preferences.
func (s *Service) GetNotificationPrefs(ctx context.Context, userID uuid.UUID) (*NotificationPrefs, error) {
	return s.repo.GetNotificationPrefs(ctx, userID)
}

// UpdateNotificationPrefs replaces notification preferences for the authenticated user.
func (s *Service) UpdateNotificationPrefs(ctx context.Context, userID uuid.UUID, emailEnabled, digestMode *bool, disabledTypes []string) (*NotificationPrefs, error) {
	prefs, err := s.repo.GetNotificationPrefs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if emailEnabled != nil {
		prefs.EmailEnabled = *emailEnabled
	}
	if digestMode != nil {
		prefs.DigestMode = *digestMode
	}
	if disabledTypes != nil {
		prefs.DisabledTypes = disabledTypes
	}
	return s.repo.UpdateNotificationPrefs(ctx, prefs)
}

// CreateAnonymous creates an anonymous user placeholder on behalf of a registered user.
func (s *Service) CreateAnonymous(ctx context.Context, displayName string, createdBy uuid.UUID) (*User, error) {
	if err := validateDisplayName(displayName); err != nil {
		return nil, err
	}
	u, err := s.repo.CreateAnonymous(ctx, strings.TrimSpace(displayName), createdBy)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionUserCreated,
		ActorID:    &createdBy,
		EntityType: "user",
		EntityID:   u.ID,
		After:      u,
		Meta:       map[string]any{"identity_type": "anonymous"},
	})
	return u, nil
}

// GenerateClaimToken creates a hashed one-time claim token for the given anonymous user.
// Returns the raw token (to be embedded in the claim URL).
func (s *Service) GenerateClaimToken(ctx context.Context, anonUserID, createdBy uuid.UUID) (rawToken string, expiresAt time.Time, err error) {
	anonUser, err := s.repo.FindByID(ctx, anonUserID)
	if err != nil {
		return "", time.Time{}, err
	}
	if !anonUser.IsAnonymous() {
		return "", time.Time{}, ErrNotAnonymous
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}
	rawToken = hex.EncodeToString(raw)
	h := sha256ClaimToken(rawToken)
	expiresAt = time.Now().Add(7 * 24 * time.Hour)

	if _, err := s.repo.CreateClaimToken(ctx, anonUserID, createdBy, h, expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("store claim token: %w", err)
	}
	return rawToken, expiresAt, nil
}

// ClaimAnonymous atomically merges an anonymous profile into the authenticated user's account.
func (s *Service) ClaimAnonymous(ctx context.Context, rawToken string, claimedByID uuid.UUID) error {
	h := sha256ClaimToken(rawToken)
	anonID, err := s.repo.Claim(ctx, h, claimedByID)
	if err != nil {
		return err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionUserClaimed,
		ActorID:    &claimedByID,
		EntityType: "user",
		EntityID:   anonID,
		Meta: map[string]any{
			"anon_user_id":    anonID.String(),
			"claimed_by":      claimedByID.String(),
		},
	})
	return nil
}

func sha256ClaimToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// GeneratePasswordResetToken creates a one-time reset token for the given email.
// The token is hashed before storage; the raw token is returned to the caller.
// In production, the caller should email the token — never log or return it in the response.
func (s *Service) GeneratePasswordResetToken(ctx context.Context, email string, store PasswordResetStore) (rawToken string, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the email exists; silently succeed.
		return "", nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	rawToken = hex.EncodeToString(raw)
	h := hashResetToken(rawToken)

	if err := store.StoreReset(ctx, h, u.ID, time.Hour); err != nil {
		return "", fmt.Errorf("store reset token: %w", err)
	}
	return rawToken, nil
}

// ResetPassword consumes a reset token and updates the password.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string, store PasswordResetStore) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	h := hashResetToken(rawToken)
	userID, err := store.GetAndDeleteReset(ctx, h)
	if err != nil {
		return ErrInvalidResetToken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.UpdatePassword(ctx, userID, string(hash))
}

// PasswordResetStore abstracts Redis (or any store) for reset tokens.
type PasswordResetStore interface {
	StoreReset(ctx context.Context, tokenHash string, userID uuid.UUID, ttl time.Duration) error
	GetAndDeleteReset(ctx context.Context, tokenHash string) (uuid.UUID, error)
}

func hashResetToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func validateDisplayName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("display_name is required")
	}
	if utf8.RuneCountInString(name) > 100 {
		return fmt.Errorf("display_name must be 100 characters or fewer")
	}
	return nil
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return fmt.Errorf("email is invalid")
	}
	return nil
}
