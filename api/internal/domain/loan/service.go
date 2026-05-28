package loan

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
)

type Service struct {
	repo    Repository
	auditor audit.Logger
}

func NewService(repo Repository, auditor audit.Logger) *Service {
	return &Service{repo: repo, auditor: auditor}
}

func (s *Service) CreateLoan(ctx context.Context, in CreateInput) (*Loan, error) {
	if in.Direction != DirectionLent && in.Direction != DirectionBorrowed {
		return nil, fmt.Errorf("%w: direction must be lent or borrowed", ErrInvalidInput)
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	if in.CounterpartyName == "" {
		return nil, fmt.Errorf("%w: counterparty_name is required", ErrInvalidInput)
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) GetLoan(ctx context.Context, actorID, loanID uuid.UUID) (*Loan, error) {
	l, err := s.repo.FindByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if l.UserID != actorID {
		return nil, ErrForbidden
	}
	return l, nil
}

func (s *Service) ListLoans(ctx context.Context, userID uuid.UUID, direction *string) ([]*Loan, error) {
	return s.repo.ListByUser(ctx, userID, direction)
}

func (s *Service) AcknowledgeLoan(ctx context.Context, actorID, loanID uuid.UUID) (*Loan, error) {
	l, err := s.repo.FindByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if l.UserID != actorID {
		return nil, ErrForbidden
	}
	if l.Status != StatusOutstanding {
		return nil, ErrInvalidStatus
	}
	updated, err := s.repo.Acknowledge(ctx, loanID)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionLoanAcknowledged,
		ActorID:    &actorID,
		EntityType: "loan",
		EntityID:   loanID,
		After:      updated,
	})
	return updated, nil
}

func (s *Service) DisputeLoan(ctx context.Context, actorID, loanID uuid.UUID, reason *string) (*Loan, error) {
	l, err := s.repo.FindByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if l.UserID != actorID {
		return nil, ErrForbidden
	}
	if l.Status == StatusSettled || l.Status == StatusDisputed {
		return nil, ErrInvalidStatus
	}
	updated, err := s.repo.Dispute(ctx, loanID, reason)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionLoanDisputed,
		ActorID:    &actorID,
		EntityType: "loan",
		EntityID:   loanID,
		After:      updated,
	})
	return updated, nil
}

func (s *Service) ClaimText(ctx context.Context, actorID, loanID uuid.UUID) (string, error) {
	l, err := s.repo.FindByID(ctx, loanID)
	if err != nil {
		return "", err
	}
	if l.UserID != actorID {
		return "", ErrForbidden
	}
	date := l.LoanDate.Format("2006-01-02")
	var text string
	if l.Direction == DirectionLent {
		text = fmt.Sprintf("Hi %s, just a reminder that you owe me %s %d from %s.",
			l.CounterpartyName, l.Currency, l.Amount, date)
	} else {
		text = fmt.Sprintf("Hi %s, I owe you %s %d from %s.",
			l.CounterpartyName, l.Currency, l.Amount, date)
	}
	if l.Note != nil && *l.Note != "" {
		text += fmt.Sprintf(" (%s)", *l.Note)
	}
	return text, nil
}
