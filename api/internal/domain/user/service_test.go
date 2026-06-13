package user_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeUserRepo struct {
	byID       map[uuid.UUID]*user.User
	byEmail    map[string]*user.User
	byOAuth    map[string]*user.User // key: provider|uid
	claimTok   map[string]*user.ClaimToken
	oauthLinks []string

	createErr   error
	claimAnonID uuid.UUID
	claimErr    error
	passwordSet map[uuid.UUID]string
}

func newFakeRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:        make(map[uuid.UUID]*user.User),
		byEmail:     make(map[string]*user.User),
		byOAuth:     make(map[string]*user.User),
		claimTok:    make(map[string]*user.ClaimToken),
		passwordSet: make(map[uuid.UUID]string),
	}
}

func (r *fakeUserRepo) put(u *user.User) {
	r.byID[u.ID] = u
	if u.Email != nil {
		r.byEmail[strings.ToLower(*u.Email)] = u
	}
}

func (r *fakeUserRepo) Create(_ context.Context, u *user.User) (*user.User, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	u.ID = uuid.New()
	u.CreatedAt = time.Now()
	r.put(u)
	return u, nil
}

func (r *fakeUserRepo) CreateAnonymous(_ context.Context, displayName string, createdBy uuid.UUID) (*user.User, error) {
	u := &user.User{
		ID:           uuid.New(),
		IdentityType: user.IdentityTypeAnonymous,
		DisplayName:  displayName,
		CreatedAt:    time.Now(),
	}
	_ = createdBy
	r.byID[u.ID] = u
	return u, nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	if u, ok := r.byEmail[strings.ToLower(email)]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (r *fakeUserRepo) FindByOAuth(_ context.Context, provider, providerUID string) (*user.User, error) {
	if u, ok := r.byOAuth[provider+"|"+providerUID]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (r *fakeUserRepo) UpsertOAuthAccount(_ context.Context, userID uuid.UUID, provider, providerUID string, _ *string) error {
	r.oauthLinks = append(r.oauthLinks, provider+"|"+providerUID)
	if u, ok := r.byID[userID]; ok {
		r.byOAuth[provider+"|"+providerUID] = u
	}
	return nil
}

func (r *fakeUserRepo) Update(_ context.Context, u *user.User) (*user.User, error) {
	r.put(u)
	return u, nil
}

func (r *fakeUserRepo) UpdateAvatarURL(_ context.Context, userID uuid.UUID, avatarURL string) error {
	if u, ok := r.byID[userID]; ok {
		u.AvatarURL = &avatarURL
	}
	return nil
}

func (r *fakeUserRepo) UpdatePassword(_ context.Context, userID uuid.UUID, passwordHash string) error {
	r.passwordSet[userID] = passwordHash
	return nil
}

func (r *fakeUserRepo) GetNotificationPrefs(_ context.Context, userID uuid.UUID) (*user.NotificationPrefs, error) {
	return &user.NotificationPrefs{UserID: userID, EmailEnabled: true}, nil
}

func (r *fakeUserRepo) UpdateNotificationPrefs(_ context.Context, prefs *user.NotificationPrefs) (*user.NotificationPrefs, error) {
	return prefs, nil
}

func (r *fakeUserRepo) CreateClaimToken(_ context.Context, anonUserID, createdBy uuid.UUID, tokenHash string, expiresAt time.Time) (*user.ClaimToken, error) {
	tok := &user.ClaimToken{ID: uuid.New(), AnonUserID: anonUserID, CreatedBy: createdBy, TokenHash: tokenHash, ExpiresAt: expiresAt}
	r.claimTok[tokenHash] = tok
	return tok, nil
}

func (r *fakeUserRepo) Claim(_ context.Context, tokenHash string, _ uuid.UUID) (uuid.UUID, error) {
	if r.claimErr != nil {
		return uuid.Nil, r.claimErr
	}
	tok, ok := r.claimTok[tokenHash]
	if !ok {
		return uuid.Nil, user.ErrClaimTokenExpired
	}
	return tok.AnonUserID, nil
}

// fakeResetStore implements user.PasswordResetStore.
type fakeResetStore struct {
	stored  map[string]uuid.UUID
	getErr  error
}

func newResetStore() *fakeResetStore { return &fakeResetStore{stored: make(map[string]uuid.UUID)} }

func (s *fakeResetStore) StoreReset(_ context.Context, tokenHash string, userID uuid.UUID, _ time.Duration) error {
	s.stored[tokenHash] = userID
	return nil
}

func (s *fakeResetStore) GetAndDeleteReset(_ context.Context, tokenHash string) (uuid.UUID, error) {
	if s.getErr != nil {
		return uuid.Nil, s.getErr
	}
	id, ok := s.stored[tokenHash]
	if !ok {
		return uuid.Nil, errors.New("not found")
	}
	delete(s.stored, tokenHash)
	return id, nil
}

func newSvc(repo user.Repository) *user.Service {
	return user.NewService(repo, audit.NopLogger())
}

// ── Register ───────────────────────────────────────────────────────────────────

func TestRegister_Valid_HashesPasswordAndNormalisesEmail(t *testing.T) {
	repo := newFakeRepo()
	got, err := newSvc(repo).Register(context.Background(), "  Kevin ", "Kevin@Example.com", "supersecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DisplayName != "Kevin" {
		t.Errorf("display name = %q, want trimmed %q", got.DisplayName, "Kevin")
	}
	if got.Email == nil || *got.Email != "kevin@example.com" {
		t.Errorf("email not normalised: %v", got.Email)
	}
	if got.PasswordHash == nil {
		t.Fatal("password hash not set")
	}
	if bcrypt.CompareHashAndPassword([]byte(*got.PasswordHash), []byte("supersecret")) != nil {
		t.Error("stored hash does not verify against original password")
	}
	if got.IdentityType != user.IdentityTypeRegistered {
		t.Errorf("identity = %q, want registered", got.IdentityType)
	}
}

func TestRegister_ShortPassword_Error(t *testing.T) {
	_, err := newSvc(newFakeRepo()).Register(context.Background(), "Kevin", "k@e.com", "short")
	if err == nil || !strings.Contains(err.Error(), "at least 8") {
		t.Fatalf("want password length error, got %v", err)
	}
}

func TestRegister_InvalidEmail_Error(t *testing.T) {
	_, err := newSvc(newFakeRepo()).Register(context.Background(), "Kevin", "not-an-email", "supersecret")
	if err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("want email error, got %v", err)
	}
}

func TestRegister_EmptyDisplayName_Error(t *testing.T) {
	_, err := newSvc(newFakeRepo()).Register(context.Background(), "   ", "k@e.com", "supersecret")
	if err == nil || !strings.Contains(err.Error(), "display_name") {
		t.Fatalf("want display_name error, got %v", err)
	}
}

// ── Login ──────────────────────────────────────────────────────────────────────

func seedRegistered(t *testing.T, repo *fakeUserRepo, email, password string) *user.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	hs := string(hash)
	e := strings.ToLower(email)
	u := &user.User{ID: uuid.New(), IdentityType: user.IdentityTypeRegistered, Email: &e, PasswordHash: &hs}
	repo.put(u)
	return u
}

func TestLogin_CorrectPassword_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	seedRegistered(t, repo, "a@b.com", "supersecret")

	got, err := newSvc(repo).Login(context.Background(), "A@B.com", "supersecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email == nil || *got.Email != "a@b.com" {
		t.Errorf("wrong user returned: %v", got)
	}
}

func TestLogin_WrongPassword_InvalidCredentials(t *testing.T) {
	repo := newFakeRepo()
	seedRegistered(t, repo, "a@b.com", "supersecret")

	_, err := newSvc(repo).Login(context.Background(), "a@b.com", "wrongpass")
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail_InvalidCredentials(t *testing.T) {
	_, err := newSvc(newFakeRepo()).Login(context.Background(), "nobody@b.com", "supersecret")
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_GoogleOnlyAccount_OAuthOnly(t *testing.T) {
	repo := newFakeRepo()
	e := "g@b.com"
	repo.put(&user.User{ID: uuid.New(), IdentityType: user.IdentityTypeRegistered, Email: &e}) // no PasswordHash

	_, err := newSvc(repo).Login(context.Background(), "g@b.com", "whatever1")
	if !errors.Is(err, user.ErrOAuthOnly) {
		t.Fatalf("want ErrOAuthOnly, got %v", err)
	}
}

// ── FindOrCreateByOAuth ────────────────────────────────────────────────────────

func TestFindOrCreateByOAuth_ExistingLink_ReturnsUser(t *testing.T) {
	repo := newFakeRepo()
	e := "x@y.com"
	u := &user.User{ID: uuid.New(), IdentityType: user.IdentityTypeRegistered, Email: &e}
	repo.put(u)
	repo.byOAuth["google|uid-1"] = u

	got, created, err := newSvc(repo).FindOrCreateByOAuth(context.Background(), "google", "uid-1", "x@y.com", "X", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected existing user, got created=true")
	}
	if got.ID != u.ID {
		t.Errorf("wrong user: %v", got.ID)
	}
}

func TestFindOrCreateByOAuth_ExistingEmail_LinksAccount(t *testing.T) {
	repo := newFakeRepo()
	seedRegistered(t, repo, "link@me.com", "supersecret")

	got, created, err := newSvc(repo).FindOrCreateByOAuth(context.Background(), "google", "uid-2", "link@me.com", "Linked", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected link to existing email, got created=true")
	}
	if len(repo.oauthLinks) != 1 {
		t.Errorf("expected oauth link created, got %d links", len(repo.oauthLinks))
	}
	_ = got
}

func TestFindOrCreateByOAuth_NewUser_CreatesAndLinks(t *testing.T) {
	repo := newFakeRepo()

	got, created, err := newSvc(repo).FindOrCreateByOAuth(context.Background(), "google", "uid-3", "new@user.com", "New", "http://pic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created=true for new oauth user")
	}
	if got.Email == nil || *got.Email != "new@user.com" {
		t.Errorf("email = %v, want new@user.com", got.Email)
	}
	if got.AvatarURL == nil || *got.AvatarURL != "http://pic" {
		t.Errorf("avatar = %v, want http://pic", got.AvatarURL)
	}
	if len(repo.oauthLinks) != 1 {
		t.Errorf("expected oauth link created, got %d", len(repo.oauthLinks))
	}
}

// ── UpdateMe ───────────────────────────────────────────────────────────────────

func TestUpdateMe_CurrencyPref_NormalisesToUpper(t *testing.T) {
	repo := newFakeRepo()
	u := seedRegistered(t, repo, "u@v.com", "supersecret")

	cp := "usd"
	got, err := newSvc(repo).UpdateMe(context.Background(), u.ID, nil, nil, &cp, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CurrencyPref != "USD" {
		t.Errorf("currency = %q, want USD", got.CurrencyPref)
	}
}

func TestUpdateMe_BadCurrencyLength_Error(t *testing.T) {
	repo := newFakeRepo()
	u := seedRegistered(t, repo, "u@v.com", "supersecret")

	cp := "DOLLAR"
	_, err := newSvc(repo).UpdateMe(context.Background(), u.ID, nil, nil, &cp, nil)
	if err == nil || !strings.Contains(err.Error(), "currency_pref") {
		t.Fatalf("want currency_pref error, got %v", err)
	}
}

func TestUpdateMe_EmptyTimezone_Error(t *testing.T) {
	repo := newFakeRepo()
	u := seedRegistered(t, repo, "u@v.com", "supersecret")

	tz := "   "
	_, err := newSvc(repo).UpdateMe(context.Background(), u.ID, nil, nil, nil, &tz)
	if err == nil || !strings.Contains(err.Error(), "timezone") {
		t.Fatalf("want timezone error, got %v", err)
	}
}

// ── CreateAnonymous / GenerateClaimToken ───────────────────────────────────────

func TestCreateAnonymous_Valid_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	got, err := newSvc(repo).CreateAnonymous(context.Background(), "Guest", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsAnonymous() {
		t.Errorf("identity = %q, want anonymous", got.IdentityType)
	}
}

func TestGenerateClaimToken_AnonUser_ReturnsRawToken(t *testing.T) {
	repo := newFakeRepo()
	anon := &user.User{ID: uuid.New(), IdentityType: user.IdentityTypeAnonymous}
	repo.byID[anon.ID] = anon

	raw, expires, err := newSvc(repo).GenerateClaimToken(context.Background(), anon.ID, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) != 64 { // 32 bytes hex-encoded
		t.Errorf("raw token length = %d, want 64", len(raw))
	}
	if !expires.After(time.Now()) {
		t.Error("expiry should be in the future")
	}
	if len(repo.claimTok) != 1 {
		t.Errorf("expected one stored claim token, got %d", len(repo.claimTok))
	}
}

func TestGenerateClaimToken_RegisteredUser_NotAnonymous(t *testing.T) {
	repo := newFakeRepo()
	reg := &user.User{ID: uuid.New(), IdentityType: user.IdentityTypeRegistered}
	repo.byID[reg.ID] = reg

	_, _, err := newSvc(repo).GenerateClaimToken(context.Background(), reg.ID, uuid.New())
	if !errors.Is(err, user.ErrNotAnonymous) {
		t.Fatalf("want ErrNotAnonymous, got %v", err)
	}
}

// ── ClaimAnonymous ─────────────────────────────────────────────────────────────

func TestClaimAnonymous_ValidToken_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	anon := &user.User{ID: uuid.New(), IdentityType: user.IdentityTypeAnonymous}
	repo.byID[anon.ID] = anon
	claimer := uuid.New()

	raw, _, err := newSvc(repo).GenerateClaimToken(context.Background(), anon.ID, claimer)
	if err != nil {
		t.Fatalf("token gen: %v", err)
	}

	if err := newSvc(repo).ClaimAnonymous(context.Background(), raw, claimer); err != nil {
		t.Fatalf("claim: %v", err)
	}
}

func TestClaimAnonymous_BadToken_Expired(t *testing.T) {
	repo := newFakeRepo()
	err := newSvc(repo).ClaimAnonymous(context.Background(), "garbage-token", uuid.New())
	if !errors.Is(err, user.ErrClaimTokenExpired) {
		t.Fatalf("want ErrClaimTokenExpired, got %v", err)
	}
}

// ── Password reset ─────────────────────────────────────────────────────────────

func TestGeneratePasswordResetToken_KnownEmail_StoresHashedToken(t *testing.T) {
	repo := newFakeRepo()
	u := seedRegistered(t, repo, "reset@me.com", "supersecret")
	store := newResetStore()

	raw, err := newSvc(repo).GeneratePasswordResetToken(context.Background(), "reset@me.com", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw == "" {
		t.Fatal("expected a raw token for a known email")
	}
	if len(store.stored) != 1 {
		t.Fatalf("expected one stored reset token, got %d", len(store.stored))
	}
	for _, id := range store.stored {
		if id != u.ID {
			t.Errorf("stored token maps to %v, want %v", id, u.ID)
		}
	}
}

func TestGeneratePasswordResetToken_UnknownEmail_SilentlySucceeds(t *testing.T) {
	store := newResetStore()
	raw, err := newSvc(newFakeRepo()).GeneratePasswordResetToken(context.Background(), "ghost@nowhere.com", store)
	if err != nil {
		t.Fatalf("expected silent success, got %v", err)
	}
	if raw != "" {
		t.Error("expected empty token for unknown email (no enumeration)")
	}
	if len(store.stored) != 0 {
		t.Error("nothing should be stored for an unknown email")
	}
}

func TestResetPassword_ValidToken_UpdatesPassword(t *testing.T) {
	repo := newFakeRepo()
	u := seedRegistered(t, repo, "reset@me.com", "oldpassword")
	store := newResetStore()
	svc := newSvc(repo)

	raw, err := svc.GeneratePasswordResetToken(context.Background(), "reset@me.com", store)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}

	if err := svc.ResetPassword(context.Background(), raw, "newpassword", store); err != nil {
		t.Fatalf("reset: %v", err)
	}
	newHash, ok := repo.passwordSet[u.ID]
	if !ok {
		t.Fatal("password was not updated")
	}
	if bcrypt.CompareHashAndPassword([]byte(newHash), []byte("newpassword")) != nil {
		t.Error("new hash does not verify against new password")
	}
	if len(store.stored) != 0 {
		t.Error("reset token should be consumed (single-use)")
	}
}

func TestResetPassword_ShortPassword_Error(t *testing.T) {
	err := newSvc(newFakeRepo()).ResetPassword(context.Background(), "tok", "short", newResetStore())
	if err == nil || !strings.Contains(err.Error(), "at least 8") {
		t.Fatalf("want length error, got %v", err)
	}
}

func TestResetPassword_InvalidToken_Error(t *testing.T) {
	err := newSvc(newFakeRepo()).ResetPassword(context.Background(), "unknown", "newpassword", newResetStore())
	if !errors.Is(err, user.ErrInvalidResetToken) {
		t.Fatalf("want ErrInvalidResetToken, got %v", err)
	}
}
