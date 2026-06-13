package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
)

func TestTeamRepo_Create_AddsOwnerMembership(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewTeamRepo(pool)

	owner := seedUser(t, pool, "Owner")
	created, err := repo.Create(ctx, &team.Team{Name: "Trip", Currency: "LKR", OwnerID: owner, CreatedBy: owner})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	m, err := repo.GetMembership(ctx, created.ID, owner)
	if err != nil {
		t.Fatalf("owner membership missing: %v", err)
	}
	if m.Role != team.RoleOwner || m.Status != team.StatusActive {
		t.Errorf("owner membership = %s/%s, want owner/active", m.Role, m.Status)
	}
}

func TestTeamRepo_InviteLink_IncrementUse(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewTeamRepo(pool)

	owner := seedUser(t, pool, "Owner")
	tm, err := repo.Create(ctx, &team.Team{Name: "Trip", Currency: "LKR", OwnerID: owner, CreatedBy: owner})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	link, err := repo.CreateInviteLink(ctx, &team.InviteLink{TeamID: tm.ID, CreatedBy: owner, TokenHash: "hash-abc"})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := repo.IncrementInviteLinkUse(ctx, link.ID, nil); err != nil {
		t.Fatalf("increment: %v", err)
	}

	found, err := repo.FindInviteLinkByHash(ctx, "hash-abc")
	if err != nil {
		t.Fatalf("find link: %v", err)
	}
	if found.UseCount != 1 {
		t.Errorf("use count = %d, want 1", found.UseCount)
	}
}

func TestTeamRepo_Invitation_CreateFindAndPendingUniqueness(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewTeamRepo(pool)

	owner := seedUser(t, pool, "Owner")
	tm, err := repo.Create(ctx, &team.Team{Name: "Trip", Currency: "LKR", OwnerID: owner, CreatedBy: owner})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	inv, err := repo.CreateInvitation(ctx, &team.Invitation{
		TeamID: tm.ID, Email: "Newbie@X.com", Role: team.RoleMember, Status: team.InvitationPending,
		TokenHash: "tok-1", InvitedBy: owner, ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	// FindPendingInvitation is case-insensitive on email.
	if _, err := repo.FindPendingInvitation(ctx, tm.ID, "newbie@x.com"); err != nil {
		t.Errorf("find pending (case-insensitive): %v", err)
	}
	if _, err := repo.FindInvitationByHash(ctx, "tok-1"); err != nil {
		t.Errorf("find by hash: %v", err)
	}

	// Partial unique index: a second pending invite for the same team+email fails.
	if _, err := repo.CreateInvitation(ctx, &team.Invitation{
		TeamID: tm.ID, Email: "newbie@x.com", Role: team.RoleMember, Status: team.InvitationPending,
		TokenHash: "tok-2", InvitedBy: owner, ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err == nil {
		t.Error("expected unique-violation on a second pending invitation for the same email")
	}

	// After cancelling, a fresh pending invite is allowed (index only covers pending).
	now := time.Now()
	inv.Status = team.InvitationCancelled
	inv.CancelledBy = &owner
	inv.CancelledAt = &now
	if _, err := repo.UpdateInvitation(ctx, inv); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := repo.CreateInvitation(ctx, &team.Invitation{
		TeamID: tm.ID, Email: "newbie@x.com", Role: team.RoleMember, Status: team.InvitationPending,
		TokenHash: "tok-3", InvitedBy: owner, ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Errorf("re-invite after cancel should be allowed: %v", err)
	}
}

func TestTeamRepo_AcceptInvitation_CreatesActiveMemberInTx(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewTeamRepo(pool)

	owner := seedUser(t, pool, "Owner")
	accepter := seedUser(t, pool, "Accepter")
	tm, err := repo.Create(ctx, &team.Team{Name: "Trip", Currency: "LKR", OwnerID: owner, CreatedBy: owner})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	inv, err := repo.CreateInvitation(ctx, &team.Invitation{
		TeamID: tm.ID, Email: "a@x.com", Role: team.RoleMember, Status: team.InvitationPending,
		TokenHash: "tok-accept", InvitedBy: owner, ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	m, err := repo.AcceptInvitation(ctx, inv, accepter)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if m.Status != team.StatusActive || m.UserID != accepter {
		t.Errorf("member = %+v, want active member for accepter", m)
	}

	// Membership persisted and invitation marked accepted.
	if got, err := repo.GetMembership(ctx, tm.ID, accepter); err != nil || got.Status != team.StatusActive {
		t.Errorf("membership not persisted active: %+v err=%v", got, err)
	}
	got, err := repo.GetInvitationByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("get invitation: %v", err)
	}
	if got.Status != team.InvitationAccepted || got.AcceptedBy == nil || *got.AcceptedBy != accepter {
		t.Errorf("invitation not marked accepted by accepter: %+v", got)
	}
}
