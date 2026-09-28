package auth

import (
	"net/http"
	"strings"

	"github.com/Ke-vin-S/ledger/api/internal/handler"
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
	handler.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid access token")
}

// RequireAuth is a convenience middleware that blocks non-authenticated requests.
// Use Middleware() to construct the actual middleware — this is just a named alias.
func RequireAuth(jwt *JWTService, store *TokenStore) func(http.Handler) http.Handler {
	return Middleware(jwt, store)
}
