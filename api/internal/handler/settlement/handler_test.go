package settlement

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/settlement"
)

// ── test harness ──────────────────────────────────────────────────────────────

// authAs returns a middleware that injects the given user as the authenticated caller,
// standing in for the real JWT middleware.
func authAs(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &jwtauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}
			next.ServeHTTP(w, r.WithContext(jwtauth.SetClaims(r.Context(), claims)))
		})
	}
}

type fakeRepo struct {
	stored          *settlement.Settlement
	expense         *settlement.ExpenseAccess
	balance         *settlement.DebtBalance
	balErr          error
	balanceDebtorID uuid.UUID
}

type fakeTeamGateway struct {
	role   string
	status string
	err    error
}

func (g *fakeTeamGateway) GetMembership(_ context.Context, _, _ uuid.UUID) (string, string, error) {
	return g.role, g.status, g.err
}

func (r *fakeRepo) Create(_ context.Context, s *settlement.Settlement) (*settlement.Settlement, error) {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	r.stored = s
	return s, nil
}

func (r *fakeRepo) RecordSettlementTx(_ context.Context, s *settlement.Settlement) (*settlement.Settlement, error) {
	return r.Create(context.Background(), s)
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*settlement.Settlement, error) {
	if r.stored != nil && r.stored.ID == id {
		return r.stored, nil
	}
	return nil, settlement.ErrNotFound
}

func (r *fakeRepo) ListByExpense(_ context.Context, _ uuid.UUID) ([]*settlement.Settlement, error) {
	if r.stored == nil {
		return nil, nil
	}
	return []*settlement.Settlement{r.stored}, nil
}

func (r *fakeRepo) FindExpense(_ context.Context, _ uuid.UUID) (*settlement.ExpenseAccess, error) {
	if r.expense != nil {
		return r.expense, nil
	}
	return nil, settlement.ErrNotFound
}

func (r *fakeRepo) Confirm(_ context.Context, id, confirmedBy uuid.UUID) (*settlement.Settlement, error) {
	if r.stored == nil || r.stored.ID != id {
		return nil, settlement.ErrNotFound
	}
	r.stored.Status = settlement.StatusConfirmed
	r.stored.ConfirmedBy = &confirmedBy
	return r.stored, nil
}

func (r *fakeRepo) Dispute(_ context.Context, id, disputedBy uuid.UUID, reason string) (*settlement.Settlement, error) {
	if r.stored == nil || r.stored.ID != id {
		return nil, settlement.ErrNotFound
	}
	r.stored.Status = settlement.StatusDisputed
	r.stored.DisputedBy = &disputedBy
	r.stored.DisputeReason = &reason
	return r.stored, nil
}

func (r *fakeRepo) GetDebtBalance(_ context.Context, _ uuid.UUID, debtorID uuid.UUID) (*settlement.DebtBalance, error) {
	r.balanceDebtorID = debtorID
	if r.balErr != nil {
		return nil, r.balErr
	}
	return r.balance, nil
}

func (r *fakeRepo) ListTeamNetBalances(_ context.Context, _, _ uuid.UUID) ([]*settlement.TeamBalance, error) {
	return nil, nil
}

func (r *fakeRepo) ListUserNetBalances(_ context.Context, _ uuid.UUID) ([]*settlement.UserBalance, error) {
	return nil, nil
}

func router(repo settlement.Repository, actor uuid.UUID) http.Handler {
	return routerWithTeam(repo, actor, &fakeTeamGateway{})
}

func routerWithTeam(repo settlement.Repository, actor uuid.UUID, teamGW settlement.TeamGateway) http.Handler {
	svc := settlement.NewService(repo, teamGW, audit.NopLogger())
	h := New(svc)
	root := chi.NewRouter()
	root.Mount("/expenses/{expenseId}/settlements", h.ExpenseRoutes(authAs(actor)))
	root.Mount("/settlements", h.SettlementRoutes(authAs(actor)))
	return root
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// errorCode extracts error.code from an error-envelope response body.
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

// ── recordSettlement ────────────────────────────────────────────────────────────

func TestRecordSettlement_Valid_201(t *testing.T) {
	actor, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{expense: &settlement.ExpenseAccess{ID: uuid.New(), PaidBy: payee}, balance: &settlement.DebtBalance{Balance: 5000}}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+uuid.New().String()+"/settlements", recordBody{
		PayerID:   actor.String(),
		PayeeID:   payee.String(),
		Amount:    3000,
		Method:    "cash",
		SettledOn: "2026-01-15",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data settlementResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Amount != 3000 || resp.Data.Status != settlement.StatusPending {
		t.Errorf("unexpected response: %+v", resp.Data)
	}
}

func TestRecordSettlement_ExceedsDebt_409(t *testing.T) {
	actor, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{expense: &settlement.ExpenseAccess{ID: uuid.New(), PaidBy: payee}, balance: &settlement.DebtBalance{Balance: 2000}}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+uuid.New().String()+"/settlements", recordBody{
		PayerID:   actor.String(),
		PayeeID:   payee.String(),
		Amount:    3000,
		Method:    "cash",
		SettledOn: "2026-01-15",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "SETTLEMENT_EXCEEDS_DEBT" {
		t.Errorf("error code = %q, want SETTLEMENT_EXCEEDS_DEBT", code)
	}
}

func TestRecordSettlement_ThirdParty_403(t *testing.T) {
	actor, payer, payee := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepo{expense: &settlement.ExpenseAccess{ID: uuid.New(), PaidBy: payee}, balance: &settlement.DebtBalance{Balance: 5000}}
	h := router(repo, actor) // actor is neither payer nor payee

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+uuid.New().String()+"/settlements", recordBody{
		PayerID:   payer.String(),
		PayeeID:   payee.String(),
		Amount:    1000,
		Method:    "cash",
		SettledOn: "2026-01-15",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecordSettlement_BadExpenseID_400(t *testing.T) {
	actor := uuid.New()
	h := router(&fakeRepo{}, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/not-a-uuid/settlements", recordBody{
		PayerID:   actor.String(),
		PayeeID:   uuid.New().String(),
		Amount:    100,
		Method:    "cash",
		SettledOn: "2026-01-15",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecordSettlement_BadDate_400(t *testing.T) {
	actor, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{balance: &settlement.DebtBalance{Balance: 5000}}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+uuid.New().String()+"/settlements", recordBody{
		PayerID:   actor.String(),
		PayeeID:   payee.String(),
		Amount:    100,
		Method:    "cash",
		SettledOn: "15/01/2026", // wrong format
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_INPUT" {
		t.Errorf("error code = %q, want INVALID_INPUT", code)
	}
}

func TestRecordSettlement_NonCreditorPayee_422(t *testing.T) {
	actor, creditor, accomplice := uuid.New(), uuid.New(), uuid.New()
	expenseID := uuid.New()
	repo := &fakeRepo{
		expense: &settlement.ExpenseAccess{ID: expenseID, PaidBy: creditor},
		balance: &settlement.DebtBalance{Balance: 5000},
	}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+expenseID.String()+"/settlements", recordBody{
		PayerID:   actor.String(),
		PayeeID:   accomplice.String(),
		Amount:    1000,
		Method:    "cash",
		SettledOn: "2026-01-15",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_PAYEE" {
		t.Fatalf("error code = %q, want INVALID_PAYEE", code)
	}
}

// ── list / balance authorization ───────────────────────────────────────────────

func TestListSettlements_UnrelatedActorForbidden(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	s := seedPending(&fakeRepo{}, payer, payee)
	repo := &fakeRepo{stored: s, expense: &settlement.ExpenseAccess{ID: s.ExpenseID, PaidBy: payer}}
	h := router(repo, uuid.New())

	rec := doJSON(t, h, http.MethodGet, "/expenses/"+s.ExpenseID.String()+"/settlements", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestListSettlements_TeamMemberAllowed(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	s := seedPending(&fakeRepo{}, payer, payee)
	teamID := uuid.New()
	repo := &fakeRepo{stored: s, expense: &settlement.ExpenseAccess{ID: s.ExpenseID, PaidBy: payer, TeamID: &teamID}}
	h := routerWithTeam(repo, uuid.New(), &fakeTeamGateway{role: "member", status: "active"})

	rec := doJSON(t, h, http.MethodGet, "/expenses/"+s.ExpenseID.String()+"/settlements", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestDebtBalance_DebtorOverrideIgnoredForNonAdmin(t *testing.T) {
	actor := uuid.New()
	expenseID := uuid.New()
	repo := &fakeRepo{
		expense: &settlement.ExpenseAccess{ID: expenseID, PaidBy: actor},
		balance: &settlement.DebtBalance{Balance: 5000},
	}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodGet, "/expenses/"+expenseID.String()+"/settlements/balance?debtor_id="+uuid.New().String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.balanceDebtorID != actor {
		t.Fatalf("queried debtor = %v, want actor %v", repo.balanceDebtorID, actor)
	}
}

func TestDebtBalance_DebtorOverrideAllowedForAdmin(t *testing.T) {
	admin, debtor := uuid.New(), uuid.New()
	expenseID, teamID := uuid.New(), uuid.New()
	repo := &fakeRepo{
		expense: &settlement.ExpenseAccess{ID: expenseID, PaidBy: admin, TeamID: &teamID},
		balance: &settlement.DebtBalance{Balance: 5000},
	}
	h := routerWithTeam(repo, admin, &fakeTeamGateway{role: "admin", status: "active"})

	rec := doJSON(t, h, http.MethodGet, "/expenses/"+expenseID.String()+"/settlements/balance?debtor_id="+debtor.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.balanceDebtorID != debtor {
		t.Fatalf("queried debtor = %v, want requested debtor %v", repo.balanceDebtorID, debtor)
	}
}

// ── confirm / dispute ───────────────────────────────────────────────────────────

func seedPending(repo *fakeRepo, payer, payee uuid.UUID) *settlement.Settlement {
	s := &settlement.Settlement{
		ID:        uuid.New(),
		ExpenseID: uuid.New(),
		PayerID:   payer,
		PayeeID:   payee,
		Amount:    1000,
		Method:    settlement.MethodCash,
		Status:    settlement.StatusPending,
		SettledOn: time.Now(),
		CreatedAt: time.Now(),
	}
	repo.stored = s
	return s
}

func TestConfirmSettlement_ByPayee_200(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{}
	s := seedPending(repo, payer, payee)
	h := router(repo, payee) // payee confirms

	rec := doJSON(t, h, http.MethodPost, "/settlements/"+s.ID.String()+"/confirm", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.stored.Status != settlement.StatusConfirmed {
		t.Errorf("status = %q, want confirmed", repo.stored.Status)
	}
}

func TestConfirmSettlement_ByThirdParty_403(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{}
	s := seedPending(repo, payer, payee)
	h := router(repo, uuid.New()) // some other user

	rec := doJSON(t, h, http.MethodPost, "/settlements/"+s.ID.String()+"/confirm", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestConfirmSettlement_Missing_404(t *testing.T) {
	h := router(&fakeRepo{}, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/settlements/"+uuid.New().String()+"/confirm", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestDisputeSettlement_ByPayee_200(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{}
	s := seedPending(repo, payer, payee)
	h := router(repo, payee)

	rec := doJSON(t, h, http.MethodPost, "/settlements/"+s.ID.String()+"/dispute", disputeBody{Reason: "never received"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.stored.Status != settlement.StatusDisputed {
		t.Errorf("status = %q, want disputed", repo.stored.Status)
	}
}

func TestDisputeSettlement_EmptyReason_400(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{}
	s := seedPending(repo, payer, payee)
	h := router(repo, payee)

	rec := doJSON(t, h, http.MethodPost, "/settlements/"+s.ID.String()+"/dispute", disputeBody{Reason: ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
