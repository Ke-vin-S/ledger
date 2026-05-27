package expense

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	ScopePersonal = "personal"
	ScopeTeam     = "team"
	ScopeDirect   = "direct"

	MethodEqual      = "equal"
	MethodExact      = "exact"
	MethodPercentage = "percentage"
	MethodShares     = "shares"
)

var (
	ErrNotFound         = errors.New("expense not found")
	ErrForbidden        = errors.New("insufficient permission")
	ErrAlreadyVoided    = errors.New("expense is already voided")
	ErrInvalidSplitSum  = errors.New("split amounts do not sum to expense amount")
	ErrInvalidSplitData = errors.New("invalid split data")
	ErrInvalidInput     = errors.New("invalid input")
)

// Expense is the core financial event.
type Expense struct {
	ID          uuid.UUID  `json:"id"`
	Scope       string     `json:"scope"`
	TeamID      *uuid.UUID `json:"team_id,omitempty"`
	Title       string     `json:"title"`
	Amount      int64      `json:"amount"`
	Currency    string     `json:"currency"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty"`
	PaidBy      uuid.UUID  `json:"paid_by"`
	ExpenseDate time.Time  `json:"expense_date"`
	SplitMethod *string    `json:"split_method,omitempty"`
	ReceiptURL  *string    `json:"receipt_url,omitempty"`
	Note        *string    `json:"note,omitempty"`
	Version     int        `json:"version"`
	IsVoid      bool       `json:"is_void"`
	VoidReason  *string    `json:"void_reason,omitempty"`
	VoidedBy    *uuid.UUID `json:"voided_by,omitempty"`
	VoidedAt    *time.Time `json:"voided_at,omitempty"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ExpenseSplit is one participant's share of an expense.
type ExpenseSplit struct {
	ID          uuid.UUID `json:"id"`
	ExpenseID   uuid.UUID `json:"expense_id"`
	UserID      uuid.UUID `json:"user_id"`
	ShareAmount int64     `json:"share_amount"`
	ShareUnits  *float64  `json:"share_units,omitempty"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExpenseVersion is a historical snapshot created on every correction.
type ExpenseVersion struct {
	ID               uuid.UUID `json:"id"`
	ExpenseID        uuid.UUID `json:"expense_id"`
	Version          int       `json:"version"`
	Snapshot         any       `json:"snapshot"`
	CorrectedBy      uuid.UUID `json:"corrected_by"`
	CorrectionReason *string   `json:"correction_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ExpenseWithSplits is the standard service response.
type ExpenseWithSplits struct {
	Expense
	Splits []ExpenseSplit `json:"splits"`
}

// SplitInput is supplied by callers for split computation.
type SplitInput struct {
	UserID      uuid.UUID
	ShareAmount int64   // exact method
	ShareUnits  float64 // percentage / shares methods
}

// SplitEntry is a computed split ready for persistence.
type SplitEntry struct {
	UserID      uuid.UUID
	ShareAmount int64
	ShareUnits  *float64
}

// CreateInput is the argument for Service.CreateExpense.
type CreateInput struct {
	Scope       string
	TeamID      *uuid.UUID // required for team scope
	Title       string
	Amount      int64
	Currency    string
	CategoryID  *uuid.UUID
	PaidBy      uuid.UUID
	ExpenseDate time.Time
	SplitMethod *string    // nil for personal
	Splits      []SplitInput
	BorrowerID  *uuid.UUID // required for direct scope
	Note        *string
}

// CorrectInput supplies the full replacement state for a correction.
// For team expenses all split fields must be provided.
type CorrectInput struct {
	Title            string
	Amount           int64
	Currency         string
	CategoryID       *uuid.UUID
	PaidBy           uuid.UUID
	ExpenseDate      time.Time
	SplitMethod      *string
	Splits           []SplitInput
	Note             *string
	ReceiptURL       *string
	CorrectionReason *string
}

// Presigner generates pre-signed PUT URLs for receipt uploads.
type Presigner interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
}

// TeamGateway is a narrow read-only view into team membership.
// It avoids importing domain/team from domain/expense.
type TeamGateway interface {
	GetMembership(ctx context.Context, teamID, userID uuid.UUID) (role, status string, err error)
}
