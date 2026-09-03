package handler

import (
	"errors"
	"net/http"

	"github.com/example/task-management-api/internal/delivery/http/middleware"
	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct{ usecase *usecase.AuthUsecase }

func NewAuthHandler(usecase *usecase.AuthUsecase) *AuthHandler { return &AuthHandler{usecase: usecase} }

type registerRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	user, err := h.usecase.Register(c.Request.Context(), usecase.RegisterInput(request))
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortInvalidJSON(c, err)
		return
	}
	result, err := h.usecase.Login(c.Request.Context(), usecase.LoginInput(request))
	if err != nil {
		abortDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	tokenID, ok := c.Get(middleware.AuthenticatedTokenIDKey)
	if !ok {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication is required", nil))
		return
	}
	if err := h.usecase.Logout(c.Request.Context(), tokenID.(uuid.UUID)); err != nil {
		abortDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func abortInvalidJSON(c *gin.Context, err error) {
	middleware.AbortWithError(c, middleware.NewAppError(http.StatusBadRequest, "INVALID_REQUEST", "request body is invalid", err))
}

func abortDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "request contains invalid data", err))
	case errors.Is(err, domain.ErrAlreadyExists):
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusConflict, "ALREADY_EXISTS", "resource already exists", err))
	case errors.Is(err, domain.ErrInvalidCredentials):
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect", err))
	case errors.Is(err, domain.ErrForbidden):
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusForbidden, "FORBIDDEN", "you are not allowed to perform this action", err))
	case errors.Is(err, domain.ErrNotFound):
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusNotFound, "NOT_FOUND", "resource not found", err))
	default:
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred", err))
	}
}
