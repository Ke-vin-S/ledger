package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ResetStore implements user.PasswordResetStore using Redis.
type ResetStore struct {
	rdb *redis.Client
}

func NewResetStore(rdb *redis.Client) *ResetStore {
	return &ResetStore{rdb: rdb}
}

func (s *ResetStore) StoreReset(ctx context.Context, tokenHash string, userID uuid.UUID, ttl time.Duration) error {
	return s.rdb.Set(ctx, resetKey(tokenHash), userID.String(), ttl).Err()
}

func (s *ResetStore) GetAndDeleteReset(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	val, err := s.rdb.GetDel(ctx, resetKey(tokenHash)).Result()
	if err == redis.Nil {
		return uuid.Nil, fmt.Errorf("token not found")
	}
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(val)
}

func resetKey(hash string) string { return "pwreset:" + hash }
