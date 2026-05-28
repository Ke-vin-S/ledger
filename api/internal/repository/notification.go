package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/notification"
)

type notificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) notification.Repository {
	return &notificationRepo{pool: pool}
}

// ── List ──────────────────────────────────────────────────────────────────────

func (r *notificationRepo) List(ctx context.Context, p notification.ListParams) ([]*notification.Notification, error) {
	args := []any{p.UserID, p.Limit}
	q := `
		SELECT id, user_id, type, entity_type, entity_id, payload,
		       is_read, read_at, created_at
		FROM notifications
		WHERE user_id = $1`

	if p.UnreadOnly {
		q += ` AND is_read = FALSE`
	}
	if p.Cursor != "" {
		ts, err := time.Parse(time.RFC3339Nano, p.Cursor)
		if err == nil {
			args = append(args, ts)
			q += ` AND created_at < $` + itoa(len(args))
		}
	}
	q += ` ORDER BY created_at DESC LIMIT $2`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*notification.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ── FindByID ──────────────────────────────────────────────────────────────────

func (r *notificationRepo) FindByID(ctx context.Context, id uuid.UUID) (*notification.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, type, entity_type, entity_id, payload,
		       is_read, read_at, created_at
		FROM notifications WHERE id = $1
	`, id)
	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notification.ErrNotFound
		}
		return nil, err
	}
	return n, nil
}

// ── MarkRead ──────────────────────────────────────────────────────────────────

func (r *notificationRepo) MarkRead(ctx context.Context, id, userID uuid.UUID) (*notification.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE id = $1 AND user_id = $2 AND is_read = FALSE
		RETURNING id, user_id, type, entity_type, entity_id, payload,
		          is_read, read_at, created_at
	`, id, userID)
	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Distinguish not-found from wrong-user via a second look-up.
			existing, lookupErr := r.FindByID(ctx, id)
			if lookupErr != nil {
				return nil, notification.ErrNotFound
			}
			if existing.UserID != userID {
				return nil, notification.ErrForbidden
			}
			// Row exists and belongs to user — already read; return it.
			return existing, nil
		}
		return nil, err
	}
	return n, nil
}

// ── MarkAllRead ───────────────────────────────────────────────────────────────

func (r *notificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE user_id = $1 AND is_read = FALSE
	`, userID)
	return err
}

// ── Delete ────────────────────────────────────────────────────────────────────

func (r *notificationRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM notifications WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Distinguish not-found from wrong-user.
		existing, lookupErr := r.FindByID(ctx, id)
		if lookupErr != nil {
			return notification.ErrNotFound
		}
		if existing.UserID != userID {
			return notification.ErrForbidden
		}
	}
	return nil
}

// ── Prefs ─────────────────────────────────────────────────────────────────────

func (r *notificationRepo) GetPrefs(ctx context.Context, userID uuid.UUID) (*notification.NotificationPrefs, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, email_enabled, digest_mode, disabled_types, updated_at
		FROM notification_prefs WHERE user_id = $1
	`, userID)
	return scanPrefs(row)
}

func (r *notificationRepo) UpsertPrefs(ctx context.Context, p *notification.NotificationPrefs) (*notification.NotificationPrefs, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO notification_prefs (user_id, email_enabled, digest_mode, disabled_types)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET email_enabled  = EXCLUDED.email_enabled,
		    digest_mode    = EXCLUDED.digest_mode,
		    disabled_types = EXCLUDED.disabled_types,
		    updated_at     = NOW()
		RETURNING id, user_id, email_enabled, digest_mode, disabled_types, updated_at
	`, p.UserID, p.EmailEnabled, p.DigestMode, p.DisabledTypes)
	return scanPrefs(row)
}

// ── scanners ──────────────────────────────────────────────────────────────────

func scanNotification(row pgx.Row) (*notification.Notification, error) {
	var n notification.Notification
	err := row.Scan(
		&n.ID, &n.UserID, &n.Type, &n.EntityType, &n.EntityID,
		&n.Payload, &n.IsRead, &n.ReadAt, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func scanPrefs(row pgx.Row) (*notification.NotificationPrefs, error) {
	var p notification.NotificationPrefs
	err := row.Scan(
		&p.ID, &p.UserID, &p.EmailEnabled, &p.DigestMode, &p.DisabledTypes, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notification.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// itoa converts a small integer to its decimal string — avoids importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
