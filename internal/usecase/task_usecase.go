package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
)

const (
	defaultTaskPageSize = 20
	maxTaskPageSize     = 100
	idempotencyTTL      = 24 * time.Hour
)

type CreateTaskInput struct {
	Title       string
	Description string
	Status      domain.TaskStatus
	DueDate     *time.Time
}

type TasksPage struct {
	Items []domain.Task `json:"items"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

type IdempotentCreateResult struct {
	Task       domain.Task
	StatusCode int
	Body       []byte
	Replayed   bool
}

type TaskUsecase struct {
	tasks       domain.TaskRepository
	assignments domain.TaskAssignmentRepository
	idempotency domain.IdempotencyRepository
	notifier    domain.NotificationSender
}

func NewTaskUsecase(tasks domain.TaskRepository) *TaskUsecase {
	assignments, _ := tasks.(domain.TaskAssignmentRepository)
	return &TaskUsecase{tasks: tasks, assignments: assignments}
}

func NewTaskUsecaseWithDependencies(
	tasks domain.TaskRepository,
	idempotency domain.IdempotencyRepository,
	notifier domain.NotificationSender,
) *TaskUsecase {
	assignments, _ := tasks.(domain.TaskAssignmentRepository)
	return &TaskUsecase{tasks: tasks, assignments: assignments, idempotency: idempotency, notifier: notifier}
}

func (u *TaskUsecase) Create(ctx context.Context, ownerID uuid.UUID, input CreateTaskInput) (domain.Task, error) {
	if ownerID == uuid.Nil {
		return domain.Task{}, domain.ErrInvalidInput
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || len(title) > 255 {
		return domain.Task{}, domain.ErrInvalidInput
	}
	status := input.Status
	if status == "" {
		status = domain.TaskStatusTodo
	}
	if !status.Valid() {
		return domain.Task{}, domain.ErrInvalidInput
	}
	return u.tasks.Create(ctx, domain.Task{
		OwnerID: ownerID, Title: title, Description: input.Description, Status: status, DueDate: input.DueDate,
	})
}

// CreateIdempotent makes POST /tasks safe to retry. The returned Body is the
// exact JSON representation stored in Redis; handlers should write it directly
// with StatusCode so repeated calls receive byte-for-byte identical responses.
func (u *TaskUsecase) CreateIdempotent(ctx context.Context, ownerID, idempotencyKey uuid.UUID, input CreateTaskInput) (IdempotentCreateResult, error) {
	if idempotencyKey == uuid.Nil || u.idempotency == nil {
		return IdempotentCreateResult{}, domain.ErrInvalidInput
	}
	key := ownerID.String() + ":" + idempotencyKey.String()

	for {
		record, acquired, err := u.idempotency.Acquire(ctx, key, idempotencyTTL)
		if errors.Is(err, domain.ErrIdempotencyNotFound) {
			continue // A failed original request released its processing key.
		}
		if err != nil {
			return IdempotentCreateResult{}, err
		}
		if acquired {
			return u.createAndStoreIdempotent(ctx, ownerID, key, input)
		}
		if record.State == domain.IdempotencyCompleted {
			return idempotentResultFromRecord(record)
		}

		record, err = u.waitForIdempotencyCompletion(ctx, key)
		if errors.Is(err, domain.ErrIdempotencyNotFound) {
			continue
		}
		if err != nil {
			return IdempotentCreateResult{}, err
		}
		return idempotentResultFromRecord(record)
	}
}

func (u *TaskUsecase) createAndStoreIdempotent(ctx context.Context, ownerID uuid.UUID, key string, input CreateTaskInput) (IdempotentCreateResult, error) {
	task, err := u.Create(ctx, ownerID, input)
	if err != nil {
		// No task was persisted, so a retry may become the owner of this key.
		if deleteErr := u.idempotency.Delete(ctx, key); deleteErr != nil {
			return IdempotentCreateResult{}, fmt.Errorf("create task: %w (release idempotency key: %v)", err, deleteErr)
		}
		return IdempotentCreateResult{}, err
	}
	body, err := json.Marshal(task)
	if err != nil {
		return IdempotentCreateResult{}, fmt.Errorf("marshal created task response: %w", err)
	}
	record := domain.IdempotencyRecord{State: domain.IdempotencyCompleted, StatusCode: 201, Body: body}
	if err := u.idempotency.Complete(ctx, key, record, idempotencyTTL); err != nil {
		// Keep the processing key in place: allowing another creator here could
		// duplicate the already-persisted task when Redis is temporarily failing.
		return IdempotentCreateResult{}, err
	}
	return IdempotentCreateResult{Task: task, StatusCode: record.StatusCode, Body: body}, nil
}

func (u *TaskUsecase) waitForIdempotencyCompletion(ctx context.Context, key string) (domain.IdempotencyRecord, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return domain.IdempotencyRecord{}, ctx.Err()
		case <-ticker.C:
			record, err := u.idempotency.Get(ctx, key)
			if err != nil {
				return domain.IdempotencyRecord{}, err
			}
			if record.State == domain.IdempotencyCompleted {
				return record, nil
			}
		}
	}
}

func idempotentResultFromRecord(record domain.IdempotencyRecord) (IdempotentCreateResult, error) {
	if record.StatusCode == 0 || len(record.Body) == 0 {
		return IdempotentCreateResult{}, fmt.Errorf("invalid stored idempotency response")
	}
	var task domain.Task
	if err := json.Unmarshal(record.Body, &task); err != nil {
		return IdempotentCreateResult{}, fmt.Errorf("decode stored idempotency response: %w", err)
	}
	return IdempotentCreateResult{Task: task, StatusCode: record.StatusCode, Body: record.Body, Replayed: true}, nil
}

func (u *TaskUsecase) List(ctx context.Context, ownerID uuid.UUID, filter domain.TaskFilter) (TasksPage, error) {
	if ownerID == uuid.Nil {
		return TasksPage{}, domain.ErrInvalidInput
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultTaskPageSize
	}
	if filter.Limit > maxTaskPageSize {
		filter.Limit = maxTaskPageSize
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Status != nil && !filter.Status.Valid() {
		return TasksPage{}, domain.ErrInvalidInput
	}
	tasks, total, err := u.tasks.List(ctx, ownerID, filter)
	if err != nil {
		return TasksPage{}, err
	}
	return TasksPage{Items: tasks, Total: total, Page: filter.Page, Limit: filter.Limit}, nil
}

func (u *TaskUsecase) GetByID(ctx context.Context, ownerID, taskID uuid.UUID) (domain.Task, error) {
	if ownerID == uuid.Nil || taskID == uuid.Nil {
		return domain.Task{}, domain.ErrInvalidInput
	}
	return u.tasks.FindByID(ctx, ownerID, taskID)
}

func (u *TaskUsecase) Update(ctx context.Context, ownerID, taskID uuid.UUID, update domain.TaskUpdate) (domain.Task, error) {
	if ownerID == uuid.Nil || taskID == uuid.Nil {
		return domain.Task{}, domain.ErrInvalidInput
	}
	if update.Title != nil {
		title := strings.TrimSpace(*update.Title)
		if title == "" || len(title) > 255 {
			return domain.Task{}, domain.ErrInvalidInput
		}
		update.Title = &title
	}
	if update.Status != nil && !update.Status.Valid() {
		return domain.Task{}, domain.ErrInvalidInput
	}
	return u.tasks.Update(ctx, ownerID, taskID, update)
}

func (u *TaskUsecase) Delete(ctx context.Context, ownerID, taskID uuid.UUID) error {
	if ownerID == uuid.Nil || taskID == uuid.Nil {
		return domain.ErrInvalidInput
	}
	return u.tasks.Delete(ctx, ownerID, taskID)
}

func (u *TaskUsecase) Assign(ctx context.Context, ownerID, taskID, assigneeID uuid.UUID) (domain.Task, error) {
	if ownerID == uuid.Nil || taskID == uuid.Nil || assigneeID == uuid.Nil || ownerID == assigneeID {
		return domain.Task{}, domain.ErrInvalidInput
	}
	if u.assignments == nil {
		return domain.Task{}, fmt.Errorf("task assignment repository is not configured")
	}
	return u.assignments.Assign(ctx, domain.TaskAssignment{
		TaskID: taskID, OwnerID: ownerID, AssigneeID: assigneeID,
	}, u.notifier)
}
