package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"

	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const rateLimitPrefix = "rate_limit:"

// RateLimit applies a fixed-window limit using Redis INCR + EXPIRE.
func RateLimit(rdb *redis.Client, limit int, window time.Duration, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitPrefix + keyFunc(r)
			count, err := rdb.Incr(r.Context(), key).Result()
			if err != nil {
				writeRateLimitError(w, r, http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "rate limiting is temporarily unavailable")
				return
			}
			if count == 1 {
				if err := rdb.Expire(r.Context(), key, window).Err(); err != nil {
					_ = rdb.Del(r.Context(), key).Err()
					writeRateLimitError(w, r, http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "rate limiting is temporarily unavailable")
					return
				}
			}
			if count > int64(limit) {
				w.Header().Set("Retry-After", strconvSeconds(window))
				writeRateLimitError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func IPAndCredentialKey(r *http.Request) string {
	ip := clientIP(r)
	credential := readCredential(r)
	if credential == "" {
		return ip
	}
	return ip + ":" + credential
}

func IPKey(r *http.Request) string { return clientIP(r) }

func readCredential(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
	var decoded struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(decoded.Email))
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, found := strings.Cut(forwarded, ","); found {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func strconvSeconds(window time.Duration) string {
	seconds := int(window.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func writeRateLimitError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
		"meta":  map[string]string{"request_id": GetRequestID(r.Context())},
	})
}
