package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/auditlog"
)

type auditLogRepo struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepo(pool *pgxpool.Pool) auditlog.Repository {
	return &auditLogRepo{pool: pool}
}

// ── ListByTeam ────────────────────────────────────────────────────────────────

func (r *auditLogRepo) ListByTeam(ctx context.Context, teamID uuid.UUID, p auditlog.ListParams) ([]*auditlog.LogEntry, error) {
	args := []any{teamID, p.Limit}
	q := `
		SELECT id, action, actor_id, team_id, entity_type, entity_id,
		       before, after, meta, created_at
		FROM audit_log
		WHERE team_id = $1`

	if p.Action != "" {
		args = append(args, p.Action)
		q += ` AND action = $` + itoa(len(args))
	}
	if p.Cursor != "" {
		ts, err := time.Parse(time.RFC3339Nano, p.Cursor)
		if err == nil {
			args = append(args, ts)
			q += ` AND created_at < $` + itoa(len(args))
		}
	}
	q += ` ORDER BY created_at DESC LIMIT $2`

	return r.query(ctx, q, args...)
}

// ── ListByActor ───────────────────────────────────────────────────────────────

func (r *auditLogRepo) ListByActor(ctx context.Context, actorID uuid.UUID, p auditlog.ListParams) ([]*auditlog.LogEntry, error) {
	args := []any{actorID, p.Limit}
	q := `
		SELECT id, action, actor_id, team_id, entity_type, entity_id,
		       before, after, meta, created_at
		FROM audit_log
		WHERE actor_id = $1`

	if p.Action != "" {
		args = append(args, p.Action)
		q += ` AND action = $` + itoa(len(args))
	}
	if p.Cursor != "" {
		ts, err := time.Parse(time.RFC3339Nano, p.Cursor)
		if err == nil {
			args = append(args, ts)
			q += ` AND created_at < $` + itoa(len(args))
		}
	}
	q += ` ORDER BY created_at DESC LIMIT $2`

	return r.query(ctx, q, args...)
}

// ── shared query executor ─────────────────────────────────────────────────────

func (r *auditLogRepo) query(ctx context.Context, q string, args ...any) ([]*auditlog.LogEntry, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*auditlog.LogEntry, 0)
	for rows.Next() {
		var e auditlog.LogEntry
		err := rows.Scan(
			&e.ID, &e.Action, &e.ActorID, &e.TeamID, &e.EntityType, &e.EntityID,
			&e.Before, &e.After, &e.Meta, &e.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
