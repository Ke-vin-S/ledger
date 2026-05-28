package flag

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, f *Flag) (*Flag, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Flag, error)
	ListByExpense(ctx context.Context, expenseID uuid.UUID) ([]*Flag, error)
	Resolve(ctx context.Context, id, resolvedBy uuid.UUID, note string) (*Flag, error)
}
