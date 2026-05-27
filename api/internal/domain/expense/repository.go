package expense

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence interface for expenses and splits.
type Repository interface {
	// Create inserts expense + splits in a single transaction.
	Create(ctx context.Context, e *Expense, splits []ExpenseSplit) (*Expense, []ExpenseSplit, error)

	FindByID(ctx context.Context, id uuid.UUID) (*Expense, error)
	FindSplitsByExpenseID(ctx context.Context, expenseID uuid.UUID, version int) ([]ExpenseSplit, error)

	// ListForTeam returns expenses for a team ordered by expense_date DESC.
	ListForTeam(ctx context.Context, teamID uuid.UUID, includeVoid bool) ([]*Expense, error)
	// ListForUser returns personal/direct expenses where paid_by = userID or user is a split participant.
	ListForUser(ctx context.Context, userID uuid.UUID, includeVoid bool) ([]*Expense, error)

	// SaveCorrection atomically snapshots the current version, inserts new splits,
	// and updates the expense row to the corrected state.
	SaveCorrection(ctx context.Context, expenseID uuid.UUID, snapshot any, newExpense *Expense, newSplits []ExpenseSplit) (*Expense, []ExpenseSplit, error)

	Void(ctx context.Context, expenseID, voidedBy uuid.UUID, reason string) error

	UpdateReceiptURL(ctx context.Context, expenseID uuid.UUID, url string) error
}
