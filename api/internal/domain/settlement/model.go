package settlement

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending   = "pending_confirmation"
	StatusConfirmed = "confirmed"
	StatusDisputed  = "disputed"

	MethodCash         = "cash"
	MethodBankTransfer = "bank_transfer"
	MethodUPI          = "upi"
	MethodCard         = "card"
	MethodOther        = "other"
)

var validMethods = map[string]bool{
	MethodCash: true, MethodBankTransfer: true,
	MethodUPI: true, MethodCard: true, MethodOther: true,
}

var (
	ErrNotFound             = errors.New("settlement not found")
	ErrForbidden            = errors.New("insufficient permission")
	ErrInvalidInput         = errors.New("invalid input")
	ErrSettlementExceedsDebt = errors.New("settlement amount exceeds outstanding balance")
	ErrInvalidStatus        = errors.New("settlement cannot be modified in its current status")
	ErrNoDebt               = errors.New("no outstanding debt for this expense/debtor pair")
)

// Settlement records that a debtor paid a creditor some amount toward an expense.
type Settlement struct {
	ID            uuid.UUID  `json:"id"`
	ExpenseID     uuid.UUID  `json:"expense_id"`
	PayerID       uuid.UUID  `json:"payer_id"`
	PayeeID       uuid.UUID  `json:"payee_id"`
	Amount        int64      `json:"amount"`
	Method        string     `json:"method"`
	MethodNote    *string    `json:"method_note,omitempty"`
	Status        string     `json:"status"`
	RecordedBy    uuid.UUID  `json:"recorded_by"`
	ConfirmedBy   *uuid.UUID `json:"confirmed_by,omitempty"`
	ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
	DisputedBy    *uuid.UUID `json:"disputed_by,omitempty"`
	DisputedAt    *time.Time `json:"disputed_at,omitempty"`
	DisputeReason *string    `json:"dispute_reason,omitempty"`
	SettledOn     time.Time  `json:"settled_on"`
	CreatedAt     time.Time  `json:"created_at"`
}

// DebtBalance is a row from the debt_balances view.
type DebtBalance struct {
	ExpenseID    uuid.UUID  `json:"expense_id"`
	DebtorID     uuid.UUID  `json:"debtor_id"`
	CreditorID   uuid.UUID  `json:"creditor_id"`
	TeamID       *uuid.UUID `json:"team_id,omitempty"`
	TotalShare   int64      `json:"total_share"`
	TotalSettled int64      `json:"total_settled"`
	Balance      int64      `json:"balance"`
	DebtStatus   string     `json:"debt_status"`
}

// TeamBalance is a net balance between the current user and one team counterparty.
// NetAmount > 0 means the counterparty owes the actor; < 0 means the actor owes them.
type TeamBalance struct {
	CounterpartyID   uuid.UUID `json:"counterparty_id"`
	CounterpartyName string    `json:"counterparty_name"`
	NetAmount        int64     `json:"net_amount"`
}

// UserBalance is a net balance between the current user and one counterparty across all teams.
// NetAmount > 0 means the counterparty owes the actor; < 0 means the actor owes them.
type UserBalance struct {
	CounterpartyID   uuid.UUID `json:"counterparty_id"`
	CounterpartyName string    `json:"counterparty_name"`
	NetAmount        int64     `json:"net_amount"`
}

// RecordInput is the argument for Service.RecordSettlement.
type RecordInput struct {
	ExpenseID  uuid.UUID
	PayerID    uuid.UUID
	PayeeID    uuid.UUID
	Amount     int64
	Method     string
	MethodNote *string
	SettledOn  time.Time
}

// TeamGateway is a narrow read-only view into team membership.
type TeamGateway interface {
	GetMembership(ctx context.Context, teamID, userID uuid.UUID) (role, status string, err error)
}
