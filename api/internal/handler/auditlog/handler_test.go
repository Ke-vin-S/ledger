package auditlog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/auditlog"
	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
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
	byTeam  []*auditlog.LogEntry
	byActor []*auditlog.LogEntry
	lastP   auditlog.ListParams
}

func (r *fakeRepo) ListByTeam(_ context.Context, _ uuid.UUID, p auditlog.ListParams) ([]*auditlog.LogEntry, error) {
	r.lastP = p
	return r.byTeam, nil
}

func (r *fakeRepo) ListByActor(_ context.Context, _ uuid.UUID, p auditlog.ListParams) ([]*auditlog.LogEntry, error) {
	r.lastP = p
	return r.byActor, nil
}

type fakeMemberships struct {
	err error
}

func (m *fakeMemberships) RequireMembership(context.Context, uuid.UUID, uuid.UUID, string) error {
	return m.err
}

func router(repo auditlog.Repository, actor uuid.UUID) (http.Handler, *Handler) {
	return routerWithMembership(repo, actor, &fakeMemberships{})
}

func routerWithMembership(repo auditlog.Repository, actor uuid.UUID, memberships auditlog.MembershipChecker) (http.Handler, *Handler) {
	svc := auditlog.NewService(repo, memberships)
	h := New(svc)
	root := chi.NewRouter()
	root.Mount("/teams/{teamId}/audit", h.TeamRoutes(authAs(actor)))
	root.Mount("/audit", h.MyRoutes(authAs(actor)))
	return root, h
}

func do(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListTeamEntries_200(t *testing.T) {
	repo := &fakeRepo{byTeam: []*auditlog.LogEntry{
		{ID: uuid.New(), Action: "expense.created", EntityType: "expense", EntityID: uuid.New(), CreatedAt: time.Now()},
	}}
	h, _ := router(repo, uuid.New())

	rec := do(t, h, "/teams/"+uuid.New().String()+"/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Items []auditlog.LogEntry `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Items) != 1 {
		t.Errorf("got %d items, want 1", len(resp.Data.Items))
	}
}

func TestListTeamEntries_BadTeamID_400(t *testing.T) {
	h, _ := router(&fakeRepo{}, uuid.New())
	rec := do(t, h, "/teams/not-a-uuid/audit")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestListMyEntries_200(t *testing.T) {
	repo := &fakeRepo{byActor: []*auditlog.LogEntry{
		{ID: uuid.New(), Action: "user.updated", EntityType: "user", EntityID: uuid.New(), CreatedAt: time.Now()},
	}}
	h, _ := router(repo, uuid.New())

	rec := do(t, h, "/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestListTeamEntries_NonMemberForbidden(t *testing.T) {
	h, _ := routerWithMembership(&fakeRepo{}, uuid.New(), &fakeMemberships{err: team.ErrNotMember})
	rec := do(t, h, "/teams/"+uuid.New().String()+"/audit")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("error code = %q, want FORBIDDEN", resp.Error.Code)
	}
}

func TestListEntries_ActionFilterParsed(t *testing.T) {
	repo := &fakeRepo{}
	h, _ := router(repo, uuid.New())

	_ = do(t, h, "/audit?action=expense.created&limit=50")
	if repo.lastP.Action != "expense.created" {
		t.Errorf("action filter = %q, want expense.created", repo.lastP.Action)
	}
	// The service requests one extra row (limit+1) to detect hasMore, so the repo sees 51.
	if repo.lastP.Limit != 51 {
		t.Errorf("limit = %d, want 51 (requested 50 + 1 hasMore probe)", repo.lastP.Limit)
	}
}
