package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
)

type expenseRepo struct {
	pool *pgxpool.Pool
}

func NewExpenseRepo(pool *pgxpool.Pool) expense.Repository {
	return &expenseRepo{pool: pool}
}

// ── Create ────────────────────────────────────────────────────────────────────

func (r *expenseRepo) Create(ctx context.Context, e *expense.Expense, splits []expense.ExpenseSplit) (*expense.Expense, []expense.ExpenseSplit, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO expenses
			(scope, team_id, title, amount, currency, category_id, paid_by,
			 expense_date, split_method, note, version, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, scope, team_id, title, amount, currency, category_id, paid_by,
		          expense_date, split_method, receipt_url, note, version,
		          is_void, void_reason, voided_by, voided_at, created_by, created_at
	`, e.Scope, e.TeamID, e.Title, e.Amount, e.Currency, e.CategoryID, e.PaidBy,
		e.ExpenseDate.Format("2006-01-02"), e.SplitMethod, e.Note, e.Version, e.CreatedBy)

	created, err := scanExpense(row)
	if err != nil {
		return nil, nil, fmt.Errorf("insert expense: %w", err)
	}

	createdSplits, err := insertSplits(ctx, tx, created.ID, splits)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	return created, createdSplits, nil
}

// ── FindByID ──────────────────────────────────────────────────────────────────

func (r *expenseRepo) FindByID(ctx context.Context, id uuid.UUID) (*expense.Expense, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, scope, team_id, title, amount, currency, category_id, paid_by,
		       expense_date, split_method, receipt_url, note, version,
		       is_void, void_reason, voided_by, voided_at, created_by, created_at
		FROM expenses WHERE id = $1
	`, id)
	e, err := scanExpense(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, expense.ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

// ── FindSplitsByExpenseID ─────────────────────────────────────────────────────

func (r *expenseRepo) FindSplitsByExpenseID(ctx context.Context, expenseID uuid.UUID, version int) ([]expense.ExpenseSplit, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, expense_id, user_id, share_amount, share_units, version, created_at
		FROM expense_splits
		WHERE expense_id = $1 AND version = $2
		ORDER BY created_at ASC
	`, expenseID, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSplits(rows)
}

// ── ListForTeam ───────────────────────────────────────────────────────────────

func (r *expenseRepo) ListForTeam(ctx context.Context, teamID uuid.UUID, includeVoid bool) ([]*expense.Expense, error) {
	q := `
		SELECT id, scope, team_id, title, amount, currency, category_id, paid_by,
		       expense_date, split_method, receipt_url, note, version,
		       is_void, void_reason, voided_by, voided_at, created_by, created_at
		FROM expenses
		WHERE team_id = $1
	`
	if !includeVoid {
		q += " AND is_void = false"
	}
	q += " ORDER BY expense_date DESC, created_at DESC LIMIT 100"

	rows, err := r.pool.Query(ctx, q, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExpenses(rows)
}

// ── ListForUser ───────────────────────────────────────────────────────────────

func (r *expenseRepo) ListForUser(ctx context.Context, userID uuid.UUID, includeVoid bool) ([]*expense.Expense, error) {
	q := `
		SELECT DISTINCT e.id, e.scope, e.team_id, e.title, e.amount, e.currency,
		       e.category_id, e.paid_by, e.expense_date, e.split_method, e.receipt_url,
		       e.note, e.version, e.is_void, e.void_reason, e.voided_by, e.voided_at,
		       e.created_by, e.created_at
		FROM expenses e
		LEFT JOIN expense_splits es ON es.expense_id = e.id AND es.version = e.version
		WHERE (e.paid_by = $1 OR es.user_id = $1)
	`
	if !includeVoid {
		q += " AND e.is_void = false"
	}
	q += " ORDER BY e.expense_date DESC, e.created_at DESC LIMIT 100"

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExpenses(rows)
}

// ── SaveCorrection ────────────────────────────────────────────────────────────

func (r *expenseRepo) SaveCorrection(ctx context.Context, expenseID uuid.UUID, snapshot any, newExpense *expense.Expense, newSplits []expense.ExpenseSplit) (*expense.Expense, []expense.ExpenseSplit, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	prevVersion := newExpense.Version - 1
	_, err = tx.Exec(ctx, `
		INSERT INTO expense_versions (expense_id, version, snapshot, corrected_by, correction_reason)
		VALUES ($1, $2, $3, $4, $5)
	`, expenseID, prevVersion, snapshotJSON, newExpense.CreatedBy, newExpense.Note)
	if err != nil {
		return nil, nil, fmt.Errorf("insert version snapshot: %w", err)
	}

	row := tx.QueryRow(ctx, `
		UPDATE expenses
		SET title=$1, amount=$2, currency=$3, category_id=$4, paid_by=$5,
		    expense_date=$6, split_method=$7, receipt_url=$8, note=$9, version=$10
		WHERE id=$11
		RETURNING id, scope, team_id, title, amount, currency, category_id, paid_by,
		          expense_date, split_method, receipt_url, note, version,
		          is_void, void_reason, voided_by, voided_at, created_by, created_at
	`, newExpense.Title, newExpense.Amount, newExpense.Currency, newExpense.CategoryID,
		newExpense.PaidBy, newExpense.ExpenseDate.Format("2006-01-02"),
		newExpense.SplitMethod, newExpense.ReceiptURL, newExpense.Note,
		newExpense.Version, expenseID)

	saved, err := scanExpense(row)
	if err != nil {
		return nil, nil, fmt.Errorf("update expense: %w", err)
	}

	savedSplits, err := insertSplits(ctx, tx, expenseID, newSplits)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	return saved, savedSplits, nil
}

// ── Void ──────────────────────────────────────────────────────────────────────

func (r *expenseRepo) Void(ctx context.Context, expenseID, voidedBy uuid.UUID, reason string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE expenses
		SET is_void=true, void_reason=$1, voided_by=$2, voided_at=NOW()
		WHERE id=$3 AND is_void=false
	`, reason, voidedBy, expenseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return expense.ErrNotFound
	}
	return nil
}

// ── UpdateReceiptURL ──────────────────────────────────────────────────────────

func (r *expenseRepo) UpdateReceiptURL(ctx context.Context, expenseID uuid.UUID, url string) error {
	_, err := r.pool.Exec(ctx, `UPDATE expenses SET receipt_url=$1 WHERE id=$2`, url, expenseID)
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func insertSplits(ctx context.Context, tx pgx.Tx, expenseID uuid.UUID, splits []expense.ExpenseSplit) ([]expense.ExpenseSplit, error) {
	out := make([]expense.ExpenseSplit, 0, len(splits))
	for _, s := range splits {
		var created expense.ExpenseSplit
		err := tx.QueryRow(ctx, `
			INSERT INTO expense_splits (expense_id, user_id, share_amount, share_units, version)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id, expense_id, user_id, share_amount, share_units, version, created_at
		`, expenseID, s.UserID, s.ShareAmount, s.ShareUnits, s.Version).Scan(
			&created.ID, &created.ExpenseID, &created.UserID,
			&created.ShareAmount, &created.ShareUnits, &created.Version, &created.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert split: %w", err)
		}
		out = append(out, created)
	}
	return out, nil
}

func scanExpense(row pgx.Row) (*expense.Expense, error) {
	var e expense.Expense
	var expenseDateRaw time.Time
	err := row.Scan(
		&e.ID, &e.Scope, &e.TeamID, &e.Title, &e.Amount, &e.Currency,
		&e.CategoryID, &e.PaidBy, &expenseDateRaw, &e.SplitMethod,
		&e.ReceiptURL, &e.Note, &e.Version,
		&e.IsVoid, &e.VoidReason, &e.VoidedBy, &e.VoidedAt,
		&e.CreatedBy, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.ExpenseDate = expenseDateRaw
	return &e, nil
}

func scanExpenses(rows pgx.Rows) ([]*expense.Expense, error) {
	var out []*expense.Expense
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanSplits(rows pgx.Rows) ([]expense.ExpenseSplit, error) {
	var out []expense.ExpenseSplit
	for rows.Next() {
		var s expense.ExpenseSplit
		if err := rows.Scan(&s.ID, &s.ExpenseID, &s.UserID, &s.ShareAmount, &s.ShareUnits, &s.Version, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
