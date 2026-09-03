package jwt

import (
	"fmt"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Manager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if secret == "" {
		return nil, fmt.Errorf("JWT secret is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("JWT TTL must be positive")
	}
	return &Manager{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

func (m *Manager) Generate(userID uuid.UUID) (token string, tokenID uuid.UUID, expiresAt time.Time, err error) {
	tokenID = uuid.New()
	expiresAt = m.now().UTC().Add(m.ttl)
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID.String(),
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(m.now().UTC()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return token, tokenID, expiresAt, err
}

func (m *Manager) Validate(rawToken string) (domain.AccessTokenClaims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected JWT signing method: %s", token.Method.Alg())
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return domain.AccessTokenClaims{}, fmt.Errorf("validate JWT: %w", err)
	}
	if claims.UserID == uuid.Nil {
		return domain.AccessTokenClaims{}, fmt.Errorf("JWT user_id is required")
	}
	tokenID, err := uuid.Parse(claims.ID)
	if err != nil {
		return domain.AccessTokenClaims{}, fmt.Errorf("JWT jti is invalid: %w", err)
	}
	return domain.AccessTokenClaims{UserID: claims.UserID, TokenID: tokenID}, nil
}
