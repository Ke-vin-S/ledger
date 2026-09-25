package expense_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeExpenseRepo struct {
	expense          *expense.Expense
	splits           []expense.ExpenseSplit
	findErr          error
	createErr        error
	voidErr          error
	correctedBy      uuid.UUID
	correctionReason *string
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

func (r *fakeExpenseRepo) SaveCorrection(_ context.Context, _ uuid.UUID, _ any, newE *expense.Expense, newS []expense.ExpenseSplit, correctedBy uuid.UUID, correctionReason *string) (*expense.Expense, []expense.ExpenseSplit, error) {
	r.expense = newE
	r.splits = newS
	r.correctedBy = correctedBy
	r.correctionReason = correctionReason
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

type fakeTeamGateway struct {
	role        string
	status      string
	err         error
	memberships map[uuid.UUID]struct {
		role   string
		status string
		err    error
	}
}

func (g *fakeTeamGateway) GetMembership(_ context.Context, _ uuid.UUID, userID uuid.UUID) (string, string, error) {
	if g.memberships != nil {
		membership, ok := g.memberships[userID]
		if !ok {
			return "", "", errors.New("not a member")
		}
		return membership.role, membership.status, membership.err
	}
	return g.role, g.status, g.err
}

type fakePresigner struct {
	url     string
	key     string
	content string
}

func (p *fakePresigner) PresignPut(_ context.Context, key, contentType string, _ time.Duration) (string, error) {
	p.key = key
	p.content = contentType
	return p.url, nil
}

func newSvc(repo expense.Repository, gw expense.TeamGateway) *expense.Service {
	return newSvcWithPresigner(repo, gw, &fakePresigner{url: "https://s3.example.com/upload"})
}

func newSvcWithPresigner(repo expense.Repository, gw expense.TeamGateway, presigner expense.Presigner) *expense.Service {
	return expense.NewService(repo, gw, audit.NopLogger(), presigner)
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

func TestCreateExpense_Team_RejectsNonMemberPayerAndParticipant(t *testing.T) {
	actor, outsider := uuid.New(), uuid.New()
	teamID := uuid.New()
	gw := &fakeTeamGateway{
		role:   "owner",
		status: "active",
		memberships: map[uuid.UUID]struct {
			role   string
			status string
			err    error
		}{actor: {role: "owner", status: "active"}},
	}
	svc := newSvc(&fakeExpenseRepo{}, gw)
	input := expense.CreateInput{
		Scope: expense.ScopeTeam, TeamID: &teamID, Title: "Dinner", Amount: 1000,
		Currency: "LKR", PaidBy: actor, ExpenseDate: time.Now(), SplitMethod: ptr(expense.MethodEqual),
		Splits: []expense.SplitInput{{UserID: actor}, {UserID: outsider}},
	}

	_, err := svc.CreateExpense(context.Background(), actor, input)
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("non-member participant: want ErrForbidden, got %v", err)
	}
}

func TestCreateExpense_Team_AdminCanPayForActiveMember(t *testing.T) {
	admin, payer := uuid.New(), uuid.New()
	teamID := uuid.New()
	gw := &fakeTeamGateway{memberships: map[uuid.UUID]struct {
		role   string
		status string
		err    error
	}{
		admin: {role: "admin", status: "active"},
		payer: {role: "member", status: "active"},
	}}
	svc := newSvc(&fakeExpenseRepo{}, gw)

	_, err := svc.CreateExpense(context.Background(), admin, expense.CreateInput{
		Scope: expense.ScopeTeam, TeamID: &teamID, Title: "Dinner", Amount: 1000,
		Currency: "LKR", PaidBy: payer, ExpenseDate: time.Now(), SplitMethod: ptr(expense.MethodEqual),
		Splits: []expense.SplitInput{{UserID: payer}},
	})
	if err != nil {
		t.Fatalf("admin payment for active member failed: %v", err)
	}
}

func TestCreateExpense_Team_MemberCannotPayForAnotherUser(t *testing.T) {
	actor, payer := uuid.New(), uuid.New()
	teamID := uuid.New()
	gw := &fakeTeamGateway{memberships: map[uuid.UUID]struct {
		role   string
		status string
		err    error
	}{
		actor: {role: "member", status: "active"},
		payer: {role: "member", status: "active"},
	}}
	svc := newSvc(&fakeExpenseRepo{}, gw)

	_, err := svc.CreateExpense(context.Background(), actor, expense.CreateInput{
		Scope: expense.ScopeTeam, TeamID: &teamID, Title: "Dinner", Amount: 1000,
		Currency: "LKR", PaidBy: payer, ExpenseDate: time.Now(), SplitMethod: ptr(expense.MethodEqual),
		Splits: []expense.SplitInput{{UserID: payer}},
	})
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
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

func TestCreateExpense_Direct_RejectsSelfBorrower(t *testing.T) {
	actor := uuid.New()
	svc := newSvc(&fakeExpenseRepo{}, nil)
	_, err := svc.CreateExpense(context.Background(), actor, expense.CreateInput{
		Scope: expense.ScopeDirect, Title: "Loan", Amount: 500, Currency: "LKR",
		PaidBy: actor, BorrowerID: &actor, ExpenseDate: time.Now(),
	})
	if !errors.Is(err, expense.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestGetExpense_PersonalByAnotherUser_Forbidden(t *testing.T) {
	owner := uuid.New()
	actor := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopePersonal, PaidBy: owner}
	svc := newSvc(&fakeExpenseRepo{expense: exp}, nil)

	_, err := svc.GetExpense(context.Background(), actor, exp.ID)
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestGetExpense_DirectByBorrower_Succeeds(t *testing.T) {
	payer := uuid.New()
	borrower := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeDirect, PaidBy: payer}
	repo := &fakeExpenseRepo{
		expense: exp,
		splits:  []expense.ExpenseSplit{{ExpenseID: exp.ID, UserID: borrower}},
	}
	svc := newSvc(repo, nil)

	result, err := svc.GetExpense(context.Background(), borrower, exp.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Expense.ID != exp.ID {
		t.Fatalf("expense ID = %v, want %v", result.Expense.ID, exp.ID)
	}
}

func TestGetExpense_DirectByUnrelatedUser_Forbidden(t *testing.T) {
	payer := uuid.New()
	borrower := uuid.New()
	actor := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeDirect, PaidBy: payer}
	repo := &fakeExpenseRepo{
		expense: exp,
		splits:  []expense.ExpenseSplit{{ExpenseID: exp.ID, UserID: borrower}},
	}
	svc := newSvc(repo, nil)

	_, err := svc.GetExpense(context.Background(), actor, exp.ID)
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
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

func TestCorrectExpense_AmountOnly_RecomputesEqualSplits(t *testing.T) {
	actor, other := uuid.New(), uuid.New()
	teamID := uuid.New()
	method := expense.MethodEqual
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeTeam, TeamID: &teamID, PaidBy: actor, Amount: 1000, Currency: "LKR", Version: 1, SplitMethod: &method}
	repo := &fakeExpenseRepo{expense: exp, splits: []expense.ExpenseSplit{{UserID: actor, ShareAmount: 500, Version: 1}, {UserID: other, ShareAmount: 500, Version: 1}}}
	svc := newSvc(repo, &fakeTeamGateway{role: "owner", status: "active"})

	result, err := svc.CorrectExpense(context.Background(), actor, exp.ID, expense.CorrectInput{Title: "Dinner", Amount: 1001, Currency: "LKR", PaidBy: actor, ExpenseDate: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var total int64
	for _, split := range result.Splits {
		total += split.ShareAmount
	}
	if total != 1001 {
		t.Fatalf("split total = %d, want 1001", total)
	}
	if result.Splits[0].ShareAmount != 501 || result.Splits[1].ShareAmount != 500 {
		t.Fatalf("unexpected recomputed splits: %+v", result.Splits)
	}
}

func TestCorrectExpense_InvalidAmountOrCurrency(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopePersonal, PaidBy: actor, Amount: 500, Currency: "LKR", Version: 1}
	svc := newSvc(&fakeExpenseRepo{expense: exp}, nil)

	_, err := svc.CorrectExpense(context.Background(), actor, exp.ID, expense.CorrectInput{Amount: 0, Currency: "LKR", PaidBy: actor})
	if !errors.Is(err, expense.ErrInvalidInput) {
		t.Fatalf("zero amount: want ErrInvalidInput, got %v", err)
	}
	_, err = svc.CorrectExpense(context.Background(), actor, exp.ID, expense.CorrectInput{Amount: 100, Currency: "lkr", PaidBy: actor})
	if !errors.Is(err, expense.ErrInvalidInput) {
		t.Fatalf("invalid currency: want ErrInvalidInput, got %v", err)
	}
}

func TestCorrectExpense_PersistsActorAndReason(t *testing.T) {
	actor, creator := uuid.New(), uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopePersonal, PaidBy: actor, CreatedBy: creator, Amount: 500, Currency: "LKR", Version: 1}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)
	reason := "corrected amount"

	_, err := svc.CorrectExpense(context.Background(), actor, exp.ID, expense.CorrectInput{Amount: 600, Currency: "LKR", PaidBy: actor, CorrectionReason: &reason})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.correctedBy != actor {
		t.Fatalf("corrected by = %v, want actor %v", repo.correctedBy, actor)
	}
	if repo.correctionReason == nil || *repo.correctionReason != reason {
		t.Fatalf("correction reason = %v, want %q", repo.correctionReason, reason)
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

func TestGetReceiptUploadURL_DirectBorrowerForbidden(t *testing.T) {
	payer := uuid.New()
	borrower := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeDirect, PaidBy: payer}
	svc := newSvc(&fakeExpenseRepo{expense: exp}, nil)

	_, _, err := svc.GetReceiptUploadURL(context.Background(), borrower, exp.ID, "image/jpeg")
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestGetReceiptUploadURL_TeamAdminAllowed(t *testing.T) {
	payer := uuid.New()
	admin := uuid.New()
	teamID := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeTeam, TeamID: &teamID, PaidBy: payer}
	svc := newSvc(&fakeExpenseRepo{expense: exp}, &fakeTeamGateway{role: "admin", status: "active"})

	if _, _, err := svc.GetReceiptUploadURL(context.Background(), admin, exp.ID, "image/png"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetReceiptUploadURL_RejectsUnsupportedContentType(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopePersonal, PaidBy: actor}
	svc := newSvc(&fakeExpenseRepo{expense: exp}, nil)

	_, _, err := svc.GetReceiptUploadURL(context.Background(), actor, exp.ID, "application/pdf")
	if !errors.Is(err, expense.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestGetReceiptUploadURL_UsesUniqueKey(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopePersonal, PaidBy: actor}
	repo := &fakeExpenseRepo{expense: exp}
	presigner := &fakePresigner{url: "https://s3.example.com/upload"}
	svc := newSvcWithPresigner(repo, nil, presigner)

	_, firstKey, err := svc.GetReceiptUploadURL(context.Background(), actor, exp.ID, "image/jpeg")
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	_, secondKey, err := svc.GetReceiptUploadURL(context.Background(), actor, exp.ID, "image/jpeg")
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if firstKey == secondKey {
		t.Fatalf("receipt keys must be unique, both were %q", firstKey)
	}
	if firstKey == fmt.Sprintf("receipts/%s", exp.ID) {
		t.Fatalf("receipt key must include a unique object ID: %q", firstKey)
	}
}

func TestFinalizeReceipt_DirectBorrowerForbidden(t *testing.T) {
	payer := uuid.New()
	borrower := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeDirect, PaidBy: payer}
	repo := &fakeExpenseRepo{expense: exp}
	svc := newSvc(repo, nil)

	_, err := svc.FinalizeReceipt(context.Background(), borrower, exp.ID, "https://cdn.example/receipt.jpg")
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
	if exp.ReceiptURL != nil {
		t.Fatal("forbidden finalize changed receipt URL")
	}
}

func TestGetExpense_TeamRequiresActiveMembership(t *testing.T) {
	actor := uuid.New()
	teamID := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopeTeam, TeamID: &teamID, PaidBy: actor}
	svc := newSvc(&fakeExpenseRepo{expense: exp}, &fakeTeamGateway{err: errors.New("not a member")})

	_, err := svc.GetExpense(context.Background(), actor, exp.ID)
	if !errors.Is(err, expense.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptr(s string) *string { return &s }
