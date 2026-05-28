package flag_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/flag"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeFlagRepo struct {
	flags      map[uuid.UUID]*flag.Flag
	resolveErr error
	createErr  error
}

func newFakeRepo() *fakeFlagRepo {
	return &fakeFlagRepo{flags: make(map[uuid.UUID]*flag.Flag)}
}

func (r *fakeFlagRepo) Create(_ context.Context, f *flag.Flag) (*flag.Flag, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	r.flags[f.ID] = f
	return f, nil
}

func (r *fakeFlagRepo) FindByID(_ context.Context, id uuid.UUID) (*flag.Flag, error) {
	f, ok := r.flags[id]
	if !ok {
		return nil, flag.ErrNotFound
	}
	return f, nil
}

func (r *fakeFlagRepo) ListByExpense(_ context.Context, expenseID uuid.UUID) ([]*flag.Flag, error) {
	var out []*flag.Flag
	for _, f := range r.flags {
		if f.ExpenseID == expenseID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *fakeFlagRepo) Resolve(_ context.Context, id, resolvedBy uuid.UUID, note string) (*flag.Flag, error) {
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}
	f, ok := r.flags[id]
	if !ok {
		return nil, flag.ErrNotFound
	}
	now := time.Now()
	f.Status = flag.StatusResolved
	f.ResolvedBy = &resolvedBy
	f.ResolutionNote = &note
	f.ResolvedAt = &now
	return f, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newSvc(repo flag.Repository) *flag.Service {
	return flag.NewService(repo, audit.NopLogger())
}

func seedOpenFlag(t *testing.T, repo *fakeFlagRepo, expenseID uuid.UUID) *flag.Flag {
	t.Helper()
	f := &flag.Flag{
		ExpenseID: expenseID,
		RaisedBy:  uuid.New(),
		Reason:    "amounts look wrong",
		Status:    flag.StatusOpen,
	}
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	repo.flags[f.ID] = f
	return f
}

// ── RaiseFlag ────────────────────────────────────────────────────────────────

func TestService_RaiseFlag_Valid_ReturnsFlagWithOpenStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	expenseID := uuid.New()
	callerID := uuid.New()

	f, err := svc.RaiseFlag(context.Background(), flag.RaiseInput{
		ExpenseID: expenseID,
		RaisedBy:  callerID,
		Reason:    "wrong amount",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Status != flag.StatusOpen {
		t.Errorf("want status %q, got %q", flag.StatusOpen, f.Status)
	}
	if f.ExpenseID != expenseID {
		t.Errorf("want expense_id %v, got %v", expenseID, f.ExpenseID)
	}
	if f.RaisedBy != callerID {
		t.Errorf("want raised_by %v, got %v", callerID, f.RaisedBy)
	}
}

func TestService_RaiseFlag_EmptyReason_ReturnsInvalidInput(t *testing.T) {
	svc := newSvc(newFakeRepo())

	_, err := svc.RaiseFlag(context.Background(), flag.RaiseInput{
		ExpenseID: uuid.New(),
		RaisedBy:  uuid.New(),
		Reason:    "",
	})

	if !errors.Is(err, flag.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestService_RaiseFlag_WhitespaceReason_ReturnsInvalidInput(t *testing.T) {
	svc := newSvc(newFakeRepo())

	_, err := svc.RaiseFlag(context.Background(), flag.RaiseInput{
		ExpenseID: uuid.New(),
		RaisedBy:  uuid.New(),
		Reason:    "   ",
	})

	if !errors.Is(err, flag.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

// ── ResolveFlag ───────────────────────────────────────────────────────────────

func TestService_ResolveFlag_OpenFlag_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	expenseID := uuid.New()
	resolverID := uuid.New()

	f := seedOpenFlag(t, repo, expenseID)

	resolved, err := svc.ResolveFlag(context.Background(), flag.ResolveInput{
		FlagID:         f.ID,
		ResolvedBy:     resolverID,
		ResolutionNote: "confirmed correct",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != flag.StatusResolved {
		t.Errorf("want status %q, got %q", flag.StatusResolved, resolved.Status)
	}
	if resolved.ResolvedBy == nil || *resolved.ResolvedBy != resolverID {
		t.Errorf("want resolved_by %v, got %v", resolverID, resolved.ResolvedBy)
	}
}

func TestService_ResolveFlag_AlreadyResolved_ReturnsError(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)

	f := &flag.Flag{
		ID:        uuid.New(),
		ExpenseID: uuid.New(),
		RaisedBy:  uuid.New(),
		Reason:    "bad data",
		Status:    flag.StatusResolved,
		CreatedAt: time.Now(),
	}
	repo.flags[f.ID] = f

	_, err := svc.ResolveFlag(context.Background(), flag.ResolveInput{
		FlagID:         f.ID,
		ResolvedBy:     uuid.New(),
		ResolutionNote: "note",
	})

	if !errors.Is(err, flag.ErrAlreadyResolved) {
		t.Fatalf("want ErrAlreadyResolved, got %v", err)
	}
}

func TestService_ResolveFlag_NotFound_ReturnsNotFound(t *testing.T) {
	svc := newSvc(newFakeRepo())

	_, err := svc.ResolveFlag(context.Background(), flag.ResolveInput{
		FlagID:         uuid.New(),
		ResolvedBy:     uuid.New(),
		ResolutionNote: "note",
	})

	if !errors.Is(err, flag.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_ResolveFlag_EmptyNote_ReturnsInvalidInput(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	f := seedOpenFlag(t, repo, uuid.New())

	_, err := svc.ResolveFlag(context.Background(), flag.ResolveInput{
		FlagID:         f.ID,
		ResolvedBy:     uuid.New(),
		ResolutionNote: "",
	})

	if !errors.Is(err, flag.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

// ── ListFlags ─────────────────────────────────────────────────────────────────

func TestService_ListFlags_ReturnsAllForExpense(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	expenseID := uuid.New()
	otherID := uuid.New()

	seedOpenFlag(t, repo, expenseID)
	seedOpenFlag(t, repo, expenseID)
	seedOpenFlag(t, repo, otherID) // should not appear

	flags, err := svc.ListFlags(context.Background(), expenseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flags) != 2 {
		t.Errorf("want 2 flags, got %d", len(flags))
	}
}

func TestService_ListFlags_NoFlags_ReturnsEmptySlice(t *testing.T) {
	svc := newSvc(newFakeRepo())

	flags, err := svc.ListFlags(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flags == nil {
		t.Error("want empty slice, got nil")
	}
}
