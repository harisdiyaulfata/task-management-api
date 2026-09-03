package configs

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	HTTPPort       string
	MigrationsPath string
	Postgres       PostgresConfig
	Redis          RedisConfig
	JWT            JWTConfig
}

type PostgresConfig struct {
	DSN      string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

func Load() (Config, error) {
	// .env is optional in production, where environment variables are normally
	// injected by the platform. Existing environment variables take precedence.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	ttl, err := time.ParseDuration(env("JWT_TTL", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse JWT_TTL: %w", err)
	}

	maxConns, err := int32Env("POSTGRES_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	minConns, err := int32Env("POSTGRES_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := intEnv("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:         env("APP_ENV", "development"),
		HTTPPort:       env("HTTP_PORT", "8080"),
		MigrationsPath: env("MIGRATIONS_PATH", "db/migrations"),
		Postgres:       PostgresConfig{DSN: os.Getenv("POSTGRES_DSN"), MaxConns: maxConns, MinConns: minConns},
		Redis:          RedisConfig{Addr: env("REDIS_ADDR", "localhost:6379"), Password: os.Getenv("REDIS_PASSWORD"), DB: redisDB},
		JWT:            JWTConfig{Secret: os.Getenv("JWT_SECRET"), TTL: ttl},
	}

	if cfg.Postgres.DSN == "" {
		return Config{}, fmt.Errorf("POSTGRES_DSN is required")
	}
	if cfg.JWT.Secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.Postgres.MinConns < 0 || cfg.Postgres.MaxConns < cfg.Postgres.MinConns {
		return Config{}, fmt.Errorf("invalid PostgreSQL pool limits")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func int32Env(key string, fallback int32) (int32, error) {
	value, err := intEnv(key, int(fallback))
	if err != nil {
		return 0, err
	}
	return int32(value), nil
}
