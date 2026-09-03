package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
)

func TestTaskRepositoryAssign_TransactionOutcome(t *testing.T) {
	ownerID, taskID, assigneeID := uuid.New(), uuid.New(), uuid.New()
	previousAssigneeID := uuid.New()
	before := domain.Task{ID: taskID, OwnerID: ownerID, AssigneeID: &previousAssigneeID, Title: "Task", Status: domain.TaskStatusTodo, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}

	tests := []struct {
		name             string
		sameTeam         bool
		execErrors       []error
		notificationErr  error
		wantCommit       bool
		wantRollback     bool
		wantNotifyCalls  int
		wantReturnedTask bool
	}{
		{name: "commits update log and notification", sameTeam: true, execErrors: []error{nil, nil}, wantCommit: true, wantNotifyCalls: 1, wantReturnedTask: true},
		{name: "rolls back when log insert fails", sameTeam: true, execErrors: []error{nil, errors.New("log insert failed")}, wantRollback: true},
		{name: "rolls back when notification fails", sameTeam: true, execErrors: []error{nil, nil}, notificationErr: errors.New("notification failed"), wantRollback: true, wantNotifyCalls: 1},
		{name: "rolls back when users are not in the same team", wantRollback: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &mockAssignmentTx{
				rows:       []rowScanner{boolRow(true), boolRow(test.sameTeam), taskRow(before)},
				execErrors: test.execErrors,
			}
			repository := &TaskRepository{assignments: mockAssignmentStore{tx: tx}}
			notifier := &mockNotifier{err: test.notificationErr}

			task, err := repository.Assign(context.Background(), domain.TaskAssignment{TaskID: taskID, OwnerID: ownerID, AssigneeID: assigneeID}, notifier)
			if (err == nil) != test.wantReturnedTask {
				t.Fatalf("Assign() error = %v, returned task = %+v", err, task)
			}
			if tx.commits != boolToInt(test.wantCommit) || tx.rollbacks != boolToInt(test.wantRollback) {
				t.Fatalf("commits=%d rollbacks=%d, want commits=%d rollbacks=%d", tx.commits, tx.rollbacks, boolToInt(test.wantCommit), boolToInt(test.wantRollback))
			}
			if notifier.calls != test.wantNotifyCalls {
				t.Fatalf("notification calls = %d, want %d", notifier.calls, test.wantNotifyCalls)
			}
			if test.wantReturnedTask && task.AssigneeID == nil || test.wantReturnedTask && *task.AssigneeID != assigneeID {
				t.Fatalf("assignee = %v, want %s", task.AssigneeID, assigneeID)
			}
		})
	}
}

type mockAssignmentStore struct{ tx assignmentTx }

func (s mockAssignmentStore) Begin(context.Context) (assignmentTx, error) { return s.tx, nil }

type mockAssignmentTx struct {
	rows       []rowScanner
	execErrors []error
	execCalls  int
	nextRow    int
	commits    int
	rollbacks  int
}

func (tx *mockAssignmentTx) QueryRow(_ context.Context, _ string, _ ...any) rowScanner {
	row := tx.rows[tx.nextRow]
	tx.nextRow++
	return row
}
func (tx *mockAssignmentTx) Exec(context.Context, string, ...any) error {
	if tx.execCalls >= len(tx.execErrors) {
		return nil
	}
	err := tx.execErrors[tx.execCalls]
	tx.execCalls++
	return err
}
func (tx *mockAssignmentTx) Commit(context.Context) error {
	tx.commits++
	return nil
}
func (tx *mockAssignmentTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

type testRow struct{ scan func(...any) error }

func (r testRow) Scan(destinations ...any) error { return r.scan(destinations...) }

func boolRow(value bool) rowScanner {
	return testRow{scan: func(destinations ...any) error {
		*destinations[0].(*bool) = value
		return nil
	}}
}

func taskRow(task domain.Task) rowScanner {
	return testRow{scan: func(destinations ...any) error {
		*destinations[0].(*uuid.UUID) = task.ID
		*destinations[1].(*uuid.UUID) = task.OwnerID
		*destinations[2].(**uuid.UUID) = task.AssigneeID
		*destinations[3].(*string) = task.Title
		*destinations[4].(*string) = task.Description
		*destinations[5].(*domain.TaskStatus) = task.Status
		*destinations[6].(**time.Time) = task.DueDate
		*destinations[7].(*time.Time) = task.CreatedAt
		*destinations[8].(*time.Time) = task.UpdatedAt
		return nil
	}}
}

type mockNotifier struct {
	err   error
	calls int
}

func (n *mockNotifier) NotifyTaskAssigned(context.Context, domain.Task, uuid.UUID) error {
	n.calls++
	return n.err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
