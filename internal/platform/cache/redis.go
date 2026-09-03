package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/example/task-management-api/configs"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and verifies the shared Redis client.
// The caller should defer client.Close() during application shutdown.
func NewRedisClient(ctx context.Context, cfg configs.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return client, nil
}
