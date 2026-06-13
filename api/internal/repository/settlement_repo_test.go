package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
	"github.com/Ke-vin-S/ledger/api/internal/domain/settlement"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
)

// seedExpenseWithDebt creates a team expense paid by creditor with a single split
// owed by debtor, and returns the expense ID. debtor owes `share` to creditor.
func seedExpenseWithDebt(t *testing.T, ctx context.Context, expRepo expense.Repository, teamID, creditor, debtor uuid.UUID, share int64) uuid.UUID {
	t.Helper()
	e := &expense.Expense{
		Scope:       expense.ScopeTeam,
		TeamID:      &teamID,
		Title:       "Dinner",
		Amount:      share,
		Currency:    "LKR",
		PaidBy:      creditor,
		ExpenseDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		SplitMethod: splitMethodPtr(expense.MethodExact),
		Version:     1,
		CreatedBy:   creditor,
	}
	created, _, err := expRepo.Create(ctx, e, []expense.ExpenseSplit{
		{UserID: debtor, ShareAmount: share, Version: 1},
	})
	if err != nil {
		t.Fatalf("seed expense: %v", err)
	}
	return created.ID
}

func TestSettlementRepo_DebtBalance_DecreasesOnConfirm(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expRepo := repository.NewExpenseRepo(pool)
	setRepo := repository.NewSettlementRepo(pool)

	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	teamID := seedTeam(t, pool, creditor)
	expID := seedExpenseWithDebt(t, ctx, expRepo, teamID, creditor, debtor, 1000)

	// Before any settlement, debtor owes the full 1000.
	bal, err := setRepo.GetDebtBalance(ctx, expID, debtor)
	if err != nil {
		t.Fatalf("debt balance: %v", err)
	}
	if bal.Balance != 1000 {
		t.Fatalf("initial balance = %d, want 1000", bal.Balance)
	}

	// Record a 400 settlement from debtor → creditor and confirm it.
	s, err := setRepo.Create(ctx, &settlement.Settlement{
		ExpenseID: expID, PayerID: debtor, PayeeID: creditor, Amount: 400,
		Method: settlement.MethodCash, RecordedBy: debtor, SettledOn: time.Now(),
	})
	if err != nil {
		t.Fatalf("create settlement: %v", err)
	}
	if _, err := setRepo.Confirm(ctx, s.ID, creditor); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// Confirmed settlement reduces the outstanding balance.
	bal, err = setRepo.GetDebtBalance(ctx, expID, debtor)
	if err != nil {
		t.Fatalf("debt balance after confirm: %v", err)
	}
	if bal.Balance != 600 {
		t.Errorf("balance after confirm = %d, want 600", bal.Balance)
	}
}

func TestSettlementRepo_Confirm_SetsStatusAndConfirmer(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expRepo := repository.NewExpenseRepo(pool)
	setRepo := repository.NewSettlementRepo(pool)

	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	teamID := seedTeam(t, pool, creditor)
	expID := seedExpenseWithDebt(t, ctx, expRepo, teamID, creditor, debtor, 1000)

	s, err := setRepo.Create(ctx, &settlement.Settlement{
		ExpenseID: expID, PayerID: debtor, PayeeID: creditor, Amount: 500,
		Method: settlement.MethodCash, RecordedBy: debtor, SettledOn: time.Now(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Status != settlement.StatusPending {
		t.Errorf("new settlement status = %q, want pending", s.Status)
	}

	confirmed, err := setRepo.Confirm(ctx, s.ID, creditor)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != settlement.StatusConfirmed {
		t.Errorf("status = %q, want confirmed", confirmed.Status)
	}
	if confirmed.ConfirmedBy == nil || *confirmed.ConfirmedBy != creditor {
		t.Error("confirmed_by not set to creditor")
	}
}
