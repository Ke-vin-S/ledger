package auth

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const claimsKey contextKey = "auth_claims"

// SetClaims injects the authenticated user's claims into the request context.
func SetClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFrom retrieves the authenticated user's claims from the context.
// Returns nil if the context has no claims (unauthenticated).
func ClaimsFrom(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// MustUserID returns the authenticated user's UUID. Panics if not authenticated.
// Only call this inside handlers protected by the auth middleware.
func MustUserID(ctx context.Context) uuid.UUID {
	c := ClaimsFrom(ctx)
	if c == nil {
		panic("auth: MustUserID called on unauthenticated context")
	}
	id, _ := uuid.Parse(c.Subject)
	return id
}
