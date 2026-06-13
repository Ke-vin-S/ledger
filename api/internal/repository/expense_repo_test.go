package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
)

func splitMethodPtr(s string) *string { return &s }

func newTeamExpense(teamID, paidBy uuid.UUID, amount int64) *expense.Expense {
	return &expense.Expense{
		Scope:       expense.ScopeTeam,
		TeamID:      &teamID,
		Title:       "Dinner",
		Amount:      amount,
		Currency:    "LKR",
		PaidBy:      paidBy,
		ExpenseDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		SplitMethod: splitMethodPtr(expense.MethodEqual),
		Version:     1,
		CreatedBy:   paidBy,
	}
}

func TestExpenseRepo_CreateAndFind(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewExpenseRepo(pool)

	owner := seedUser(t, pool, "Owner")
	teamID := seedTeam(t, pool, owner)

	created, _, err := repo.Create(ctx, newTeamExpense(teamID, owner, 3000), nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("created expense has nil ID")
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.Amount != 3000 || found.Version != 1 {
		t.Errorf("found = amount %d version %d, want 3000/1", found.Amount, found.Version)
	}
}

func TestExpenseRepo_CreateWithSplits_Atomic(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewExpenseRepo(pool)

	owner := seedUser(t, pool, "Owner")
	other := seedUser(t, pool, "Other")
	teamID := seedTeam(t, pool, owner)

	splits := []expense.ExpenseSplit{
		{UserID: owner, ShareAmount: 1500, Version: 1},
		{UserID: other, ShareAmount: 1500, Version: 1},
	}
	created, createdSplits, err := repo.Create(ctx, newTeamExpense(teamID, owner, 3000), splits)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(createdSplits) != 2 {
		t.Fatalf("got %d splits, want 2", len(createdSplits))
	}

	fetched, err := repo.FindSplitsByExpenseID(ctx, created.ID, 1)
	if err != nil {
		t.Fatalf("find splits: %v", err)
	}
	var sum int64
	for _, s := range fetched {
		sum += s.ShareAmount
	}
	if sum != 3000 {
		t.Errorf("persisted split sum = %d, want 3000", sum)
	}
}

func TestExpenseRepo_Void_SoftDeletes(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewExpenseRepo(pool)

	owner := seedUser(t, pool, "Owner")
	teamID := seedTeam(t, pool, owner)
	created, _, err := repo.Create(ctx, newTeamExpense(teamID, owner, 3000), nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Void(ctx, created.ID, owner, "duplicate"); err != nil {
		t.Fatalf("void: %v", err)
	}

	// Row must still exist (soft delete), flagged void.
	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find after void: %v", err)
	}
	if !found.IsVoid {
		t.Error("expense not marked void")
	}

	// Voiding again must report not-found (no second void).
	if err := repo.Void(ctx, created.ID, owner, "again"); err != expense.ErrNotFound {
		t.Errorf("second void err = %v, want ErrNotFound", err)
	}
}

func TestExpenseRepo_SaveCorrection_SnapshotsAndBumpsVersion(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewExpenseRepo(pool)

	owner := seedUser(t, pool, "Owner")
	teamID := seedTeam(t, pool, owner)
	created, _, err := repo.Create(ctx, newTeamExpense(teamID, owner, 3000), nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	corrected := newTeamExpense(teamID, owner, 4000)
	corrected.ID = created.ID
	corrected.Version = 2
	saved, _, err := repo.SaveCorrection(ctx, created.ID, created, corrected, nil)
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if saved.Amount != 4000 || saved.Version != 2 {
		t.Errorf("saved = amount %d version %d, want 4000/2", saved.Amount, saved.Version)
	}

	// A version snapshot of the prior state must have been written.
	var snapCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM expense_versions WHERE expense_id = $1 AND version = 1`, created.ID).Scan(&snapCount); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if snapCount != 1 {
		t.Errorf("snapshot count for v1 = %d, want 1", snapCount)
	}
}
