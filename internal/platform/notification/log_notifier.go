package notification

import (
	"context"
	"log/slog"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
)

// LogNotifier is the current mock notification implementation. Replacing it
// with email, push, or queue delivery requires no changes to the use case.
type LogNotifier struct{ log *slog.Logger }

func NewLogNotifier(log *slog.Logger) *LogNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &LogNotifier{log: log}
}

func (n *LogNotifier) NotifyTaskAssigned(_ context.Context, task domain.Task, assigneeID uuid.UUID) error {
	n.log.Info("mock task-assignment notification", slog.String("task_id", task.ID.String()), slog.String("assignee_id", assigneeID.String()))
	return nil
}
