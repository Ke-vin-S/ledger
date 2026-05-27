package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

const refreshCookie = "refresh_token"

type Handler struct {
	users          *user.Service
	jwt            *jwtauth.JWTService
	tokens         *jwtauth.TokenStore
	resetStore     *jwtauth.ResetStore
	isLocal        bool
	googleClientID string
}

func New(
	users *user.Service,
	jwt *jwtauth.JWTService,
	tokens *jwtauth.TokenStore,
	resetStore *jwtauth.ResetStore,
	isLocal bool,
	googleClientID string,
) *Handler {
	return &Handler{
		users:          users,
		jwt:            jwt,
		tokens:         tokens,
		resetStore:     resetStore,
		isLocal:        isLocal,
		googleClientID: googleClientID,
	}
}

// Routes mounts all auth endpoints. The /auth prefix is applied by the caller.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/oauth/google", h.OAuthGoogle)
	r.Post("/refresh", h.Refresh)
	r.Post("/password/reset-request", h.PasswordResetRequest)
	r.Post("/password/reset", h.PasswordReset)

	// Logout requires a valid JWT to identify the user for family invalidation.
	r.With(authMiddleware).Post("/logout", h.Logout)
	return r
}

// POST /auth/register
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	u, err := h.users.Register(r.Context(), body.DisplayName, body.Email, body.Password)
	if err != nil {
		h.handleUserError(w, r, err)
		return
	}

	resp, err := h.issueTokens(w, r, u)
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "failed to issue tokens")
		return
	}
	handler.JSON(w, r, http.StatusCreated, resp)
}

// POST /auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	u, err := h.users.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		handler.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
		return
	}

	resp, err := h.issueTokens(w, r, u)
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "failed to issue tokens")
		return
	}
	handler.JSON(w, r, http.StatusOK, resp)
}

// POST /auth/oauth/google — expects { "credential": "<Google ID token>" }
func (h *Handler) OAuthGoogle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credential string `json:"credential"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}
	if body.Credential == "" {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "credential is required")
		return
	}

	info, err := verifyGoogleIDToken(r.Context(), body.Credential, h.googleClientID)
	if err != nil {
		handler.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "Google ID token verification failed")
		return
	}

	u, _, err := h.users.FindOrCreateByOAuth(
		r.Context(), "google", info.Sub, info.Email, info.Name, info.Picture,
	)
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "OAuth sign-in failed")
		return
	}

	resp, err := h.issueTokens(w, r, u)
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "failed to issue tokens")
		return
	}
	handler.JSON(w, r, http.StatusOK, resp)
}

// POST /auth/refresh — reads token from HttpOnly cookie (web) or request body (mobile).
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawToken := tokenFromCookieOrBody(r)
	if rawToken == "" {
		handler.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "refresh token is required")
		return
	}

	newToken, userID, err := h.tokens.Rotate(r.Context(), rawToken)
	if err != nil {
		handler.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "refresh token is invalid or expired")
		return
	}

	u, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		handler.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "user not found")
		return
	}

	accessToken, _, err := h.jwt.Sign(u.ID, u.IdentityType)
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, "SERVER_ERROR", "failed to sign token")
		return
	}

	setRefreshCookie(w, newToken, h.isLocal)
	handler.JSON(w, r, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newToken,
		ExpiresIn:    int(jwtauth.AccessTokenTTL.Seconds()),
	})
}

// POST /auth/logout — requires JWT; invalidates the entire refresh token family.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := jwtauth.ClaimsFrom(r.Context())
	if claims != nil {
		if uid, err := uuid.Parse(claims.Subject); err == nil {
			_ = h.tokens.InvalidateFamily(r.Context(), uid)
		}
		_ = h.tokens.MarkRevoked(r.Context(), claims.ID)
	}
	// Also revoke the refresh token if sent in the cookie/body.
	if rawToken := tokenFromCookieOrBody(r); rawToken != "" {
		_ = h.tokens.Revoke(r.Context(), rawToken)
	}
	clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/password/reset-request
func (h *Handler) PasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	rawToken, err := h.users.GeneratePasswordResetToken(r.Context(), body.Email, h.resetStore)
	if err != nil {
		// Internal error; still return 200 to avoid leaking info.
		handler.JSON(w, r, http.StatusOK, map[string]string{
			"message": "If that email exists, a reset link has been sent.",
		})
		return
	}

	var resp any = map[string]string{"message": "If that email exists, a reset link has been sent."}
	if h.isLocal && rawToken != "" {
		resp = map[string]any{
			"message":     "Password reset token (dev only — not sent in production)",
			"reset_token": rawToken,
		}
	}
	handler.JSON(w, r, http.StatusOK, resp)
}

// POST /auth/password/reset
func (h *Handler) PasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !handler.Decode(w, r, &body) {
		return
	}

	if err := h.users.ResetPassword(r.Context(), body.Token, body.NewPassword, h.resetStore); err != nil {
		switch err {
		case user.ErrInvalidResetToken:
			handler.Error(w, r, http.StatusBadRequest, "CLAIM_TOKEN_EXPIRED", "reset token is invalid or expired")
		default:
			handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		}
		return
	}
	handler.JSON(w, r, http.StatusOK, map[string]string{"message": "Password updated successfully."})
}

// --- internal helpers ---

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, u *user.User) (tokenResponse, error) {
	accessToken, _, err := h.jwt.Sign(u.ID, u.IdentityType)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("sign: %w", err)
	}
	refreshToken, err := h.tokens.Issue(r.Context(), u.ID)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("issue refresh: %w", err)
	}
	setRefreshCookie(w, refreshToken, h.isLocal)
	return tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(jwtauth.AccessTokenTTL.Seconds()),
	}, nil
}

func setRefreshCookie(w http.ResponseWriter, token string, isLocal bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(jwtauth.RefreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   !isLocal,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func tokenFromCookieOrBody(r *http.Request) string {
	if c, err := r.Cookie(refreshCookie); err == nil && c.Value != "" {
		return c.Value
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 512))
	var m map[string]string
	if err := json.Unmarshal(body, &m); err == nil {
		return m["refresh_token"]
	}
	return ""
}

func (h *Handler) handleUserError(w http.ResponseWriter, r *http.Request, err error) {
	switch err {
	case user.ErrEmailAlreadyExists:
		handler.ErrorField(w, r, http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "this email is already registered", "email")
	case user.ErrInvalidCredentials:
		handler.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
	default:
		handler.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	}
}

type googleTokenInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Aud     string `json:"aud"`
}

func verifyGoogleIDToken(ctx context.Context, idToken, expectedClientID string) (*googleTokenInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://oauth2.googleapis.com/tokeninfo?id_token="+idToken, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tokeninfo request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tokeninfo returned status %d", resp.StatusCode)
	}
	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode tokeninfo: %w", err)
	}
	if expectedClientID != "" && info.Aud != expectedClientID {
		return nil, fmt.Errorf("token audience %q does not match client ID", info.Aud)
	}
	if info.Sub == "" {
		return nil, fmt.Errorf("missing sub in tokeninfo")
	}
	return &info, nil
}

