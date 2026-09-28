package repository_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

func TestSettlementRepo_DebtBalance_IgnoresNonCreditorPayment(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expRepo := repository.NewExpenseRepo(pool)
	setRepo := repository.NewSettlementRepo(pool)

	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	accomplice := seedUser(t, pool, "Accomplice")
	teamID := seedTeam(t, pool, creditor)
	expID := seedExpenseWithDebt(t, ctx, expRepo, teamID, creditor, debtor, 1000)
	if _, err := pool.Exec(ctx, `
		INSERT INTO settlements
			(expense_id, payer_id, payee_id, amount, method, status, recorded_by, settled_on)
		VALUES ($1, $2, $3, 1000, 'cash', 'confirmed', $2, CURRENT_DATE)
	`, expID, debtor, accomplice); err != nil {
		t.Fatalf("seed legacy non-creditor settlement: %v", err)
	}

	bal, err := setRepo.GetDebtBalance(ctx, expID, debtor)
	if err != nil {
		t.Fatalf("debt balance: %v", err)
	}
	if bal.Balance != 1000 {
		t.Fatalf("balance = %d, want 1000", bal.Balance)
	}
}

func TestSettlementRepo_RecordSettlementTx_ParallelFullBalance_AtMostOneSuccess(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expRepo := repository.NewExpenseRepo(pool)
	setRepo := repository.NewSettlementRepo(pool)

	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	teamID := seedTeam(t, pool, creditor)
	expID := seedExpenseWithDebt(t, ctx, expRepo, teamID, creditor, debtor, 1000)

	const attempts = 8
	var successes atomic.Int32
	var unexpected atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := setRepo.RecordSettlementTx(ctx, &settlement.Settlement{
				ExpenseID: expID, PayerID: debtor, PayeeID: creditor, Amount: 1000,
				Method: settlement.MethodCash, RecordedBy: debtor, SettledOn: time.Now(),
			})
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, settlement.ErrSettlementExceedsDebt):
			default:
				unexpected.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("successful settlements = %d, want 1", got)
	}
	if got := unexpected.Load(); got != 0 {
		t.Fatalf("unexpected errors = %d, want 0", got)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM settlements WHERE expense_id = $1`, expID).Scan(&count); err != nil {
		t.Fatalf("count settlements: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored settlements = %d, want 1", count)
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

func TestSettlementRepo_TeamAndUserNetBalances_AgreeOnSign(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expRepo := repository.NewExpenseRepo(pool)
	setRepo := repository.NewSettlementRepo(pool)

	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	teamID := seedTeam(t, pool, creditor)
	seedExpenseWithDebt(t, ctx, expRepo, teamID, creditor, debtor, 1000)

	teamBalances, err := setRepo.ListTeamNetBalances(ctx, teamID, debtor)
	if err != nil {
		t.Fatalf("team balances: %v", err)
	}
	if len(teamBalances) != 1 || teamBalances[0].NetAmount != -1000 {
		t.Fatalf("team balance = %+v, want debtor -1000", teamBalances)
	}
	userBalances, err := setRepo.ListUserNetBalances(ctx, debtor)
	if err != nil {
		t.Fatalf("user balances: %v", err)
	}
	if len(userBalances) != 1 || userBalances[0].NetAmount != -1000 {
		t.Fatalf("user balance = %+v, want debtor -1000", userBalances)
	}
}

func TestSettlementRepo_RecordSettlementTx_NotifiesPayee(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expenseRepo := repository.NewExpenseRepo(pool)
	setRepo := repository.NewSettlementRepo(pool)
	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	teamID := seedTeam(t, pool, creditor)
	expenseID := seedExpenseWithDebt(t, ctx, expenseRepo, teamID, creditor, debtor, 1000)

	created, err := setRepo.RecordSettlementTx(ctx, &settlement.Settlement{ExpenseID: expenseID, PayerID: debtor, PayeeID: creditor, Amount: 400, Method: settlement.MethodCash, RecordedBy: debtor, SettledOn: time.Now()})
	if err != nil {
		t.Fatalf("record settlement: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND type='settlement.created' AND entity_id=$2`, creditor, created.ID).Scan(&count); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("payee notifications = %d, want 1", count)
	}
}
