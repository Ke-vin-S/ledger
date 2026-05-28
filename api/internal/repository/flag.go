package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/flag"
)

type flagRepo struct {
	pool *pgxpool.Pool
}

func NewFlagRepo(pool *pgxpool.Pool) flag.Repository {
	return &flagRepo{pool: pool}
}

// ── Create ────────────────────────────────────────────────────────────────────

func (r *flagRepo) Create(ctx context.Context, f *flag.Flag) (*flag.Flag, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO expense_flags (expense_id, raised_by, reason, status)
		VALUES ($1, $2, $3, 'open')
		RETURNING id, expense_id, raised_by, reason, status,
		          resolved_by, resolution_note, resolved_at, created_at
	`, f.ExpenseID, f.RaisedBy, f.Reason)
	return scanFlag(row)
}

// ── FindByID ──────────────────────────────────────────────────────────────────

func (r *flagRepo) FindByID(ctx context.Context, id uuid.UUID) (*flag.Flag, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, expense_id, raised_by, reason, status,
		       resolved_by, resolution_note, resolved_at, created_at
		FROM expense_flags WHERE id = $1
	`, id)
	f, err := scanFlag(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, flag.ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// ── ListByExpense ─────────────────────────────────────────────────────────────

func (r *flagRepo) ListByExpense(ctx context.Context, expenseID uuid.UUID) ([]*flag.Flag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, expense_id, raised_by, reason, status,
		       resolved_by, resolution_note, resolved_at, created_at
		FROM expense_flags WHERE expense_id = $1
		ORDER BY created_at ASC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*flag.Flag, 0)
	for rows.Next() {
		f, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ── Resolve ───────────────────────────────────────────────────────────────────

func (r *flagRepo) Resolve(ctx context.Context, id, resolvedBy uuid.UUID, note string) (*flag.Flag, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE expense_flags
		SET status='resolved', resolved_by=$1, resolution_note=$2, resolved_at=NOW()
		WHERE id=$3 AND status='open'
		RETURNING id, expense_id, raised_by, reason, status,
		          resolved_by, resolution_note, resolved_at, created_at
	`, resolvedBy, note, id)
	f, err := scanFlag(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, flag.ErrAlreadyResolved
		}
		return nil, err
	}
	return f, nil
}

// ── scanner ───────────────────────────────────────────────────────────────────

func scanFlag(row pgx.Row) (*flag.Flag, error) {
	var f flag.Flag
	err := row.Scan(
		&f.ID, &f.ExpenseID, &f.RaisedBy, &f.Reason, &f.Status,
		&f.ResolvedBy, &f.ResolutionNote, &f.ResolvedAt, &f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
