package usecase_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/usecase"
	"github.com/google/uuid"
)

func TestTaskUsecaseList_NormalizesPaginationAndFilters(t *testing.T) {
	ownerID := uuid.New()
	done := domain.TaskStatusDone
	invalid := domain.TaskStatus("blocked")
	tests := []struct {
		name       string
		ownerID    uuid.UUID
		input      domain.TaskFilter
		wantFilter domain.TaskFilter
		wantErr    error
	}{
		{
			name:       "applies defaults and trims search",
			ownerID:    ownerID,
			input:      domain.TaskFilter{Search: "  report  "},
			wantFilter: domain.TaskFilter{Page: 1, Limit: 20, Search: "report"},
		},
		{
			name:       "keeps explicit pagination and status filter",
			ownerID:    ownerID,
			input:      domain.TaskFilter{Page: 3, Limit: 5, Search: "invoice", Status: &done},
			wantFilter: domain.TaskFilter{Page: 3, Limit: 5, Search: "invoice", Status: &done},
		},
		{
			name:       "caps page size",
			ownerID:    ownerID,
			input:      domain.TaskFilter{Page: 2, Limit: 999},
			wantFilter: domain.TaskFilter{Page: 2, Limit: 100},
		},
		{
			name:    "rejects unsupported status",
			ownerID: ownerID,
			input:   domain.TaskFilter{Status: &invalid},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name:    "rejects missing owner",
			input:   domain.TaskFilter{},
			wantErr: domain.ErrInvalidInput,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &listTaskRepository{tasks: []domain.Task{{ID: uuid.New(), OwnerID: ownerID, Title: "Report"}}, total: 1}
			service := usecase.NewTaskUsecase(repository)

			result, err := service.List(context.Background(), test.ownerID, test.input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("List() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				if repository.listCalls != 0 {
					t.Fatalf("repository List calls = %d, want 0", repository.listCalls)
				}
				return
			}
			if !reflect.DeepEqual(repository.receivedFilter, test.wantFilter) {
				t.Fatalf("repository filter = %+v, want %+v", repository.receivedFilter, test.wantFilter)
			}
			if result.Page != test.wantFilter.Page || result.Limit != test.wantFilter.Limit || result.Total != 1 || len(result.Items) != 1 {
				t.Fatalf("unexpected page result: %+v", result)
			}
		})
	}
}

type listTaskRepository struct {
	tasks          []domain.Task
	total          int
	listCalls      int
	receivedFilter domain.TaskFilter
}

func (r *listTaskRepository) Create(context.Context, domain.Task) (domain.Task, error) {
	return domain.Task{}, nil
}
func (r *listTaskRepository) List(_ context.Context, _ uuid.UUID, filter domain.TaskFilter) ([]domain.Task, int, error) {
	r.listCalls++
	r.receivedFilter = filter
	return r.tasks, r.total, nil
}
func (r *listTaskRepository) FindByID(context.Context, uuid.UUID, uuid.UUID) (domain.Task, error) {
	return domain.Task{}, nil
}
func (r *listTaskRepository) Update(context.Context, uuid.UUID, uuid.UUID, domain.TaskUpdate) (domain.Task, error) {
	return domain.Task{}, nil
}
func (r *listTaskRepository) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
