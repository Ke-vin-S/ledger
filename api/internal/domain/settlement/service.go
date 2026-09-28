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
	teamGW  TeamGateway
	auditor audit.Logger
}

func NewService(repo Repository, teamGW TeamGateway, auditor audit.Logger) *Service {
	return &Service{repo: repo, teamGW: teamGW, auditor: auditor}
}

// ── RecordSettlement ──────────────────────────────────────────────────────────

func (s *Service) RecordSettlement(ctx context.Context, actorID uuid.UUID, in RecordInput) (*Settlement, error) {
	if actorID != in.PayerID && actorID != in.PayeeID {
		return nil, ErrForbidden
	}
	if !validMethods[in.Method] {
		return nil, fmt.Errorf("%w: unknown method %q", ErrInvalidInput, in.Method)
	}
	if in.PayerID == in.PayeeID {
		return nil, ErrInvalidInput
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	expense, err := s.repo.FindExpense(ctx, in.ExpenseID)
	if err != nil {
		return nil, err
	}
	if in.PayeeID != expense.PaidBy {
		return nil, ErrInvalidPayee
	}

	debt, err := s.repo.GetDebtBalance(ctx, in.ExpenseID, in.PayerID)
	if err != nil {
		return nil, err
	}
	if in.Amount > debt.Balance {
		return nil, fmt.Errorf("%w: amount %d exceeds balance %d", ErrSettlementExceedsDebt, in.Amount, debt.Balance)
	}

	created, err := s.repo.RecordSettlementTx(ctx, &Settlement{
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

func (s *Service) GetDebtBalance(ctx context.Context, actorID, expenseID uuid.UUID, requestedDebtorID *uuid.UUID) (*DebtBalance, error) {
	expense, _, err := s.authorizeExpenseRead(ctx, actorID, expenseID)
	if err != nil {
		return nil, err
	}

	debtorID := actorID
	if requestedDebtorID != nil && *requestedDebtorID != actorID && s.isTeamAdmin(ctx, expense, actorID) {
		debtorID = *requestedDebtorID
	}
	return s.repo.GetDebtBalance(ctx, expenseID, debtorID)
}

func (s *Service) ListSettlementsByExpense(ctx context.Context, actorID, expenseID uuid.UUID) ([]*Settlement, error) {
	_, settlements, err := s.authorizeExpenseRead(ctx, actorID, expenseID)
	if err != nil {
		return nil, err
	}
	return settlements, nil
}

func (s *Service) authorizeExpenseRead(ctx context.Context, actorID, expenseID uuid.UUID) (*ExpenseAccess, []*Settlement, error) {
	expense, err := s.repo.FindExpense(ctx, expenseID)
	if err != nil {
		return nil, nil, err
	}
	settlements, err := s.repo.ListByExpense(ctx, expenseID)
	if err != nil {
		return nil, nil, err
	}
	if actorID == expense.PaidBy || s.isActiveTeamMember(ctx, expense, actorID) {
		return expense, settlements, nil
	}
	for _, existing := range settlements {
		if actorID == existing.PayerID || actorID == existing.PayeeID {
			return expense, settlements, nil
		}
	}
	return nil, nil, ErrForbidden
}

func (s *Service) isActiveTeamMember(ctx context.Context, expense *ExpenseAccess, actorID uuid.UUID) bool {
	if expense.TeamID == nil || s.teamGW == nil {
		return false
	}
	_, status, err := s.teamGW.GetMembership(ctx, *expense.TeamID, actorID)
	return err == nil && status == "active"
}

func (s *Service) isTeamAdmin(ctx context.Context, expense *ExpenseAccess, actorID uuid.UUID) bool {
	if expense.TeamID == nil || s.teamGW == nil {
		return false
	}
	role, status, err := s.teamGW.GetMembership(ctx, *expense.TeamID, actorID)
	return err == nil && status == "active" && (role == "admin" || role == "owner")
}

func (s *Service) ListTeamBalances(ctx context.Context, teamID, actorID uuid.UUID) ([]*TeamBalance, error) {
	return s.repo.ListTeamNetBalances(ctx, teamID, actorID)
}

func (s *Service) ListMyBalances(ctx context.Context, userID uuid.UUID) ([]*UserBalance, error) {
	return s.repo.ListUserNetBalances(ctx, userID)
}
