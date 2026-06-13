package team_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
)

// hashFor mirrors the service's internal sha256 invite-token hashing so tests
// can seed a link whose stored hash matches a known raw token.
func hashFor(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeTeamRepo struct {
	teams       map[uuid.UUID]*team.Team
	members     map[uuid.UUID]*team.TeamMember // keyed by member ID
	links       map[uuid.UUID]*team.InviteLink
	linkByHash  map[string]*team.InviteLink
	invitations map[uuid.UUID]*team.Invitation

	createErr error
}

func newFakeRepo() *fakeTeamRepo {
	return &fakeTeamRepo{
		teams:       make(map[uuid.UUID]*team.Team),
		members:     make(map[uuid.UUID]*team.TeamMember),
		links:       make(map[uuid.UUID]*team.InviteLink),
		linkByHash:  make(map[string]*team.InviteLink),
		invitations: make(map[uuid.UUID]*team.Invitation),
	}
}

func (r *fakeTeamRepo) Create(_ context.Context, t *team.Team) (*team.Team, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	r.teams[t.ID] = t
	// Owner membership, as the real repo does in one transaction.
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
			if t, ok := r.teams[m.TeamID]; ok {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func (r *fakeTeamRepo) Update(_ context.Context, t *team.Team) (*team.Team, error) {
	r.teams[t.ID] = t
	return t, nil
}

func (r *fakeTeamRepo) SoftDelete(_ context.Context, teamID, deletedBy uuid.UUID) error {
	t, ok := r.teams[teamID]
	if !ok {
		return team.ErrNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	t.DeletedBy = &deletedBy
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

func (r *fakeTeamRepo) GetMemberByID(_ context.Context, memberID uuid.UUID) (*team.TeamMember, error) {
	if m, ok := r.members[memberID]; ok {
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
	var out []*team.TeamMember
	for _, m := range r.members {
		if m.TeamID == teamID && m.Status == team.StatusRequested {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeTeamRepo) InsertMember(_ context.Context, m *team.TeamMember) (*team.TeamMember, error) {
	m.ID = uuid.New()
	m.CreatedAt = time.Now()
	r.members[m.ID] = m
	return m, nil
}

func (r *fakeTeamRepo) UpdateMember(_ context.Context, m *team.TeamMember) (*team.TeamMember, error) {
	r.members[m.ID] = m
	return m, nil
}

func (r *fakeTeamRepo) DeleteMember(_ context.Context, memberID uuid.UUID) error {
	delete(r.members, memberID)
	return nil
}

func (r *fakeTeamRepo) CreateInviteLink(_ context.Context, l *team.InviteLink) (*team.InviteLink, error) {
	l.ID = uuid.New()
	l.CreatedAt = time.Now()
	r.links[l.ID] = l
	r.linkByHash[l.TokenHash] = l
	return l, nil
}

func (r *fakeTeamRepo) ListInviteLinks(_ context.Context, teamID uuid.UUID) ([]*team.InviteLink, error) {
	var out []*team.InviteLink
	for _, l := range r.links {
		if l.TeamID == teamID && l.RevokedAt == nil {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *fakeTeamRepo) FindInviteLinkByHash(_ context.Context, tokenHash string) (*team.InviteLink, error) {
	if l, ok := r.linkByHash[tokenHash]; ok {
		return l, nil
	}
	return nil, errors.New("not found")
}

func (r *fakeTeamRepo) RevokeInviteLink(_ context.Context, linkID uuid.UUID) error {
	if l, ok := r.links[linkID]; ok {
		now := time.Now()
		l.RevokedAt = &now
	}
	return nil
}

func (r *fakeTeamRepo) IncrementInviteLinkUse(_ context.Context, linkID uuid.UUID, _ *time.Time) error {
	if l, ok := r.links[linkID]; ok {
		l.UseCount++
	}
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
	var m *team.TeamMember
	for _, existing := range r.members {
		if existing.TeamID == inv.TeamID && existing.UserID == userID {
			m = existing
			break
		}
	}
	if m == nil {
		m = &team.TeamMember{ID: uuid.New(), TeamID: inv.TeamID, UserID: userID, CreatedAt: now}
		r.members[m.ID] = m
	}
	m.Role = inv.Role
	m.Status = team.StatusActive
	m.InvitedBy = &inv.InvitedBy
	m.JoinedAt = &now

	inv.Status = team.InvitationAccepted
	inv.AcceptedBy = &userID
	inv.AcceptedAt = &now
	r.invitations[inv.ID] = inv
	return m, nil
}

// fakeUserRepo is a minimal user.Repository — only FindByEmail is exercised by team.Service.
type fakeUserRepo struct {
	byEmail map[string]*user.User
	byID    map[uuid.UUID]*user.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: make(map[string]*user.User), byID: make(map[uuid.UUID]*user.User)}
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (r *fakeUserRepo) Create(context.Context, *user.User) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) CreateAnonymous(context.Context, string, uuid.UUID) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) FindByOAuth(context.Context, string, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) UpsertOAuthAccount(context.Context, uuid.UUID, string, string, *string) error {
	return nil
}
func (r *fakeUserRepo) Update(context.Context, *user.User) (*user.User, error)   { return nil, nil }
func (r *fakeUserRepo) UpdateAvatarURL(context.Context, uuid.UUID, string) error { return nil }
func (r *fakeUserRepo) UpdatePassword(context.Context, uuid.UUID, string) error  { return nil }
func (r *fakeUserRepo) GetNotificationPrefs(context.Context, uuid.UUID) (*user.NotificationPrefs, error) {
	return nil, nil
}
func (r *fakeUserRepo) UpdateNotificationPrefs(context.Context, *user.NotificationPrefs) (*user.NotificationPrefs, error) {
	return nil, nil
}
func (r *fakeUserRepo) CreateClaimToken(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (*user.ClaimToken, error) {
	return nil, nil
}
func (r *fakeUserRepo) Claim(context.Context, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

// recordMailer records team email calls for assertions.
type recordMailer struct {
	invites     []mailCall
	approved    []mailCall
	rejected    []mailCall
	inviteLinks []mailCall
}

type mailCall struct {
	to       string
	teamName string
	extra    string // inviter name (invite) or raw token (invite link)
}

func (m *recordMailer) InvitationEmail(_ context.Context, to, teamName, inviterName, rawToken string) error {
	m.invites = append(m.invites, mailCall{to: to, teamName: teamName, extra: rawToken})
	return nil
}

func (m *recordMailer) JoinApproved(_ context.Context, to, _, teamName string) error {
	m.approved = append(m.approved, mailCall{to: to, teamName: teamName})
	return nil
}

func (m *recordMailer) JoinRejected(_ context.Context, to, _, teamName string) error {
	m.rejected = append(m.rejected, mailCall{to: to, teamName: teamName})
	return nil
}

func (m *recordMailer) InviteLink(_ context.Context, to, teamName, rawToken string) error {
	m.inviteLinks = append(m.inviteLinks, mailCall{to: to, teamName: teamName, extra: rawToken})
	return nil
}

func newSvcWithMailer(repo team.Repository, userRepo user.Repository, mailer team.Mailer) *team.Service {
	return team.NewService(repo, userRepo, audit.NopLogger(), mailer)
}

func newSvc(repo team.Repository, userRepo user.Repository) *team.Service {
	return team.NewService(repo, userRepo, audit.NopLogger(), nil)
}

// addMember directly seeds a membership of the given role/status.
func addMember(repo *fakeTeamRepo, teamID, userID uuid.UUID, role, status string) *team.TeamMember {
	m := &team.TeamMember{ID: uuid.New(), TeamID: teamID, UserID: userID, Role: role, Status: status}
	repo.members[m.ID] = m
	return m
}

// seedTeam creates a team owned by owner (with owner membership) and returns it.
func seedTeam(repo *fakeTeamRepo, owner uuid.UUID, isPublic bool) *team.Team {
	t := &team.Team{ID: uuid.New(), Name: "Trip", Currency: "LKR", IsPublic: isPublic, OwnerID: owner, CreatedBy: owner, CreatedAt: time.Now()}
	repo.teams[t.ID] = t
	addMember(repo, t.ID, owner, team.RoleOwner, team.StatusActive)
	return t
}

// ── Create ─────────────────────────────────────────────────────────────────────

func TestCreate_Valid_AddsOwnerMembership(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()

	got, err := newSvc(repo, newFakeUserRepo()).Create(context.Background(), owner, "Trip", "desc", "lkr", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Currency != "LKR" {
		t.Errorf("currency = %q, want normalised LKR", got.Currency)
	}
	m, err := repo.GetMembership(context.Background(), got.ID, owner)
	if err != nil {
		t.Fatalf("owner membership missing: %v", err)
	}
	if m.Role != team.RoleOwner || m.Status != team.StatusActive {
		t.Errorf("owner membership = %s/%s, want owner/active", m.Role, m.Status)
	}
}

func TestCreate_EmptyName_Error(t *testing.T) {
	_, err := newSvc(newFakeRepo(), newFakeUserRepo()).Create(context.Background(), uuid.New(), "  ", "", "LKR", false)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreate_BadCurrency_Error(t *testing.T) {
	_, err := newSvc(newFakeRepo(), newFakeUserRepo()).Create(context.Background(), uuid.New(), "Trip", "", "RUPEES", false)
	if err == nil {
		t.Fatal("expected error for bad currency length")
	}
}

// ── permission gating via requireMembership ────────────────────────────────────

func TestUpdate_NonMember_NotMember(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)

	name := "Renamed"
	_, err := newSvc(repo, newFakeUserRepo()).Update(context.Background(), tm.ID, uuid.New(), &name, nil, nil)
	if !errors.Is(err, team.ErrNotMember) {
		t.Fatalf("want ErrNotMember, got %v", err)
	}
}

func TestUpdate_PlainMember_InsufficientRole(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	member := uuid.New()
	addMember(repo, tm.ID, member, team.RoleMember, team.StatusActive)

	name := "Renamed"
	_, err := newSvc(repo, newFakeUserRepo()).Update(context.Background(), tm.ID, member, &name, nil, nil)
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

func TestUpdate_Admin_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)
	admin := uuid.New()
	addMember(repo, tm.ID, admin, team.RoleAdmin, team.StatusActive)

	name := "Renamed"
	pub := true
	got, err := newSvc(repo, newFakeUserRepo()).Update(context.Background(), tm.ID, admin, &name, nil, &pub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Renamed" || !got.IsPublic {
		t.Errorf("update not applied: %+v", got)
	}
}

func TestUpdate_InvitedMember_TreatedAsNotMember(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)
	pending := uuid.New()
	addMember(repo, tm.ID, pending, team.RoleAdmin, team.StatusInvited) // not active

	name := "X"
	_, err := newSvc(repo, newFakeUserRepo()).Update(context.Background(), tm.ID, pending, &name, nil, nil)
	if !errors.Is(err, team.ErrNotMember) {
		t.Fatalf("want ErrNotMember for non-active membership, got %v", err)
	}
}

// ── Delete (owner only) ────────────────────────────────────────────────────────

func TestDelete_Owner_SoftDeletes(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)

	if err := newSvc(repo, newFakeUserRepo()).Delete(context.Background(), tm.ID, owner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.teams[tm.ID].DeletedAt == nil {
		t.Error("team was not soft-deleted")
	}
}

func TestDelete_Admin_InsufficientRole(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)
	admin := uuid.New()
	addMember(repo, tm.ID, admin, team.RoleAdmin, team.StatusActive)

	err := newSvc(repo, newFakeUserRepo()).Delete(context.Background(), tm.ID, admin)
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

// ── InviteByEmail / invitations ─────────────────────────────────────────────────

func TestInviteByEmail_NonMember_CreatesPendingAndEmails(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	users := newFakeUserRepo()
	users.byID[owner] = &user.User{ID: owner, DisplayName: "Owner Olive"}

	mailer := &recordMailer{}
	inv, err := newSvcWithMailer(repo, users, mailer).
		InviteByEmail(context.Background(), tm.ID, owner, "Stranger@X.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Status != team.InvitationPending {
		t.Errorf("status = %q, want pending", inv.Status)
	}
	if inv.Email != "stranger@x.com" {
		t.Errorf("email = %q, want lowercased", inv.Email)
	}
	if inv.Role != team.RoleMember {
		t.Errorf("role = %q, want member", inv.Role)
	}
	if inv.ExpiresAt.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Errorf("expiry %v should be ~7 days out", inv.ExpiresAt)
	}
	if len(mailer.invites) != 1 || mailer.invites[0].to != "stranger@x.com" {
		t.Fatalf("want 1 invite email to the address, got %+v", mailer.invites)
	}
}

func TestInviteByEmail_InvalidEmail(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)

	_, err := newSvc(repo, newFakeUserRepo()).InviteByEmail(context.Background(), tm.ID, owner, "not-an-email")
	if !errors.Is(err, team.ErrInvalidEmail) {
		t.Fatalf("want ErrInvalidEmail, got %v", err)
	}
}

func TestInviteByEmail_AlreadyActiveMember(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)

	users := newFakeUserRepo()
	existing := &user.User{ID: uuid.New()}
	users.byEmail["dup@x.com"] = existing
	addMember(repo, tm.ID, existing.ID, team.RoleMember, team.StatusActive)

	_, err := newSvc(repo, users).InviteByEmail(context.Background(), tm.ID, owner, "dup@x.com")
	if !errors.Is(err, team.ErrAlreadyMember) {
		t.Fatalf("want ErrAlreadyMember, got %v", err)
	}
}

func TestInviteByEmail_DuplicatePending(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	svc := newSvc(repo, newFakeUserRepo())

	if _, err := svc.InviteByEmail(context.Background(), tm.ID, owner, "dup@x.com"); err != nil {
		t.Fatalf("first invite failed: %v", err)
	}
	_, err := svc.InviteByEmail(context.Background(), tm.ID, owner, "dup@x.com")
	if !errors.Is(err, team.ErrInvitationExists) {
		t.Fatalf("want ErrInvitationExists, got %v", err)
	}
}

func TestInviteByEmail_PlainMember_InsufficientRole(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)
	member := uuid.New()
	addMember(repo, tm.ID, member, team.RoleMember, team.StatusActive)

	_, err := newSvc(repo, newFakeUserRepo()).InviteByEmail(context.Background(), tm.ID, member, "x@x.com")
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

// inviteAndToken creates a pending invitation and returns it with its raw token,
// which the recordMailer captures (the hash alone can't be reversed).
func inviteAndToken(t *testing.T, repo *fakeTeamRepo, users *fakeUserRepo, teamID, inviter uuid.UUID, email string) (*team.Invitation, string) {
	t.Helper()
	mailer := &recordMailer{}
	inv, err := newSvcWithMailer(repo, users, mailer).InviteByEmail(context.Background(), teamID, inviter, email)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if len(mailer.invites) == 0 {
		t.Fatalf("no invite email captured")
	}
	return inv, mailer.invites[len(mailer.invites)-1].extra
}

func TestAcceptInvitation_HappyPath_CreatesActiveMember(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	inv, rawToken := inviteAndToken(t, repo, newFakeUserRepo(), tm.ID, owner, "newbie@x.com")

	accepter := uuid.New()
	m, err := newSvc(repo, newFakeUserRepo()).AcceptInvitation(context.Background(), rawToken, accepter)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if m.Status != team.StatusActive || m.UserID != accepter || m.Role != team.RoleMember {
		t.Errorf("member = %+v, want active member for accepter", m)
	}
	if repo.invitations[inv.ID].Status != team.InvitationAccepted {
		t.Errorf("invitation not marked accepted: %q", repo.invitations[inv.ID].Status)
	}
}

func TestAcceptInvitation_InvalidToken(t *testing.T) {
	repo := newFakeRepo()
	_, err := newSvc(repo, newFakeUserRepo()).AcceptInvitation(context.Background(), "nope", uuid.New())
	if !errors.Is(err, team.ErrInvitationInvalid) {
		t.Fatalf("want ErrInvitationInvalid, got %v", err)
	}
}

func TestAcceptInvitation_Expired(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	inv, raw := inviteAndToken(t, repo, newFakeUserRepo(), tm.ID, owner, "late@x.com")
	repo.invitations[inv.ID].ExpiresAt = time.Now().Add(-time.Hour)

	_, err := newSvc(repo, newFakeUserRepo()).AcceptInvitation(context.Background(), raw, uuid.New())
	if !errors.Is(err, team.ErrInvitationInvalid) {
		t.Fatalf("want ErrInvitationInvalid for expired, got %v", err)
	}
}

func TestAcceptInvitation_Cancelled_CannotAccept(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	inv, raw := inviteAndToken(t, repo, newFakeUserRepo(), tm.ID, owner, "x@x.com")

	if err := newSvc(repo, newFakeUserRepo()).CancelInvitation(context.Background(), tm.ID, inv.ID, owner); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	_, err := newSvc(repo, newFakeUserRepo()).AcceptInvitation(context.Background(), raw, uuid.New())
	if !errors.Is(err, team.ErrInvitationInvalid) {
		t.Fatalf("want ErrInvitationInvalid after cancel, got %v", err)
	}
}

func TestCancelInvitation_Admin(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	svc := newSvc(repo, newFakeUserRepo())
	inv, _ := svc.InviteByEmail(context.Background(), tm.ID, owner, "x@x.com")

	if err := svc.CancelInvitation(context.Background(), tm.ID, inv.ID, owner); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if repo.invitations[inv.ID].Status != team.InvitationCancelled {
		t.Errorf("status = %q, want cancelled", repo.invitations[inv.ID].Status)
	}
	// No longer listed as pending.
	pending, _ := svc.ListInvitations(context.Background(), tm.ID, owner)
	if len(pending) != 0 {
		t.Errorf("want 0 pending after cancel, got %d", len(pending))
	}
}

func TestCancelInvitation_PlainMember_InsufficientRole(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	member := uuid.New()
	addMember(repo, tm.ID, member, team.RoleMember, team.StatusActive)
	inv, _ := newSvc(repo, newFakeUserRepo()).InviteByEmail(context.Background(), tm.ID, owner, "x@x.com")

	err := newSvc(repo, newFakeUserRepo()).CancelInvitation(context.Background(), tm.ID, inv.ID, member)
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

func TestResendInvitation_RotatesTokenAndReEmails(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	mailer := &recordMailer{}
	svc := newSvcWithMailer(repo, newFakeUserRepo(), mailer)
	inv, _ := svc.InviteByEmail(context.Background(), tm.ID, owner, "x@x.com")
	oldHash := repo.invitations[inv.ID].TokenHash

	updated, err := svc.ResendInvitation(context.Background(), tm.ID, inv.ID, owner)
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	if updated.TokenHash == oldHash {
		t.Error("resend should rotate the token hash")
	}
	if len(mailer.invites) != 2 {
		t.Errorf("want 2 emails (invite + resend), got %d", len(mailer.invites))
	}
}

func TestApproveJoin_SendsApprovedEmail(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, true)
	joiner := uuid.New()
	req := addMember(repo, tm.ID, joiner, team.RoleMember, team.StatusRequested)

	users := newFakeUserRepo()
	email := "joiner@x.com"
	users.byID[joiner] = &user.User{ID: joiner, DisplayName: "Joe", Email: &email}

	mailer := &recordMailer{}
	if _, err := newSvcWithMailer(repo, users, mailer).ApproveJoin(context.Background(), tm.ID, req.ID, owner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.approved) != 1 || mailer.approved[0].to != email {
		t.Fatalf("want 1 approved email to %s, got %+v", email, mailer.approved)
	}
	if len(mailer.rejected) != 0 {
		t.Error("did not expect a rejected email")
	}
}

func TestRejectJoin_SendsRejectedEmail(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, true)
	joiner := uuid.New()
	req := addMember(repo, tm.ID, joiner, team.RoleMember, team.StatusRequested)

	users := newFakeUserRepo()
	email := "joiner@x.com"
	users.byID[joiner] = &user.User{ID: joiner, DisplayName: "Joe", Email: &email}

	mailer := &recordMailer{}
	if _, err := newSvcWithMailer(repo, users, mailer).RejectJoin(context.Background(), tm.ID, req.ID, owner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.rejected) != 1 || mailer.rejected[0].to != email {
		t.Fatalf("want 1 rejected email to %s, got %+v", email, mailer.rejected)
	}
}

func TestApproveJoin_AnonymousRequester_NoEmail(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, true)
	joiner := uuid.New() // not seeded in users.byID → no email on file
	req := addMember(repo, tm.ID, joiner, team.RoleMember, team.StatusRequested)

	mailer := &recordMailer{}
	if _, err := newSvcWithMailer(repo, newFakeUserRepo(), mailer).ApproveJoin(context.Background(), tm.ID, req.ID, owner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.approved) != 0 {
		t.Errorf("no email expected when requester has no email, got %+v", mailer.approved)
	}
}

func TestCreateInviteLink_WithEmail_SendsInviteLinkEmail(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)

	mailer := &recordMailer{}
	to := "friend@x.com"
	_, raw, err := newSvcWithMailer(repo, newFakeUserRepo(), mailer).
		CreateInviteLink(context.Background(), tm.ID, owner, nil, nil, &to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.inviteLinks) != 1 {
		t.Fatalf("want 1 invite-link email, got %d", len(mailer.inviteLinks))
	}
	got := mailer.inviteLinks[0]
	if got.to != to || got.teamName != tm.Name || got.extra != raw {
		t.Errorf("invite-link email = %+v, want to=%s team=%s token=%s", got, to, tm.Name, raw)
	}
}

func TestCreateInviteLink_NoEmail_NoEmailSent(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)

	mailer := &recordMailer{}
	if _, _, err := newSvcWithMailer(repo, newFakeUserRepo(), mailer).
		CreateInviteLink(context.Background(), tm.ID, owner, nil, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.inviteLinks) != 0 {
		t.Errorf("no email expected when email is nil, got %+v", mailer.inviteLinks)
	}
}

// ── RequestJoin / ApproveJoin / RejectJoin ─────────────────────────────────────

func TestRequestJoin_PublicTeam_CreatesRequest(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), true)
	joiner := uuid.New()

	m, err := newSvc(repo, newFakeUserRepo()).RequestJoin(context.Background(), tm.ID, joiner, "let me in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != team.StatusRequested {
		t.Errorf("status = %q, want requested", m.Status)
	}
}

func TestRequestJoin_PrivateTeam_NotPublic(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)

	_, err := newSvc(repo, newFakeUserRepo()).RequestJoin(context.Background(), tm.ID, uuid.New(), "")
	if !errors.Is(err, team.ErrTeamNotPublic) {
		t.Fatalf("want ErrTeamNotPublic, got %v", err)
	}
}

func TestApproveJoin_Admin_ActivatesMember(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, true)
	joiner := uuid.New()
	req := addMember(repo, tm.ID, joiner, team.RoleMember, team.StatusRequested)

	m, err := newSvc(repo, newFakeUserRepo()).ApproveJoin(context.Background(), tm.ID, req.ID, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != team.StatusActive || m.JoinedAt == nil {
		t.Errorf("approve did not activate member: %+v", m)
	}
}

func TestRejectJoin_Admin_SetsRejected(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, true)
	req := addMember(repo, tm.ID, uuid.New(), team.RoleMember, team.StatusRequested)

	m, err := newSvc(repo, newFakeUserRepo()).RejectJoin(context.Background(), tm.ID, req.ID, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != team.StatusRejected {
		t.Errorf("status = %q, want rejected", m.Status)
	}
}

// ── ChangeRole (the permission matrix + ownership transfer) ─────────────────────

func TestChangeRole_OwnerPromotesMemberToAdmin(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	target := uuid.New()
	addMember(repo, tm.ID, target, team.RoleMember, team.StatusActive)

	m, err := newSvc(repo, newFakeUserRepo()).ChangeRole(context.Background(), tm.ID, target, owner, team.RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Role != team.RoleAdmin {
		t.Errorf("role = %q, want admin", m.Role)
	}
}

func TestChangeRole_AdminCannotPromoteToOwner(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)
	admin := uuid.New()
	addMember(repo, tm.ID, admin, team.RoleAdmin, team.StatusActive)
	target := uuid.New()
	addMember(repo, tm.ID, target, team.RoleMember, team.StatusActive)

	_, err := newSvc(repo, newFakeUserRepo()).ChangeRole(context.Background(), tm.ID, target, admin, team.RoleOwner)
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

func TestChangeRole_OwnershipTransfer_DemotesPreviousOwner(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	target := uuid.New()
	addMember(repo, tm.ID, target, team.RoleAdmin, team.StatusActive)

	m, err := newSvc(repo, newFakeUserRepo()).ChangeRole(context.Background(), tm.ID, target, owner, team.RoleOwner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Role != team.RoleOwner {
		t.Errorf("target role = %q, want owner", m.Role)
	}
	// Team owner_id reassigned.
	if repo.teams[tm.ID].OwnerID != target {
		t.Errorf("team owner = %v, want %v", repo.teams[tm.ID].OwnerID, target)
	}
	// Previous owner demoted to admin.
	prev, _ := repo.GetMembership(context.Background(), tm.ID, owner)
	if prev.Role != team.RoleAdmin {
		t.Errorf("previous owner role = %q, want admin", prev.Role)
	}
}

func TestChangeRole_InactiveTarget_Error(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	target := uuid.New()
	addMember(repo, tm.ID, target, team.RoleMember, team.StatusInvited)

	_, err := newSvc(repo, newFakeUserRepo()).ChangeRole(context.Background(), tm.ID, target, owner, team.RoleAdmin)
	if err == nil {
		t.Fatal("expected error changing role of non-active member")
	}
}

// ── RemoveMember ───────────────────────────────────────────────────────────────

func TestRemoveMember_AdminRemovesMember(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	target := uuid.New()
	addMember(repo, tm.ID, target, team.RoleMember, team.StatusActive)

	if err := newSvc(repo, newFakeUserRepo()).RemoveMember(context.Background(), tm.ID, target, owner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, _ := repo.GetMembership(context.Background(), tm.ID, target)
	if m.Status != team.StatusRemoved {
		t.Errorf("status = %q, want removed", m.Status)
	}
}

func TestRemoveMember_SelfLeave_SetsLeft(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	member := uuid.New()
	addMember(repo, tm.ID, member, team.RoleMember, team.StatusActive)

	if err := newSvc(repo, newFakeUserRepo()).RemoveMember(context.Background(), tm.ID, member, member); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, _ := repo.GetMembership(context.Background(), tm.ID, member)
	if m.Status != team.StatusLeft {
		t.Errorf("status = %q, want left", m.Status)
	}
}

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	admin := uuid.New()
	addMember(repo, tm.ID, admin, team.RoleAdmin, team.StatusActive)

	err := newSvc(repo, newFakeUserRepo()).RemoveMember(context.Background(), tm.ID, owner, admin)
	if !errors.Is(err, team.ErrCannotRemoveOwner) {
		t.Fatalf("want ErrCannotRemoveOwner, got %v", err)
	}
}

func TestRemoveMember_MemberCannotRemoveOther(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	a := uuid.New()
	b := uuid.New()
	addMember(repo, tm.ID, a, team.RoleMember, team.StatusActive)
	addMember(repo, tm.ID, b, team.RoleMember, team.StatusActive)

	err := newSvc(repo, newFakeUserRepo()).RemoveMember(context.Background(), tm.ID, b, a)
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

// ── Invite links ───────────────────────────────────────────────────────────────

func TestCreateInviteLink_Admin_ReturnsRawToken(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)

	maxUses := 5
	link, raw, err := newSvc(repo, newFakeUserRepo()).CreateInviteLink(context.Background(), tm.ID, owner, &maxUses, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw == "" {
		t.Error("expected a raw token")
	}
	if link.TokenHash == raw {
		t.Error("stored token must be hashed, not the raw token")
	}
}

func TestCreateInviteLink_PlainMember_InsufficientRole(t *testing.T) {
	repo := newFakeRepo()
	tm := seedTeam(repo, uuid.New(), false)
	member := uuid.New()
	addMember(repo, tm.ID, member, team.RoleMember, team.StatusActive)

	_, _, err := newSvc(repo, newFakeUserRepo()).CreateInviteLink(context.Background(), tm.ID, member, nil, nil, nil)
	if !errors.Is(err, team.ErrInsufficientRole) {
		t.Fatalf("want ErrInsufficientRole, got %v", err)
	}
}

func TestJoinViaInviteLink_Valid_AddsActiveMemberAndIncrements(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	_, raw, err := newSvc(repo, newFakeUserRepo()).CreateInviteLink(context.Background(), tm.ID, owner, nil, nil, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	joiner := uuid.New()
	m, err := newSvc(repo, newFakeUserRepo()).JoinViaInviteLink(context.Background(), raw, joiner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != team.StatusActive {
		t.Errorf("status = %q, want active", m.Status)
	}
	// Use count incremented.
	for _, l := range repo.links {
		if l.UseCount != 1 {
			t.Errorf("use count = %d, want 1", l.UseCount)
		}
	}
}

func TestJoinViaInviteLink_Revoked_Invalid(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	link, raw, err := newSvc(repo, newFakeUserRepo()).CreateInviteLink(context.Background(), tm.ID, owner, nil, nil, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	_ = newSvc(repo, newFakeUserRepo()).RevokeInviteLink(context.Background(), tm.ID, link.ID, owner)

	_, err = newSvc(repo, newFakeUserRepo()).JoinViaInviteLink(context.Background(), raw, uuid.New())
	if !errors.Is(err, team.ErrInviteLinkInvalid) {
		t.Fatalf("want ErrInviteLinkInvalid, got %v", err)
	}
}

func TestJoinViaInviteLink_Exhausted_Invalid(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	maxUses := 1
	_, raw, err := newSvc(repo, newFakeUserRepo()).CreateInviteLink(context.Background(), tm.ID, owner, &maxUses, nil, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	// First join consumes the single use.
	if _, err := newSvc(repo, newFakeUserRepo()).JoinViaInviteLink(context.Background(), raw, uuid.New()); err != nil {
		t.Fatalf("first join: %v", err)
	}
	// Second join must be rejected.
	_, err = newSvc(repo, newFakeUserRepo()).JoinViaInviteLink(context.Background(), raw, uuid.New())
	if !errors.Is(err, team.ErrInviteLinkInvalid) {
		t.Fatalf("want ErrInviteLinkInvalid on exhausted link, got %v", err)
	}
}

func TestJoinViaInviteLink_Expired_Invalid(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	// Insert an already-expired link directly.
	past := time.Now().Add(-time.Hour)
	link := &team.InviteLink{TeamID: tm.ID, CreatedBy: owner, TokenHash: hashFor("expired"), ExpiresAt: &past}
	link.ID = uuid.New()
	repo.links[link.ID] = link
	repo.linkByHash[link.TokenHash] = link

	_, err := newSvc(repo, newFakeUserRepo()).JoinViaInviteLink(context.Background(), "expired", uuid.New())
	if !errors.Is(err, team.ErrInviteLinkInvalid) {
		t.Fatalf("want ErrInviteLinkInvalid on expired link, got %v", err)
	}
}

func TestJoinViaInviteLink_AlreadyActive_AlreadyMember(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	tm := seedTeam(repo, owner, false)
	_, raw, err := newSvc(repo, newFakeUserRepo()).CreateInviteLink(context.Background(), tm.ID, owner, nil, nil, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	_, err = newSvc(repo, newFakeUserRepo()).JoinViaInviteLink(context.Background(), raw, owner)
	if !errors.Is(err, team.ErrAlreadyMember) {
		t.Fatalf("want ErrAlreadyMember, got %v", err)
	}
}
