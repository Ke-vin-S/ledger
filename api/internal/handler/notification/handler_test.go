package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/notification"
)

func authAs(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &jwtauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}
			next.ServeHTTP(w, r.WithContext(jwtauth.SetClaims(r.Context(), claims)))
		})
	}
}

type fakeRepo struct {
	items     []*notification.Notification
	prefs     *notification.NotificationPrefs
	prefsErr  error
	markErr   error
	deleteErr error
}

func (r *fakeRepo) List(_ context.Context, p notification.ListParams) ([]*notification.Notification, error) {
	var out []*notification.Notification
	for _, n := range r.items {
		if p.UnreadOnly && n.IsRead {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*notification.Notification, error) {
	for _, n := range r.items {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, notification.ErrNotFound
}

func (r *fakeRepo) MarkRead(_ context.Context, id, _ uuid.UUID) (*notification.Notification, error) {
	if r.markErr != nil {
		return nil, r.markErr
	}
	for _, n := range r.items {
		if n.ID == id {
			n.IsRead = true
			return n, nil
		}
	}
	return nil, notification.ErrNotFound
}

func (r *fakeRepo) MarkAllRead(_ context.Context, _ uuid.UUID) error {
	for _, n := range r.items {
		n.IsRead = true
	}
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id, _ uuid.UUID) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	return nil
}

func (r *fakeRepo) GetPrefs(_ context.Context, _ uuid.UUID) (*notification.NotificationPrefs, error) {
	if r.prefsErr != nil {
		return nil, r.prefsErr
	}
	return r.prefs, nil
}

func (r *fakeRepo) UpsertPrefs(_ context.Context, prefs *notification.NotificationPrefs) (*notification.NotificationPrefs, error) {
	r.prefs = prefs
	return prefs, nil
}

func router(repo notification.Repository, actor uuid.UUID) http.Handler {
	svc := notification.NewService(repo)
	h := New(svc)
	root := chi.NewRouter()
	root.Mount("/notifications", h.Routes(authAs(actor)))
	return root
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestList_ReturnsPaginatedEnvelope(t *testing.T) {
	actor := uuid.New()
	repo := &fakeRepo{items: []*notification.Notification{
		{ID: uuid.New(), UserID: actor, Type: "expense.created", CreatedAt: time.Now()},
	}}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodGet, "/notifications", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Items   []notification.Notification `json:"items"`
			HasMore bool                        `json:"has_more"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Items) != 1 {
		t.Errorf("got %d items, want 1", len(resp.Data.Items))
	}
}

func TestMarkRead_200(t *testing.T) {
	actor := uuid.New()
	n := &notification.Notification{ID: uuid.New(), UserID: actor, CreatedAt: time.Now()}
	repo := &fakeRepo{items: []*notification.Notification{n}}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPost, "/notifications/"+n.ID.String()+"/read", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !n.IsRead {
		t.Error("notification was not marked read")
	}
}

func TestMarkRead_BadID_400(t *testing.T) {
	h := router(&fakeRepo{}, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/notifications/not-a-uuid/read", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestMarkRead_Missing_404(t *testing.T) {
	h := router(&fakeRepo{}, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/notifications/"+uuid.New().String()+"/read", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestDismiss_204(t *testing.T) {
	actor := uuid.New()
	n := &notification.Notification{ID: uuid.New(), UserID: actor}
	repo := &fakeRepo{items: []*notification.Notification{n}}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodDelete, "/notifications/"+n.ID.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetPrefs_NoRow_SynthesisesDefaults(t *testing.T) {
	actor := uuid.New()
	repo := &fakeRepo{prefsErr: notification.ErrNotFound}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodGet, "/notifications/prefs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data notification.NotificationPrefs `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Data.EmailEnabled {
		t.Error("default prefs should have email enabled")
	}
}

func TestUpdatePrefs_AppliesPartialUpdate(t *testing.T) {
	actor := uuid.New()
	repo := &fakeRepo{prefs: &notification.NotificationPrefs{UserID: actor, EmailEnabled: true, DigestMode: false}}
	h := router(repo, actor)

	digest := true
	rec := doJSON(t, h, http.MethodPatch, "/notifications/prefs", updatePrefsBody{DigestMode: &digest})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !repo.prefs.DigestMode {
		t.Error("digest_mode was not updated")
	}
}
