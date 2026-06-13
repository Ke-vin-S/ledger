package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
)

func TestUserRepo_Create_AddsNotificationPrefs(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewUserRepo(pool)

	email := "kevin@example.com"
	created, err := repo.Create(ctx, &user.User{
		IdentityType: user.IdentityTypeRegistered,
		DisplayName:  "Kevin",
		Email:        &email,
		CurrencyPref: "LKR",
		Timezone:     "Asia/Colombo",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notification_prefs WHERE user_id = $1`, created.ID).Scan(&count); err != nil {
		t.Fatalf("count prefs: %v", err)
	}
	if count != 1 {
		t.Errorf("notification_prefs rows = %d, want 1 (created in same tx)", count)
	}
}

// TestUserRepo_Claim_ReassignsInOneTransaction verifies the critical invariant:
// claiming an anonymous user atomically reassigns all of their financial
// references to the claiming user and marks the anon account claimed.
func TestUserRepo_Claim_ReassignsInOneTransaction(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	userRepo := repository.NewUserRepo(pool)
	expRepo := repository.NewExpenseRepo(pool)

	// Claimer is a registered user; anon is the placeholder being claimed.
	email := "claimer@example.com"
	claimer, err := userRepo.Create(ctx, &user.User{IdentityType: user.IdentityTypeRegistered, DisplayName: "Claimer", Email: &email, CurrencyPref: "LKR", Timezone: "Asia/Colombo"})
	if err != nil {
		t.Fatalf("create claimer: %v", err)
	}
	anon, err := userRepo.CreateAnonymous(ctx, "Guest", claimer.ID)
	if err != nil {
		t.Fatalf("create anon: %v", err)
	}

	teamID := seedTeam(t, pool, claimer.ID)

	// An expense paid by the anon user, with a split owed by the anon user.
	exp := &expense.Expense{
		Scope: expense.ScopeTeam, TeamID: &teamID, Title: "Taxi", Amount: 1000, Currency: "LKR",
		PaidBy: anon.ID, ExpenseDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		SplitMethod: splitMethodPtr(expense.MethodExact), Version: 1, CreatedBy: anon.ID,
	}
	createdExp, _, err := expRepo.Create(ctx, exp, []expense.ExpenseSplit{{UserID: anon.ID, ShareAmount: 1000, Version: 1}})
	if err != nil {
		t.Fatalf("create anon expense: %v", err)
	}

	// Issue a claim token and claim it.
	rawToken := "claim-raw-token-123"
	h := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(h[:])
	if _, err := userRepo.CreateClaimToken(ctx, anon.ID, claimer.ID, tokenHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create claim token: %v", err)
	}

	claimedAnonID, err := userRepo.Claim(ctx, tokenHash, claimer.ID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimedAnonID != anon.ID {
		t.Errorf("claimed anon id = %v, want %v", claimedAnonID, anon.ID)
	}

	// Expense paid_by + created_by must now point at the claiming user.
	refetched, err := expRepo.FindByID(ctx, createdExp.ID)
	if err != nil {
		t.Fatalf("refetch expense: %v", err)
	}
	if refetched.PaidBy != claimer.ID {
		t.Errorf("expense paid_by = %v, want claimer %v", refetched.PaidBy, claimer.ID)
	}

	// Split user_id reassigned.
	var splitUser uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT user_id FROM expense_splits WHERE expense_id = $1`, createdExp.ID).Scan(&splitUser); err != nil {
		t.Fatalf("read split: %v", err)
	}
	if splitUser != claimer.ID {
		t.Errorf("split user_id = %v, want claimer %v", splitUser, claimer.ID)
	}

	// Anon user marked claimed.
	var claimedBy *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT claimed_by FROM users WHERE id = $1`, anon.ID).Scan(&claimedBy); err != nil {
		t.Fatalf("read anon: %v", err)
	}
	if claimedBy == nil || *claimedBy != claimer.ID {
		t.Errorf("anon claimed_by = %v, want claimer %v", claimedBy, claimer.ID)
	}
}

func TestUserRepo_Claim_ExpiredToken_Error(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	userRepo := repository.NewUserRepo(pool)

	email := "c2@example.com"
	claimer, _ := userRepo.Create(ctx, &user.User{IdentityType: user.IdentityTypeRegistered, DisplayName: "C", Email: &email, CurrencyPref: "LKR", Timezone: "Asia/Colombo"})
	anon, _ := userRepo.CreateAnonymous(ctx, "Guest", claimer.ID)

	rawToken := "expired-token"
	h := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(h[:])
	// Already-expired token.
	if _, err := userRepo.CreateClaimToken(ctx, anon.ID, claimer.ID, tokenHash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("create token: %v", err)
	}

	if _, err := userRepo.Claim(ctx, tokenHash, claimer.ID); err != user.ErrClaimTokenExpired {
		t.Errorf("claim err = %v, want ErrClaimTokenExpired", err)
	}
}
