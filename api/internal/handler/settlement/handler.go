package settlement

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/settlement"
)

type Handler struct {
	svc *settlement.Service
}

func New(svc *settlement.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts settlement routes.
//
//	POST   /v1/expenses/{expenseId}/settlements
//	GET    /v1/expenses/{expenseId}/settlements
//	POST   /v1/settlements/{id}/confirm
//	POST   /v1/settlements/{id}/dispute
//	GET    /v1/expenses/{expenseId}/balance          (debt balance for caller)
//	GET    /v1/teams/{teamId}/balances               (net balances in team)
//	GET    /v1/balances                              (caller's cross-team balances)
func (h *Handler) ExpenseRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Post("/", h.recordSettlement)
	r.Get("/", h.listSettlements)
	r.Get("/balance", h.debtBalance)
	return r
}

func (h *Handler) SettlementRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Post("/{id}/confirm", h.confirmSettlement)
	r.Post("/{id}/dispute", h.disputeSettlement)
	return r
}

func (h *Handler) TeamBalanceRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Get("/", h.teamBalances)
	return r
}

func (h *Handler) MyBalancesHandler(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Get("/", h.myBalances)
	return r
}

// ── request / response types ──────────────────────────────────────────────────

type recordBody struct {
	PayerID    string  `json:"payer_id"`
	PayeeID    string  `json:"payee_id"`
	Amount     int64   `json:"amount"`
	Method     string  `json:"method"`
	MethodNote *string `json:"method_note"`
	SettledOn  string  `json:"settled_on"` // "2006-01-02"
}

type disputeBody struct {
	Reason string `json:"reason"`
}

type settlementResponse struct {
	ID            uuid.UUID  `json:"id"`
	ExpenseID     uuid.UUID  `json:"expense_id"`
	PayerID       uuid.UUID  `json:"payer_id"`
	PayeeID       uuid.UUID  `json:"payee_id"`
	Amount        int64      `json:"amount"`
	Method        string     `json:"method"`
	MethodNote    *string    `json:"method_note,omitempty"`
	Status        string     `json:"status"`
	RecordedBy    uuid.UUID  `json:"recorded_by"`
	ConfirmedBy   *uuid.UUID `json:"confirmed_by,omitempty"`
	ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
	DisputedBy    *uuid.UUID `json:"disputed_by,omitempty"`
	DisputedAt    *time.Time `json:"disputed_at,omitempty"`
	DisputeReason *string    `json:"dispute_reason,omitempty"`
	SettledOn     string     `json:"settled_on"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) recordSettlement(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	var body recordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	payerID, err := uuid.Parse(body.PayerID)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid payer_id", "payer_id")
		return
	}
	payeeID, err := uuid.Parse(body.PayeeID)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid payee_id", "payee_id")
		return
	}
	settledOn, err := time.Parse("2006-01-02", body.SettledOn)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "settled_on must be YYYY-MM-DD", "settled_on")
		return
	}

	result, svcErr := h.svc.RecordSettlement(r.Context(), actorID, settlement.RecordInput{
		ExpenseID:  expID,
		PayerID:    payerID,
		PayeeID:    payeeID,
		Amount:     body.Amount,
		Method:     body.Method,
		MethodNote: body.MethodNote,
		SettledOn:  settledOn,
	})
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	respondJSON(w, r, http.StatusCreated, map[string]any{"data": toSettlementResponse(result)})
}

func (h *Handler) listSettlements(w http.ResponseWriter, r *http.Request) {
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}
	results, err := h.svc.ListSettlementsByExpense(r.Context(), expID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	items := make([]settlementResponse, 0, len(results))
	for _, s := range results {
		items = append(items, toSettlementResponse(s))
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) debtBalance(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	// Default debtor to caller; allow explicit override via query param.
	debtorID := actorID
	if raw := r.URL.Query().Get("debtor_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid debtor_id", "debtor_id")
			return
		}
		debtorID = id
	}

	bal, err := h.svc.GetDebtBalance(r.Context(), expID, debtorID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": bal})
}

func (h *Handler) confirmSettlement(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, ok := parseUUID(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	result, err := h.svc.ConfirmSettlement(r.Context(), actorID, id)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": toSettlementResponse(result)})
}

func (h *Handler) disputeSettlement(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, ok := parseUUID(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	var body disputeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	result, err := h.svc.DisputeSettlement(r.Context(), actorID, id, body.Reason)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": toSettlementResponse(result)})
}

func (h *Handler) teamBalances(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, chi.URLParam(r, "teamId"))
	if !ok {
		return
	}
	balances, err := h.svc.ListTeamBalances(r.Context(), teamID, actorID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	if balances == nil {
		balances = []*settlement.TeamBalance{}
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": balances})
}

func (h *Handler) myBalances(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	balances, err := h.svc.ListMyBalances(r.Context(), actorID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	if balances == nil {
		balances = []*settlement.UserBalance{}
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": balances})
}

// ── mapping ───────────────────────────────────────────────────────────────────

func toSettlementResponse(s *settlement.Settlement) settlementResponse {
	return settlementResponse{
		ID:            s.ID,
		ExpenseID:     s.ExpenseID,
		PayerID:       s.PayerID,
		PayeeID:       s.PayeeID,
		Amount:        s.Amount,
		Method:        s.Method,
		MethodNote:    s.MethodNote,
		Status:        s.Status,
		RecordedBy:    s.RecordedBy,
		ConfirmedBy:   s.ConfirmedBy,
		ConfirmedAt:   s.ConfirmedAt,
		DisputedBy:    s.DisputedBy,
		DisputedAt:    s.DisputedAt,
		DisputeReason: s.DisputeReason,
		SettledOn:     s.SettledOn.Format("2006-01-02"),
		CreatedAt:     s.CreatedAt,
	}
}

// ── error handling ────────────────────────────────────────────────────────────

func handleErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, settlement.ErrNotFound):
		respondErr(w, r, http.StatusNotFound, "NOT_FOUND", "settlement not found", "")
	case errors.Is(err, settlement.ErrForbidden):
		respondErr(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission", "")
	case errors.Is(err, settlement.ErrSettlementExceedsDebt):
		respondErr(w, r, http.StatusConflict, "SETTLEMENT_EXCEEDS_DEBT", err.Error(), "amount")
	case errors.Is(err, settlement.ErrInvalidStatus):
		respondErr(w, r, http.StatusConflict, "INVALID_STATUS", "settlement cannot be modified in its current status", "")
	case errors.Is(err, settlement.ErrNoDebt):
		respondErr(w, r, http.StatusNotFound, "NO_DEBT", "no outstanding debt for this expense/debtor pair", "")
	case errors.Is(err, settlement.ErrInvalidInput):
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), "")
	default:
		respondErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", "")
	}
}

// ── shared helpers ────────────────────────────────────────────────────────────

func parseUUID(w http.ResponseWriter, r *http.Request, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_ID", "invalid uuid", "")
		return uuid.Nil, false
	}
	return id, true
}

func respondJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondErr(w http.ResponseWriter, r *http.Request, status int, code, message, field string) {
	respondJSON(w, r, status, map[string]any{
		"error": map[string]string{"code": code, "message": message, "field": field},
	})
}
