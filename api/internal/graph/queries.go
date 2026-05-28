package graph

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/graph/model"
)

// ActivityFeedStore fetches team audit log entries.
type ActivityFeedStore interface {
	QueryTeamActivityFeed(ctx context.Context, teamID uuid.UUID, limit int, before *time.Time) ([]*model.ActivityEntry, error)
}

// DashboardStore fetches balance aggregates for a user.
type DashboardStore interface {
	QueryDashboardAggregates(ctx context.Context, userID uuid.UUID) (*model.DashboardAggregates, error)
}

// ExpenseHistoryStore fetches correction versions for an expense.
type ExpenseHistoryStore interface {
	QueryExpenseHistory(ctx context.Context, expenseID uuid.UUID) ([]*model.ExpenseVersion, error)
}
