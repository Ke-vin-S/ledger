package settlement

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence interface for settlements and balance views.
type Repository interface {
	Create(ctx context.Context, s *Settlement) (*Settlement, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Settlement, error)
	ListByExpense(ctx context.Context, expenseID uuid.UUID) ([]*Settlement, error)

	// Confirm transitions status to 'confirmed'. Returns ErrInvalidStatus if not pending.
	Confirm(ctx context.Context, id, confirmedBy uuid.UUID) (*Settlement, error)
	// Dispute transitions status to 'disputed'. Returns ErrInvalidStatus if not pending.
	Dispute(ctx context.Context, id, disputedBy uuid.UUID, reason string) (*Settlement, error)

	// Balance views
	GetDebtBalance(ctx context.Context, expenseID, debtorID uuid.UUID) (*DebtBalance, error)
	ListTeamNetBalances(ctx context.Context, teamID uuid.UUID) ([]*TeamNetBalance, error)
	ListUserNetBalances(ctx context.Context, userID uuid.UUID) ([]*UserNetBalance, error)
}
