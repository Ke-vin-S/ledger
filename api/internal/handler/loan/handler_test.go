package loan

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
	domainloan "github.com/Ke-vin-S/ledger/api/internal/domain/loan"
)

func authAs(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &jwtauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}
			next.ServeHTTP(w, r.WithContext(jwtauth.SetClaims(r.Context(), claims)))
		})
	}
}

type fakeRepo struct {
	loans map[uuid.UUID]*domainloan.Loan
}

func newFakeRepo() *fakeRepo { return &fakeRepo{loans: make(map[uuid.UUID]*domainloan.Loan)} }

func (r *fakeRepo) Create(_ context.Context, in domainloan.CreateInput) (*domainloan.Loan, error) {
	l := &domainloan.Loan{
		ID: uuid.New(), UserID: in.UserID, Direction: in.Direction, Amount: in.Amount,
		Currency: in.Currency, CounterpartyName: in.CounterpartyName, Status: domainloan.StatusOutstanding,
		LoanDate: in.LoanDate, CreatedAt: time.Now(),
	}
	r.loans[l.ID] = l
	return l, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*domainloan.Loan, error) {
	if l, ok := r.loans[id]; ok {
		return l, nil
	}
	return nil, domainloan.ErrNotFound
}

func (r *fakeRepo) ListByUser(_ context.Context, userID uuid.UUID, direction *string) ([]*domainloan.Loan, error) {
	var out []*domainloan.Loan
	for _, l := range r.loans {
		if l.UserID == userID && (direction == nil || l.Direction == *direction) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *fakeRepo) Acknowledge(_ context.Context, id uuid.UUID) (*domainloan.Loan, error) {
	l := r.loans[id]
	l.Status = domainloan.StatusSettled
	return l, nil
}

func (r *fakeRepo) Dispute(_ context.Context, id uuid.UUID, _ *string) (*domainloan.Loan, error) {
	l := r.loans[id]
	l.Status = domainloan.StatusDisputed
	return l, nil
}

func router(repo domainloan.Repository, actor uuid.UUID) http.Handler {
	svc := domainloan.NewService(repo, audit.NopLogger())
	h := New(svc)
	root := chi.NewRouter()
	root.Mount("/loans", h.Routes(authAs(actor)))
	return root
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateLoan_Valid_201(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/loans", createBody{
		Direction: "lent", Amount: 5000, Currency: "LKR", CounterpartyName: "Alice", LoanDate: "2026-01-15",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	// Response is enveloped as {data, meta}.
	var resp struct {
		Data loanResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Status != domainloan.StatusOutstanding {
		t.Errorf("status = %q, want outstanding", resp.Data.Status)
	}
}

func TestCreateLoan_BadDirection_400(t *testing.T) {
	actor := uuid.New()
	h := router(newFakeRepo(), actor)

	rec := doJSON(t, h, http.MethodPost, "/loans", createBody{
		Direction: "sideways", Amount: 5000, Currency: "LKR", CounterpartyName: "Alice", LoanDate: "2026-01-15",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateLoan_BadDate_400(t *testing.T) {
	actor := uuid.New()
	h := router(newFakeRepo(), actor)

	rec := doJSON(t, h, http.MethodPost, "/loans", createBody{
		Direction: "lent", Amount: 5000, Currency: "LKR", CounterpartyName: "Alice", LoanDate: "2026/01/15",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetLoan_Owner_200(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	l, _ := repo.Create(context.Background(), domainloan.CreateInput{UserID: actor, Direction: "lent", Amount: 100, CounterpartyName: "A", LoanDate: time.Now()})
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodGet, "/loans/"+l.ID.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetLoan_NotOwner_403(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	l, _ := repo.Create(context.Background(), domainloan.CreateInput{UserID: owner, Direction: "lent", Amount: 100, CounterpartyName: "A", LoanDate: time.Now()})
	h := router(repo, uuid.New()) // different caller

	rec := doJSON(t, h, http.MethodGet, "/loans/"+l.ID.String(), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetLoan_Missing_404(t *testing.T) {
	h := router(newFakeRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/loans/"+uuid.New().String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetLoan_BadID_400(t *testing.T) {
	h := router(newFakeRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/loans/not-a-uuid", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestAcknowledgeLoan_AlreadySettled_409(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	l, _ := repo.Create(context.Background(), domainloan.CreateInput{UserID: actor, Direction: "lent", Amount: 100, CounterpartyName: "A", LoanDate: time.Now()})
	l.Status = domainloan.StatusSettled
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/loans/"+l.ID.String()+"/acknowledge", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

func TestClaimText_Owner_200(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	l, _ := repo.Create(context.Background(), domainloan.CreateInput{UserID: actor, Direction: "lent", Amount: 5000, Currency: "LKR", CounterpartyName: "Alice", LoanDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)})
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/loans/"+l.ID.String()+"/claim-text", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Text == "" {
		t.Error("expected non-empty claim text")
	}
}
