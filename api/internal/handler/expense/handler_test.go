package expense

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
)

// ── test harness ──────────────────────────────────────────────────────────────

func authAs(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &jwtauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}
			next.ServeHTTP(w, r.WithContext(jwtauth.SetClaims(r.Context(), claims)))
		})
	}
}

type fakeRepo struct {
	expense       *expense.Expense
	splits        []expense.ExpenseSplit
	correctionErr error
}

func (r *fakeRepo) Create(_ context.Context, e *expense.Expense, s []expense.ExpenseSplit) (*expense.Expense, []expense.ExpenseSplit, error) {
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	r.expense = e
	r.splits = s
	return e, s, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*expense.Expense, error) {
	if r.expense != nil && r.expense.ID == id {
		return r.expense, nil
	}
	return nil, expense.ErrNotFound
}

func (r *fakeRepo) FindSplitsByExpenseID(_ context.Context, _ uuid.UUID, _ int) ([]expense.ExpenseSplit, error) {
	return r.splits, nil
}

func (r *fakeRepo) ListForTeam(_ context.Context, _ uuid.UUID, _ bool) ([]*expense.Expense, error) {
	return nil, nil
}

func (r *fakeRepo) ListForUser(_ context.Context, _ uuid.UUID, _ bool) ([]*expense.Expense, error) {
	return nil, nil
}

func (r *fakeRepo) SaveCorrection(_ context.Context, _ uuid.UUID, _ any, newE *expense.Expense, newS []expense.ExpenseSplit, _ uuid.UUID, _ *string) (*expense.Expense, []expense.ExpenseSplit, error) {
	if r.correctionErr != nil {
		return nil, nil, r.correctionErr
	}
	r.expense = newE
	r.splits = newS
	return newE, newS, nil
}

func (r *fakeRepo) Void(_ context.Context, id, _ uuid.UUID, _ string) error {
	if r.expense == nil || r.expense.ID != id {
		return expense.ErrNotFound
	}
	r.expense.IsVoid = true
	return nil
}

func (r *fakeRepo) UpdateReceiptURL(_ context.Context, _ uuid.UUID, url string) error {
	if r.expense != nil {
		r.expense.ReceiptURL = &url
	}
	return nil
}

// activeGateway always reports the caller as an active member.
type activeGateway struct{}

func (activeGateway) GetMembership(_ context.Context, _, _ uuid.UUID) (string, string, error) {
	return "member", "active", nil
}

type fakePresigner struct{}

func (fakePresigner) PresignPut(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "https://s3.example.com/upload", nil
}

func router(repo *fakeRepo, actor uuid.UUID) http.Handler {
	svc := expense.NewService(repo, activeGateway{}, audit.NopLogger(), fakePresigner{})
	h := New(svc, "https://app.example.com")
	root := chi.NewRouter()
	root.Mount("/expenses", h.Routes(authAs(actor)))
	root.Mount("/teams/{teamId}/expenses", h.TeamRoutes(authAs(actor)))
	return root
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return resp.Error.Code
}

func strPtr(s string) *string { return &s }

// ── createTeamExpense ───────────────────────────────────────────────────────────

func TestCreateTeamExpense_EqualSplit_201(t *testing.T) {
	actor, other := uuid.New(), uuid.New()
	repo := &fakeRepo{}
	h := router(repo, actor)
	teamID := uuid.New()

	rec := doJSON(t, h, http.MethodPost, "/teams/"+teamID.String()+"/expenses", createExpenseBody{
		Title:       "Dinner",
		Amount:      3000,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
		SplitMethod: strPtr("equal"),
		Splits: []splitInputJSON{
			{UserID: actor.String()},
			{UserID: other.String()},
		},
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data expenseResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Splits) != 2 {
		t.Errorf("got %d splits, want 2", len(resp.Data.Splits))
	}
	// Equal split of 3000 across 2 → 1500 each.
	var sum int64
	for _, s := range resp.Data.Splits {
		sum += s.ShareAmount
	}
	if sum != 3000 {
		t.Errorf("split sum = %d, want 3000", sum)
	}
}

func TestCreateTeamExpense_ExactSplitMismatch_422(t *testing.T) {
	actor, other := uuid.New(), uuid.New()
	repo := &fakeRepo{}
	h := router(repo, actor)
	teamID := uuid.New()

	rec := doJSON(t, h, http.MethodPost, "/teams/"+teamID.String()+"/expenses", createExpenseBody{
		Title:       "Dinner",
		Amount:      3000,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
		SplitMethod: strPtr("exact"),
		Splits: []splitInputJSON{
			{UserID: actor.String(), ShareAmount: 1000},
			{UserID: other.String(), ShareAmount: 1000}, // sums to 2000, not 3000
		},
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_SPLIT_SUM" {
		t.Errorf("error code = %q, want INVALID_SPLIT_SUM", code)
	}
}

func TestCreateTeamExpense_MalformedSplitUUID_400(t *testing.T) {
	actor, other := uuid.New(), uuid.New()
	h := router(&fakeRepo{}, actor)
	teamID := uuid.New()

	rec := doJSON(t, h, http.MethodPost, "/teams/"+teamID.String()+"/expenses", createExpenseBody{
		Title:       "Dinner",
		Amount:      1000,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
		SplitMethod: strPtr("equal"),
		Splits: []splitInputJSON{
			{UserID: actor.String()},
			{UserID: "not-a-uuid"},
			{UserID: other.String()},
		},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_INPUT" {
		t.Fatalf("error code = %q, want INVALID_INPUT", code)
	}
	if !strings.Contains(rec.Body.String(), "splits[1].user_id") {
		t.Fatalf("error body does not identify bad split: %s", rec.Body.String())
	}
}

func TestCreateTeamExpense_BadTeamID_400(t *testing.T) {
	actor := uuid.New()
	h := router(&fakeRepo{}, actor)

	rec := doJSON(t, h, http.MethodPost, "/teams/not-a-uuid/expenses", createExpenseBody{
		Title:       "X",
		Amount:      100,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
		SplitMethod: strPtr("equal"),
		Splits:      []splitInputJSON{{UserID: actor.String()}},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateTeamExpense_BadDate_400(t *testing.T) {
	actor := uuid.New()
	h := router(&fakeRepo{}, actor)
	teamID := uuid.New()

	rec := doJSON(t, h, http.MethodPost, "/teams/"+teamID.String()+"/expenses", createExpenseBody{
		Title:       "X",
		Amount:      100,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "Jan 15 2026",
		SplitMethod: strPtr("equal"),
		Splits:      []splitInputJSON{{UserID: actor.String()}},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// ── createPersonalExpense ───────────────────────────────────────────────────────

func TestCreatePersonalExpense_NoSplits_201(t *testing.T) {
	actor := uuid.New()
	repo := &fakeRepo{}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses", createExpenseBody{
		Title:       "Coffee",
		Amount:      350,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCorrectExpense_VersionConflict_409(t *testing.T) {
	actor := uuid.New()
	exp := &expense.Expense{ID: uuid.New(), Scope: expense.ScopePersonal, PaidBy: actor, Amount: 100, Currency: "LKR", Version: 1}
	repo := &fakeRepo{expense: exp, correctionErr: expense.ErrVersionConflict}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPatch, "/expenses/"+exp.ID.String(), correctExpenseBody{
		Title:       "Coffee",
		Amount:      200,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "VERSION_CONFLICT" {
		t.Fatalf("error code = %q, want VERSION_CONFLICT", code)
	}
}

// ── getExpense / voidExpense ─────────────────────────────────────────────────────

func TestGetExpense_Missing_404(t *testing.T) {
	actor := uuid.New()
	h := router(&fakeRepo{}, actor)

	rec := doJSON(t, h, http.MethodGet, "/expenses/"+uuid.New().String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "NOT_FOUND" {
		t.Errorf("error code = %q, want NOT_FOUND", code)
	}
}

func TestVoidExpense_204(t *testing.T) {
	actor := uuid.New()
	repo := &fakeRepo{}
	h := router(repo, actor)

	// Seed a personal expense to void.
	created := doJSON(t, h, http.MethodPost, "/expenses", createExpenseBody{
		Title:       "Coffee",
		Amount:      350,
		Currency:    "LKR",
		PaidBy:      actor.String(),
		ExpenseDate: "2026-01-15",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("seed create failed: %s", created.Body.String())
	}
	var resp struct {
		Data expenseResponse `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec := doJSON(t, h, http.MethodDelete, "/expenses/"+resp.Data.ID.String(), voidBody{Reason: "duplicate"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if !repo.expense.IsVoid {
		t.Error("expense was not marked void")
	}
}
