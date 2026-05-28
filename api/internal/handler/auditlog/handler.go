package auditlog

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/auditlog"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

type Handler struct {
	svc *auditlog.Service
}

func New(svc *auditlog.Service) *Handler {
	return &Handler{svc: svc}
}

// TeamRoutes mounts audit routes under /v1/teams/{teamId}/audit.
//
//	GET / — list team audit entries (query: action=, limit=, cursor=)
func (h *Handler) TeamRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Get("/", h.listTeamEntries)
	return r
}

// MyRoutes mounts audit routes under /v1/audit.
//
//	GET / — list caller's own audit entries (query: action=, limit=, cursor=)
func (h *Handler) MyRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Get("/", h.listMyEntries)
	return r
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) listTeamEntries(w http.ResponseWriter, r *http.Request) {
	teamID, ok := parseUUID(w, r, chi.URLParam(r, "teamId"))
	if !ok {
		return
	}

	params := parseListParams(r)
	items, hasMore, err := h.svc.ListTeamEntries(r.Context(), teamID, params)
	if err != nil {
		handleErr(w, r, err)
		return
	}

	nextCursor := nextCursorFrom(items, hasMore)
	handler.JSONPaginated(w, r, items, nextCursor, hasMore)
}

func (h *Handler) listMyEntries(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	params := parseListParams(r)
	items, hasMore, err := h.svc.ListMyEntries(r.Context(), actorID, params)
	if err != nil {
		handleErr(w, r, err)
		return
	}

	nextCursor := nextCursorFrom(items, hasMore)
	handler.JSONPaginated(w, r, items, nextCursor, hasMore)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseListParams(r *http.Request) auditlog.ListParams {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return auditlog.ListParams{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
		Action: r.URL.Query().Get("action"),
	}
}

func nextCursorFrom(items []*auditlog.LogEntry, hasMore bool) string {
	if hasMore && len(items) > 0 {
		return items[len(items)-1].CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z")
	}
	return ""
}

func handleErr(w http.ResponseWriter, r *http.Request, err error) {
	_ = err
	if errors.Is(err, nil) {
		return
	}
	handler.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}

func parseUUID(w http.ResponseWriter, r *http.Request, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}
