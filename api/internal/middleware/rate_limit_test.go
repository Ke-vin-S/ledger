package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRateLimit_FixedWindowRejectsEleventhRequest(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	called := 0
	handler := RateLimit(rdb, 10, time.Minute, IPKey)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))

	for attempt := 1; attempt <= 11; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/auth/login", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if attempt <= 10 && rec.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d, want 204", attempt, rec.Code)
		}
		if attempt == 11 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d status = %d, want 429", attempt, rec.Code)
		}
	}
	if called != 10 {
		t.Fatalf("downstream calls = %d, want 10", called)
	}
}

func TestIPAndCredentialKey_PreservesRequestBody(t *testing.T) {
	body := []byte("{\"email\":\"User@Example.com\",\"password\":\"secret\"}")
	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.2:1234"

	key := IPAndCredentialKey(req)
	if key != "192.0.2.2:user@example.com" {
		t.Fatalf("key = %q", key)
	}
	got, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read preserved body: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body = %q, want %q", got, body)
	}
}
