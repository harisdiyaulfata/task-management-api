package middleware

import (
	"strings"

	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/platform/jwt"
	"github.com/gin-gonic/gin"
)

const (
	AuthenticatedUserIDKey  = "authenticated_user_id"
	AuthenticatedTokenIDKey = "authenticated_token_id"
)

func Authenticate(tokens *jwt.Manager, sessions domain.SessionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(rawToken) == "" {
			AbortWithError(c, NewAppError(401, "UNAUTHORIZED", "authentication is required", nil))
			return
		}

		claims, err := tokens.Validate(rawToken)
		if err != nil {
			AbortWithError(c, NewAppError(401, "INVALID_TOKEN", "invalid or expired access token", err))
			return
		}
		active, err := sessions.IsActive(c.Request.Context(), claims.TokenID, claims.UserID)
		if err != nil {
			AbortWithError(c, NewAppError(500, "SESSION_CHECK_FAILED", "unable to validate session", err))
			return
		}
		if !active {
			AbortWithError(c, NewAppError(401, "SESSION_EXPIRED", "session is no longer active", nil))
			return
		}

		c.Set(AuthenticatedUserIDKey, claims.UserID)
		c.Set(AuthenticatedTokenIDKey, claims.TokenID)
		c.Next()
	}
}
