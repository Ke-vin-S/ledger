package user

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

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
)

func authAs(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &jwtauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}
			next.ServeHTTP(w, r.WithContext(jwtauth.SetClaims(r.Context(), claims)))
		})
	}
}

type fakeUserRepo struct {
	byID map[uuid.UUID]*user.User
}

func newFakeRepo() *fakeUserRepo { return &fakeUserRepo{byID: make(map[uuid.UUID]*user.User)} }

func (r *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) Update(_ context.Context, u *user.User) (*user.User, error) {
	r.byID[u.ID] = u
	return u, nil
}
func (r *fakeUserRepo) Create(context.Context, *user.User) (*user.User, error) { return nil, nil }
func (r *fakeUserRepo) CreateAnonymous(_ context.Context, name string, _ uuid.UUID) (*user.User, error) {
	u := &user.User{ID: uuid.New(), IdentityType: user.IdentityTypeAnonymous, DisplayName: name}
	r.byID[u.ID] = u
	return u, nil
}
func (r *fakeUserRepo) FindByEmail(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) FindByOAuth(context.Context, string, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (r *fakeUserRepo) UpsertOAuthAccount(context.Context, uuid.UUID, string, string, *string) error {
	return nil
}
func (r *fakeUserRepo) UpdateAvatarURL(context.Context, uuid.UUID, string) error { return nil }
func (r *fakeUserRepo) UpdatePassword(context.Context, uuid.UUID, string) error  { return nil }
func (r *fakeUserRepo) GetNotificationPrefs(_ context.Context, id uuid.UUID) (*user.NotificationPrefs, error) {
	return &user.NotificationPrefs{UserID: id, EmailEnabled: true}, nil
}
func (r *fakeUserRepo) UpdateNotificationPrefs(_ context.Context, p *user.NotificationPrefs) (*user.NotificationPrefs, error) {
	return p, nil
}
func (r *fakeUserRepo) CreateClaimToken(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (*user.ClaimToken, error) {
	return nil, nil
}
func (r *fakeUserRepo) Claim(context.Context, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func router(repo user.Repository, actor uuid.UUID) http.Handler {
	svc := user.NewService(repo, audit.NopLogger())
	h := New(svc, "https://app.example.com")
	root := chi.NewRouter()
	root.Mount("/users", h.Routes(authAs(actor)))
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

func TestGetMe_200(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	repo.byID[actor] = &user.User{ID: actor, IdentityType: user.IdentityTypeRegistered, DisplayName: "Kevin", CurrencyPref: "LKR", Timezone: "Asia/Colombo"}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodGet, "/users/me", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data fullUserResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.DisplayName != "Kevin" {
		t.Errorf("display name = %q, want Kevin", resp.Data.DisplayName)
	}
}

func TestGetMe_Missing_404(t *testing.T) {
	h := router(newFakeRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/users/me", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMe_BadCurrency_400(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	repo.byID[actor] = &user.User{ID: actor, IdentityType: user.IdentityTypeRegistered, DisplayName: "K"}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPatch, "/users/me", map[string]any{"currency_pref": "RUPEES"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMe_DisplayName_200(t *testing.T) {
	actor := uuid.New()
	repo := newFakeRepo()
	repo.byID[actor] = &user.User{ID: actor, IdentityType: user.IdentityTypeRegistered, DisplayName: "Old"}
	h := router(repo, actor)

	rec := doJSON(t, h, http.MethodPatch, "/users/me", map[string]any{"display_name": "New"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.byID[actor].DisplayName != "New" {
		t.Errorf("display name not updated: %q", repo.byID[actor].DisplayName)
	}
}

func TestGetUser_Missing_404(t *testing.T) {
	h := router(newFakeRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/users/"+uuid.New().String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetUser_BadID_400(t *testing.T) {
	h := router(newFakeRepo(), uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/users/not-a-uuid", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateAnonymous_201(t *testing.T) {
	actor := uuid.New()
	h := router(newFakeRepo(), actor)
	rec := doJSON(t, h, http.MethodPost, "/users/anonymous", map[string]any{"display_name": "Guest"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
}

func TestClaim_EmptyToken_400(t *testing.T) {
	actor := uuid.New()
	h := router(newFakeRepo(), actor)
	rec := doJSON(t, h, http.MethodPost, "/users/claim", map[string]any{"claim_token": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
