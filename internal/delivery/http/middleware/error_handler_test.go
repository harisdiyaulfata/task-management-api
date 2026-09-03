package middleware_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/task-management-api/internal/delivery/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestErrorHandlerReturnsStandardError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ErrorHandler(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))))
	router.GET("/test", func(c *gin.Context) {
		middleware.AbortWithError(c, middleware.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "title is required", errors.New("missing title")))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	for _, field := range []string{"status", "code", "message", "timestamp"} {
		if !bytes.Contains(recorder.Body.Bytes(), []byte(`"`+field+`"`)) {
			t.Errorf("response missing %q: %s", field, recorder.Body.String())
		}
	}
}
