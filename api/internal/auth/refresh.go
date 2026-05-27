package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Refresh token format: "<user_id>.<32_random_bytes_hex>"
// Encoding the user_id allows family invalidation even when the token is already rotated.

type TokenStore struct {
	rdb *redis.Client
}

func NewTokenStore(rdb *redis.Client) *TokenStore {
	return &TokenStore{rdb: rdb}
}

// Issue generates a new refresh token, stores it in Redis, and returns the raw token string.
func (s *TokenStore) Issue(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := generateToken(userID)
	if err != nil {
		return "", err
	}
	h := hashToken(token)

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, refreshKey(h), userID.String(), RefreshTokenTTL)
	pipe.SAdd(ctx, familyKey(userID), h)
	pipe.Expire(ctx, familyKey(userID), RefreshTokenTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return token, nil
}

// Rotate validates oldToken, deletes it, and issues a new one.
// Returns the new token and the authenticated user ID.
// If oldToken is a replayed (already-rotated) token, it invalidates the entire family.
func (s *TokenStore) Rotate(ctx context.Context, oldToken string) (newToken string, userID uuid.UUID, err error) {
	parsedUID, err := parseUserID(oldToken)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("invalid refresh token format")
	}

	h := hashToken(oldToken)
	val, err := s.rdb.GetDel(ctx, refreshKey(h)).Result()
	if err == redis.Nil {
		// Token not found — could be replay attack; invalidate family
		_ = s.InvalidateFamily(ctx, parsedUID)
		return "", uuid.Nil, ErrRefreshTokenInvalid
	}
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("get refresh token: %w", err)
	}

	storedUID, err := uuid.Parse(val)
	if err != nil || storedUID != parsedUID {
		// Mismatch — invalidate family as a precaution
		_ = s.InvalidateFamily(ctx, parsedUID)
		return "", uuid.Nil, ErrRefreshTokenInvalid
	}

	// Remove from family set
	s.rdb.SRem(ctx, familyKey(storedUID), h)

	newToken, err = s.Issue(ctx, storedUID)
	if err != nil {
		return "", uuid.Nil, err
	}
	return newToken, storedUID, nil
}

// Revoke removes a single refresh token from Redis.
func (s *TokenStore) Revoke(ctx context.Context, token string) error {
	parsedUID, err := parseUserID(token)
	if err != nil {
		return nil // not a valid token; nothing to revoke
	}
	h := hashToken(token)
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, refreshKey(h))
	pipe.SRem(ctx, familyKey(parsedUID), h)
	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateFamily deletes all refresh tokens for a user (used on replay or explicit logout).
func (s *TokenStore) InvalidateFamily(ctx context.Context, userID uuid.UUID) error {
	fKey := familyKey(userID)
	hashes, err := s.rdb.SMembers(ctx, fKey).Result()
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		return nil
	}
	keys := make([]string, len(hashes)+1)
	for i, h := range hashes {
		keys[i] = refreshKey(h)
	}
	keys[len(hashes)] = fKey
	return s.rdb.Del(ctx, keys...).Err()
}

// MarkRevoked stores a JWT JTI in Redis as revoked for the duration of the access token TTL.
func (s *TokenStore) MarkRevoked(ctx context.Context, jti string) error {
	return s.rdb.Set(ctx, revokedKey(jti), "1", AccessTokenTTL+time.Minute).Err()
}

// IsRevoked checks whether a JWT JTI has been explicitly revoked.
func (s *TokenStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, revokedKey(jti)).Result()
	return n > 0, err
}

func generateToken(userID uuid.UUID) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate token secret: %w", err)
	}
	return userID.String() + "." + hex.EncodeToString(secret), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func parseUserID(token string) (uuid.UUID, error) {
	idx := strings.Index(token, ".")
	if idx < 0 {
		return uuid.Nil, fmt.Errorf("missing separator")
	}
	return uuid.Parse(token[:idx])
}

func refreshKey(hash string) string  { return "refresh:" + hash }
func familyKey(uid uuid.UUID) string { return "rfam:" + uid.String() }
func revokedKey(jti string) string   { return "revoked:" + jti }
