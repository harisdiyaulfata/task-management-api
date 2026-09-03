package usecase_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/usecase"
	"github.com/google/uuid"
)

func TestCreateIdempotent_ConcurrentRequestsCreateExactlyOneTask(t *testing.T) {
	const requests = 50
	tasks := &fakeTaskRepository{}
	keys := &fakeIdempotencyRepository{records: make(map[string]domain.IdempotencyRecord)}
	service := usecase.NewTaskUsecaseWithDependencies(tasks, keys, nil)
	ownerID, key := uuid.New(), uuid.New()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	results := make([]usecase.IdempotentCreateResult, requests)
	errs := make([]error, requests)
	var group sync.WaitGroup
	start := make(chan struct{})
	for i := range requests {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start // Force all callers to contend for the same key together.
			results[index], errs[index] = service.CreateIdempotent(ctx, ownerID, key, usecase.CreateTaskInput{Title: "Concurrency test"})
		}(i)
	}
	close(start)
	group.Wait()

	if created := tasks.creates.Load(); created != 1 {
		t.Fatalf("created tasks = %d, want 1", created)
	}
	freshResponses := 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d returned error: %v", i, err)
		}
		if string(results[i].Body) != string(results[0].Body) || results[i].StatusCode != results[0].StatusCode {
			t.Fatalf("request %d did not receive the original response", i)
		}
		if !results[i].Replayed {
			freshResponses++
		}
	}
	if freshResponses != 1 {
		t.Fatalf("fresh responses = %d, want 1", freshResponses)
	}
}

func TestCreateIdempotent_SequentialDuplicateReplaysOriginalResponse(t *testing.T) {
	tasks := &fakeTaskRepository{}
	keys := &fakeIdempotencyRepository{records: make(map[string]domain.IdempotencyRecord)}
	service := usecase.NewTaskUsecaseWithDependencies(tasks, keys, nil)
	ownerID, key := uuid.New(), uuid.New()
	input := usecase.CreateTaskInput{Title: "Sequential idempotency test"}

	first, err := service.CreateIdempotent(context.Background(), ownerID, key, input)
	if err != nil {
		t.Fatalf("first CreateIdempotent() error = %v", err)
	}
	second, err := service.CreateIdempotent(context.Background(), ownerID, key, input)
	if err != nil {
		t.Fatalf("second CreateIdempotent() error = %v", err)
	}

	if created := tasks.creates.Load(); created != 1 {
		t.Fatalf("created tasks = %d, want 1", created)
	}
	if first.Replayed {
		t.Fatal("first response must not be replayed")
	}
	if !second.Replayed {
		t.Fatal("second response must be replayed")
	}
	if first.StatusCode != second.StatusCode || string(first.Body) != string(second.Body) {
		t.Fatalf("second response differs from first: first=%d %s, second=%d %s", first.StatusCode, first.Body, second.StatusCode, second.Body)
	}
}

type fakeTaskRepository struct{ creates atomic.Int32 }

func (r *fakeTaskRepository) Create(_ context.Context, task domain.Task) (domain.Task, error) {
	r.creates.Add(1)
	time.Sleep(20 * time.Millisecond) // Ensures concurrent callers contend for the key.
	task.ID, task.CreatedAt, task.UpdatedAt = uuid.New(), time.Now().UTC(), time.Now().UTC()
	return task, nil
}
func (r *fakeTaskRepository) List(context.Context, uuid.UUID, domain.TaskFilter) ([]domain.Task, int, error) {
	return nil, 0, nil
}
func (r *fakeTaskRepository) FindByID(context.Context, uuid.UUID, uuid.UUID) (domain.Task, error) {
	return domain.Task{}, domain.ErrNotFound
}
func (r *fakeTaskRepository) Update(context.Context, uuid.UUID, uuid.UUID, domain.TaskUpdate) (domain.Task, error) {
	return domain.Task{}, domain.ErrNotFound
}
func (r *fakeTaskRepository) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type fakeIdempotencyRepository struct {
	mu      sync.Mutex
	records map[string]domain.IdempotencyRecord
}

func (r *fakeIdempotencyRepository) Acquire(_ context.Context, key string, _ time.Duration) (domain.IdempotencyRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if record, exists := r.records[key]; exists {
		return record, false, nil
	}
	record := domain.IdempotencyRecord{State: domain.IdempotencyProcessing}
	r.records[key] = record
	return record, true, nil
}
func (r *fakeIdempotencyRepository) Get(_ context.Context, key string) (domain.IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, exists := r.records[key]
	if !exists {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyNotFound
	}
	return record, nil
}
func (r *fakeIdempotencyRepository) Complete(_ context.Context, key string, record domain.IdempotencyRecord, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[key] = record
	return nil
}
func (r *fakeIdempotencyRepository) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.records, key)
	return nil
}
