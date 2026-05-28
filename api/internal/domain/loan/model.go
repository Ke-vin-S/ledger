package loan

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	DirectionLent     = "lent"
	DirectionBorrowed = "borrowed"

	StatusOutstanding     = "outstanding"
	StatusPartiallyRepaid = "partially_repaid"
	StatusSettled         = "settled"
	StatusDisputed        = "disputed"
)

var (
	ErrNotFound      = errors.New("loan not found")
	ErrForbidden     = errors.New("insufficient permission")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInvalidStatus = errors.New("loan cannot be modified in its current status")
)

type Loan struct {
	ID               uuid.UUID   `json:"id"`
	UserID           uuid.UUID   `json:"user_id"`
	Direction        string      `json:"direction"`
	Amount           int64       `json:"amount"`
	Currency         string      `json:"currency"`
	CounterpartyID   *uuid.UUID  `json:"counterparty_id,omitempty"`
	CounterpartyName string      `json:"counterparty_name"`
	Note             *string     `json:"note,omitempty"`
	Status           string      `json:"status"`
	AcknowledgedAt   *time.Time  `json:"acknowledged_at,omitempty"`
	LoanDate         time.Time   `json:"loan_date"`
	CreatedAt        time.Time   `json:"created_at"`
	Repayments       []Repayment `json:"repayments,omitempty"`
}

type Repayment struct {
	ID       uuid.UUID `json:"id"`
	Amount   int64     `json:"amount"`
	Note     *string   `json:"note,omitempty"`
	RepaidAt time.Time `json:"repaid_at"`
}

type CreateInput struct {
	UserID           uuid.UUID
	Direction        string
	Amount           int64
	Currency         string
	CounterpartyID   *uuid.UUID
	CounterpartyName string
	Note             *string
	LoanDate         time.Time
}

type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Loan, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Loan, error)
	ListByUser(ctx context.Context, userID uuid.UUID, direction *string) ([]*Loan, error)
	Acknowledge(ctx context.Context, id uuid.UUID) (*Loan, error)
	Dispute(ctx context.Context, id uuid.UUID, reason *string) (*Loan, error)
}
