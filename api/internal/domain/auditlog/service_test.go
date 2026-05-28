package auditlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/auditlog"
)

// ── fake ──────────────────────────────────────────────────────────────────────

type fakeRepo struct {
	teamEntries  []*auditlog.LogEntry
	actorEntries []*auditlog.LogEntry
}

func (r *fakeRepo) ListByTeam(_ context.Context, _ uuid.UUID, p auditlog.ListParams) ([]*auditlog.LogEntry, error) {
	var out []*auditlog.LogEntry
	for _, e := range r.teamEntries {
		if p.Action != "" && e.Action != p.Action {
			continue
		}
		out = append(out, e)
	}
	if p.Limit > 0 && len(out) > p.Limit {
		out = out[:p.Limit]
	}
	return out, nil
}

func (r *fakeRepo) ListByActor(_ context.Context, _ uuid.UUID, p auditlog.ListParams) ([]*auditlog.LogEntry, error) {
	var out []*auditlog.LogEntry
	for _, e := range r.actorEntries {
		if p.Action != "" && e.Action != p.Action {
			continue
		}
		out = append(out, e)
	}
	if p.Limit > 0 && len(out) > p.Limit {
		out = out[:p.Limit]
	}
	return out, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newSvc(repo auditlog.Repository) *auditlog.Service {
	return auditlog.NewService(repo)
}

func makeEntries(n int, action string) []*auditlog.LogEntry {
	entries := make([]*auditlog.LogEntry, n)
	for i := range entries {
		entries[i] = &auditlog.LogEntry{
			ID:         uuid.New(),
			Action:     action,
			EntityType: "expense",
			EntityID:   uuid.New(),
			CreatedAt:  time.Now(),
		}
	}
	return entries
}

// ── ListTeamEntries ───────────────────────────────────────────────────────────

func TestService_ListTeamEntries_ReturnsItems(t *testing.T) {
	repo := &fakeRepo{teamEntries: makeEntries(3, "expense.created")}
	svc := newSvc(repo)

	items, hasMore, err := svc.ListTeamEntries(context.Background(), uuid.New(), auditlog.ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("want 3 items, got %d", len(items))
	}
	if hasMore {
		t.Error("want hasMore=false")
	}
}

func TestService_ListTeamEntries_HasMore_WhenExceedsLimit(t *testing.T) {
	repo := &fakeRepo{teamEntries: makeEntries(21, "expense.created")}
	svc := newSvc(repo)

	items, hasMore, err := svc.ListTeamEntries(context.Background(), uuid.New(), auditlog.ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMore {
		t.Error("want hasMore=true")
	}
	if len(items) != 20 {
		t.Errorf("want 20 items, got %d", len(items))
	}
}

func TestService_ListTeamEntries_ActionFilter_OnlyMatchingReturned(t *testing.T) {
	entries := append(
		makeEntries(2, "expense.created"),
		makeEntries(3, "member.invited")...,
	)
	repo := &fakeRepo{teamEntries: entries}
	svc := newSvc(repo)

	items, _, err := svc.ListTeamEntries(context.Background(), uuid.New(), auditlog.ListParams{
		Limit:  20,
		Action: "expense.created",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("want 2 items matching action filter, got %d", len(items))
	}
}

func TestService_ListTeamEntries_Empty_ReturnsEmptySlice(t *testing.T) {
	svc := newSvc(&fakeRepo{})

	items, hasMore, err := svc.ListTeamEntries(context.Background(), uuid.New(), auditlog.ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items == nil {
		t.Error("want empty slice, got nil")
	}
	if hasMore {
		t.Error("want hasMore=false for empty result")
	}
}

func TestService_ListTeamEntries_DefaultLimit_AppliedWhenZero(t *testing.T) {
	repo := &fakeRepo{teamEntries: makeEntries(5, "expense.created")}
	svc := newSvc(repo)

	// Limit=0 should use the default (20), not break
	items, _, err := svc.ListTeamEntries(context.Background(), uuid.New(), auditlog.ListParams{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("want 5 items with default limit, got %d", len(items))
	}
}

// ── ListMyEntries ─────────────────────────────────────────────────────────────

func TestService_ListMyEntries_ReturnsActorEntries(t *testing.T) {
	repo := &fakeRepo{actorEntries: makeEntries(4, "settlement.created")}
	svc := newSvc(repo)

	items, hasMore, err := svc.ListMyEntries(context.Background(), uuid.New(), auditlog.ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 4 {
		t.Errorf("want 4 items, got %d", len(items))
	}
	if hasMore {
		t.Error("want hasMore=false")
	}
}

func TestService_ListMyEntries_HasMore_WhenExceedsLimit(t *testing.T) {
	repo := &fakeRepo{actorEntries: makeEntries(11, "flag.opened")}
	svc := newSvc(repo)

	items, hasMore, err := svc.ListMyEntries(context.Background(), uuid.New(), auditlog.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMore {
		t.Error("want hasMore=true")
	}
	if len(items) != 10 {
		t.Errorf("want 10 items, got %d", len(items))
	}
}
