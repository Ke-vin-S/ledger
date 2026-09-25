package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/flag"
	"github.com/Ke-vin-S/ledger/api/internal/domain/loan"
	"github.com/Ke-vin-S/ledger/api/internal/domain/notification"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
)

func TestLoanRepo_CreateFindAcknowledge(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewLoanRepo(pool)

	owner := seedUser(t, pool, "Owner")
	created, err := repo.Create(ctx, loan.CreateInput{
		UserID: owner, Direction: loan.DirectionLent, Amount: 5000, Currency: "LKR",
		CounterpartyName: "Alice", LoanDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find loan: %v", err)
	}
	if found.Amount != 5000 || found.Status != loan.StatusOutstanding {
		t.Errorf("found = amount %d status %q, want 5000/outstanding", found.Amount, found.Status)
	}

	ack, err := repo.Acknowledge(ctx, created.ID)
	if err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if ack.AcknowledgedAt == nil {
		t.Error("acknowledged_at should be set after acknowledge")
	}
}

func TestFlagRepo_CreateAndResolve(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	flagRepo := repository.NewFlagRepo(pool)
	expRepo := repository.NewExpenseRepo(pool)

	owner := seedUser(t, pool, "Owner")
	teamID := seedTeam(t, pool, owner)
	exp, _, err := expRepo.Create(ctx, newTeamExpense(teamID, owner, 3000), nil)
	if err != nil {
		t.Fatalf("create expense: %v", err)
	}

	created, err := flagRepo.Create(ctx, &flag.Flag{ExpenseID: exp.ID, RaisedBy: owner, Reason: "looks off", Status: flag.StatusOpen})
	if err != nil {
		t.Fatalf("create flag: %v", err)
	}

	resolved, err := flagRepo.Resolve(ctx, created.ID, owner, "checked, fine")
	if err != nil {
		t.Fatalf("resolve flag: %v", err)
	}
	if resolved.Status != flag.StatusResolved {
		t.Errorf("status = %q, want resolved", resolved.Status)
	}

	list, err := flagRepo.ListByExpense(ctx, exp.ID)
	if err != nil {
		t.Fatalf("list flags: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("flag count = %d, want 1", len(list))
	}
}

func TestFlagRepo_Create_NotifiesExpenseParticipants(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	expenseRepo := repository.NewExpenseRepo(pool)
	flagRepo := repository.NewFlagRepo(pool)
	creditor := seedUser(t, pool, "Creditor")
	debtor := seedUser(t, pool, "Debtor")
	teamID := seedTeam(t, pool, creditor)
	expenseID := seedExpenseWithDebt(t, ctx, expenseRepo, teamID, creditor, debtor, 1000)

	created, err := flagRepo.Create(ctx, &flag.Flag{ExpenseID: expenseID, RaisedBy: debtor, Reason: "wrong amount"})
	if err != nil {
		t.Fatalf("create flag: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE type='flag.raised' AND entity_id=$1`, created.ID).Scan(&count); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 2 {
		t.Fatalf("participant notifications = %d, want 2", count)
	}
}

func TestNotificationRepo_ListAndPrefs(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewNotificationRepo(pool)

	userID := seedUser(t, pool, "User")

	entityID := uuid.New()
	if err := repo.Create(ctx, []uuid.UUID{userID}, "expense.created", "expense", &entityID, map[string]any{"title": "Dinner"}); err != nil {
		t.Fatalf("create notification: %v", err)
	}

	items, err := repo.List(ctx, notification.ListParams{UserID: userID, Limit: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("notification count = %d, want 1", len(items))
	}

	// Upsert prefs and read them back.
	updated, err := repo.UpsertPrefs(ctx, &notification.NotificationPrefs{UserID: userID, EmailEnabled: false, DigestMode: true, DisabledTypes: []string{"expense.created"}})
	if err != nil {
		t.Fatalf("upsert prefs: %v", err)
	}
	if updated.EmailEnabled || !updated.DigestMode {
		t.Errorf("prefs = email %v digest %v, want false/true", updated.EmailEnabled, updated.DigestMode)
	}

	got, err := repo.GetPrefs(ctx, userID)
	if err != nil {
		t.Fatalf("get prefs: %v", err)
	}
	if len(got.DisabledTypes) != 1 || got.DisabledTypes[0] != "expense.created" {
		t.Errorf("disabled types = %v, want [expense.created]", got.DisabledTypes)
	}
}

func TestNotificationRepo_List_CompositeCursorDoesNotDropTimestampTies(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()
	repo := repository.NewNotificationRepo(pool)
	userID := seedUser(t, pool, "User")
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		if _, err := pool.Exec(ctx, `INSERT INTO notifications (id, user_id, type, entity_type, entity_id, created_at) VALUES ($1,$2,'expense.created','expense',$3,$4)`, id, userID, uuid.New(), createdAt); err != nil {
			t.Fatalf("seed notification: %v", err)
		}
	}
	first, err := repo.List(ctx, notification.ListParams{UserID: userID, Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page length = %d, want 2", len(first))
	}
	cursor := first[1].CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + first[1].ID.String()
	second, err := repo.List(ctx, notification.ListParams{UserID: userID, Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 1 || second[0].ID == first[0].ID || second[0].ID == first[1].ID {
		t.Fatalf("second page = %+v, want the remaining tied row", second)
	}
}
