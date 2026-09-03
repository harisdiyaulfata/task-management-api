package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type TokenManager interface {
	Generate(userID uuid.UUID) (token string, tokenID uuid.UUID, expiresAt time.Time, err error)
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	User      domain.User `json:"user"`
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
}

type AuthUsecase struct {
	users    domain.UserRepository
	sessions domain.SessionRepository
	tokens   TokenManager
}

func NewAuthUsecase(users domain.UserRepository, sessions domain.SessionRepository, tokens TokenManager) *AuthUsecase {
	return &AuthUsecase{users: users, sessions: sessions, tokens: tokens}
}

func (u *AuthUsecase) Register(ctx context.Context, input RegisterInput) (domain.User, error) {
	name, email, password, err := normalizeRegistration(input)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}
	return u.users.Create(ctx, domain.User{Name: name, Email: email, PasswordHash: string(hash)})
}

func (u *AuthUsecase) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Password == "" {
		return AuthResult{}, domain.ErrInvalidCredentials
	}
	user, err := u.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AuthResult{}, domain.ErrInvalidCredentials
		}
		return AuthResult{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return AuthResult{}, domain.ErrInvalidCredentials
	}

	token, tokenID, expiresAt, err := u.tokens.Generate(user.ID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate access token: %w", err)
	}
	if err := u.sessions.Create(ctx, tokenID, user.ID, time.Until(expiresAt)); err != nil {
		return AuthResult{}, err
	}
	return AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (u *AuthUsecase) Logout(ctx context.Context, tokenID uuid.UUID) error {
	if tokenID == uuid.Nil {
		return domain.ErrInvalidInput
	}
	return u.sessions.Delete(ctx, tokenID)
}

func normalizeRegistration(input RegisterInput) (name, email, password string, err error) {
	name = strings.TrimSpace(input.Name)
	email = strings.ToLower(strings.TrimSpace(input.Email))
	password = input.Password
	parsedEmail, parseErr := mail.ParseAddress(email)
	if name == "" || len(name) > 100 || parseErr != nil || parsedEmail.Address != email || len(email) > 255 || len(password) < 8 {
		return "", "", "", domain.ErrInvalidInput
	}
	return name, email, password, nil
}
