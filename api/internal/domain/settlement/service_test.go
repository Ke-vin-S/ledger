package settlement_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/settlement"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeRepo struct {
	stored   *settlement.Settlement
	balance  *settlement.DebtBalance
	balErr   error
	findErr  error
	createErr error
}

func (r *fakeRepo) Create(_ context.Context, s *settlement.Settlement) (*settlement.Settlement, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	r.stored = s
	return s, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*settlement.Settlement, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.stored != nil && r.stored.ID == id {
		return r.stored, nil
	}
	return nil, settlement.ErrNotFound
}

func (r *fakeRepo) ListByExpense(_ context.Context, _ uuid.UUID) ([]*settlement.Settlement, error) {
	if r.stored == nil {
		return nil, nil
	}
	return []*settlement.Settlement{r.stored}, nil
}

func (r *fakeRepo) Confirm(_ context.Context, id, confirmedBy uuid.UUID) (*settlement.Settlement, error) {
	if r.stored == nil || r.stored.ID != id {
		return nil, settlement.ErrNotFound
	}
	if r.stored.Status != settlement.StatusPending {
		return nil, settlement.ErrInvalidStatus
	}
	r.stored.Status = settlement.StatusConfirmed
	r.stored.ConfirmedBy = &confirmedBy
	return r.stored, nil
}

func (r *fakeRepo) Dispute(_ context.Context, id, disputedBy uuid.UUID, reason string) (*settlement.Settlement, error) {
	if r.stored == nil || r.stored.ID != id {
		return nil, settlement.ErrNotFound
	}
	if r.stored.Status != settlement.StatusPending {
		return nil, settlement.ErrInvalidStatus
	}
	r.stored.Status = settlement.StatusDisputed
	r.stored.DisputedBy = &disputedBy
	r.stored.DisputeReason = &reason
	return r.stored, nil
}

func (r *fakeRepo) GetDebtBalance(_ context.Context, _, _ uuid.UUID) (*settlement.DebtBalance, error) {
	if r.balErr != nil {
		return nil, r.balErr
	}
	return r.balance, nil
}

func (r *fakeRepo) ListTeamNetBalances(_ context.Context, _, _ uuid.UUID) ([]*settlement.TeamBalance, error) {
	return nil, nil
}

func (r *fakeRepo) ListUserNetBalances(_ context.Context, _ uuid.UUID) ([]*settlement.UserBalance, error) {
	return nil, nil
}

func newSvc(repo settlement.Repository) *settlement.Service {
	return settlement.NewService(repo, audit.NopLogger())
}

func makeInput(payerID, payeeID uuid.UUID, amount int64) settlement.RecordInput {
	return settlement.RecordInput{
		ExpenseID: uuid.New(),
		PayerID:   payerID,
		PayeeID:   payeeID,
		Amount:    amount,
		Method:    settlement.MethodCash,
		SettledOn: time.Now(),
	}
}

// ── RecordSettlement ──────────────────────────────────────────────────────────

func TestRecordSettlement_PayerCanRecord(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{
		balance: &settlement.DebtBalance{Balance: 5000},
	}
	svc := newSvc(repo)

	result, err := svc.RecordSettlement(context.Background(), payer, makeInput(payer, payee, 3000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != settlement.StatusPending {
		t.Errorf("status = %q, want %q", result.Status, settlement.StatusPending)
	}
	if result.Amount != 3000 {
		t.Errorf("amount = %d, want 3000", result.Amount)
	}
}

func TestRecordSettlement_PayeeCanRecord(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{
		balance: &settlement.DebtBalance{Balance: 1000},
	}
	svc := newSvc(repo)

	// Payee acts as the actor (recording on behalf)
	_, err := svc.RecordSettlement(context.Background(), payee, makeInput(payer, payee, 500))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordSettlement_ThirdParty_Forbidden(t *testing.T) {
	payer, payee, other := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepo{
		balance: &settlement.DebtBalance{Balance: 5000},
	}
	svc := newSvc(repo)

	_, err := svc.RecordSettlement(context.Background(), other, makeInput(payer, payee, 1000))
	if !errors.Is(err, settlement.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestRecordSettlement_ExceedsBalance_Error(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{
		balance: &settlement.DebtBalance{Balance: 2000},
	}
	svc := newSvc(repo)

	_, err := svc.RecordSettlement(context.Background(), payer, makeInput(payer, payee, 2001))
	if !errors.Is(err, settlement.ErrSettlementExceedsDebt) {
		t.Fatalf("want ErrSettlementExceedsDebt, got %v", err)
	}
}

func TestRecordSettlement_ExactBalance_Succeeds(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{
		balance: &settlement.DebtBalance{Balance: 1500},
	}
	svc := newSvc(repo)

	_, err := svc.RecordSettlement(context.Background(), payer, makeInput(payer, payee, 1500))
	if err != nil {
		t.Fatalf("exact-balance settlement must succeed: %v", err)
	}
}

func TestRecordSettlement_NoDebt_Error(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{
		balErr: settlement.ErrNoDebt,
	}
	svc := newSvc(repo)

	_, err := svc.RecordSettlement(context.Background(), payer, makeInput(payer, payee, 100))
	if !errors.Is(err, settlement.ErrNoDebt) {
		t.Fatalf("want ErrNoDebt, got %v", err)
	}
}

func TestRecordSettlement_InvalidMethod_Error(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{
		balance: &settlement.DebtBalance{Balance: 5000},
	}
	svc := newSvc(repo)

	in := makeInput(payer, payee, 500)
	in.Method = "bitcoin"
	_, err := svc.RecordSettlement(context.Background(), payer, in)
	if !errors.Is(err, settlement.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

// ── ConfirmSettlement ─────────────────────────────────────────────────────────

func TestConfirmSettlement_ByPayee_Succeeds(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	pending := pendingSettlement(payer, payee)
	repo := &fakeRepo{stored: pending}
	svc := newSvc(repo)

	result, err := svc.ConfirmSettlement(context.Background(), payee, pending.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != settlement.StatusConfirmed {
		t.Errorf("status = %q, want %q", result.Status, settlement.StatusConfirmed)
	}
	if result.ConfirmedBy == nil || *result.ConfirmedBy != payee {
		t.Error("confirmed_by must be set to payee")
	}
}

func TestConfirmSettlement_NotPayee_Forbidden(t *testing.T) {
	payer, payee, other := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepo{stored: pendingSettlement(payer, payee)}
	svc := newSvc(repo)

	_, err := svc.ConfirmSettlement(context.Background(), other, repo.stored.ID)
	if !errors.Is(err, settlement.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestConfirmSettlement_NotPending_Error(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	s := pendingSettlement(payer, payee)
	s.Status = settlement.StatusConfirmed // already confirmed
	repo := &fakeRepo{stored: s}
	svc := newSvc(repo)

	_, err := svc.ConfirmSettlement(context.Background(), payee, s.ID)
	if !errors.Is(err, settlement.ErrInvalidStatus) {
		t.Fatalf("want ErrInvalidStatus, got %v", err)
	}
}

// ── DisputeSettlement ─────────────────────────────────────────────────────────

func TestDisputeSettlement_ByPayee_Succeeds(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	pending := pendingSettlement(payer, payee)
	repo := &fakeRepo{stored: pending}
	svc := newSvc(repo)

	result, err := svc.DisputeSettlement(context.Background(), payee, pending.ID, "never received")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != settlement.StatusDisputed {
		t.Errorf("status = %q, want %q", result.Status, settlement.StatusDisputed)
	}
	if result.DisputeReason == nil || *result.DisputeReason != "never received" {
		t.Error("dispute_reason must be set")
	}
}

func TestDisputeSettlement_NotPayee_Forbidden(t *testing.T) {
	payer, payee, other := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepo{stored: pendingSettlement(payer, payee)}
	svc := newSvc(repo)

	_, err := svc.DisputeSettlement(context.Background(), other, repo.stored.ID, "reason")
	if !errors.Is(err, settlement.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestDisputeSettlement_EmptyReason_Error(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	repo := &fakeRepo{stored: pendingSettlement(payer, payee)}
	svc := newSvc(repo)

	_, err := svc.DisputeSettlement(context.Background(), payee, repo.stored.ID, "")
	if !errors.Is(err, settlement.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput for empty reason, got %v", err)
	}
}

func TestDisputeSettlement_AlreadyDisputed_Error(t *testing.T) {
	payer, payee := uuid.New(), uuid.New()
	s := pendingSettlement(payer, payee)
	s.Status = settlement.StatusDisputed
	repo := &fakeRepo{stored: s}
	svc := newSvc(repo)

	_, err := svc.DisputeSettlement(context.Background(), payee, s.ID, "still wrong")
	if !errors.Is(err, settlement.ErrInvalidStatus) {
		t.Fatalf("want ErrInvalidStatus, got %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func pendingSettlement(payer, payee uuid.UUID) *settlement.Settlement {
	return &settlement.Settlement{
		ID:        uuid.New(),
		ExpenseID: uuid.New(),
		PayerID:   payer,
		PayeeID:   payee,
		Amount:    1000,
		Method:    settlement.MethodCash,
		Status:    settlement.StatusPending,
		SettledOn: time.Now(),
		CreatedAt: time.Now(),
	}
}
