package notification

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/notification"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

type Handler struct {
	svc *notification.Service
}

func New(svc *notification.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts all notification routes.
//
//	GET    /v1/notifications              — list (query: unread=true, limit, cursor)
//	POST   /v1/notifications/read-all     — mark all read
//	POST   /v1/notifications/{id}/read    — mark single read
//	DELETE /v1/notifications/{id}         — dismiss
//	GET    /v1/notifications/prefs        — get prefs
//	PATCH  /v1/notifications/prefs        — update prefs
func (h *Handler) Routes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)

	r.Get("/", h.list)
	r.Post("/read-all", h.markAllRead)
	r.Get("/prefs", h.getPrefs)
	r.Patch("/prefs", h.updatePrefs)
	r.Post("/{id}/read", h.markRead)
	r.Delete("/{id}", h.dismiss)

	return r
}

// ── request / response types ──────────────────────────────────────────────────

type updatePrefsBody struct {
	EmailEnabled  *bool     `json:"email_enabled"`
	DigestMode    *bool     `json:"digest_mode"`
	DisabledTypes *[]string `json:"disabled_types"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	params := notification.ListParams{
		UserID:     actorID,
		UnreadOnly: r.URL.Query().Get("unread") == "true",
		Limit:      limit,
		Cursor:     r.URL.Query().Get("cursor"),
	}

	items, hasMore, err := h.svc.List(r.Context(), params)
	if err != nil {
		handleErr(w, r, err)
		return
	}

	var nextCursor string
	if hasMore && len(items) > 0 {
		nextCursor = items[len(items)-1].CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z")
	}
	handler.JSONPaginated(w, r, items, nextCursor, hasMore)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, ok := parseUUID(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	n, err := h.svc.MarkRead(r.Context(), id, actorID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, n)
}

func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	if err := h.svc.MarkAllRead(r.Context(), actorID); err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, ok := parseUUID(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	if err := h.svc.Dismiss(r.Context(), id, actorID); err != nil {
		handleErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getPrefs(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	prefs, err := h.svc.GetPrefs(r.Context(), actorID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, prefs)
}

func (h *Handler) updatePrefs(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	var body updatePrefsBody
	if !handler.Decode(w, r, &body) {
		return
	}

	prefs, err := h.svc.UpdatePrefs(r.Context(), notification.UpdatePrefsInput{
		UserID:        actorID,
		EmailEnabled:  body.EmailEnabled,
		DigestMode:    body.DigestMode,
		DisabledTypes: body.DisabledTypes,
	})
	if err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, prefs)
}

// ── error mapping ─────────────────────────────────────────────────────────────

func handleErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, notification.ErrNotFound):
		handler.Error(w, r, http.StatusNotFound, "NOT_FOUND", "notification not found")
	case errors.Is(err, notification.ErrForbidden):
		handler.Error(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	default:
		handler.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseUUID(w http.ResponseWriter, r *http.Request, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}
