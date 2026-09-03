package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	return s == TaskStatusTodo || s == TaskStatusInProgress || s == TaskStatusDone
}

type Task struct {
	ID          uuid.UUID  `json:"id"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TaskFilter struct {
	Page   int         `json:"page"`
	Limit  int         `json:"limit"`
	Search string      `json:"search"`
	Status *TaskStatus `json:"status,omitempty"`
}

type TaskUpdate struct {
	Title       *string     `json:"title,omitempty"`
	Description *string     `json:"description,omitempty"`
	Status      *TaskStatus `json:"status,omitempty"`
	DueDate     *time.Time  `json:"due_date,omitempty"`
}

type TaskRepository interface {
	Create(ctx context.Context, task Task) (Task, error)
	List(ctx context.Context, ownerID uuid.UUID, filter TaskFilter) ([]Task, int, error)
	FindByID(ctx context.Context, ownerID, taskID uuid.UUID) (Task, error)
	Update(ctx context.Context, ownerID, taskID uuid.UUID, update TaskUpdate) (Task, error)
	Delete(ctx context.Context, ownerID, taskID uuid.UUID) error
}

type TaskAssignment struct {
	TaskID     uuid.UUID
	OwnerID    uuid.UUID
	AssigneeID uuid.UUID
}

// NotificationSender remains an interface so tests can force a notification
// failure and verify that the database transaction is rolled back.
type NotificationSender interface {
	NotifyTaskAssigned(ctx context.Context, task Task, assigneeID uuid.UUID) error
}

type TaskAssignmentRepository interface {
	Assign(ctx context.Context, assignment TaskAssignment, notifier NotificationSender) (Task, error)
}
