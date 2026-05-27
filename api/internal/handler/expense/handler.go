package expense

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
)

type Handler struct {
	svc         *expense.Service
	frontendURL string
}

func New(svc *expense.Service, frontendURL string) *Handler {
	return &Handler{svc: svc, frontendURL: frontendURL}
}

// Routes mounts all expense routes. authMW is the JWT middleware.
func (h *Handler) Routes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)

	// Personal / direct expenses
	r.Post("/", h.createPersonalExpense)
	r.Get("/", h.listMyExpenses)
	r.Get("/{expenseId}", h.getExpense)
	r.Patch("/{expenseId}", h.correctExpense)
	r.Delete("/{expenseId}", h.voidExpense)
	r.Get("/{expenseId}/receipt-url", h.receiptUploadURL)

	return r
}

// TeamRoutes returns routes scoped under /v1/teams/{teamId}/expenses.
func (h *Handler) TeamRoutes(authMW func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(authMW)

	r.Post("/", h.createTeamExpense)
	r.Get("/", h.listTeamExpenses)
	r.Get("/{expenseId}", h.getExpense)
	r.Patch("/{expenseId}", h.correctExpense)
	r.Delete("/{expenseId}", h.voidExpense)
	r.Get("/{expenseId}/receipt-url", h.receiptUploadURL)

	return r
}

// ── request / response types ──────────────────────────────────────────────────

type splitInputJSON struct {
	UserID      string  `json:"user_id"`
	ShareAmount int64   `json:"share_amount"`
	ShareUnits  float64 `json:"share_units"`
}

type createExpenseBody struct {
	Title       string           `json:"title"`
	Amount      int64            `json:"amount"`
	Currency    string           `json:"currency"`
	CategoryID  *string          `json:"category_id"`
	PaidBy      string           `json:"paid_by"`
	ExpenseDate string           `json:"expense_date"` // "2006-01-02"
	SplitMethod *string          `json:"split_method"`
	Splits      []splitInputJSON `json:"splits"`
	BorrowerID  *string          `json:"borrower_id"`
	Note        *string          `json:"note"`
}

type correctExpenseBody struct {
	Title            string           `json:"title"`
	Amount           int64            `json:"amount"`
	Currency         string           `json:"currency"`
	CategoryID       *string          `json:"category_id"`
	PaidBy           string           `json:"paid_by"`
	ExpenseDate      string           `json:"expense_date"`
	SplitMethod      *string          `json:"split_method"`
	Splits           []splitInputJSON `json:"splits"`
	Note             *string          `json:"note"`
	ReceiptURL       *string          `json:"receipt_url"`
	CorrectionReason *string          `json:"correction_reason"`
}

type voidBody struct {
	Reason string `json:"reason"`
}

type expenseResponse struct {
	ID          uuid.UUID              `json:"id"`
	Scope       string                 `json:"scope"`
	TeamID      *uuid.UUID             `json:"team_id,omitempty"`
	Title       string                 `json:"title"`
	Amount      int64                  `json:"amount"`
	Currency    string                 `json:"currency"`
	CategoryID  *uuid.UUID             `json:"category_id,omitempty"`
	PaidBy      uuid.UUID              `json:"paid_by"`
	ExpenseDate string                 `json:"expense_date"`
	SplitMethod *string                `json:"split_method,omitempty"`
	ReceiptURL  *string                `json:"receipt_url,omitempty"`
	Note        *string                `json:"note,omitempty"`
	Version     int                    `json:"version"`
	IsVoid      bool                   `json:"is_void"`
	VoidReason  *string                `json:"void_reason,omitempty"`
	CreatedBy   uuid.UUID              `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	Splits      []splitResponse        `json:"splits"`
}

type splitResponse struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	ShareAmount int64     `json:"share_amount"`
	ShareUnits  *float64  `json:"share_units,omitempty"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) createPersonalExpense(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())

	var body createExpenseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	scope := body.scope()
	if scope == expense.ScopeTeam {
		respondErr(w, r, http.StatusBadRequest, "INVALID_SCOPE", "use POST /v1/teams/{teamId}/expenses for team expenses", "scope")
		return
	}

	input, err := buildCreateInput(body, nil, actorID)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), "")
		return
	}

	result, svcErr := h.svc.CreateExpense(r.Context(), actorID, input)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	respondJSON(w, r, http.StatusCreated, map[string]any{"data": toExpenseResponse(result)})
}

func (h *Handler) createTeamExpense(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, chi.URLParam(r, "teamId"))
	if !ok {
		return
	}

	var body createExpenseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	input, err := buildCreateInput(body, &teamID, actorID)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), "")
		return
	}
	input.Scope = expense.ScopeTeam
	input.TeamID = &teamID

	result, svcErr := h.svc.CreateExpense(r.Context(), actorID, input)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	respondJSON(w, r, http.StatusCreated, map[string]any{"data": toExpenseResponse(result)})
}

func (h *Handler) getExpense(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	result, err := h.svc.GetExpense(r.Context(), actorID, expID)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": toExpenseResponse(result)})
}

func (h *Handler) listMyExpenses(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	includeVoid := r.URL.Query().Get("include_void") == "true"

	results, err := h.svc.ListMyExpenses(r.Context(), actorID, includeVoid)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	items := make([]expenseResponse, 0, len(results))
	for _, res := range results {
		items = append(items, toExpenseResponse(res))
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) listTeamExpenses(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	teamID, ok := parseUUID(w, r, chi.URLParam(r, "teamId"))
	if !ok {
		return
	}
	includeVoid := r.URL.Query().Get("include_void") == "true"

	results, err := h.svc.ListTeamExpenses(r.Context(), actorID, teamID, includeVoid)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	items := make([]expenseResponse, 0, len(results))
	for _, res := range results {
		items = append(items, toExpenseResponse(res))
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) correctExpense(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	var body correctExpenseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	paidBy, err := uuid.Parse(body.PaidBy)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid paid_by uuid", "paid_by")
		return
	}
	expDate, err := time.Parse("2006-01-02", body.ExpenseDate)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "expense_date must be YYYY-MM-DD", "expense_date")
		return
	}
	catID, err := parseOptUUID(body.CategoryID)
	if err != nil {
		respondErr(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid category_id", "category_id")
		return
	}

	input := expense.CorrectInput{
		Title:            body.Title,
		Amount:           body.Amount,
		Currency:         body.Currency,
		CategoryID:       catID,
		PaidBy:           paidBy,
		ExpenseDate:      expDate,
		SplitMethod:      body.SplitMethod,
		Splits:           toSplitInputs(body.Splits),
		Note:             body.Note,
		ReceiptURL:       body.ReceiptURL,
		CorrectionReason: body.CorrectionReason,
	}

	result, svcErr := h.svc.CorrectExpense(r.Context(), actorID, expID, input)
	if svcErr != nil {
		handleErr(w, r, svcErr)
		return
	}
	respondJSON(w, r, http.StatusOK, map[string]any{"data": toExpenseResponse(result)})
}

func (h *Handler) voidExpense(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	var body voidBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := h.svc.VoidExpense(r.Context(), actorID, expID, body.Reason); err != nil {
		handleErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) receiptUploadURL(w http.ResponseWriter, r *http.Request) {
	actorID := jwtauth.MustUserID(r.Context())
	expID, ok := parseUUID(w, r, chi.URLParam(r, "expenseId"))
	if !ok {
		return
	}

	contentType := r.URL.Query().Get("content_type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	uploadURL, key, err := h.svc.GetReceiptUploadURL(r.Context(), actorID, expID, contentType)
	if err != nil {
		handleErr(w, r, err)
		return
	}
	respondJSON(w, r, http.StatusOK, map[string]any{
		"data": map[string]string{"upload_url": uploadURL, "key": key},
	})
}

// ── mapping helpers ───────────────────────────────────────────────────────────

func buildCreateInput(body createExpenseBody, teamID *uuid.UUID, actorID uuid.UUID) (expense.CreateInput, error) {
	paidBy, err := uuid.Parse(body.PaidBy)
	if err != nil {
		// default to actor if not provided
		paidBy = actorID
	}
	expDate, err := time.Parse("2006-01-02", body.ExpenseDate)
	if err != nil {
		return expense.CreateInput{}, errors.New("expense_date must be YYYY-MM-DD")
	}
	catID, err := parseOptUUID(body.CategoryID)
	if err != nil {
		return expense.CreateInput{}, errors.New("invalid category_id")
	}
	borrowerID, err := parseOptUUID(body.BorrowerID)
	if err != nil {
		return expense.CreateInput{}, errors.New("invalid borrower_id")
	}

	scope := body.scope()
	if teamID != nil {
		scope = expense.ScopeTeam
	}

	return expense.CreateInput{
		Scope:       scope,
		TeamID:      teamID,
		Title:       body.Title,
		Amount:      body.Amount,
		Currency:    body.Currency,
		CategoryID:  catID,
		PaidBy:      paidBy,
		ExpenseDate: expDate,
		SplitMethod: body.SplitMethod,
		Splits:      toSplitInputs(body.Splits),
		BorrowerID:  borrowerID,
		Note:        body.Note,
	}, nil
}

func (b createExpenseBody) scope() string {
	if b.BorrowerID != nil {
		return expense.ScopeDirect
	}
	return expense.ScopePersonal
}

func toSplitInputs(raw []splitInputJSON) []expense.SplitInput {
	out := make([]expense.SplitInput, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s.UserID)
		if err != nil {
			continue
		}
		out = append(out, expense.SplitInput{
			UserID:      id,
			ShareAmount: s.ShareAmount,
			ShareUnits:  s.ShareUnits,
		})
	}
	return out
}

func toExpenseResponse(e *expense.ExpenseWithSplits) expenseResponse {
	splits := make([]splitResponse, 0, len(e.Splits))
	for _, s := range e.Splits {
		splits = append(splits, splitResponse{
			ID:          s.ID,
			UserID:      s.UserID,
			ShareAmount: s.ShareAmount,
			ShareUnits:  s.ShareUnits,
		})
	}
	return expenseResponse{
		ID:          e.ID,
		Scope:       e.Scope,
		TeamID:      e.TeamID,
		Title:       e.Title,
		Amount:      e.Amount,
		Currency:    e.Currency,
		CategoryID:  e.CategoryID,
		PaidBy:      e.PaidBy,
		ExpenseDate: e.ExpenseDate.Format("2006-01-02"),
		SplitMethod: e.SplitMethod,
		ReceiptURL:  e.ReceiptURL,
		Note:        e.Note,
		Version:     e.Version,
		IsVoid:      e.IsVoid,
		VoidReason:  e.VoidReason,
		CreatedBy:   e.CreatedBy,
		CreatedAt:   e.CreatedAt,
		Splits:      splits,
	}
}

// ── error handling ────────────────────────────────────────────────────────────

func handleErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, expense.ErrNotFound):
		respondErr(w, r, http.StatusNotFound, "NOT_FOUND", "expense not found", "")
	case errors.Is(err, expense.ErrForbidden):
		respondErr(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission", "")
	case errors.Is(err, expense.ErrAlreadyVoided):
		respondErr(w, r, http.StatusConflict, "ALREADY_VOIDED", "expense is already voided", "")
	case errors.Is(err, expense.ErrInvalidSplitSum):
		respondErr(w, r, http.StatusUnprocessableEntity, "INVALID_SPLIT_SUM", "split amounts do not sum to expense amount", "splits")
	case errors.Is(err, expense.ErrInvalidSplitData):
		respondErr(w, r, http.StatusUnprocessableEntity, "INVALID_SPLIT_DATA", err.Error(), "splits")
	case errors.Is(err, expense.ErrInvalidInput):
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

func parseOptUUID(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func respondJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondErr(w http.ResponseWriter, r *http.Request, status int, code, message, field string) {
	payload := map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
			"field":   field,
		},
	}
	respondJSON(w, r, status, payload)
}
