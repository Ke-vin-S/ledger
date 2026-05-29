package handler

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/Ke-vin-S/ledger/api/internal/logger"
	"github.com/Ke-vin-S/ledger/api/internal/middleware"
)

type envelope struct {
	Data any  `json:"data,omitempty"`
	Meta meta `json:"meta"`
}

type meta struct {
	RequestID string `json:"request_id"`
}

type paginatedData struct {
	Items      any    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
	Meta  meta     `json:"meta"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{
		Data: data,
		Meta: meta{RequestID: middleware.GetRequestID(r.Context())},
	})
}

func JSONPaginated(w http.ResponseWriter, r *http.Request, items any, nextCursor string, hasMore bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(envelope{
		Data: paginatedData{
			Items:      items,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
		Meta: meta{RequestID: middleware.GetRequestID(r.Context())},
	})
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	ErrorField(w, r, status, code, message, "")
}

func ErrorField(w http.ResponseWriter, r *http.Request, status int, code, message, field string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: apiError{Code: code, Message: message, Field: field},
		Meta:  meta{RequestID: middleware.GetRequestID(r.Context())},
	})
	if status >= 500 {
		logger.FromContext(r.Context()).Error("unhandled error",
			zap.Int("status", status),
			zap.String("code", code),
			zap.String("message", message),
		)
	}
}
