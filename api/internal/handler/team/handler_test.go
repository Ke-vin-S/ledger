package team

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
)

func authAs(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &jwtauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}
			next.ServeHTTP(w, r.WithContext(jwtauth.SetClaims(r.Context(), claims)))
		})
	}
}

// fakeTeamRepo is a minimal in-memory team.Repository.
type fakeTeamRepo struct {
	teams       map[uuid.UUID]*team.Team
	members     map[uuid.UUID]*team.TeamMember
	invitations map[uuid.UUID]*team.Invitation
}

func newFakeTeamRepo() *fakeTeamRepo {
	return &fakeTeamRepo{
		teams:       make(map[uuid.UUID]*team.Team),
		members:     make(map[uuid.UUID]*team.TeamMember),
		invitations: make(map[uuid.UUID]*team.Invitation),
	}
}

func (r *fakeTeamRepo) Create(_ context.Context, t *team.Team) (*team.Team, error) {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	r.teams[t.ID] = t
	m := &team.TeamMember{ID: uuid.New(), TeamID: t.ID, UserID: t.OwnerID, Role: team.RoleOwner, Status: team.StatusActive}
	r.members[m.ID] = m
	return t, nil
}
func (r *fakeTeamRepo) FindByID(_ context.Context, id uuid.UUID) (*team.Team, error) {
	if t, ok := r.teams[id]; ok && t.DeletedAt == nil {
		return t, nil
	}
	return nil, team.ErrNotFound
}
func (r *fakeTeamRepo) ListForUser(_ context.Context, userID uuid.UUID) ([]*team.Team, error) {
	var out []*team.Team
	for _, m := range r.members {
		if m.UserID == userID && m.Status == team.StatusActive {
			out = append(out, r.teams[m.TeamID])
		}
	}
	return out, nil
}
func (r *fakeTeamRepo) Update(_ context.Context, t *team.Team) (*team.Team, error) {
	r.teams[t.ID] = t
	return t, nil
}
func (r *fakeTeamRepo) SoftDelete(_ context.Context, teamID, deletedBy uuid.UUID) error {
	now := time.Now()
	r.teams[teamID].DeletedAt = &now
	r.teams[teamID].DeletedBy = &deletedBy
	return nil
}
func (r *fakeTeamRepo) GetMembership(_ context.Context, teamID, userID uuid.UUID) (*team.TeamMember, error) {
	for _, m := range r.members {
		if m.TeamID == teamID && m.UserID == userID {
			return m, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *fakeTeamRepo) GetMemberByID(_ context.Context, id uuid.UUID) (*team.TeamMember, error) {
	if m, ok := r.members[id]; ok {
		return m, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeTeamRepo) ListMembers(_ context.Context, teamID uuid.UUID) ([]*team.TeamMember, error) {
	var out []*team.TeamMember
	for _, m := range r.members {
		if m.TeamID == teamID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r *fakeTeamRepo) ListJoinRequests(_ context.Context, teamID uuid.UUID) ([]*team.TeamMember, error) {
	return nil, nil
}
func (r *fakeTeamRepo) InsertMember(_ context.Context, m *team.TeamMember) (*team.TeamMember, error) {
	m.ID = uuid.New()
	r.members[m.ID] = m
	return m, nil
}
func (r *fakeTeamRepo) UpdateMember(_ context.Context, m *team.TeamMember) (*team.TeamMember, error) {
	r.members[m.ID] = m
	return m, nil
}
func (r *fakeTeamRepo) DeleteMember(_ context.Context, id uuid.UUID) error {
	delete(r.members, id)
	return nil
}
func (r *fakeTeamRepo) CreateInviteLink(_ context.Context, l *team.InviteLink) (*team.InviteLink, error) {
	l.ID = uuid.New()
	return l, nil
}
func (r *fakeTeamRepo) ListInviteLinks(_ context.Context, _ uuid.UUID) ([]*team.InviteLink, error) {
	return nil, nil
}
func (r *fakeTeamRepo) FindInviteLinkByHash(_ context.Context, _ string) (*team.InviteLink, error) {
	return nil, errors.New("not found")
}
func (r *fakeTeamRepo) GetInviteLinkByID(_ context.Context, _ uuid.UUID) (*team.InviteLink, error) {
	return nil, team.ErrInviteLinkInvalid
}
func (r *fakeTeamRepo) RevokeInviteLink(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeTeamRepo) IncrementInviteLinkUse(_ context.Context, _ uuid.UUID, _ *time.Time) error {
	return nil
}

func (r *fakeTeamRepo) CreateInvitation(_ context.Context, inv *team.Invitation) (*team.Invitation, error) {
	inv.ID = uuid.New()
	inv.CreatedAt = time.Now()
	r.invitations[inv.ID] = inv
	return inv, nil
}
func (r *fakeTeamRepo) GetInvitationByID(_ context.Context, id uuid.UUID) (*team.Invitation, error) {
	if inv, ok := r.invitations[id]; ok {
		return inv, nil
	}
	return nil, team.ErrInvitationNotFound
}
func (r *fakeTeamRepo) FindInvitationByHash(_ context.Context, hash string) (*team.Invitation, error) {
	for _, inv := range r.invitations {
		if inv.TokenHash == hash {
			return inv, nil
		}
	}
	return nil, team.ErrInvitationInvalid
}
func (r *fakeTeamRepo) FindPendingInvitation(_ context.Context, teamID uuid.UUID, email string) (*team.Invitation, error) {
	for _, inv := range r.invitations {
		if inv.TeamID == teamID && strings.EqualFold(inv.Email, email) && inv.Status == team.InvitationPending {
			return inv, nil
		}
	}
	return nil, team.ErrInvitationNotFound
}
func (r *fakeTeamRepo) ListPendingInvitations(_ context.Context, teamID uuid.UUID) ([]*team.Invitation, error) {
	var out []*team.Invitation
	for _, inv := range r.invitations {
		if inv.TeamID == teamID && inv.Status == team.InvitationPending {
			out = append(out, inv)
		}
	}
	return out, nil
}
func (r *fakeTeamRepo) UpdateInvitation(_ context.Context, inv *team.Invitation) (*team.Invitation, error) {
	r.invitations[inv.ID] = inv
	return inv, nil
}
func (r *fakeTeamRepo) AcceptInvitation(_ context.Context, inv *team.Invitation, userID uuid.UUID) (*team.TeamMember, error) {
	now := time.Now()
	m := &team.TeamMember{ID: uuid.New(), TeamID: inv.TeamID, UserID: userID, Role: inv.Role, Status: team.StatusActive, JoinedAt: &now}
	r.members[m.ID] = m
	inv.Status = team.InvitationAccepted
	r.invitations[inv.ID] = inv
	return m, nil
}

func addMember(r *fakeTeamRepo, teamID, userID uuid.UUID, role, status string) {
	m := &team.TeamMember{ID: uuid.New(), TeamID: teamID, UserID: userID, Role: role, Status: status}
	r.members[m.ID] = m
}

// stubUserRepo is a no-op user.Repository (team handler tests don't exercise email invites).
type stubUserRepo struct{}

func (stubUserRepo) FindByEmail(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (stubUserRepo) Create(context.Context, *user.User) (*user.User, error) { return nil, nil }
func (stubUserRepo) CreateAnonymous(context.Context, string, uuid.UUID) (*user.User, error) {
	return nil, nil
}
func (stubUserRepo) GetAnonymousOwner(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, user.ErrNotFound
}
func (stubUserRepo) FindByID(context.Context, uuid.UUID) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (stubUserRepo) FindByOAuth(context.Context, string, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (stubUserRepo) UpsertOAuthAccount(context.Context, uuid.UUID, string, string, *string) error {
	return nil
}
func (stubUserRepo) Update(context.Context, *user.User) (*user.User, error)   { return nil, nil }
func (stubUserRepo) UpdateAvatarURL(context.Context, uuid.UUID, string) error { return nil }
func (stubUserRepo) UpdatePassword(context.Context, uuid.UUID, string) error  { return nil }
func (stubUserRepo) GetNotificationPrefs(context.Context, uuid.UUID) (*user.NotificationPrefs, error) {
	return nil, nil
}
func (stubUserRepo) UpdateNotificationPrefs(context.Context, *user.NotificationPrefs) (*user.NotificationPrefs, error) {
	return nil, nil
}
func (stubUserRepo) CreateClaimToken(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (*user.ClaimToken, error) {
	return nil, nil
}
func (stubUserRepo) Claim(context.Context, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func router(repo *fakeTeamRepo, actor uuid.UUID) http.Handler {
	svc := team.NewService(repo, stubUserRepo{}, audit.NopLogger(), nil)
	h := New(svc, "https://app.example.com")
	root := chi.NewRouter()
	root.Mount("/teams", h.Routes(authAs(actor)))
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

func seedTeam(repo *fakeTeamRepo, owner uuid.UUID) *team.Team {
	t := &team.Team{ID: uuid.New(), Name: "Trip", Currency: "LKR", OwnerID: owner, CreatedBy: owner}
	repo.teams[t.ID] = t
	addMember(repo, t.ID, owner, team.RoleOwner, team.StatusActive)
	return t
}

func TestCreateTeam_201(t *testing.T) {
	actor := uuid.New()
	repo := newFakeTeamRepo()
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/teams", map[string]any{"name": "Trip", "currency": "LKR"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateTeam_EmptyName_400(t *testing.T) {
	h := router(newFakeTeamRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/teams", map[string]any{"name": "", "currency": "LKR"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestListTeams_200(t *testing.T) {
	actor := uuid.New()
	repo := newFakeTeamRepo()
	seedTeam(repo, actor)
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodGet, "/teams", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetTeam_NonMember_403(t *testing.T) {
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, uuid.New())
	h := router(repo, uuid.New()) // caller is not a member

	rec := doJSON(t, h, http.MethodGet, "/teams/"+tm.ID.String(), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "NOT_TEAM_MEMBER" {
		t.Errorf("error code = %q, want NOT_TEAM_MEMBER", code)
	}
}

func TestDeleteTeam_Admin_403(t *testing.T) {
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, uuid.New())
	admin := uuid.New()
	addMember(repo, tm.ID, admin, team.RoleAdmin, team.StatusActive)
	h := router(repo, admin)

	rec := doJSON(t, h, http.MethodDelete, "/teams/"+tm.ID.String(), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INSUFFICIENT_ROLE" {
		t.Errorf("error code = %q, want INSUFFICIENT_ROLE", code)
	}
}

func TestDeleteTeam_Owner_204orOK(t *testing.T) {
	owner := uuid.New()
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, owner)
	h := router(repo, owner)

	rec := doJSON(t, h, http.MethodDelete, "/teams/"+tm.ID.String(), nil)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 200/204; body=%s", rec.Code, rec.Body.String())
	}
	if repo.teams[tm.ID].DeletedAt == nil {
		t.Error("team was not soft-deleted")
	}
}

func TestGetTeam_BadID_400(t *testing.T) {
	h := router(newFakeTeamRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/teams/not-a-uuid", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// ── Email invitations ────────────────────────────────────────────────────────

func TestInviteByEmail_Admin_201(t *testing.T) {
	owner := uuid.New()
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, owner)
	h := router(repo, owner)

	rec := doJSON(t, h, http.MethodPost, "/teams/"+tm.ID.String()+"/invitations",
		map[string]any{"email": "newbie@x.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Email  string `json:"email"`
			Status string `json:"status"`
			Role   string `json:"role"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data.Email != "newbie@x.com" || resp.Data.Status != "pending" || resp.Data.Role != "member" {
		t.Errorf("unexpected invitation response: %+v", resp.Data)
	}
}

func TestInviteByEmail_NonAdmin_403(t *testing.T) {
	owner := uuid.New()
	member := uuid.New()
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, owner)
	addMember(repo, tm.ID, member, team.RoleMember, team.StatusActive)
	h := router(repo, member)

	rec := doJSON(t, h, http.MethodPost, "/teams/"+tm.ID.String()+"/invitations",
		map[string]any{"email": "x@x.com"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestInviteByEmail_InvalidEmail_400(t *testing.T) {
	owner := uuid.New()
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, owner)
	h := router(repo, owner)

	rec := doJSON(t, h, http.MethodPost, "/teams/"+tm.ID.String()+"/invitations",
		map[string]any{"email": "nope"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_EMAIL" {
		t.Errorf("error code = %q, want INVALID_EMAIL", code)
	}
}

func TestListAndCancelInvitation(t *testing.T) {
	owner := uuid.New()
	repo := newFakeTeamRepo()
	tm := seedTeam(repo, owner)
	h := router(repo, owner)

	// Create one.
	rec := doJSON(t, h, http.MethodPost, "/teams/"+tm.ID.String()+"/invitations",
		map[string]any{"email": "x@x.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// List shows it.
	rec = doJSON(t, h, http.MethodGet, "/teams/"+tm.ID.String()+"/invitations", nil)
	var list struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Data) != 1 {
		t.Fatalf("want 1 pending invitation, got %d", len(list.Data))
	}

	// Cancel it.
	rec = doJSON(t, h, http.MethodDelete, "/teams/"+tm.ID.String()+"/invitations/"+created.Data.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cancel status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}

	// List is now empty.
	rec = doJSON(t, h, http.MethodGet, "/teams/"+tm.ID.String()+"/invitations", nil)
	list.Data = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Data) != 0 {
		t.Errorf("want 0 pending after cancel, got %d", len(list.Data))
	}
}
