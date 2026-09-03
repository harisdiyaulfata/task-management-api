package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/usecase"
	"github.com/google/uuid"
)

func TestTaskUsecaseAssign_DelegatesTransactionalAssignment(t *testing.T) {
	ownerID, taskID, assigneeID := uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name      string
		ownerID   uuid.UUID
		assignee  uuid.UUID
		repoErr   error
		wantErr   error
		wantCalls int
	}{
		{name: "success", ownerID: ownerID, assignee: assigneeID, wantCalls: 1},
		{name: "transaction failure is returned", ownerID: ownerID, assignee: assigneeID, repoErr: errors.New("notification failed"), wantCalls: 1},
		{name: "same owner and assignee is rejected", ownerID: ownerID, assignee: ownerID, wantErr: domain.ErrInvalidInput},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &assignmentUsecaseRepository{assignErr: test.repoErr}
			service := usecase.NewTaskUsecaseWithDependencies(repository, nil, nil)

			_, err := service.Assign(context.Background(), test.ownerID, taskID, test.assignee)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Assign() error = %v, want %v", err, test.wantErr)
				}
			} else if test.repoErr != nil {
				if !errors.Is(err, test.repoErr) {
					t.Fatalf("Assign() error = %v, want repository error %v", err, test.repoErr)
				}
			} else if err != nil {
				t.Fatalf("Assign() unexpected error = %v", err)
			}
			if repository.assignCalls != test.wantCalls {
				t.Fatalf("repository Assign calls = %d, want %d", repository.assignCalls, test.wantCalls)
			}
			if test.wantCalls == 1 && (repository.assignment.TaskID != taskID || repository.assignment.OwnerID != ownerID || repository.assignment.AssigneeID != assigneeID) {
				t.Fatalf("unexpected assignment: %+v", repository.assignment)
			}
		})
	}
}

type assignmentUsecaseRepository struct {
	assignment  domain.TaskAssignment
	assignCalls int
	assignErr   error
}

func (r *assignmentUsecaseRepository) Create(context.Context, domain.Task) (domain.Task, error) {
	return domain.Task{}, nil
}
func (r *assignmentUsecaseRepository) List(context.Context, uuid.UUID, domain.TaskFilter) ([]domain.Task, int, error) {
	return nil, 0, nil
}
func (r *assignmentUsecaseRepository) FindByID(context.Context, uuid.UUID, uuid.UUID) (domain.Task, error) {
	return domain.Task{}, nil
}
func (r *assignmentUsecaseRepository) Update(context.Context, uuid.UUID, uuid.UUID, domain.TaskUpdate) (domain.Task, error) {
	return domain.Task{}, nil
}
func (r *assignmentUsecaseRepository) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *assignmentUsecaseRepository) Assign(_ context.Context, assignment domain.TaskAssignment, _ domain.NotificationSender) (domain.Task, error) {
	r.assignCalls++
	r.assignment = assignment
	if r.assignErr != nil {
		return domain.Task{}, r.assignErr
	}
	return domain.Task{ID: assignment.TaskID, OwnerID: assignment.OwnerID, AssigneeID: &assignment.AssigneeID}, nil
}
