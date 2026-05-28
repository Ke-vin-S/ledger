package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/graph"
	"github.com/Ke-vin-S/ledger/api/internal/graph/model"
)

// ── ActivityFeedStore ─────────────────────────────────────────────────────────

type pgActivityStore struct{ pool *pgxpool.Pool }

func NewActivityStore(pool *pgxpool.Pool) graph.ActivityFeedStore {
	return &pgActivityStore{pool: pool}
}

func (s *pgActivityStore) QueryTeamActivityFeed(
	ctx context.Context, teamID uuid.UUID, limit int, before *time.Time,
) ([]*model.ActivityEntry, error) {
	args := []any{teamID, limit}
	q := `
		SELECT id::text, action, actor_id::text, entity_type, entity_id::text, meta, created_at
		FROM audit_log
		WHERE team_id = $1`
	if before != nil {
		args = append(args, *before)
		q += ` AND created_at < $` + itoa(len(args))
	}
	q += ` ORDER BY created_at DESC LIMIT $2`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.ActivityEntry
	for rows.Next() {
		var e model.ActivityEntry
		var actorID *string
		var createdAt time.Time
		if err := rows.Scan(&e.ID, &e.Action, &actorID, &e.EntityType, &e.EntityID, &e.Meta, &createdAt); err != nil {
			return nil, err
		}
		e.ActorID = actorID
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, &e)
	}
	return out, rows.Err()
}

// ── DashboardStore ────────────────────────────────────────────────────────────

type pgDashStore struct{ pool *pgxpool.Pool }

func NewDashboardStore(pool *pgxpool.Pool) graph.DashboardStore {
	return &pgDashStore{pool: pool}
}

func (s *pgDashStore) QueryDashboardAggregates(
	ctx context.Context, userID uuid.UUID,
) (*model.DashboardAggregates, error) {
	// user_net_balances: user_id = debtor, counterparty_id = creditor, net_amount = how much user owes counterparty.
	// Positive net_amount: user owes counterparty → totalOwing.
	// Negative net_amount: counterparty owes user → totalOwed (abs value).
	row := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN net_amount < 0 THEN -net_amount ELSE 0 END), 0) AS total_owed,
			COALESCE(SUM(CASE WHEN net_amount > 0 THEN net_amount  ELSE 0 END), 0) AS total_owing
		FROM user_net_balances
		WHERE user_id = $1
	`, userID)

	var agg model.DashboardAggregates
	if err := row.Scan(&agg.TotalOwed, &agg.TotalOwing); err != nil {
		return nil, err
	}
	agg.NetBalance = agg.TotalOwed - agg.TotalOwing
	return &agg, nil
}

// ── ExpenseHistoryStore ───────────────────────────────────────────────────────

type pgHistoryStore struct{ pool *pgxpool.Pool }

func NewExpenseHistoryStore(pool *pgxpool.Pool) graph.ExpenseHistoryStore {
	return &pgHistoryStore{pool: pool}
}

func (s *pgHistoryStore) QueryExpenseHistory(
	ctx context.Context, expenseID uuid.UUID,
) ([]*model.ExpenseVersion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, expense_id::text, version, snapshot,
		       corrected_by::text, correction_reason, created_at
		FROM expense_versions
		WHERE expense_id = $1
		ORDER BY version DESC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.ExpenseVersion
	for rows.Next() {
		var v model.ExpenseVersion
		var createdAt time.Time
		if err := rows.Scan(
			&v.ID, &v.ExpenseID, &v.Version, &v.Snapshot,
			&v.CorrectedBy, &v.CorrectionReason, &createdAt,
		); err != nil {
			return nil, err
		}
		v.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, &v)
	}
	return out, rows.Err()
}
