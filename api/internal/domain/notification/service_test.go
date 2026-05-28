package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/notification"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeRepo struct {
	notifications map[uuid.UUID]*notification.Notification
	prefs         map[uuid.UUID]*notification.NotificationPrefs
	markReadErr   error
	deleteErr     error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		notifications: make(map[uuid.UUID]*notification.Notification),
		prefs:         make(map[uuid.UUID]*notification.NotificationPrefs),
	}
}

func (r *fakeRepo) List(_ context.Context, p notification.ListParams) ([]*notification.Notification, error) {
	var out []*notification.Notification
	for _, n := range r.notifications {
		if n.UserID != p.UserID {
			continue
		}
		if p.UnreadOnly && n.IsRead {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*notification.Notification, error) {
	n, ok := r.notifications[id]
	if !ok {
		return nil, notification.ErrNotFound
	}
	return n, nil
}

func (r *fakeRepo) MarkRead(_ context.Context, id, userID uuid.UUID) (*notification.Notification, error) {
	if r.markReadErr != nil {
		return nil, r.markReadErr
	}
	n, ok := r.notifications[id]
	if !ok {
		return nil, notification.ErrNotFound
	}
	if n.UserID != userID {
		return nil, notification.ErrForbidden
	}
	now := time.Now()
	n.IsRead = true
	n.ReadAt = &now
	return n, nil
}

func (r *fakeRepo) MarkAllRead(_ context.Context, userID uuid.UUID) error {
	now := time.Now()
	for _, n := range r.notifications {
		if n.UserID == userID && !n.IsRead {
			n.IsRead = true
			n.ReadAt = &now
		}
	}
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id, userID uuid.UUID) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	n, ok := r.notifications[id]
	if !ok {
		return notification.ErrNotFound
	}
	if n.UserID != userID {
		return notification.ErrForbidden
	}
	delete(r.notifications, id)
	return nil
}

func (r *fakeRepo) GetPrefs(_ context.Context, userID uuid.UUID) (*notification.NotificationPrefs, error) {
	p, ok := r.prefs[userID]
	if !ok {
		return nil, notification.ErrNotFound
	}
	return p, nil
}

func (r *fakeRepo) UpsertPrefs(_ context.Context, p *notification.NotificationPrefs) (*notification.NotificationPrefs, error) {
	r.prefs[p.UserID] = p
	return p, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newSvc(repo notification.Repository) *notification.Service {
	return notification.NewService(repo)
}

func seedNotification(r *fakeRepo, userID uuid.UUID, isRead bool) *notification.Notification {
	n := &notification.Notification{
		ID:         uuid.New(),
		UserID:     userID,
		Type:       "expense.created",
		EntityType: "expense",
		EntityID:   uuid.New(),
		Payload:    map[string]any{},
		IsRead:     isRead,
		CreatedAt:  time.Now(),
	}
	r.notifications[n.ID] = n
	return n
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestService_List_AllNotifications_ReturnsBothReadAndUnread(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()

	seedNotification(repo, userID, false)
	seedNotification(repo, userID, true)
	seedNotification(repo, uuid.New(), false) // different user — must not appear

	items, _, err := svc.List(context.Background(), notification.ListParams{
		UserID: userID,
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("want 2 notifications, got %d", len(items))
	}
}

func TestService_List_UnreadOnly_FiltersRead(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()

	seedNotification(repo, userID, false) // unread
	seedNotification(repo, userID, true)  // read — filtered out

	items, _, err := svc.List(context.Background(), notification.ListParams{
		UserID:     userID,
		UnreadOnly: true,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("want 1 unread notification, got %d", len(items))
	}
}

func TestService_List_Empty_ReturnsEmptySlice(t *testing.T) {
	svc := newSvc(newFakeRepo())

	items, _, err := svc.List(context.Background(), notification.ListParams{
		UserID: uuid.New(),
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items == nil {
		t.Error("want empty slice, got nil")
	}
}

// ── MarkRead ──────────────────────────────────────────────────────────────────

func TestService_MarkRead_OwnNotification_SetsIsRead(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()
	n := seedNotification(repo, userID, false)

	updated, err := svc.MarkRead(context.Background(), n.ID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.IsRead {
		t.Error("want is_read=true, got false")
	}
	if updated.ReadAt == nil {
		t.Error("want read_at to be set, got nil")
	}
}

func TestService_MarkRead_NotFound_ReturnsError(t *testing.T) {
	svc := newSvc(newFakeRepo())

	_, err := svc.MarkRead(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, notification.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_MarkRead_OtherUsersNotification_ReturnsForbidden(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	n := seedNotification(repo, uuid.New(), false)

	_, err := svc.MarkRead(context.Background(), n.ID, uuid.New())
	if !errors.Is(err, notification.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

// ── MarkAllRead ───────────────────────────────────────────────────────────────

func TestService_MarkAllRead_MarksOnlyOwnUnread(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()
	other := uuid.New()

	n1 := seedNotification(repo, userID, false)
	n2 := seedNotification(repo, userID, false)
	n3 := seedNotification(repo, other, false) // other user — untouched

	if err := svc.MarkAllRead(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.notifications[n1.ID].IsRead {
		t.Error("n1 should be read")
	}
	if !repo.notifications[n2.ID].IsRead {
		t.Error("n2 should be read")
	}
	if repo.notifications[n3.ID].IsRead {
		t.Error("n3 (other user) should not be read")
	}
}

// ── Dismiss ───────────────────────────────────────────────────────────────────

func TestService_Dismiss_OwnNotification_Deletes(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()
	n := seedNotification(repo, userID, false)

	if err := svc.Dismiss(context.Background(), n.ID, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := repo.notifications[n.ID]; exists {
		t.Error("notification should have been deleted")
	}
}

func TestService_Dismiss_NotFound_ReturnsError(t *testing.T) {
	svc := newSvc(newFakeRepo())

	err := svc.Dismiss(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, notification.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_Dismiss_OtherUsersNotification_ReturnsForbidden(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	n := seedNotification(repo, uuid.New(), false)

	err := svc.Dismiss(context.Background(), n.ID, uuid.New())
	if !errors.Is(err, notification.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

// ── Prefs ─────────────────────────────────────────────────────────────────────

func TestService_GetPrefs_NoRowYet_ReturnsDefaults(t *testing.T) {
	svc := newSvc(newFakeRepo())
	userID := uuid.New()

	prefs, err := svc.GetPrefs(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prefs.EmailEnabled {
		t.Error("default email_enabled should be true")
	}
	if prefs.DigestMode {
		t.Error("default digest_mode should be false")
	}
	if len(prefs.DisabledTypes) != 0 {
		t.Errorf("default disabled_types should be empty, got %v", prefs.DisabledTypes)
	}
}

func TestService_GetPrefs_ExistingRow_ReturnsStored(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()
	stored := &notification.NotificationPrefs{
		ID:            uuid.New(),
		UserID:        userID,
		EmailEnabled:  false,
		DigestMode:    true,
		DisabledTypes: []string{"expense.created"},
		UpdatedAt:     time.Now(),
	}
	repo.prefs[userID] = stored

	prefs, err := svc.GetPrefs(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prefs.EmailEnabled {
		t.Error("want email_enabled=false")
	}
	if !prefs.DigestMode {
		t.Error("want digest_mode=true")
	}
}

func TestService_UpdatePrefs_PartialUpdate_OnlyChangesProvidedFields(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	userID := uuid.New()

	// Seed existing prefs
	repo.prefs[userID] = &notification.NotificationPrefs{
		ID:           uuid.New(),
		UserID:       userID,
		EmailEnabled: true,
		DigestMode:   false,
		UpdatedAt:    time.Now(),
	}

	digestTrue := true
	updated, err := svc.UpdatePrefs(context.Background(), notification.UpdatePrefsInput{
		UserID:     userID,
		DigestMode: &digestTrue,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.DigestMode {
		t.Error("want digest_mode=true after update")
	}
	// email_enabled should be unchanged
	if !updated.EmailEnabled {
		t.Error("want email_enabled still true (unchanged)")
	}
}
