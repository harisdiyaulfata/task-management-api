package logger

import (
	"io"
	"log/slog"
	"os"
)

func NewJSON(output io.Writer) *slog.Logger {
	if output == nil {
		output = os.Stdout
	}
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
