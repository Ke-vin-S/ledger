package repository_test

import (
	"context"
	"testing"

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
