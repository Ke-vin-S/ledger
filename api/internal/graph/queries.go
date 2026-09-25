package graph

import (
	"context"
	"time"

	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/graph/model"
)

type ActivityCursor struct {
	Time time.Time
	ID   uuid.UUID
}

// ActivityFeedStore fetches team audit log entries.
type ActivityFeedStore interface {
	QueryTeamActivityFeed(ctx context.Context, teamID uuid.UUID, limit int, before *ActivityCursor) ([]*model.ActivityEntry, error)
}

// DashboardStore fetches balance aggregates for a user.
type DashboardStore interface {
	QueryDashboardAggregates(ctx context.Context, userID uuid.UUID) (*model.DashboardAggregates, error)
}

// ExpenseHistoryStore fetches correction versions for an expense.
type ExpenseHistoryStore interface {
	QueryExpenseHistory(ctx context.Context, expenseID uuid.UUID) ([]*model.ExpenseVersion, error)
}

// MembershipChecker authorizes team-scoped GraphQL reads.
type MembershipChecker interface {
	RequireMembership(ctx context.Context, teamID, actorID uuid.UUID, minRole string) error
}

type ExpenseReader interface {
	GetExpense(ctx context.Context, actorID, expenseID uuid.UUID) (*expense.ExpenseWithSplits, error)
}
