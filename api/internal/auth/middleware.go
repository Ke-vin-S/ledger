package auth

import (
	"net/http"
	"strings"

	"github.com/Ke-vin-S/ledger/api/internal/middleware"
)

// Middleware returns a Chi-compatible middleware that validates the JWT Bearer token.
// On success it injects the claims into the request context.
// On failure it returns 401.
func Middleware(jwt *JWTService, store *TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				unauthorized(w, r)
				return
			}

			claims, err := jwt.Verify(token)
			if err != nil {
				unauthorized(w, r)
				return
			}

			revoked, err := store.IsRevoked(r.Context(), claims.ID)
			if err != nil || revoked {
				unauthorized(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(SetClaims(r.Context(), claims)))
		})
	}
}

// Optional is like Middleware but continues even when no token is present.
// Claims will be nil if unauthenticated.
func Optional(jwt *JWTService, store *TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token, ok := bearerToken(r); ok {
				if claims, err := jwt.Verify(token); err == nil {
					revoked, _ := store.IsRevoked(r.Context(), claims.ID)
					if !revoked {
						r = r.WithContext(SetClaims(r.Context(), claims))
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	tok := strings.TrimPrefix(h, "Bearer ")
	if tok == "" {
		return "", false
	}
	return tok, true
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	reqID := middleware.GetRequestID(r.Context())
	w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"missing or invalid access token"},"meta":{"request_id":"` + reqID + `"}}`))
}

// RequireAuth is a convenience middleware that blocks non-authenticated requests.
// Use Middleware() to construct the actual middleware — this is just a named alias.
func RequireAuth(jwt *JWTService, store *TokenStore) func(http.Handler) http.Handler {
	return Middleware(jwt, store)
}

