package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/task-management-api/internal/delivery/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestStructuredLogger_UsesExpectedLevelAndFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{name: "successful request is info", status: http.StatusOK, wantLevel: "INFO"},
		{name: "client error is warn", status: http.StatusBadRequest, wantLevel: "WARN"},
		{name: "server error is error", status: http.StatusInternalServerError, wantLevel: "ERROR"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&logs, nil))
			router := gin.New()
			router.Use(middleware.RequestID(), middleware.StructuredLogger(log))
			router.GET("/tasks", func(c *gin.Context) { c.Status(test.status) })

			requestID := uuid.NewString()
			request := httptest.NewRequest(http.MethodGet, "/tasks", nil)
			request.Header.Set("X-Request-ID", requestID)
			router.ServeHTTP(httptest.NewRecorder(), request)

			var entry map[string]any
			if err := json.NewDecoder(&logs).Decode(&entry); err != nil {
				t.Fatalf("decode log entry: %v; output=%s", err, logs.String())
			}
			if entry["level"] != test.wantLevel || entry["request_id"] != requestID || entry["method"] != http.MethodGet || entry["path"] != "/tasks" {
				t.Fatalf("unexpected structured log: %#v", entry)
			}
			if entry["status"] != float64(test.status) || entry["latency"] == nil {
				t.Fatalf("missing status or latency: %#v", entry)
			}
		})
	}
}
