package expense_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeExpenseRepo struct {
	expense  *expense.Expense
	splits   []expense.ExpenseSplit
	findErr  error
	createErr error
	voidErr  error
}

func (r *fakeExpenseRepo) Create(_ context.Context, e *expense.Expense, s []expense.ExpenseSplit) (*expense.Expense, []expense.ExpenseSplit, error) {
	if r.createErr != nil {
		return nil, nil, r.createErr
	}
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	r.expense = e
	r.splits = s
	return e, s, nil
}

func (r *fakeExpenseRepo) FindByID(_ context.Context, id uuid.UUID) (*expense.Expense, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.expense != nil && r.expense.ID == id {
		return r.expense, nil
	}
	return nil, expense.ErrNotFound
}

func (r *fakeExpenseRepo) FindSplitsByExpenseID(_ context.Context, expenseID uuid.UUID, _ int) ([]expense.ExpenseSplit, error) {
	return r.splits, nil
}

func (r *fakeExpenseRepo) ListForTeam(_ context.Context, _ uuid.UUID, _ bool) ([]*expense.Expense, error) {
	if r.expense == nil {
		return nil, nil
	}
	return []*expense.Expense{r.expense}, nil
}

func (r *fakeExpenseRepo) ListForUser(_ context.Context, _ uuid.UUID, _ bool) ([]*expense.Expense, error) {
	if r.expense == nil {
		return nil, nil
	}
	return []*expense.Expense{r.expense}, nil
}

func (r *fakeExpenseRepo) SaveCorrection(_ context.Context, _ uuid.UUID, _ any, newE *expense.Expense, newS []expense.ExpenseSplit) (*expense.Expense, []expense.ExpenseSplit, error) {
	r.expense = newE
	r.splits = newS
	return newE, newS, nil
}

func (r *fakeExpenseRepo) Void(_ context.Context, id, _ uuid.UUID, _ string) error {
	if r.voidErr != nil {
		return r.voidErr
	}
	if r.expense == nil || r.expense.ID != id {
		return expense.ErrNotFound
	}
	r.expense.IsVoid = true
	return nil
}

func (r *fakeExpenseRepo) UpdateReceiptURL(_ context.Context, _ uuid.UUID, url string) error {
	if r.expense != nil {
		r.expense.ReceiptURL = &url
	}
	return nil
}

// fakeTeamGateway lets tests control what role/status the actor has.
type fakeTeamGateway struct {
	role   string
	status string
	err    error
}

func (g *fakeTeamGateway) GetMembership(_ context.Context, _, _ uuid.UUID) (string, string, error) {
	return g.role, g.status, g.err
}

type fakePresigner struct{ url string }

func (p *fakePresigner) PresignPut(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return p.url, nil
}

func newSvc(repo expense.Repository, gw expense.TeamGateway) *expense.Service {
	return expense.NewService(repo, gw, audit.NopLogger(), &fakePresigner{url: "https://s3.example.com/upload"})
}

// ── CreateExpense — personal ───────────────────────────────────────────────────

func TestCreateExpense_Personal_NoSplits(t *testing.T) {
	actor := uuid.New()
	repo := &fakeExpenseRepo{}
	svc := newSvc(repo, nil)

	input := expense.CreateInput{
		Scope:       expense.ScopePersonal,
		Title:       "Coffee",
		Amount:      350,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
	}
	result, err := svc.CreateExpense(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Scope != expense.ScopePersonal {
		t.Errorf("scope = %q, want %q", result.Scope, expense.ScopePersonal)
	}
	if len(result.Splits) != 0 {
		t.Errorf("personal expense must have 0 splits, got %d", len(result.Splits))
	}
}

// ── CreateExpense — team ──────────────────────────────────────────────────────

func TestCreateExpense_Team_EqualSplit(t *testing.T) {
	actor := uuid.New()
	teamID := uuid.New()
	repo := &fakeExpenseRepo{}
	gw := &fakeTeamGateway{role: "owner", status: "active"}
	svc := newSvc(repo, gw)

	input := expense.CreateInput{
		Scope:       expense.ScopeTeam,
		TeamID:      &teamID,
		Title:       "Dinner",
		Amount:      3000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		SplitMethod: ptr("equal"),
		Splits:      []expense.SplitInput{{UserID: actor}, {UserID: uuid.New()}, {UserID: uuid.New()}},
	}
	result, err := svc.CreateExpense(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Splits) != 3 {
		t.Fatalf("want 3 splits, got %d", len(result.Splits))
	}
	var total int64
	for _, s := range result.Splits {
		total += s.ShareAmount
	}
	if total != 3000 {
		t.Errorf("splits total = %d, want 3000", total)
	}
}

func TestCreateExpense_Team_ExactSplit_SumMismatch_Error(t *testing.T) {
	actor := uuid.New()
	teamID := uuid.New()
	repo := &fakeExpenseRepo{}
	gw := &fakeTeamGateway{role: "member", status: "active"}
	svc := newSvc(repo, gw)

	input := expense.CreateInput{
		Scope:       expense.ScopeTeam,
		TeamID:      &teamID,
		Title:       "Hotel",
		Amount:      5000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		SplitMethod: ptr("exact"),
		Splits: []expense.SplitInput{
			{UserID: actor, ShareAmount: 3000},
			{UserID: uuid.New(), ShareAmount: 1000}, // only 4000 total
		},
	}
	_, err := svc.CreateExpense(context.Background(), actor, input)
	if !errors.Is(err, expense.ErrInvalidSplitSum) {
		t.Fatalf("want ErrInvalidSplitSum, got %v", err)
	}
}

func TestCreateExpense_Team_ActorNotMember_Error(t *testing.T) {
	actor := uuid.New()
	teamID := uuid.New()
	repo := &fakeExpenseRepo{}
	gw := &fakeTeamGateway{err: errors.New("not member")}
	svc := newSvc(repo, gw)

	input := expense.CreateInput{
		Scope:       expense.ScopeTeam,
		TeamID:      &teamID,
		Title:       "Lunch",
		Amount:      1000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		SplitMethod: ptr("equal"),
		Splits:      []expense.SplitInput{{UserID: actor}},
	}
	_, err := svc.CreateExpense(context.Background(), actor, input)
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestCreateExpense_Team_InactiveMember_Error(t *testing.T) {
	actor := uuid.New()
	teamID := uuid.New()
	repo := &fakeExpenseRepo{}
	gw := &fakeTeamGateway{role: "member", status: "invited"}
	svc := newSvc(repo, gw)

	input := expense.CreateInput{
		Scope:       expense.ScopeTeam,
		TeamID:      &teamID,
		Title:       "Lunch",
		Amount:      1000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		SplitMethod: ptr("equal"),
		Splits:      []expense.SplitInput{{UserID: actor}},
	}
	_, err := svc.CreateExpense(context.Background(), actor, input)
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestCreateExpense_Team_MissingTeamID_Error(t *testing.T) {
	actor := uuid.New()
	svc := newSvc(&fakeExpenseRepo{}, &fakeTeamGateway{role: "member", status: "active"})

	input := expense.CreateInput{
		Scope:       expense.ScopeTeam,
		TeamID:      nil, // missing
		Title:       "Lunch",
		Amount:      1000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		SplitMethod: ptr("equal"),
		Splits:      []expense.SplitInput{{UserID: actor}},
	}
	_, err := svc.CreateExpense(context.Background(), actor, input)
	if !errors.Is(err, expense.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

// ── CreateExpense — direct ────────────────────────────────────────────────────

func TestCreateExpense_Direct_CreatesSingleBorrowerSplit(t *testing.T) {
	actor := uuid.New()
	borrower := uuid.New()
	repo := &fakeExpenseRepo{}
	svc := newSvc(repo, nil)

	input := expense.CreateInput{
		Scope:       expense.ScopeDirect,
		Title:       "Borrowed cash",
		Amount:      2000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		BorrowerID:  &borrower,
	}
	result, err := svc.CreateExpense(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Splits) != 1 {
		t.Fatalf("direct expense must have 1 split, got %d", len(result.Splits))
	}
	if result.Splits[0].UserID != borrower {
		t.Errorf("split.UserID = %v, want borrower %v", result.Splits[0].UserID, borrower)
	}
	if result.Splits[0].ShareAmount != 2000 {
		t.Errorf("split.ShareAmount = %d, want 2000", result.Splits[0].ShareAmount)
	}
}

func TestCreateExpense_Direct_MissingBorrower_Error(t *testing.T) {
	actor := uuid.New()
	svc := newSvc(&fakeExpenseRepo{}, nil)

	input := expense.CreateInput{
		Scope:       expense.ScopeDirect,
		Title:       "Loan",
		Amount:      500,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
	}
	_, err := svc.CreateExpense(context.Background(), actor, input)
	if !errors.Is(err, expense.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

// ── VoidExpense ───────────────────────────────────────────────────────────────

func TestVoidExpense_ByPaidBy_Succeeds(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopePersonal,
		PaidBy: actor,
		IsVoid: false,
	}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)

	if err := svc.VoidExpense(context.Background(), actor, exp.ID, "wrong entry"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.expense.IsVoid {
		t.Error("expense must be voided")
	}
}

func TestVoidExpense_AlreadyVoided_Error(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopePersonal,
		PaidBy: actor,
		IsVoid: true,
	}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)

	err := svc.VoidExpense(context.Background(), actor, exp.ID, "oops")
	if !errors.Is(err, expense.ErrAlreadyVoided) {
		t.Fatalf("want ErrAlreadyVoided, got %v", err)
	}
}

func TestVoidExpense_NotPaidBy_PersonalScope_Error(t *testing.T) {
	owner := uuid.New()
	actor := uuid.New() // different user
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopePersonal,
		PaidBy: owner,
		IsVoid: false,
	}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)

	err := svc.VoidExpense(context.Background(), actor, exp.ID, "reason")
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestVoidExpense_TeamAdmin_CanVoidOthersPaidExpense(t *testing.T) {
	payer := uuid.New()
	admin := uuid.New()
	teamID := uuid.New()
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopeTeam,
		TeamID: &teamID,
		PaidBy: payer,
		IsVoid: false,
	}
	repo := &fakeExpenseRepo{expense: exp}
	gw := &fakeTeamGateway{role: "admin", status: "active"}
	svc := newSvc(repo, gw)

	if err := svc.VoidExpense(context.Background(), admin, exp.ID, "duplicate"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── CorrectExpense ────────────────────────────────────────────────────────────

func TestCorrectExpense_PersonalByPaidBy_Succeeds(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopePersonal,
		PaidBy: actor,
		Amount: 500,
		Title:  "Old title",
	}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)

	input := expense.CorrectInput{
		Title:       "New title",
		Amount:      600,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
	}
	result, err := svc.CorrectExpense(context.Background(), actor, exp.ID, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "New title" {
		t.Errorf("title = %q, want %q", result.Title, "New title")
	}
	if result.Amount != 600 {
		t.Errorf("amount = %d, want 600", result.Amount)
	}
}

func TestCorrectExpense_TeamExpense_InvalidSplitSum_Error(t *testing.T) {
	actor := uuid.New()
	teamID := uuid.New()
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopeTeam,
		TeamID: &teamID,
		PaidBy: actor,
		Amount: 5000,
	}
	repo := &fakeExpenseRepo{expense: exp}
	gw := &fakeTeamGateway{role: "owner", status: "active"}
	svc := newSvc(repo, gw)

	input := expense.CorrectInput{
		Title:       "Trip",
		Amount:      5000,
		Currency:    "LKR",
		PaidBy:      actor,
		ExpenseDate: time.Now(),
		SplitMethod: ptr("exact"),
		Splits: []expense.SplitInput{
			{UserID: actor, ShareAmount: 3000},
			{UserID: uuid.New(), ShareAmount: 1000}, // 4000 ≠ 5000
		},
	}
	_, err := svc.CorrectExpense(context.Background(), actor, exp.ID, input)
	if !errors.Is(err, expense.ErrInvalidSplitSum) {
		t.Fatalf("want ErrInvalidSplitSum, got %v", err)
	}
}

// ── GetReceiptUploadURL ───────────────────────────────────────────────────────

func TestGetReceiptUploadURL_ByPaidBy_ReturnsURL(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{
		ID:     uuid.New(),
		Scope:  expense.ScopePersonal,
		PaidBy: actor,
	}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)

	url, key, err := svc.GetReceiptUploadURL(context.Background(), actor, exp.ID, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("url must not be empty")
	}
	if key == "" {
		t.Error("key must not be empty")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptr(s string) *string { return &s }
