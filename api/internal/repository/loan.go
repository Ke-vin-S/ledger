package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/loan"
)

type loanRepo struct {
	pool *pgxpool.Pool
}

func NewLoanRepo(pool *pgxpool.Pool) loan.Repository {
	return &loanRepo{pool: pool}
}

func (r *loanRepo) Create(ctx context.Context, in loan.CreateInput) (*loan.Loan, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO loans
			(user_id, direction, amount, currency, counterparty_id, counterparty_name, note, loan_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, direction, amount, currency, counterparty_id, counterparty_name,
		          note, status, acknowledged_at, loan_date, created_at
	`, in.UserID, in.Direction, in.Amount, in.Currency,
		in.CounterpartyID, in.CounterpartyName, in.Note,
		in.LoanDate.Format("2006-01-02"))
	return scanLoan(row)
}

func (r *loanRepo) FindByID(ctx context.Context, id uuid.UUID) (*loan.Loan, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, direction, amount, currency, counterparty_id, counterparty_name,
		       note, status, acknowledged_at, loan_date, created_at
		FROM loans WHERE id = $1
	`, id)
	l, err := scanLoan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, loan.ErrNotFound
		}
		return nil, err
	}
	repayments, err := r.listRepayments(ctx, id)
	if err != nil {
		return nil, err
	}
	l.Repayments = repayments
	return l, nil
}

func (r *loanRepo) ListByUser(ctx context.Context, userID uuid.UUID, direction *string) ([]*loan.Loan, error) {
	args := []any{userID}
	q := `SELECT id, user_id, direction, amount, currency, counterparty_id, counterparty_name,
	             note, status, acknowledged_at, loan_date, created_at
	      FROM loans WHERE user_id = $1`
	if direction != nil {
		args = append(args, *direction)
		q += ` AND direction = $2`
	}
	q += ` ORDER BY loan_date DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*loan.Loan
	for rows.Next() {
		l, err := scanLoan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *loanRepo) Acknowledge(ctx context.Context, id uuid.UUID) (*loan.Loan, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE loans
		SET acknowledged_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, direction, amount, currency, counterparty_id, counterparty_name,
		          note, status, acknowledged_at, loan_date, created_at
	`, id)
	l, err := scanLoan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, loan.ErrNotFound
		}
		return nil, err
	}
	return l, nil
}

func (r *loanRepo) Dispute(ctx context.Context, id uuid.UUID, _ *string) (*loan.Loan, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE loans
		SET status = 'disputed', updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, direction, amount, currency, counterparty_id, counterparty_name,
		          note, status, acknowledged_at, loan_date, created_at
	`, id)
	l, err := scanLoan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, loan.ErrNotFound
		}
		return nil, err
	}
	return l, nil
}

func (r *loanRepo) listRepayments(ctx context.Context, loanID uuid.UUID) ([]loan.Repayment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, amount, note, repaid_at
		FROM loan_repayments
		WHERE loan_id = $1
		ORDER BY repaid_at ASC
	`, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []loan.Repayment
	for rows.Next() {
		var rep loan.Repayment
		var repaidAt time.Time
		if err := rows.Scan(&rep.ID, &rep.Amount, &rep.Note, &repaidAt); err != nil {
			return nil, err
		}
		rep.RepaidAt = repaidAt
		out = append(out, rep)
	}
	return out, rows.Err()
}

type loanScanner interface {
	Scan(dest ...any) error
}

func scanLoan(row loanScanner) (*loan.Loan, error) {
	var l loan.Loan
	var loanDate time.Time
	err := row.Scan(
		&l.ID, &l.UserID, &l.Direction, &l.Amount, &l.Currency,
		&l.CounterpartyID, &l.CounterpartyName, &l.Note,
		&l.Status, &l.AcknowledgedAt, &loanDate, &l.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	l.LoanDate = loanDate
	return &l, nil
}
