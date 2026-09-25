package flag

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusOpen     = "open"
	StatusResolved = "resolved"
)

var (
	ErrNotFound        = errors.New("flag not found")
	ErrForbidden       = errors.New("insufficient permission")
	ErrInvalidInput    = errors.New("invalid input")
	ErrAlreadyResolved = errors.New("flag is already resolved")
)

// ExpenseAccessChecker centralizes expense authorization for flag operations.
type ExpenseAccessChecker interface {
	CanRead(ctx context.Context, actorID, expenseID uuid.UUID) error
	CanWrite(ctx context.Context, actorID, expenseID uuid.UUID) error
}

type Flag struct {
	ID             uuid.UUID  `json:"id"`
	ExpenseID      uuid.UUID  `json:"expense_id"`
	RaisedBy       uuid.UUID  `json:"raised_by"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	ResolvedBy     *uuid.UUID `json:"resolved_by,omitempty"`
	ResolutionNote *string    `json:"resolution_note,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type RaiseInput struct {
	ExpenseID uuid.UUID
	RaisedBy  uuid.UUID
	Reason    string
}

type ResolveInput struct {
	FlagID         uuid.UUID
	ResolvedBy     uuid.UUID
	ResolutionNote string
}
