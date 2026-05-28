package flag

import (
	"context"
	"strings"

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

func (s *Service) RaiseFlag(ctx context.Context, in RaiseInput) (*Flag, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrInvalidInput
	}

	f := &Flag{
		ExpenseID: in.ExpenseID,
		RaisedBy:  in.RaisedBy,
		Reason:    strings.TrimSpace(in.Reason),
		Status:    StatusOpen,
	}

	created, err := s.repo.Create(ctx, f)
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionFlagOpened,
		ActorID:    &in.RaisedBy,
		EntityType: "expense_flag",
		EntityID:   created.ID,
		After:      created,
	})

	return created, nil
}

func (s *Service) ResolveFlag(ctx context.Context, in ResolveInput) (*Flag, error) {
	if strings.TrimSpace(in.ResolutionNote) == "" {
		return nil, ErrInvalidInput
	}

	existing, err := s.repo.FindByID(ctx, in.FlagID)
	if err != nil {
		return nil, err
	}

	if existing.Status == StatusResolved {
		return nil, ErrAlreadyResolved
	}

	resolved, err := s.repo.Resolve(ctx, in.FlagID, in.ResolvedBy, strings.TrimSpace(in.ResolutionNote))
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionFlagResolved,
		ActorID:    &in.ResolvedBy,
		EntityType: "expense_flag",
		EntityID:   in.FlagID,
		Before:     existing,
		After:      resolved,
	})

	return resolved, nil
}

func (s *Service) ListFlags(ctx context.Context, expenseID uuid.UUID) ([]*Flag, error) {
	flags, err := s.repo.ListByExpense(ctx, expenseID)
	if err != nil {
		return nil, err
	}
	if flags == nil {
		return []*Flag{}, nil
	}
	return flags, nil
}
