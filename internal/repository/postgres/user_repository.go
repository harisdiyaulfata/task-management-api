package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct{ pool *pgxpool.Pool }

func NewUserRepository(pool *pgxpool.Pool) *UserRepository { return &UserRepository{pool: pool} }

func (r *UserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	const query = `WITH created_user AS (
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, public_id, name, email, password_hash, created_at, updated_at
	), default_membership AS (
		INSERT INTO team_members (team_id, user_id)
		SELECT teams.id, created_user.id
		FROM teams
		CROSS JOIN created_user
		WHERE teams.name = 'Default Team'
		ON CONFLICT DO NOTHING
	)
	SELECT public_id, name, email, password_hash, created_at, updated_at FROM created_user`
	err := r.pool.QueryRow(ctx, query, user.Name, user.Email, user.PasswordHash).Scan(
		&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, domain.ErrAlreadyExists
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	const query = `SELECT public_id, name, email, password_hash, created_at, updated_at FROM users WHERE email = $1`
	return scanUser(r.pool.QueryRow(ctx, query, email))
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const query = `SELECT public_id, name, email, password_hash, created_at, updated_at FROM users WHERE public_id = $1`
	return scanUser(r.pool.QueryRow(ctx, query, id))
}

type rowScanner interface{ Scan(...any) error }

func scanUser(row rowScanner) (domain.User, error) {
	var user domain.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
