package loan_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/loan"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeLoanRepo struct {
	loans     map[uuid.UUID]*loan.Loan
	createErr error
	findErr   error
	ackErr    error
	dispErr   error
}

func newFakeRepo() *fakeLoanRepo {
	return &fakeLoanRepo{loans: make(map[uuid.UUID]*loan.Loan)}
}

func (r *fakeLoanRepo) Create(_ context.Context, in loan.CreateInput) (*loan.Loan, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	l := &loan.Loan{
		ID:               uuid.New(),
		UserID:           in.UserID,
		Direction:        in.Direction,
		Amount:           in.Amount,
		Currency:         in.Currency,
		CounterpartyID:   in.CounterpartyID,
		CounterpartyName: in.CounterpartyName,
		Note:             in.Note,
		Status:           loan.StatusOutstanding,
		LoanDate:         in.LoanDate,
		CreatedAt:        time.Now(),
	}
	r.loans[l.ID] = l
	return l, nil
}

func (r *fakeLoanRepo) FindByID(_ context.Context, id uuid.UUID) (*loan.Loan, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if l, ok := r.loans[id]; ok {
		return l, nil
	}
	return nil, loan.ErrNotFound
}

func (r *fakeLoanRepo) ListByUser(_ context.Context, userID uuid.UUID, direction *string) ([]*loan.Loan, error) {
	var out []*loan.Loan
	for _, l := range r.loans {
		if l.UserID != userID {
			continue
		}
		if direction != nil && l.Direction != *direction {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func (r *fakeLoanRepo) Acknowledge(_ context.Context, id uuid.UUID) (*loan.Loan, error) {
	if r.ackErr != nil {
		return nil, r.ackErr
	}
	l := r.loans[id]
	now := time.Now()
	l.Status = loan.StatusSettled
	l.AcknowledgedAt = &now
	return l, nil
}

func (r *fakeLoanRepo) Dispute(_ context.Context, id uuid.UUID, _ *string) (*loan.Loan, error) {
	if r.dispErr != nil {
		return nil, r.dispErr
	}
	l := r.loans[id]
	l.Status = loan.StatusDisputed
	return l, nil
}

func newSvc(repo loan.Repository) *loan.Service {
	return loan.NewService(repo, audit.NopLogger())
}

func ptr(s string) *string { return &s }

// seed inserts a loan owned by owner and returns it.
func seed(t *testing.T, repo *fakeLoanRepo, owner uuid.UUID, direction, status string) *loan.Loan {
	t.Helper()
	l := &loan.Loan{
		ID:               uuid.New(),
		UserID:           owner,
		Direction:        direction,
		Amount:           5000,
		Currency:         "LKR",
		CounterpartyName: "Alice",
		Status:           status,
		LoanDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		CreatedAt:        time.Now(),
	}
	repo.loans[l.ID] = l
	return l
}

// ── CreateLoan ─────────────────────────────────────────────────────────────────

func TestCreateLoan_Valid_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)

	got, err := svc.CreateLoan(context.Background(), loan.CreateInput{
		UserID:           uuid.New(),
		Direction:        loan.DirectionLent,
		Amount:           5000,
		Currency:         "LKR",
		CounterpartyName: "Alice",
		LoanDate:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != loan.StatusOutstanding {
		t.Errorf("status = %q, want %q", got.Status, loan.StatusOutstanding)
	}
}

func TestCreateLoan_InvalidDirection_Error(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.CreateLoan(context.Background(), loan.CreateInput{
		UserID:           uuid.New(),
		Direction:        "sideways",
		Amount:           5000,
		CounterpartyName: "Alice",
	})
	if !errors.Is(err, loan.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestCreateLoan_NonPositiveAmount_Error(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.CreateLoan(context.Background(), loan.CreateInput{
		UserID:           uuid.New(),
		Direction:        loan.DirectionLent,
		Amount:           0,
		CounterpartyName: "Alice",
	})
	if !errors.Is(err, loan.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestCreateLoan_MissingCounterparty_Error(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.CreateLoan(context.Background(), loan.CreateInput{
		UserID:    uuid.New(),
		Direction: loan.DirectionBorrowed,
		Amount:    100,
	})
	if !errors.Is(err, loan.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

// ── GetLoan ────────────────────────────────────────────────────────────────────

func TestGetLoan_Owner_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionLent, loan.StatusOutstanding)

	got, err := newSvc(repo).GetLoan(context.Background(), owner, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != l.ID {
		t.Errorf("got loan %v, want %v", got.ID, l.ID)
	}
}

func TestGetLoan_NonOwner_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	l := seed(t, repo, uuid.New(), loan.DirectionLent, loan.StatusOutstanding)

	_, err := newSvc(repo).GetLoan(context.Background(), uuid.New(), l.ID)
	if !errors.Is(err, loan.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestGetLoan_Missing_NotFound(t *testing.T) {
	_, err := newSvc(newFakeRepo()).GetLoan(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, loan.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// ── ListLoans ──────────────────────────────────────────────────────────────────

func TestListLoans_DirectionFilter(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	seed(t, repo, owner, loan.DirectionLent, loan.StatusOutstanding)
	seed(t, repo, owner, loan.DirectionBorrowed, loan.StatusOutstanding)

	lent := loan.DirectionLent
	got, err := newSvc(repo).ListLoans(context.Background(), owner, &lent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Direction != loan.DirectionLent {
		t.Errorf("want 1 lent loan, got %d", len(got))
	}
}

func TestListLoans_NoFilter_ReturnsAll(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	seed(t, repo, owner, loan.DirectionLent, loan.StatusOutstanding)
	seed(t, repo, owner, loan.DirectionBorrowed, loan.StatusOutstanding)

	got, err := newSvc(repo).ListLoans(context.Background(), owner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 loans, got %d", len(got))
	}
}

// ── AcknowledgeLoan ────────────────────────────────────────────────────────────

func TestAcknowledgeLoan_Owner_Outstanding_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionLent, loan.StatusOutstanding)

	got, err := newSvc(repo).AcknowledgeLoan(context.Background(), owner, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != loan.StatusSettled {
		t.Errorf("status = %q, want %q", got.Status, loan.StatusSettled)
	}
}

func TestAcknowledgeLoan_NonOwner_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	l := seed(t, repo, uuid.New(), loan.DirectionLent, loan.StatusOutstanding)

	_, err := newSvc(repo).AcknowledgeLoan(context.Background(), uuid.New(), l.ID)
	if !errors.Is(err, loan.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestAcknowledgeLoan_NotOutstanding_InvalidStatus(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionLent, loan.StatusSettled)

	_, err := newSvc(repo).AcknowledgeLoan(context.Background(), owner, l.ID)
	if !errors.Is(err, loan.ErrInvalidStatus) {
		t.Fatalf("want ErrInvalidStatus, got %v", err)
	}
}

// ── DisputeLoan ────────────────────────────────────────────────────────────────

func TestDisputeLoan_Owner_Outstanding_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionBorrowed, loan.StatusOutstanding)

	got, err := newSvc(repo).DisputeLoan(context.Background(), owner, l.ID, ptr("not mine"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != loan.StatusDisputed {
		t.Errorf("status = %q, want %q", got.Status, loan.StatusDisputed)
	}
}

func TestDisputeLoan_AlreadySettled_InvalidStatus(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionLent, loan.StatusSettled)

	_, err := newSvc(repo).DisputeLoan(context.Background(), owner, l.ID, ptr("x"))
	if !errors.Is(err, loan.ErrInvalidStatus) {
		t.Fatalf("want ErrInvalidStatus, got %v", err)
	}
}

func TestDisputeLoan_NonOwner_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	l := seed(t, repo, uuid.New(), loan.DirectionLent, loan.StatusOutstanding)

	_, err := newSvc(repo).DisputeLoan(context.Background(), uuid.New(), l.ID, nil)
	if !errors.Is(err, loan.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

// ── ClaimText ──────────────────────────────────────────────────────────────────

func TestClaimText_Lent_FormatsOwedToUser(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionLent, loan.StatusOutstanding)

	got, err := newSvc(repo).ClaimText(context.Background(), owner, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Hi Alice, just a reminder that you owe me LKR 5000 from 2026-01-15."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClaimText_Borrowed_FormatsUserOwes(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionBorrowed, loan.StatusOutstanding)

	got, err := newSvc(repo).ClaimText(context.Background(), owner, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Hi Alice, I owe you LKR 5000 from 2026-01-15."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClaimText_WithNote_AppendsNote(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	l := seed(t, repo, owner, loan.DirectionLent, loan.StatusOutstanding)
	l.Note = ptr("lunch money")

	got, err := newSvc(repo).ClaimText(context.Background(), owner, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Hi Alice, just a reminder that you owe me LKR 5000 from 2026-01-15. (lunch money)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClaimText_NonOwner_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	l := seed(t, repo, uuid.New(), loan.DirectionLent, loan.StatusOutstanding)

	_, err := newSvc(repo).ClaimText(context.Background(), uuid.New(), l.ID)
	if !errors.Is(err, loan.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}
