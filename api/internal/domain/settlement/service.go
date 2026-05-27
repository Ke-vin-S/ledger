package settlement

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
)

// Service implements all settlement business logic.
type Service struct {
	repo    Repository
	auditor audit.Logger
}

func NewService(repo Repository, auditor audit.Logger) *Service {
	return &Service{repo: repo, auditor: auditor}
}

// ── RecordSettlement ──────────────────────────────────────────────────────────

func (s *Service) RecordSettlement(ctx context.Context, actorID uuid.UUID, in RecordInput) (*Settlement, error) {
	if actorID != in.PayerID && actorID != in.PayeeID {
		return nil, ErrForbidden
	}
	if !validMethods[in.Method] {
		return nil, fmt.Errorf("%w: unknown method %q", ErrInvalidInput, in.Method)
	}

	debt, err := s.repo.GetDebtBalance(ctx, in.ExpenseID, in.PayerID)
	if err != nil {
		return nil, err
	}
	if in.Amount > debt.Balance {
		return nil, fmt.Errorf("%w: amount %d exceeds balance %d", ErrSettlementExceedsDebt, in.Amount, debt.Balance)
	}

	created, err := s.repo.Create(ctx, &Settlement{
		ExpenseID:  in.ExpenseID,
		PayerID:    in.PayerID,
		PayeeID:    in.PayeeID,
		Amount:     in.Amount,
		Method:     in.Method,
		MethodNote: in.MethodNote,
		Status:     StatusPending,
		RecordedBy: actorID,
		SettledOn:  in.SettledOn,
	})
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionSettlementCreated,
		ActorID:    &actorID,
		EntityType: "settlement",
		EntityID:   created.ID,
		After:      created,
	})

	return created, nil
}

// ── ConfirmSettlement ─────────────────────────────────────────────────────────

func (s *Service) ConfirmSettlement(ctx context.Context, actorID, settlementID uuid.UUID) (*Settlement, error) {
	existing, err := s.repo.FindByID(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	if actorID != existing.PayeeID {
		return nil, ErrForbidden
	}
	if existing.Status != StatusPending {
		return nil, ErrInvalidStatus
	}

	updated, err := s.repo.Confirm(ctx, settlementID, actorID)
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionSettlementConfirmed,
		ActorID:    &actorID,
		EntityType: "settlement",
		EntityID:   settlementID,
		After:      updated,
	})

	return updated, nil
}

// ── DisputeSettlement ─────────────────────────────────────────────────────────

func (s *Service) DisputeSettlement(ctx context.Context, actorID, settlementID uuid.UUID, reason string) (*Settlement, error) {
	if reason == "" {
		return nil, fmt.Errorf("%w: dispute reason is required", ErrInvalidInput)
	}

	existing, err := s.repo.FindByID(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	if actorID != existing.PayeeID {
		return nil, ErrForbidden
	}
	if existing.Status != StatusPending {
		return nil, ErrInvalidStatus
	}

	updated, err := s.repo.Dispute(ctx, settlementID, actorID, reason)
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionSettlementDisputed,
		ActorID:    &actorID,
		EntityType: "settlement",
		EntityID:   settlementID,
		After:      updated,
	})

	return updated, nil
}

// ── Balance queries ───────────────────────────────────────────────────────────

func (s *Service) GetDebtBalance(ctx context.Context, expenseID, debtorID uuid.UUID) (*DebtBalance, error) {
	return s.repo.GetDebtBalance(ctx, expenseID, debtorID)
}

func (s *Service) ListSettlementsByExpense(ctx context.Context, expenseID uuid.UUID) ([]*Settlement, error) {
	return s.repo.ListByExpense(ctx, expenseID)
}

func (s *Service) ListTeamBalances(ctx context.Context, teamID uuid.UUID) ([]*TeamNetBalance, error) {
	return s.repo.ListTeamNetBalances(ctx, teamID)
}

func (s *Service) ListMyBalances(ctx context.Context, userID uuid.UUID) ([]*UserNetBalance, error) {
	return s.repo.ListUserNetBalances(ctx, userID)
}
