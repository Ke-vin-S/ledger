package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/settlement"
)

type settlementRepo struct {
	pool *pgxpool.Pool
}

func NewSettlementRepo(pool *pgxpool.Pool) settlement.Repository {
	return &settlementRepo{pool: pool}
}

// ── Create ────────────────────────────────────────────────────────────────────

func (r *settlementRepo) Create(ctx context.Context, s *settlement.Settlement) (*settlement.Settlement, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO settlements
			(expense_id, payer_id, payee_id, amount, method, method_note,
			 status, recorded_by, settled_on)
		VALUES ($1,$2,$3,$4,$5,$6,'pending_confirmation',$7,$8)
		RETURNING id, expense_id, payer_id, payee_id, amount, method, method_note,
		          status, recorded_by, confirmed_by, confirmed_at,
		          disputed_by, disputed_at, dispute_reason, settled_on, created_at
	`, s.ExpenseID, s.PayerID, s.PayeeID, s.Amount, s.Method, s.MethodNote,
		s.RecordedBy, s.SettledOn.Format("2006-01-02"))
	return scanSettlement(row)
}

// ── FindByID ──────────────────────────────────────────────────────────────────

func (r *settlementRepo) FindByID(ctx context.Context, id uuid.UUID) (*settlement.Settlement, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, expense_id, payer_id, payee_id, amount, method, method_note,
		       status, recorded_by, confirmed_by, confirmed_at,
		       disputed_by, disputed_at, dispute_reason, settled_on, created_at
		FROM settlements WHERE id = $1
	`, id)
	s, err := scanSettlement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, settlement.ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

// ── ListByExpense ─────────────────────────────────────────────────────────────

func (r *settlementRepo) ListByExpense(ctx context.Context, expenseID uuid.UUID) ([]*settlement.Settlement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, expense_id, payer_id, payee_id, amount, method, method_note,
		       status, recorded_by, confirmed_by, confirmed_at,
		       disputed_by, disputed_at, dispute_reason, settled_on, created_at
		FROM settlements WHERE expense_id = $1
		ORDER BY created_at ASC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*settlement.Settlement
	for rows.Next() {
		s, err := scanSettlement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── Confirm ───────────────────────────────────────────────────────────────────

func (r *settlementRepo) Confirm(ctx context.Context, id, confirmedBy uuid.UUID) (*settlement.Settlement, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE settlements
		SET status='confirmed', confirmed_by=$1, confirmed_at=NOW()
		WHERE id=$2 AND status='pending_confirmation'
		RETURNING id, expense_id, payer_id, payee_id, amount, method, method_note,
		          status, recorded_by, confirmed_by, confirmed_at,
		          disputed_by, disputed_at, dispute_reason, settled_on, created_at
	`, confirmedBy, id)
	s, err := scanSettlement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, settlement.ErrInvalidStatus
		}
		return nil, err
	}
	return s, nil
}

// ── Dispute ───────────────────────────────────────────────────────────────────

func (r *settlementRepo) Dispute(ctx context.Context, id, disputedBy uuid.UUID, reason string) (*settlement.Settlement, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE settlements
		SET status='disputed', disputed_by=$1, disputed_at=NOW(), dispute_reason=$2
		WHERE id=$3 AND status='pending_confirmation'
		RETURNING id, expense_id, payer_id, payee_id, amount, method, method_note,
		          status, recorded_by, confirmed_by, confirmed_at,
		          disputed_by, disputed_at, dispute_reason, settled_on, created_at
	`, disputedBy, reason, id)
	s, err := scanSettlement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, settlement.ErrInvalidStatus
		}
		return nil, err
	}
	return s, nil
}

// ── Balance views ─────────────────────────────────────────────────────────────

func (r *settlementRepo) GetDebtBalance(ctx context.Context, expenseID, debtorID uuid.UUID) (*settlement.DebtBalance, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT expense_id, debtor_id, creditor_id, team_id,
		       total_share, total_settled, balance, debt_status
		FROM debt_balances
		WHERE expense_id = $1 AND debtor_id = $2
	`, expenseID, debtorID)

	var d settlement.DebtBalance
	err := row.Scan(
		&d.ExpenseID, &d.DebtorID, &d.CreditorID, &d.TeamID,
		&d.TotalShare, &d.TotalSettled, &d.Balance, &d.DebtStatus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, settlement.ErrNoDebt
		}
		return nil, err
	}
	return &d, nil
}

func (r *settlementRepo) ListTeamNetBalances(ctx context.Context, teamID uuid.UUID) ([]*settlement.TeamNetBalance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT team_id, user_a, user_b, net_amount
		FROM team_net_balances
		WHERE team_id = $1
		ORDER BY user_a, user_b
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*settlement.TeamNetBalance
	for rows.Next() {
		var nb settlement.TeamNetBalance
		if err := rows.Scan(&nb.TeamID, &nb.UserA, &nb.UserB, &nb.NetAmount); err != nil {
			return nil, err
		}
		out = append(out, &nb)
	}
	return out, rows.Err()
}

func (r *settlementRepo) ListUserNetBalances(ctx context.Context, userID uuid.UUID) ([]*settlement.UserNetBalance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT counterparty_id, net_amount
		FROM user_net_balances
		WHERE user_id = $1
		ORDER BY net_amount DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*settlement.UserNetBalance
	for rows.Next() {
		var nb settlement.UserNetBalance
		if err := rows.Scan(&nb.CounterpartyID, &nb.NetAmount); err != nil {
			return nil, err
		}
		out = append(out, &nb)
	}
	return out, rows.Err()
}

// ── scanners ──────────────────────────────────────────────────────────────────

func scanSettlement(row pgx.Row) (*settlement.Settlement, error) {
	var s settlement.Settlement
	var settledOnRaw time.Time
	err := row.Scan(
		&s.ID, &s.ExpenseID, &s.PayerID, &s.PayeeID, &s.Amount,
		&s.Method, &s.MethodNote, &s.Status, &s.RecordedBy,
		&s.ConfirmedBy, &s.ConfirmedAt,
		&s.DisputedBy, &s.DisputedAt, &s.DisputeReason,
		&settledOnRaw, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.SettledOn = settledOnRaw
	return &s, nil
}

