package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name   string
		header string
		valid  bool
	}{
		{name: "accepted", header: "client-request_123", valid: true},
		{name: "missing", header: "", valid: true},
		{name: "injection", header: `bad"id`, valid: false},
		{name: "too long", header: string(make([]byte, 65)), valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(GetRequestID(r.Context())))
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Request-ID", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			got := rec.Header().Get("X-Request-ID")
			if got == "" {
				t.Fatal("missing response request ID")
			}
			if tt.header != "" && requestIDPattern.MatchString(tt.header) != tt.valid {
				t.Fatalf("test fixture validity mismatch for %q", tt.header)
			}
			if tt.header != "" && tt.valid && got != tt.header {
				t.Fatalf("request ID = %q, want %q", got, tt.header)
			}
			if !tt.valid && !requestIDPattern.MatchString(got) {
				t.Fatalf("generated request ID is invalid: %q", got)
			}
		})
	}
}
