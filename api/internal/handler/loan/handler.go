package loan

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	domainloan "github.com/Ke-vin-S/ledger/api/internal/domain/loan"
	"github.com/Ke-vin-S/ledger/api/internal/handler"
)

type Handler struct {
	svc *domainloan.Service
}

func New(svc *domainloan.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts all loan endpoints:
//
//	POST   /v1/loans
//	GET    /v1/loans?direction=lent|borrowed
//	GET    /v1/loans/{id}
//	POST   /v1/loans/{id}/acknowledge
//	POST   /v1/loans/{id}/dispute
//	POST   /v1/loans/{id}/claim-text
func (h *Handler) Routes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/acknowledge", h.acknowledge)
	r.Post("/{id}/dispute", h.dispute)
	r.Post("/{id}/claim-text", h.claimText)
	return r
}

// ── request bodies ────────────────────────────────────────────────────────────

type createBody struct {
	Direction        string  `json:"direction"`
	Amount           int64   `json:"amount"`
	Currency         string  `json:"currency"`
	CounterpartyID   *string `json:"counterparty_id"`
	CounterpartyName string  `json:"counterparty_name"`
	Note             *string `json:"note"`
	LoanDate         string  `json:"loan_date"` // "2006-01-02"
}

type disputeBody struct {
	Reason *string `json:"reason"`
}

// ── response type ─────────────────────────────────────────────────────────────

type loanResponse struct {
	ID               string              `json:"id"`
	Direction        string              `json:"direction"`
	Amount           int64               `json:"amount"`
	Currency         string              `json:"currency"`
	CounterpartyID   string              `json:"counterparty_id"`
	CounterpartyName string              `json:"counterparty_name"`
	Note             *string             `json:"note,omitempty"`
	Status           string              `json:"status"`
	LoanDate         string              `json:"loan_date"`
	CreatedAt        string              `json:"created_at"`
	Repayments       []repaymentResponse `json:"repayments,omitempty"`
}

type repaymentResponse struct {
	ID       string  `json:"id"`
	Amount   int64   `json:"amount"`
	Note     *string `json:"note,omitempty"`
	RepaidAt string  `json:"repaid_at"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	var body createBody
	if !handler.Decode(w, r, &body) {
		return
	}

	loanDate, err := time.Parse("2006-01-02", body.LoanDate)
	if err != nil {
		handler.ErrorField(w, r, http.StatusBadRequest, "INVALID_INPUT", "loan_date must be YYYY-MM-DD", "loan_date")
		return
	}

	var cpID *uuid.UUID
	if body.CounterpartyID != nil && *body.CounterpartyID != "" {
		parsed, err := uuid.Parse(*body.CounterpartyID)
		if err != nil {
			handler.ErrorField(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid counterparty_id", "counterparty_id")
			return
		}
		cpID = &parsed
	}

	l, svcErr := h.svc.CreateLoan(r.Context(), domainloan.CreateInput{
		UserID:           actorID,
		Direction:        body.Direction,
		Amount:           body.Amount,
		Currency:         body.Currency,
		CounterpartyID:   cpID,
		CounterpartyName: body.CounterpartyName,
		Note:             body.Note,
		LoanDate:         loanDate,
	})
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	handler.JSON(w, r, http.StatusCreated, toLoanResponse(l))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	var direction *string
	if d := r.URL.Query().Get("direction"); d != "" {
		direction = &d
	}

	loans, err := h.svc.ListLoans(r.Context(), actorID, direction)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	items := make([]loanResponse, 0, len(loans))
	for _, l := range loans {
		items = append(items, toLoanResponse(l))
	}
	handler.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid loan id")
		return
	}

	l, svcErr := h.svc.GetLoan(r.Context(), actorID, id)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	handler.JSON(w, r, http.StatusOK, toLoanResponse(l))
}

func (h *Handler) acknowledge(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid loan id")
		return
	}

	l, svcErr := h.svc.AcknowledgeLoan(r.Context(), actorID, id)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	handler.JSON(w, r, http.StatusOK, toLoanResponse(l))
}

func (h *Handler) dispute(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid loan id")
		return
	}

	var body disputeBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	l, svcErr := h.svc.DisputeLoan(r.Context(), actorID, id, body.Reason)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	handler.JSON(w, r, http.StatusOK, toLoanResponse(l))
}

func (h *Handler) claimText(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, "INVALID_ID", "invalid loan id")
		return
	}

	text, svcErr := h.svc.ClaimText(r.Context(), actorID, id)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	handler.JSON(w, r, http.StatusOK, map[string]string{"text": text})
}

// ── mapping ───────────────────────────────────────────────────────────────────

func toLoanResponse(l *domainloan.Loan) loanResponse {
	cpID := ""
	if l.CounterpartyID != nil {
		cpID = l.CounterpartyID.String()
	}
	resp := loanResponse{
		ID:               l.ID.String(),
		Direction:        l.Direction,
		Amount:           l.Amount,
		Currency:         l.Currency,
		CounterpartyID:   cpID,
		CounterpartyName: l.CounterpartyName,
		Note:             l.Note,
		Status:           l.Status,
		LoanDate:         l.LoanDate.Format("2006-01-02"),
		CreatedAt:        l.CreatedAt.UTC().Format(time.RFC3339),
	}
	for _, rep := range l.Repayments {
		resp.Repayments = append(resp.Repayments, repaymentResponse{
			ID:       rep.ID.String(),
			Amount:   rep.Amount,
			Note:     rep.Note,
			RepaidAt: rep.RepaidAt.Format("2006-01-02"),
		})
	}
	return resp
}

// ── error handling ────────────────────────────────────────────────────────────

func handleErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domainloan.ErrNotFound):
		handler.Error(w, r, http.StatusNotFound, "NOT_FOUND", "loan not found")
	case errors.Is(err, domainloan.ErrForbidden):
		handler.Error(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
	case errors.Is(err, domainloan.ErrInvalidStatus):
		handler.Error(w, r, http.StatusConflict, "INVALID_STATUS", "loan cannot be modified in its current status")
	case errors.Is(err, domainloan.ErrInvalidInput):
		handler.ErrorField(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), "")
	default:
		handler.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
