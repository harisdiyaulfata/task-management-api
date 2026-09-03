package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SessionRepository interface {
	Create(ctx context.Context, tokenID, userID uuid.UUID, ttl time.Duration) error
	IsActive(ctx context.Context, tokenID, userID uuid.UUID) (bool, error)
	Delete(ctx context.Context, tokenID uuid.UUID) error
}

type AccessTokenClaims struct {
	UserID  uuid.UUID
	TokenID uuid.UUID
}

type IdempotencyState string

const (
	IdempotencyProcessing IdempotencyState = "processing"
	IdempotencyCompleted  IdempotencyState = "completed"
)

// IdempotencyRecord contains the exact HTTP status and JSON body that must be
// returned for a repeated request. Body is intentionally transport-ready.
type IdempotencyRecord struct {
	State      IdempotencyState `json:"state"`
	StatusCode int              `json:"status_code,omitempty"`
	Body       []byte           `json:"body,omitempty"`
}

type IdempotencyRepository interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (record IdempotencyRecord, acquired bool, err error)
	Get(ctx context.Context, key string) (IdempotencyRecord, error)
	Complete(ctx context.Context, key string, record IdempotencyRecord, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
