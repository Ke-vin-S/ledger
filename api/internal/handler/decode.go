package handler

import (
	"encoding/json"
	"net/http"
)

// Decode reads and decodes a JSON request body into v.
// Returns false and writes a 400 error response if decoding fails.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body: "+err.Error())
		return false
	}
	return true
}
