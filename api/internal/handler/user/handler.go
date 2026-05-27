package user

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

type Handler struct {
	users       *user.Service
	frontendURL string
}

func New(users *user.Service, frontendURL string) *Handler {
	return &Handler{users: users, frontendURL: frontendURL}
}

// Routes mounts all user endpoints under /users (no /v1 prefix).
// authMW must be applied by the caller for protected sub-routes.
func (h *Handler) Routes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// All user routes require JWT except POST /users/claim (public but still JWT in practice).
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Get("/me", h.GetMe)
		r.Patch("/me", h.UpdateMe)
		r.Get("/me/notification-prefs", h.GetNotificationPrefs)
		r.Patch("/me/notification-prefs", h.UpdateNotificationPrefs)
		r.Get("/{id}", h.GetUser)
		r.Post("/anonymous", h.CreateAnonymous)
		r.Post("/anonymous/{id}/claim-token", h.GenerateClaimToken)
		r.Post("/claim", h.Claim)
	})

	return r
}

// GET /users/me
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	u, err := h.users.GetByID(r.Context(), uid)
	if err != nil {
		handler.Error(w, r, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	handler.JSON(w, r, http.StatusOK, toFullResponse(u))
}

// PATCH /users/me
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())

	var body struct {
		DisplayName  *string `json:"display_name"`
		AvatarURL    *string `json:"avatar_url"`
		CurrencyPref *string `json:"currency_pref"`
		Timezone     *string `json:"timezone"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	u, err := h.users.UpdateMe(r.Context(), uid, body.DisplayName, body.AvatarURL, body.CurrencyPref, body.Timezone)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	handler.JSON(w, r, http.StatusOK, toFullResponse(u))
}

// GET /users/me/notification-prefs
func (h *Handler) GetNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())
	prefs, err := h.users.GetNotificationPrefs(r.Context(), uid)
	if err != nil {
		handler.Error(w, r, http.StatusNotFound, "NOT_FOUND", "notification preferences not found")
		return
	}
	handler.JSON(w, r, http.StatusOK, toPrefsResponse(prefs))
}

// PATCH /users/me/notification-prefs
func (h *Handler) UpdateNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())

	var body struct {
		EmailEnabled  *bool    `json:"email_enabled"`
		DigestMode    *bool    `json:"digest_mode"`
		DisabledTypes []string `json:"disabled_types"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	prefs, err := h.users.UpdateNotificationPrefs(r.Context(), uid, body.EmailEnabled, body.DigestMode, body.DisabledTypes)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	handler.JSON(w, r, http.StatusOK, toPrefsResponse(prefs))
}

// GET /users/:id — returns limited public profile
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid user ID")
		return
	}
	u, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		handler.Error(w, r, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	handler.JSON(w, r, http.StatusOK, toPublicResponse(u))
}

// POST /users/anonymous
func (h *Handler) CreateAnonymous(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())

	var body struct {
		DisplayName string `json:"display_name"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	u, err := h.users.CreateAnonymous(r.Context(), body.DisplayName, uid)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	handler.JSON(w, r, http.StatusCreated, toPublicResponse(u))
}

// POST /users/anonymous/:id/claim-token
func (h *Handler) GenerateClaimToken(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())

	anonID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid user ID")
		return
	}

	rawToken, expiresAt, err := h.users.GenerateClaimToken(r.Context(), anonID, uid)
	if err != nil {
		switch err {
		case user.ErrNotFound:
			handler.Error(w, r, http.StatusNotFound, "USER_NOT_FOUND", "anonymous user not found")
		case user.ErrNotAnonymous:
			handler.Error(w, r, http.StatusBadRequest, "NOT_ANONYMOUS", "user is not anonymous")
		default:
			handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "failed to generate claim token")
		}
		return
	}

	claimURL := h.frontendURL + "/claim/" + rawToken
	handler.JSON(w, r, http.StatusCreated, map[string]any{
		"claim_url":  claimURL,
		"token":      rawToken,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// POST /users/claim
func (h *Handler) Claim(w http.ResponseWriter, r *http.Request) {
	uid := jwtauth.MustUserID(r.Context())

	var body struct {
		ClaimToken string `json:"claim_token"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	if body.ClaimToken == "" {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "claim_token is required")
		return
	}

	if err := h.users.ClaimAnonymous(r.Context(), body.ClaimToken, uid); err != nil {
		switch err {
		case user.ErrClaimTokenExpired:
			handler.Error(w, r, http.StatusBadRequest, "CLAIM_TOKEN_EXPIRED", "claim token is expired or already used")
		case user.ErrAnonAlreadyClaimed:
			handler.Error(w, r, http.StatusConflict, "ANON_ALREADY_CLAIMED", "anonymous profile has already been claimed")
		default:
			handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "claim failed")
		}
		return
	}

	u, err := h.users.GetByID(r.Context(), uid)
	if err != nil {
		handler.Error(w, r, http.StatusOK, "SERVER_ERROR", "claimed successfully but could not fetch profile")
		return
	}
	handler.JSON(w, r, http.StatusOK, toFullResponse(u))
}

// --- response DTOs ---

type fullUserResponse struct {
	ID           uuid.UUID  `json:"id"`
	IdentityType string     `json:"identity_type"`
	DisplayName  string     `json:"display_name"`
	Email        *string    `json:"email,omitempty"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	CurrencyPref string     `json:"currency_pref"`
	Timezone     string     `json:"timezone"`
	CreatedAt    time.Time  `json:"created_at"`
}

type publicUserResponse struct {
	ID           uuid.UUID  `json:"id"`
	IdentityType string     `json:"identity_type"`
	DisplayName  string     `json:"display_name"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type notificationPrefsResponse struct {
	EmailEnabled  bool     `json:"email_enabled"`
	DigestMode    bool     `json:"digest_mode"`
	DisabledTypes []string `json:"disabled_types"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toFullResponse(u *user.User) fullUserResponse {
	return fullUserResponse{
		ID:           u.ID,
		IdentityType: u.IdentityType,
		DisplayName:  u.DisplayName,
		Email:        u.Email,
		AvatarURL:    u.AvatarURL,
		CurrencyPref: u.CurrencyPref,
		Timezone:     u.Timezone,
		CreatedAt:    u.CreatedAt,
	}
}

func toPublicResponse(u *user.User) publicUserResponse {
	return publicUserResponse{
		ID:           u.ID,
		IdentityType: u.IdentityType,
		DisplayName:  u.DisplayName,
		AvatarURL:    u.AvatarURL,
		CreatedAt:    u.CreatedAt,
	}
}

func toPrefsResponse(p *user.NotificationPrefs) notificationPrefsResponse {
	types := p.DisabledTypes
	if types == nil {
		types = []string{}
	}
	return notificationPrefsResponse{
		EmailEnabled:  p.EmailEnabled,
		DigestMode:    p.DigestMode,
		DisabledTypes: types,
		UpdatedAt:     p.UpdatedAt,
	}
}
