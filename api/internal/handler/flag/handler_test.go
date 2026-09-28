package flag

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
	"github.com/Ke-vin-S/ledger/api/internal/domain/flag"
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
	flags map[uuid.UUID]*flag.Flag
}

func newFakeRepo() *fakeRepo { return &fakeRepo{flags: make(map[uuid.UUID]*flag.Flag)} }

func (r *fakeRepo) Create(_ context.Context, f *flag.Flag) (*flag.Flag, error) {
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	r.flags[f.ID] = f
	return f, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*flag.Flag, error) {
	if f, ok := r.flags[id]; ok {
		return f, nil
	}
	return nil, flag.ErrNotFound
}

func (r *fakeRepo) ListByExpense(_ context.Context, expenseID uuid.UUID) ([]*flag.Flag, error) {
	var out []*flag.Flag
	for _, f := range r.flags {
		if f.ExpenseID == expenseID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *fakeRepo) Resolve(_ context.Context, id, resolvedBy uuid.UUID, note string) (*flag.Flag, error) {
	f := r.flags[id]
	f.Status = flag.StatusResolved
	f.ResolvedBy = &resolvedBy
	f.ResolutionNote = &note
	return f, nil
}

type allowAccess struct{}

func (allowAccess) CanRead(context.Context, uuid.UUID, uuid.UUID) error  { return nil }
func (allowAccess) CanWrite(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type denyAccess struct{}

func (denyAccess) CanRead(context.Context, uuid.UUID, uuid.UUID) error  { return flag.ErrForbidden }
func (denyAccess) CanWrite(context.Context, uuid.UUID, uuid.UUID) error { return flag.ErrForbidden }

func router(repo flag.Repository, actor uuid.UUID) http.Handler {
	return routerWithAccess(repo, actor, allowAccess{})
}

func routerWithAccess(repo flag.Repository, actor uuid.UUID, access flag.ExpenseAccessChecker) http.Handler {
	svc := flag.NewService(repo, access, audit.NopLogger())
	h := New(svc)
	root := chi.NewRouter()
	root.Mount("/expenses/{expenseId}/flags", h.ExpenseRoutes(authAs(actor)))
	root.Mount("/flags", h.FlagRoutes(authAs(actor)))
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

func TestRaiseFlag_Valid_201(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+uuid.New().String()+"/flags", raiseBody{Reason: "amounts look wrong"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.flags) != 1 {
		t.Errorf("expected one flag stored, got %d", len(repo.flags))
	}
}

func TestRaiseFlag_EmptyReason_400(t *testing.T) {
	actor := uuid.New()
	h := router(newFakeRepo(), actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/"+uuid.New().String()+"/flags", raiseBody{Reason: "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRaiseFlag_BadExpenseID_400(t *testing.T) {
	actor := uuid.New()
	h := router(newFakeRepo(), actor)

	rec := doJSON(t, h, http.MethodPost, "/expenses/not-a-uuid/flags", raiseBody{Reason: "x"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestResolveFlag_Open_200(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	f := &flag.Flag{ID: uuid.New(), ExpenseID: uuid.New(), RaisedBy: uuid.New(), Reason: "x", Status: flag.StatusOpen}
	repo.flags[f.ID] = f
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/flags/"+f.ID.String()+"/resolve", resolveBody{ResolutionNote: "looked into it"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.flags[f.ID].Status != flag.StatusResolved {
		t.Errorf("status = %q, want resolved", repo.flags[f.ID].Status)
	}
}

func TestResolveFlag_AlreadyResolved_409(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	f := &flag.Flag{ID: uuid.New(), ExpenseID: uuid.New(), RaisedBy: uuid.New(), Reason: "x", Status: flag.StatusResolved}
	repo.flags[f.ID] = f
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/flags/"+f.ID.String()+"/resolve", resolveBody{ResolutionNote: "again"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

func TestResolveFlag_Missing_404(t *testing.T) {
	h := router(newFakeRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/flags/"+uuid.New().String()+"/resolve", resolveBody{ResolutionNote: "x"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestListFlags_Forbidden_403(t *testing.T) {
	h := routerWithAccess(newFakeRepo(), uuid.New(), denyAccess{})
	rec := doJSON(t, h, http.MethodGet, "/expenses/"+uuid.New().String()+"/flags", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestResolveFlag_Forbidden_403(t *testing.T) {
	repo := newFakeRepo()
	f := &flag.Flag{ID: uuid.New(), ExpenseID: uuid.New(), RaisedBy: uuid.New(), Reason: "x", Status: flag.StatusOpen}
	repo.flags[f.ID] = f
	h := routerWithAccess(repo, uuid.New(), denyAccess{})

	rec := doJSON(t, h, http.MethodPost, "/flags/"+f.ID.String()+"/resolve", resolveBody{ResolutionNote: "no"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if f.Status != flag.StatusOpen {
		t.Fatal("denied resolution changed flag status")
	}
}
