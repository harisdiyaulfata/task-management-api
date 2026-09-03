package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The database uses BIGINT primary and foreign keys. This repository only
// exposes public UUIDs to the domain/API and resolves them at the SQL boundary.
type TaskRepository struct {
	pool        *pgxpool.Pool
	assignments assignmentStore
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool, assignments: pgAssignmentStore{pool: pool}}
}

type assignmentStore interface {
	Begin(ctx context.Context) (assignmentTx, error)
}

type assignmentTx interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Exec(ctx context.Context, sql string, args ...any) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type pgAssignmentStore struct{ pool *pgxpool.Pool }

func (s pgAssignmentStore) Begin(ctx context.Context) (assignmentTx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgAssignmentTx{Tx: tx}, nil
}

type pgAssignmentTx struct{ pgx.Tx }

func (tx pgAssignmentTx) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx pgAssignmentTx) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := tx.Tx.Exec(ctx, sql, args...)
	return err
}

func (r *TaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	const query = `INSERT INTO tasks (owner_id, title, description, status, due_date)
		VALUES ((SELECT id FROM users WHERE public_id = $1), $2, $3, $4, $5)
		RETURNING public_id`
	var taskID uuid.UUID
	if err := r.pool.QueryRow(ctx, query, task.OwnerID, task.Title, task.Description, task.Status, task.DueDate).Scan(&taskID); err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	return r.FindByID(ctx, task.OwnerID, taskID)
}

func (r *TaskRepository) Assign(ctx context.Context, assignment domain.TaskAssignment, notifier domain.NotificationSender) (task domain.Task, err error) {
	tx, err := r.assignments.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin task assignment transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var assigneeExists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE public_id = $1)`, assignment.AssigneeID).Scan(&assigneeExists); err != nil {
		return domain.Task{}, fmt.Errorf("check assignee: %w", err)
	}
	if !assigneeExists {
		return domain.Task{}, domain.ErrNotFound
	}

	var sameTeam bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1
		FROM team_members owner_members
		JOIN team_members assignee_members ON assignee_members.team_id = owner_members.team_id
		WHERE owner_members.user_id = (SELECT id FROM users WHERE public_id = $1)
		  AND assignee_members.user_id = (SELECT id FROM users WHERE public_id = $2)
	)`, assignment.OwnerID, assignment.AssigneeID).Scan(&sameTeam); err != nil {
		return domain.Task{}, fmt.Errorf("check team membership: %w", err)
	}
	if !sameTeam {
		return domain.Task{}, domain.ErrForbidden
	}

	task, err = scanTask(tx.QueryRow(ctx, `SELECT t.public_id, owner.public_id, assignee.public_id, t.title, t.description, t.status, t.due_date, t.created_at, t.updated_at
		FROM tasks t
		JOIN users owner ON owner.id = t.owner_id
		LEFT JOIN users assignee ON assignee.id = t.assignee_id
		WHERE t.public_id = $1 AND owner.public_id = $2 FOR UPDATE OF t`, assignment.TaskID, assignment.OwnerID))
	if err != nil {
		return domain.Task{}, err
	}
	oldAssigneeID := task.AssigneeID

	if err = tx.Exec(ctx, `UPDATE tasks SET assignee_id = (SELECT id FROM users WHERE public_id = $1), updated_at = NOW()
		WHERE public_id = $2 AND owner_id = (SELECT id FROM users WHERE public_id = $3)`,
		assignment.AssigneeID, assignment.TaskID, assignment.OwnerID); err != nil {
		return domain.Task{}, fmt.Errorf("update assignee: %w", err)
	}
	task.AssigneeID = &assignment.AssigneeID
	task.UpdatedAt = time.Now().UTC()

	oldValue, err := json.Marshal(map[string]*uuid.UUID{"assignee_id": oldAssigneeID})
	if err != nil {
		return domain.Task{}, fmt.Errorf("marshal assignment log old value: %w", err)
	}
	newAssigneeID := assignment.AssigneeID
	newValue, err := json.Marshal(map[string]*uuid.UUID{"assignee_id": &newAssigneeID})
	if err != nil {
		return domain.Task{}, fmt.Errorf("marshal assignment log new value: %w", err)
	}
	if err = tx.Exec(ctx, `INSERT INTO task_logs (task_id, actor_id, action, old_value, new_value)
		VALUES ((SELECT id FROM tasks WHERE public_id = $1), (SELECT id FROM users WHERE public_id = $2), $3, $4, $5)`,
		task.ID, assignment.OwnerID, "assigned", oldValue, newValue); err != nil {
		return domain.Task{}, fmt.Errorf("insert assignment task log: %w", err)
	}
	if notifier != nil {
		if err = notifier.NotifyTaskAssigned(ctx, task, assignment.AssigneeID); err != nil {
			return domain.Task{}, fmt.Errorf("send assignment notification: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit task assignment transaction: %w", err)
	}
	return task, nil
}

func (r *TaskRepository) List(ctx context.Context, ownerID uuid.UUID, filter domain.TaskFilter) ([]domain.Task, int, error) {
	where, args := taskListWhere(ownerID, filter)
	countQuery := `SELECT COUNT(*) FROM tasks t JOIN users owner ON owner.id = t.owner_id` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	query := `SELECT t.public_id, owner.public_id, assignee.public_id, t.title, t.description, t.status, t.due_date, t.created_at, t.updated_at
		FROM tasks t
		JOIN users owner ON owner.id = t.owner_id
		LEFT JOIN users assignee ON assignee.id = t.assignee_id` + where + fmt.Sprintf(" ORDER BY t.created_at DESC, t.id DESC LIMIT $%d OFFSET $%d", limitPosition, offsetPosition)
	args = append(args, filter.Limit, (filter.Page-1)*filter.Limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, total, nil
}

func (r *TaskRepository) FindByID(ctx context.Context, ownerID, taskID uuid.UUID) (domain.Task, error) {
	const query = `SELECT t.public_id, owner.public_id, assignee.public_id, t.title, t.description, t.status, t.due_date, t.created_at, t.updated_at
		FROM tasks t
		JOIN users owner ON owner.id = t.owner_id
		LEFT JOIN users assignee ON assignee.id = t.assignee_id
		WHERE t.public_id = $1 AND owner.public_id = $2`
	return scanTask(r.pool.QueryRow(ctx, query, taskID, ownerID))
}

func (r *TaskRepository) Update(ctx context.Context, ownerID, taskID uuid.UUID, update domain.TaskUpdate) (domain.Task, error) {
	sets := make([]string, 0, 4)
	args := make([]any, 0, 6)
	add := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if update.Title != nil {
		add("title", *update.Title)
	}
	if update.Description != nil {
		add("description", *update.Description)
	}
	if update.Status != nil {
		add("status", *update.Status)
	}
	if update.DueDate != nil {
		add("due_date", *update.DueDate)
	}
	if len(sets) == 0 {
		return r.FindByID(ctx, ownerID, taskID)
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, taskID, ownerID)
	query := `UPDATE tasks SET ` + strings.Join(sets, ", ") + fmt.Sprintf(
		` WHERE public_id = $%d AND owner_id = (SELECT id FROM users WHERE public_id = $%d)
		RETURNING public_id`, len(args)-1, len(args),
	)
	var updatedID uuid.UUID
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&updatedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	return r.FindByID(ctx, ownerID, updatedID)
}

func (r *TaskRepository) Delete(ctx context.Context, ownerID, taskID uuid.UUID) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE public_id = $1 AND owner_id = (SELECT id FROM users WHERE public_id = $2)`, taskID, ownerID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func taskListWhere(ownerID uuid.UUID, filter domain.TaskFilter) (string, []any) {
	conditions := []string{"owner.public_id = $1"}
	args := []any{ownerID}
	if filter.Status != nil {
		args = append(args, *filter.Status)
		conditions = append(conditions, fmt.Sprintf("t.status = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		conditions = append(conditions, fmt.Sprintf("t.title ILIKE $%d", len(args)))
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func scanTask(row rowScanner) (domain.Task, error) {
	var task domain.Task
	if err := row.Scan(
		&task.ID, &task.OwnerID, &task.AssigneeID, &task.Title, &task.Description,
		&task.Status, &task.DueDate, &task.CreatedAt, &task.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("scan task: %w", err)
	}
	return task, nil
}
