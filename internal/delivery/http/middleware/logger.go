package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func StructuredLogger(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}

	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		status := c.Writer.Status()
		attributes := []slog.Attr{
			slog.String("request_id", requestID(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(startedAt)),
		}

		switch {
		case status >= 500:
			log.LogAttrs(c.Request.Context(), slog.LevelError, "HTTP request completed", attributes...)
		case status >= 400:
			log.LogAttrs(c.Request.Context(), slog.LevelWarn, "HTTP request completed", attributes...)
		default:
			log.LogAttrs(c.Request.Context(), slog.LevelInfo, "HTTP request completed", attributes...)
		}
	}
}

func requestID(c *gin.Context) string {
	if value, ok := c.Get(RequestIDKey); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}
