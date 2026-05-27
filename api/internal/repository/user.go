package repository

import (
	"context"
	"errors"
	"fmt"

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

	_, err = tx.Exec(ctx, `INSERT INTO notification_prefs (user_id) VALUES ($1)`, created.ID)
	if err != nil {
		return nil, fmt.Errorf("create notification_prefs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return created, nil
}

func (r *userRepo) CreateAnonymous(ctx context.Context, displayName string, createdBy uuid.UUID) (*user.User, error) {
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

func (r *userRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID)
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

// insertUser inserts a user row using either a pool or a transaction (both implement pgx.Querier via QueryRow).
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
	// pgx wraps pgconn.PgError; check the SQLState code 23505 (unique_violation).
	type pgErr interface{ SQLState() string }
	var pg pgErr
	if errors.As(err, &pg) {
		return pg.SQLState() == "23505"
	}
	return false
}
