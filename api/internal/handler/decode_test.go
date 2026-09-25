package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecode_RejectsTrailingJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"first"}{"name":"second"}`))
	rec := httptest.NewRecorder()
	var body struct {
		Name string `json:"name"`
	}
	if Decode(rec, req, &body) {
		t.Fatal("Decode accepted multiple JSON values")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDecode_RejectsUnknownField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"first","extra":true}`))
	rec := httptest.NewRecorder()
	var body struct {
		Name string `json:"name"`
	}
	if Decode(rec, req, &body) {
		t.Fatal("Decode accepted an unknown field")
	}
}
