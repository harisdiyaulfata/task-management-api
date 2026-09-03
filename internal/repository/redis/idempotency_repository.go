package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/redis/go-redis/v9"
)

const idempotencyKeyPrefix = "idempotency:task:create:"

type IdempotencyRepository struct{ client redis.UniversalClient }

func NewIdempotencyRepository(client redis.UniversalClient) *IdempotencyRepository {
	return &IdempotencyRepository{client: client}
}

// Acquire uses Redis SET NX, which is atomic. Exactly one concurrent caller can
// receive acquired=true for a key while the processing record exists.
func (r *IdempotencyRepository) Acquire(ctx context.Context, key string, ttl time.Duration) (domain.IdempotencyRecord, bool, error) {
	processing := domain.IdempotencyRecord{State: domain.IdempotencyProcessing}
	payload, err := json.Marshal(processing)
	if err != nil {
		return domain.IdempotencyRecord{}, false, fmt.Errorf("marshal processing record: %w", err)
	}
	created, err := r.client.SetNX(ctx, redisIdempotencyKey(key), payload, ttl).Result()
	if err != nil {
		return domain.IdempotencyRecord{}, false, fmt.Errorf("acquire idempotency key: %w", err)
	}
	if created {
		return processing, true, nil
	}
	record, err := r.Get(ctx, key)
	return record, false, err
}

func (r *IdempotencyRepository) Get(ctx context.Context, key string) (domain.IdempotencyRecord, error) {
	payload, err := r.client.Get(ctx, redisIdempotencyKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyNotFound
	}
	if err != nil {
		return domain.IdempotencyRecord{}, fmt.Errorf("get idempotency key: %w", err)
	}
	var record domain.IdempotencyRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return domain.IdempotencyRecord{}, fmt.Errorf("decode idempotency record: %w", err)
	}
	if record.State != domain.IdempotencyProcessing && record.State != domain.IdempotencyCompleted {
		return domain.IdempotencyRecord{}, fmt.Errorf("unknown idempotency state %q", record.State)
	}
	return record, nil
}

func (r *IdempotencyRepository) Complete(ctx context.Context, key string, record domain.IdempotencyRecord, ttl time.Duration) error {
	if record.State != domain.IdempotencyCompleted || record.StatusCode == 0 || len(record.Body) == 0 {
		return fmt.Errorf("invalid completed idempotency record")
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal completed idempotency record: %w", err)
	}
	if err := r.client.Set(ctx, redisIdempotencyKey(key), payload, ttl).Err(); err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}

func (r *IdempotencyRepository) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, redisIdempotencyKey(key)).Err(); err != nil {
		return fmt.Errorf("delete idempotency key: %w", err)
	}
	return nil
}

func redisIdempotencyKey(key string) string { return idempotencyKeyPrefix + key }
