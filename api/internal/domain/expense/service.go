package expense

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
)

const receiptURLTTL = 15 * time.Minute

var allowedReceiptContentTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/heic": {},
}

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
	if in.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	if !validCurrency(in.Currency) {
		return nil, fmt.Errorf("%w: currency must be a three-letter uppercase ISO code", ErrInvalidInput)
	}

	currentSplits, err := s.repo.FindSplitsByExpenseID(ctx, expenseID, current.Version)
	if err != nil {
		return nil, err
	}

	// Build new splits if needed (team / direct scope).
	var newPersistSplits []ExpenseSplit
	splitMethod := current.SplitMethod
	if in.SplitMethod != nil || len(in.Splits) > 0 {
		if in.SplitMethod == nil || len(in.Splits) == 0 {
			return nil, fmt.Errorf("%w: split_method and splits must be supplied together", ErrInvalidSplitData)
		}
		computed, cerr := ComputeSplits(*in.SplitMethod, in.Amount, in.Splits)
		if cerr != nil {
			return nil, cerr
		}
		newPersistSplits = toExpenseSplits(computed, expenseID, current.Version+1)
		splitMethod = in.SplitMethod
	} else if current.Scope == ScopeDirect {
		if len(currentSplits) != 1 {
			return nil, fmt.Errorf("%w: direct expense must have one borrower split", ErrInvalidSplitData)
		}
		newPersistSplits = toExpenseSplits([]SplitEntry{{UserID: currentSplits[0].UserID, ShareAmount: in.Amount}}, expenseID, current.Version+1)
	} else if current.Scope == ScopeTeam {
		if in.Amount == current.Amount {
			newPersistSplits = bumpSplitVersion(currentSplits, current.Version+1)
		} else {
			if current.SplitMethod == nil {
				return nil, fmt.Errorf("%w: current expense has no split method", ErrInvalidSplitData)
			}
			inputs := make([]SplitInput, len(currentSplits))
			for i, split := range currentSplits {
				inputs[i] = SplitInput{UserID: split.UserID, ShareAmount: split.ShareAmount}
				if split.ShareUnits != nil {
					inputs[i].ShareUnits = *split.ShareUnits
				}
			}
			computed, cerr := ComputeSplits(*current.SplitMethod, in.Amount, inputs)
			if cerr != nil {
				return nil, cerr
			}
			newPersistSplits = toExpenseSplits(computed, expenseID, current.Version+1)
		}
	}
	var splitTotal int64
	for _, split := range newPersistSplits {
		splitTotal += split.ShareAmount
	}
	if (current.Scope == ScopeTeam || current.Scope == ScopeDirect) && splitTotal != in.Amount {
		return nil, ErrInvalidSplitSum
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
		SplitMethod: splitMethod,
		ReceiptURL:  in.ReceiptURL,
		Note:        in.Note,
		Version:     current.Version + 1,
		CreatedBy:   current.CreatedBy,
	}

	saved, savedSplits, err := s.repo.SaveCorrection(ctx, expenseID, snapshot, newExpense, newPersistSplits, actorID, in.CorrectionReason)
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
	if err := s.checkReceiptWriteAccess(ctx, actorID, e); err != nil {
		return "", "", err
	}
	if _, allowed := allowedReceiptContentTypes[contentType]; !allowed {
		return "", "", fmt.Errorf("%w: unsupported receipt content type", ErrInvalidInput)
	}

	key = fmt.Sprintf("receipts/%s/%s", expenseID, uuid.NewString())
	url, err := s.presigner.PresignPut(ctx, key, contentType, receiptURLTTL)
	if err != nil {
		return "", "", fmt.Errorf("presign: %w", err)
	}
	return url, key, nil
}

func (s *Service) FinalizeReceipt(ctx context.Context, actorID, expenseID uuid.UUID, receiptURL string) (*Expense, error) {
	e, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return nil, err
	}
	if err := s.checkReceiptWriteAccess(ctx, actorID, e); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateReceiptURL(ctx, expenseID, receiptURL); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, expenseID)
}

func (s *Service) CanRead(ctx context.Context, actorID, expenseID uuid.UUID) error {
	e, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return err
	}
	return s.checkReadAccess(ctx, actorID, e)
}

func (s *Service) CanWrite(ctx context.Context, actorID, expenseID uuid.UUID) error {
	e, err := s.repo.FindByID(ctx, expenseID)
	if err != nil {
		return err
	}
	return s.checkWriteAccess(ctx, actorID, e)
}

// ── access helpers ────────────────────────────────────────────────────────────

func (s *Service) checkReadAccess(ctx context.Context, actorID uuid.UUID, e *Expense) error {
	switch e.Scope {
	case ScopeTeam:
		if e.TeamID == nil {
			return ErrForbidden
		}
		_, status, err := s.teamGW.GetMembership(ctx, *e.TeamID, actorID)
		if err != nil || status != "active" {
			return ErrForbidden
		}
		return nil
	case ScopeDirect:
		if actorID == e.PaidBy {
			return nil
		}
		splits, err := s.repo.FindSplitsByExpenseID(ctx, e.ID, e.Version)
		if err != nil {
			return err
		}
		for _, split := range splits {
			if split.UserID == actorID {
				return nil
			}
		}
		return ErrForbidden
	case ScopePersonal:
		if actorID == e.PaidBy {
			return nil
		}
		return ErrForbidden
	default:
		return ErrForbidden
	}
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

func (s *Service) checkReceiptWriteAccess(ctx context.Context, actorID uuid.UUID, e *Expense) error {
	if e.Scope == ScopeTeam && e.TeamID != nil {
		role, status, err := s.teamGW.GetMembership(ctx, *e.TeamID, actorID)
		if err != nil || status != "active" {
			return ErrForbidden
		}
		if actorID == e.PaidBy || role == "admin" || role == "owner" {
			return nil
		}
		return ErrForbidden
	}
	if actorID == e.PaidBy {
		return nil
	}
	return ErrForbidden
}

// ── validation ────────────────────────────────────────────────────────────────

func (s *Service) validateCreate(ctx context.Context, actorID uuid.UUID, in *CreateInput) error {
	if in.PaidBy != actorID && in.Scope != ScopeTeam {
		return ErrForbidden
	}

	switch in.Scope {
	case ScopeTeam:
		if in.TeamID == nil {
			return fmt.Errorf("%w: team_id is required for team expenses", ErrInvalidInput)
		}
		actorRole, actorStatus, err := s.teamGW.GetMembership(ctx, *in.TeamID, actorID)
		if err != nil || actorStatus != "active" {
			return ErrForbidden
		}
		if in.PaidBy != actorID && actorRole != "admin" && actorRole != "owner" {
			return ErrForbidden
		}
		if err := s.requireActiveMember(ctx, *in.TeamID, in.PaidBy); err != nil {
			return err
		}
		for _, split := range in.Splits {
			if err := s.requireActiveMember(ctx, *in.TeamID, split.UserID); err != nil {
				return err
			}
		}
	case ScopeDirect:
		if in.BorrowerID == nil {
			return fmt.Errorf("%w: borrower_id is required for direct expenses", ErrInvalidInput)
		}
		if *in.BorrowerID == in.PaidBy {
			return fmt.Errorf("%w: borrower_id must differ from paid_by", ErrInvalidInput)
		}
	case ScopePersonal:
	default:
		return fmt.Errorf("%w: unknown scope %q", ErrInvalidInput, in.Scope)
	}
	return nil
}

func (s *Service) requireActiveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	_, status, err := s.teamGW.GetMembership(ctx, teamID, userID)
	if err != nil || status != "active" {
		return ErrForbidden
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

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
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
