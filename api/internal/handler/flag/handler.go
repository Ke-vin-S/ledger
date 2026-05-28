package flag

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/flag"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

type Handler struct {
	svc *flag.Service
}

func New(svc *flag.Service) *Handler {
	return &Handler{svc: svc}
}

// ExpenseRoutes mounts flag routes under /v1/expenses/{expenseId}/flags.
//
//	POST /   — raise a flag on the expense
//	GET  /   — list all flags for the expense
func (h *Handler) ExpenseRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Post("/", h.raiseFlag)
	r.Get("/", h.listFlags)
	return r
}

// FlagRoutes mounts flag action routes under /v1/flags.
//
//	POST /{id}/resolve — resolve an open flag
func (h *Handler) FlagRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Post("/{id}/resolve", h.resolveFlag)
	return r
}

// ── request / response types ──────────────────────────────────────────────────

type raiseBody struct {
	Reason string `json:"reason"`
}

type resolveBody struct {
	ResolutionNote string `json:"resolution_note"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) raiseFlag(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expenseID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	var body raiseBody
	if !handler.Decode(w, r, &body) {
		return
	}

	f, err := h.svc.RaiseFlag(r.Context(), flag.RaiseInput{
		ExpenseID: expenseID,
		RaisedBy:  actorID,
		Reason:    body.Reason,
	})
	if err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusCreated, f)
}

func (h *Handler) listFlags(w http.ResponseWriter, r *http.Request) {
	expenseID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	flags, err := h.svc.ListFlags(r.Context(), expenseID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, flags)
}

func (h *Handler) resolveFlag(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	flagID, ok := parseUUID(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	var body resolveBody
	if !handler.Decode(w, r, &body) {
		return
	}

	f, err := h.svc.ResolveFlag(r.Context(), flag.ResolveInput{
		FlagID:         flagID,
		ResolvedBy:     actorID,
		ResolutionNote: body.ResolutionNote,
	})
	if err != nil {
		handleErr(w, r, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, f)
}

// ── error mapping ─────────────────────────────────────────────────────────────

func handleErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, flag.ErrNotFound):
		handler.Error(w, r, http.StatusNotFound, "NOT_FOUND", "flag not found")
	case errors.Is(err, flag.ErrForbidden):
		handler.Error(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	case errors.Is(err, flag.ErrAlreadyResolved):
		handler.Error(w, r, http.StatusConflict, "ALREADY_RESOLVED", "flag is already resolved")
	case errors.Is(err, flag.ErrInvalidInput):
		handler.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
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
