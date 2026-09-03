package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/task-management-api/internal/delivery/http/response"
	"github.com/gin-gonic/gin"
)

// AppError is the only error type handlers and use cases need to return for a
// known API failure. Its cause is deliberately never serialized to clients.
type AppError struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (e *AppError) Error() string { return e.Message }

func (e *AppError) Unwrap() error { return e.Cause }

func NewAppError(status int, code, message string, cause error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Cause: cause}
}

func ErrorHandler(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered",
					slog.String("request_id", requestID(c)),
					slog.Any("panic", recovered),
				)
				writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
			}
		}()

		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		apiError := toAppError(c.Errors.Last().Err)
		writeError(c, apiError.Status, apiError.Code, apiError.Message)
	}
}

// AbortWithError records a failure for ErrorHandler and stops the handler chain.
func AbortWithError(c *gin.Context, err error) {
	c.Error(err) //nolint:errcheck // the error is intentionally consumed by ErrorHandler.
	c.Abort()
}

func toAppError(err error) *AppError {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError
	}
	return NewAppError(http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred", err)
}

func writeError(c *gin.Context, status int, code, message string) {
	if c.Writer.Written() {
		return
	}
	c.AbortWithStatusJSON(status, response.Error{
		Status:    status,
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC(),
	})
}
