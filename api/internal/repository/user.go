package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
)

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) user.Repository {
	return &userRepo{pool: pool}
}

func (r *userRepo) Create(ctx context.Context, u *user.User) (*user.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := insertUser(ctx, tx, u)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notification_prefs (user_id) VALUES ($1)`, created.ID); err != nil {
		return nil, fmt.Errorf("create notification_prefs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return created, nil
}

func (r *userRepo) CreateAnonymous(ctx context.Context, displayName string, _ uuid.UUID) (*user.User, error) {
	u := &user.User{
		IdentityType: user.IdentityTypeAnonymous,
		DisplayName:  displayName,
		CurrencyPref: "LKR",
		Timezone:     "Asia/Colombo",
	}
	return insertUser(ctx, r.pool, u)
}

func (r *userRepo) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, identity_type, display_name, email, password_hash, avatar_url,
		       currency_pref, timezone, claimed_by, claimed_at, created_at, deleted_at, deleted_by
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return scanUser(row)
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, identity_type, display_name, email, password_hash, avatar_url,
		       currency_pref, timezone, claimed_by, claimed_at, created_at, deleted_at, deleted_by
		FROM users WHERE email = $1 AND deleted_at IS NULL
	`, email)
	return scanUser(row)
}

func (r *userRepo) FindByOAuth(ctx context.Context, provider, providerUID string) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT u.id, u.identity_type, u.display_name, u.email, u.password_hash, u.avatar_url,
		       u.currency_pref, u.timezone, u.claimed_by, u.claimed_at, u.created_at, u.deleted_at, u.deleted_by
		FROM users u
		JOIN oauth_accounts oa ON oa.user_id = u.id
		WHERE oa.provider = $1 AND oa.provider_uid = $2 AND u.deleted_at IS NULL
	`, provider, providerUID)
	return scanUser(row)
}

func (r *userRepo) UpsertOAuthAccount(ctx context.Context, userID uuid.UUID, provider, providerUID string, email *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO oauth_accounts (user_id, provider, provider_uid, email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_uid) DO UPDATE SET email = EXCLUDED.email
	`, userID, provider, providerUID, email)
	return err
}

func (r *userRepo) Update(ctx context.Context, u *user.User) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE users
		SET display_name = $1, avatar_url = $2, currency_pref = $3, timezone = $4
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING id, identity_type, display_name, email, password_hash, avatar_url,
		          currency_pref, timezone, claimed_by, claimed_at, created_at, deleted_at, deleted_by
	`, u.DisplayName, u.AvatarURL, u.CurrencyPref, u.Timezone, u.ID)
	return scanUser(row)
}

func (r *userRepo) UpdateAvatarURL(ctx context.Context, userID uuid.UUID, avatarURL string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET avatar_url = $1 WHERE id = $2 AND deleted_at IS NULL`, avatarURL, userID)
	return err
}

func (r *userRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID)
	return err
}

func (r *userRepo) GetNotificationPrefs(ctx context.Context, userID uuid.UUID) (*user.NotificationPrefs, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT user_id, email_enabled, digest_mode, disabled_types, updated_at
		FROM notification_prefs WHERE user_id = $1
	`, userID)
	var p user.NotificationPrefs
	if err := row.Scan(&p.UserID, &p.EmailEnabled, &p.DigestMode, &p.DisabledTypes, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *userRepo) UpdateNotificationPrefs(ctx context.Context, p *user.NotificationPrefs) (*user.NotificationPrefs, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE notification_prefs
		SET email_enabled = $1, digest_mode = $2, disabled_types = $3, updated_at = NOW()
		WHERE user_id = $4
		RETURNING user_id, email_enabled, digest_mode, disabled_types, updated_at
	`, p.EmailEnabled, p.DigestMode, p.DisabledTypes, p.UserID)
	var updated user.NotificationPrefs
	if err := row.Scan(&updated.UserID, &updated.EmailEnabled, &updated.DigestMode, &updated.DisabledTypes, &updated.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return &updated, nil
}

func (r *userRepo) CreateClaimToken(ctx context.Context, anonUserID, createdBy uuid.UUID, tokenHash string, expiresAt time.Time) (*user.ClaimToken, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO claim_tokens (anon_user_id, created_by, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, anon_user_id, created_by, token_hash, expires_at, used_at, created_at
	`, anonUserID, createdBy, tokenHash, expiresAt)
	var ct user.ClaimToken
	if err := row.Scan(&ct.ID, &ct.AnonUserID, &ct.CreatedBy, &ct.TokenHash, &ct.ExpiresAt, &ct.UsedAt, &ct.CreatedAt); err != nil {
		return nil, fmt.Errorf("create claim token: %w", err)
	}
	return &ct, nil
}

// Claim atomically merges the anonymous user into the claiming user in a single transaction.
// Steps:
//  1. Validate + consume claim token
//  2. Mark anon user as claimed
//  3. Reassign all financial references
//  4. Handle team membership conflicts with INSERT ... ON CONFLICT DO NOTHING + DELETE
func (r *userRepo) Claim(ctx context.Context, tokenHash string, claimedByID uuid.UUID) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Consume the claim token and get the anon user ID.
	var anonUserID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE claim_tokens
		SET used_at = NOW()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
		RETURNING anon_user_id
	`, tokenHash).Scan(&anonUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, user.ErrClaimTokenExpired
		}
		return uuid.Nil, fmt.Errorf("consume claim token: %w", err)
	}

	// 2. Mark the anonymous user as claimed.
	tag, err := tx.Exec(ctx, `
		UPDATE users SET claimed_by = $1, claimed_at = NOW()
		WHERE id = $2 AND identity_type = 'anonymous' AND claimed_by IS NULL
	`, claimedByID, anonUserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("mark claimed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return uuid.Nil, user.ErrAnonAlreadyClaimed
	}

	// 3. Reassign financial references from anon → claiming user.
	for _, q := range []string{
		`UPDATE expenses SET created_by = $1 WHERE created_by = $2`,
		`UPDATE expenses SET paid_by = $1 WHERE paid_by = $2`,
		`UPDATE expense_splits SET user_id = $1 WHERE user_id = $2`,
		`UPDATE settlements SET payer_id = $1 WHERE payer_id = $2`,
		`UPDATE settlements SET payee_id = $1 WHERE payee_id = $2`,
	} {
		if _, err := tx.Exec(ctx, q, claimedByID, anonUserID); err != nil {
			return uuid.Nil, fmt.Errorf("reassign reference: %w", err)
		}
	}

	// 4. Reassign team memberships; skip teams where claiming user is already a member.
	//    INSERT ... ON CONFLICT DO NOTHING preserves the claiming user's existing membership.
	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (id, team_id, user_id, role, status, invited_by, request_message, resolved_by, resolved_at, joined_at, created_at)
		SELECT gen_random_uuid(), team_id, $1, role, status, invited_by, request_message, resolved_by, resolved_at, joined_at, created_at
		FROM team_members
		WHERE user_id = $2
		ON CONFLICT (team_id, user_id) DO NOTHING
	`, claimedByID, anonUserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("reassign memberships: %w", err)
	}

	// Delete the original anon membership rows.
	if _, err = tx.Exec(ctx, `DELETE FROM team_members WHERE user_id = $1`, anonUserID); err != nil {
		return uuid.Nil, fmt.Errorf("delete anon memberships: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit claim: %w", err)
	}
	return anonUserID, nil
}

// insertUser inserts a user row, working against either a pool or an in-flight transaction.
func insertUser(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, u *user.User) (*user.User, error) {
	row := q.QueryRow(ctx, `
		INSERT INTO users (identity_type, display_name, email, password_hash, avatar_url, currency_pref, timezone)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, identity_type, display_name, email, password_hash, avatar_url,
		          currency_pref, timezone, claimed_by, claimed_at, created_at, deleted_at, deleted_by
	`, u.IdentityType, u.DisplayName, u.Email, u.PasswordHash, u.AvatarURL, u.CurrencyPref, u.Timezone)

	created, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, user.ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return created, nil
}

func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	err := row.Scan(
		&u.ID, &u.IdentityType, &u.DisplayName, &u.Email, &u.PasswordHash, &u.AvatarURL,
		&u.CurrencyPref, &u.Timezone, &u.ClaimedBy, &u.ClaimedAt, &u.CreatedAt, &u.DeletedAt, &u.DeletedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func isUniqueViolation(err error) bool {
	type pgErr interface{ SQLState() string }
	var pg pgErr
	if errors.As(err, &pg) {
		return pg.SQLState() == "23505"
	}
	return false
}
