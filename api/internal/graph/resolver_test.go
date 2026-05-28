package graph_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/graph"
	"github.com/Ke-vin-S/ledger/api/internal/graph/model"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeActivityStore struct {
	entries []*model.ActivityEntry
	err     error
}

func (s *fakeActivityStore) QueryTeamActivityFeed(_ context.Context, _ uuid.UUID, _ int, _ *time.Time) ([]*model.ActivityEntry, error) {
	return s.entries, s.err
}

type fakeDashStore struct {
	agg *model.DashboardAggregates
	err error
}

func (s *fakeDashStore) QueryDashboardAggregates(_ context.Context, _ uuid.UUID) (*model.DashboardAggregates, error) {
	return s.agg, s.err
}

type fakeHistoryStore struct {
	versions []*model.ExpenseVersion
	err      error
}

func (s *fakeHistoryStore) QueryExpenseHistory(_ context.Context, _ uuid.UUID) ([]*model.ExpenseVersion, error) {
	return s.versions, s.err
}

// ── helper ────────────────────────────────────────────────────────────────────

func newResolver(act graph.ActivityFeedStore, dash graph.DashboardStore, hist graph.ExpenseHistoryStore) *graph.Resolver {
	return graph.NewResolver(act, dash, hist)
}

// ── TeamActivityFeed ──────────────────────────────────────────────────────────

func TestResolver_TeamActivityFeed_ReturnsItems(t *testing.T) {
	teamID := uuid.New().String()
	act := &fakeActivityStore{
		entries: []*model.ActivityEntry{
			{ID: uuid.New().String(), Action: "expense.created", EntityType: "expense", EntityID: uuid.New().String(), CreatedAt: time.Now().Format(time.RFC3339)},
			{ID: uuid.New().String(), Action: "member.invited", EntityType: "team_member", EntityID: uuid.New().String(), CreatedAt: time.Now().Format(time.RFC3339)},
		},
	}
	r := newResolver(act, &fakeDashStore{}, &fakeHistoryStore{})
	qr := r.Query()

	limit := 20
	page, err := qr.TeamActivityFeed(context.Background(), teamID, &limit, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("want 2 items, got %d", len(page.Items))
	}
	if page.HasMore {
		t.Error("want hasMore=false for 2 items with limit 20")
	}
}

func TestResolver_TeamActivityFeed_HasMore_WhenItemsExceedLimit(t *testing.T) {
	teamID := uuid.New().String()
	// Store returns limit+1 items, resolver must detect hasMore
	entries := make([]*model.ActivityEntry, 21)
	for i := range entries {
		entries[i] = &model.ActivityEntry{
			ID: uuid.New().String(), Action: "expense.created",
			EntityType: "expense", EntityID: uuid.New().String(),
			CreatedAt: time.Now().Format(time.RFC3339),
		}
	}
	act := &fakeActivityStore{entries: entries}
	r := newResolver(act, &fakeDashStore{}, &fakeHistoryStore{})

	limit := 20
	page, err := r.Query().TeamActivityFeed(context.Background(), teamID, &limit, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !page.HasMore {
		t.Error("want hasMore=true")
	}
	if len(page.Items) != 20 {
		t.Errorf("want 20 items (not 21), got %d", len(page.Items))
	}
	if page.NextCursor == nil {
		t.Error("want nextCursor to be set")
	}
}

func TestResolver_TeamActivityFeed_InvalidTeamID_ReturnsError(t *testing.T) {
	r := newResolver(&fakeActivityStore{}, &fakeDashStore{}, &fakeHistoryStore{})
	_, err := r.Query().TeamActivityFeed(context.Background(), "not-a-uuid", nil, nil)
	if err == nil {
		t.Fatal("want error for invalid teamId, got nil")
	}
}

func TestResolver_TeamActivityFeed_EmptyResult_ReturnsEmptyItems(t *testing.T) {
	r := newResolver(&fakeActivityStore{entries: nil}, &fakeDashStore{}, &fakeHistoryStore{})
	limit := 20
	page, err := r.Query().TeamActivityFeed(context.Background(), uuid.New().String(), &limit, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Items == nil {
		t.Error("want empty slice, got nil")
	}
}

// ── DashboardAggregates ───────────────────────────────────────────────────────

func TestResolver_DashboardAggregates_ReturnsSums(t *testing.T) {
	dash := &fakeDashStore{
		agg: &model.DashboardAggregates{TotalOwed: 5000, TotalOwing: 2000, NetBalance: 3000},
	}
	r := newResolver(&fakeActivityStore{}, dash, &fakeHistoryStore{})

	ctx := contextWithUserID(uuid.New())
	agg, err := r.Query().DashboardAggregates(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agg.TotalOwed != 5000 {
		t.Errorf("want TotalOwed=5000, got %d", agg.TotalOwed)
	}
	if agg.NetBalance != 3000 {
		t.Errorf("want NetBalance=3000, got %d", agg.NetBalance)
	}
}

func TestResolver_DashboardAggregates_NoAuth_ReturnsError(t *testing.T) {
	r := newResolver(&fakeActivityStore{}, &fakeDashStore{}, &fakeHistoryStore{})
	// context without user ID
	_, err := r.Query().DashboardAggregates(context.Background())
	if err == nil {
		t.Fatal("want error when no auth in context, got nil")
	}
}

// ── ExpenseHistory ────────────────────────────────────────────────────────────

func TestResolver_ExpenseHistory_ReturnsVersions(t *testing.T) {
	expenseID := uuid.New().String()
	hist := &fakeHistoryStore{
		versions: []*model.ExpenseVersion{
			{ID: uuid.New().String(), ExpenseID: expenseID, Version: 2, Snapshot: map[string]any{"title": "dinner"}, CorrectedBy: uuid.New().String(), CreatedAt: time.Now().Format(time.RFC3339)},
			{ID: uuid.New().String(), ExpenseID: expenseID, Version: 1, Snapshot: map[string]any{"title": "lunch"}, CorrectedBy: uuid.New().String(), CreatedAt: time.Now().Format(time.RFC3339)},
		},
	}
	r := newResolver(&fakeActivityStore{}, &fakeDashStore{}, hist)

	versions, err := r.Query().ExpenseHistory(context.Background(), expenseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("want 2 versions, got %d", len(versions))
	}
}

func TestResolver_ExpenseHistory_InvalidID_ReturnsError(t *testing.T) {
	r := newResolver(&fakeActivityStore{}, &fakeDashStore{}, &fakeHistoryStore{})
	_, err := r.Query().ExpenseHistory(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("want error for invalid expenseId, got nil")
	}
}

func TestResolver_ExpenseHistory_Empty_ReturnsEmptySlice(t *testing.T) {
	r := newResolver(&fakeActivityStore{}, &fakeDashStore{}, &fakeHistoryStore{versions: nil})
	versions, err := r.Query().ExpenseHistory(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if versions == nil {
		t.Error("want empty slice, got nil")
	}
}

// ── context helper ────────────────────────────────────────────────────────────

func contextWithUserID(id uuid.UUID) context.Context {
	claims := &jwtauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: id.String()},
	}
	return jwtauth.SetClaims(context.Background(), claims)
}
