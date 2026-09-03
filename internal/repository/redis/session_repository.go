package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:jwt:"

// SessionRepository is a JWT whitelist. A token is accepted only while its JTI
// key is present in Redis; deleting the key invalidates it immediately.
type SessionRepository struct{ client redis.UniversalClient }

func NewSessionRepository(client redis.UniversalClient) *SessionRepository {
	return &SessionRepository{client: client}
}

func (r *SessionRepository) Create(ctx context.Context, tokenID, userID uuid.UUID, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("session TTL must be positive")
	}
	if err := r.client.Set(ctx, sessionKey(tokenID), userID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("store JWT session: %w", err)
	}
	return nil
}

func (r *SessionRepository) IsActive(ctx context.Context, tokenID, userID uuid.UUID) (bool, error) {
	storedUserID, err := r.client.Get(ctx, sessionKey(tokenID)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get JWT session: %w", err)
	}
	return storedUserID == userID.String(), nil
}

func (r *SessionRepository) Delete(ctx context.Context, tokenID uuid.UUID) error {
	if err := r.client.Del(ctx, sessionKey(tokenID)).Err(); err != nil {
		return fmt.Errorf("delete JWT session: %w", err)
	}
	return nil
}

func sessionKey(tokenID uuid.UUID) string { return sessionKeyPrefix + tokenID.String() }
