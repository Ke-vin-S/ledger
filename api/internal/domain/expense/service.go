package expense

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
)

const receiptURLTTL = 15 * time.Minute

// Service implements all expense business logic.
type Service struct {
	repo      Repository
	teamGW    TeamGateway
	auditor   audit.Logger
	presigner Presigner
}

func NewService(repo Repository, teamGW TeamGateway, auditor audit.Logger, presigner Presigner) *Service {
	return &Service{repo: repo, teamGW: teamGW, auditor: auditor, presigner: presigner}
}

// ── CreateExpense ─────────────────────────────────────────────────────────────

func (s *Service) CreateExpense(ctx context.Context, actorID uuid.UUID, in CreateInput) (*ExpenseWithSplits, error) {
	if err := s.validateCreate(ctx, actorID, &in); err != nil {
		return nil, err
	}

	splits, err := s.buildSplits(in)
	if err != nil {
		return nil, err
	}

	method := in.SplitMethod
	e := &Expense{
		Scope:       in.Scope,
		TeamID:      in.TeamID,
		Title:       in.Title,
		Amount:      in.Amount,
		Currency:    in.Currency,
		CategoryID:  in.CategoryID,
		PaidBy:      in.PaidBy,
		ExpenseDate: in.ExpenseDate,
		SplitMethod: method,
		Note:        in.Note,
		Version:     1,
		CreatedBy:   actorID,
	}

	persistSplits := toExpenseSplits(splits, uuid.Nil, 1)
	created, createdSplits, err := s.repo.Create(ctx, e, persistSplits)
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionExpenseCreated,
		ActorID:    &actorID,
		TeamID:     in.TeamID,
		EntityType: "expense",
		EntityID:   created.ID,
		After:      created,
	})

	return &ExpenseWithSplits{Expense: *created, Splits: createdSplits}, nil
}

// ── GetExpense ────────────────────────────────────────────────────────────────

func (s *Service) GetExpense(ctx context.Context, actorID, expenseID uuid.UUID) (*ExpenseWithSplits, error) {
	e, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return nil, err
	}
	if err := s.checkReadAccess(ctx, actorID, e); err != nil {
		return nil, err
	}
	splits, err := s.repo.FindSplitsByExpenseID(ctx, expenseID, e.Version)
	if err != nil {
		return nil, err
	}
	return &ExpenseWithSplits{Expense: *e, Splits: splits}, nil
}

// ── ListTeamExpenses ──────────────────────────────────────────────────────────

func (s *Service) ListTeamExpenses(ctx context.Context, actorID, teamID uuid.UUID, includeVoid bool) ([]*ExpenseWithSplits, error) {
	if _, _, err := s.teamGW.GetMembership(ctx, teamID, actorID); err != nil {
		return nil, ErrForbidden
	}
	expenses, err := s.repo.ListForTeam(ctx, teamID, includeVoid)
	if err != nil {
		return nil, err
	}
	return s.attachSplits(ctx, expenses)
}

// ── ListMyExpenses ────────────────────────────────────────────────────────────

func (s *Service) ListMyExpenses(ctx context.Context, actorID uuid.UUID, includeVoid bool) ([]*ExpenseWithSplits, error) {
	expenses, err := s.repo.ListForUser(ctx, actorID, includeVoid)
	if err != nil {
		return nil, err
	}
	return s.attachSplits(ctx, expenses)
}

// ── CorrectExpense ────────────────────────────────────────────────────────────

func (s *Service) CorrectExpense(ctx context.Context, actorID, expenseID uuid.UUID, in CorrectInput) (*ExpenseWithSplits, error) {
	current, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return nil, err
	}
	if current.IsVoid {
		return nil, ErrAlreadyVoided
	}
	if err := s.checkWriteAccess(ctx, actorID, current); err != nil {
		return nil, err
	}

	currentSplits, err := s.repo.FindSplitsByExpenseID(ctx, expenseID, current.Version)
	if err != nil {
		return nil, err
	}

	// Build new splits if needed (team / direct scope).
	var newPersistSplits []ExpenseSplit
	if in.SplitMethod != nil && len(in.Splits) > 0 {
		computed, cerr := ComputeSplits(*in.SplitMethod, in.Amount, in.Splits)
		if cerr != nil {
			return nil, cerr
		}
		newPersistSplits = toExpenseSplits(computed, expenseID, current.Version+1)
	} else if current.Scope == ScopeTeam || current.Scope == ScopeDirect {
		// Re-use current splits at new version with potentially new amount.
		newPersistSplits = bumpSplitVersion(currentSplits, current.Version+1)
	}

	snapshot := map[string]any{
		"expense": current,
		"splits":  currentSplits,
	}

	newExpense := &Expense{
		ID:          expenseID,
		Scope:       current.Scope,
		TeamID:      current.TeamID,
		Title:       in.Title,
		Amount:      in.Amount,
		Currency:    in.Currency,
		CategoryID:  in.CategoryID,
		PaidBy:      in.PaidBy,
		ExpenseDate: in.ExpenseDate,
		SplitMethod: in.SplitMethod,
		ReceiptURL:  in.ReceiptURL,
		Note:        in.Note,
		Version:     current.Version + 1,
		CreatedBy:   current.CreatedBy,
	}

	saved, savedSplits, err := s.repo.SaveCorrection(ctx, expenseID, snapshot, newExpense, newPersistSplits)
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionExpenseCorrected,
		ActorID:    &actorID,
		TeamID:     current.TeamID,
		EntityType: "expense",
		EntityID:   expenseID,
		Before:     snapshot,
		After:      saved,
	})

	return &ExpenseWithSplits{Expense: *saved, Splits: savedSplits}, nil
}

// ── VoidExpense ───────────────────────────────────────────────────────────────

func (s *Service) VoidExpense(ctx context.Context, actorID, expenseID uuid.UUID, reason string) error {
	e, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return err
	}
	if e.IsVoid {
		return ErrAlreadyVoided
	}
	if err := s.checkWriteAccess(ctx, actorID, e); err != nil {
		return err
	}

	if err := s.repo.Void(ctx, expenseID, actorID, reason); err != nil {
		return err
	}

	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionExpenseVoided,
		ActorID:    &actorID,
		TeamID:     e.TeamID,
		EntityType: "expense",
		EntityID:   expenseID,
	})

	return nil
}

// ── GetReceiptUploadURL ───────────────────────────────────────────────────────

func (s *Service) GetReceiptUploadURL(ctx context.Context, actorID, expenseID uuid.UUID, contentType string) (uploadURL, key string, err error) {
	e, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return "", "", err
	}
	if err := s.checkReadAccess(ctx, actorID, e); err != nil {
		return "", "", err
	}

	key = fmt.Sprintf("receipts/%s", expenseID)
	url, err := s.presigner.PresignPut(ctx, key, contentType, receiptURLTTL)
	if err != nil {
		return "", "", fmt.Errorf("presign: %w", err)
	}
	return url, key, nil
}

// ── access helpers ────────────────────────────────────────────────────────────

func (s *Service) checkReadAccess(ctx context.Context, actorID uuid.UUID, e *Expense) error {
	if e.Scope == ScopeTeam && e.TeamID != nil {
		_, status, err := s.teamGW.GetMembership(ctx, *e.TeamID, actorID)
		if err != nil || status != "active" {
			return ErrForbidden
		}
		return nil
	}
	// personal / direct: paid_by can always read; for direct, borrower checked at list level
	return nil
}

func (s *Service) checkWriteAccess(ctx context.Context, actorID uuid.UUID, e *Expense) error {
	if e.Scope == ScopeTeam && e.TeamID != nil {
		role, status, err := s.teamGW.GetMembership(ctx, *e.TeamID, actorID)
		if err != nil || status != "active" {
			return ErrForbidden
		}
		if actorID == e.PaidBy {
			return nil
		}
		if role == "admin" || role == "owner" {
			return nil
		}
		return ErrForbidden
	}
	// personal / direct: only paid_by
	if actorID != e.PaidBy {
		return ErrForbidden
	}
	return nil
}

// ── validation ────────────────────────────────────────────────────────────────

func (s *Service) validateCreate(ctx context.Context, actorID uuid.UUID, in *CreateInput) error {
	switch in.Scope {
	case ScopeTeam:
		if in.TeamID == nil {
			return fmt.Errorf("%w: team_id is required for team expenses", ErrInvalidInput)
		}
		_, status, err := s.teamGW.GetMembership(ctx, *in.TeamID, actorID)
		if err != nil {
			return ErrForbidden
		}
		if status != "active" {
			return ErrForbidden
		}
	case ScopeDirect:
		if in.BorrowerID == nil {
			return fmt.Errorf("%w: borrower_id is required for direct expenses", ErrInvalidInput)
		}
	case ScopePersonal:
		// no additional checks
	default:
		return fmt.Errorf("%w: unknown scope %q", ErrInvalidInput, in.Scope)
	}
	return nil
}

// ── split building ────────────────────────────────────────────────────────────

func (s *Service) buildSplits(in CreateInput) ([]SplitEntry, error) {
	switch in.Scope {
	case ScopePersonal:
		return nil, nil
	case ScopeDirect:
		return []SplitEntry{{UserID: *in.BorrowerID, ShareAmount: in.Amount}}, nil
	case ScopeTeam:
		if in.SplitMethod == nil {
			return nil, fmt.Errorf("%w: split_method is required for team expenses", ErrInvalidInput)
		}
		return ComputeSplits(*in.SplitMethod, in.Amount, in.Splits)
	}
	return nil, nil
}

// ── misc helpers ──────────────────────────────────────────────────────────────

func toExpenseSplits(entries []SplitEntry, expenseID uuid.UUID, version int) []ExpenseSplit {
	out := make([]ExpenseSplit, len(entries))
	for i, e := range entries {
		out[i] = ExpenseSplit{
			ExpenseID:   expenseID,
			UserID:      e.UserID,
			ShareAmount: e.ShareAmount,
			ShareUnits:  e.ShareUnits,
			Version:     version,
		}
	}
	return out
}

func bumpSplitVersion(splits []ExpenseSplit, version int) []ExpenseSplit {
	out := make([]ExpenseSplit, len(splits))
	for i, s := range splits {
		out[i] = s
		out[i].Version = version
		out[i].ID = uuid.Nil // will be assigned by DB
	}
	return out
}

func (s *Service) attachSplits(ctx context.Context, expenses []*Expense) ([]*ExpenseWithSplits, error) {
	out := make([]*ExpenseWithSplits, len(expenses))
	for i, e := range expenses {
		splits, err := s.repo.FindSplitsByExpenseID(ctx, e.ID, e.Version)
		if err != nil {
			return nil, err
		}
		out[i] = &ExpenseWithSplits{Expense: *e, Splits: splits}
	}
	return out, nil
}
