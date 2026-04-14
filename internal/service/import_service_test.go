package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"async-import-service/internal/model"
	"async-import-service/internal/repository"
)

type mockTaskRepo struct {
	createdTask  model.Task
	createErr    error
	getTask      model.Task
	getErr       error
	listTasks    []model.Task
	listErr      error
	updateCalled bool
}

func (m *mockTaskRepo) Create(ctx context.Context, taskType string, payload model.ImportPayload) (model.Task, error) {
	return m.createdTask, m.createErr
}

func (m *mockTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (model.Task, error) {
	return m.getTask, m.getErr
}

func (m *mockTaskRepo) List(ctx context.Context) ([]model.Task, error) {
	return m.listTasks, m.listErr
}

func (m *mockTaskRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus, result *model.ImportResult, errorMessage *string) error {
	m.updateCalled = true
	return nil
}

type mockPublisher struct {
	publishErr error
	published  []model.TaskMessage
}

func (m *mockPublisher) PublishTask(ctx context.Context, msg model.TaskMessage) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, msg)
	return nil
}

func TestCreateImportTask(t *testing.T) {
	t.Parallel()

	taskID := uuid.New()
	repo := &mockTaskRepo{createdTask: model.Task{ID: taskID, Type: model.TaskTypeClientImport, Status: model.TaskStatusQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	publisher := &mockPublisher{}

	svc := NewImportService(repo, publisher)
	payload := model.ImportPayload{Clients: []model.ClientInput{{Name: "Ivan", Email: "ivan@example.com"}}}

	task, err := svc.CreateImportTask(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.ID != taskID {
		t.Fatalf("task id mismatch: got %s, want %s", task.ID, taskID)
	}

	if len(publisher.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(publisher.published))
	}
}

func TestCreateImportTask_PublishErrorUpdatesStatus(t *testing.T) {
	t.Parallel()

	repo := &mockTaskRepo{createdTask: model.Task{ID: uuid.New(), Type: model.TaskTypeClientImport, Status: model.TaskStatusQueued}}
	publisher := &mockPublisher{publishErr: errors.New("rabbit down")}
	svc := NewImportService(repo, publisher)

	_, err := svc.CreateImportTask(context.Background(), model.ImportPayload{Clients: []model.ClientInput{{Name: "Ivan", Email: "ivan@example.com"}}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !repo.updateCalled {
		t.Fatal("expected task status update on publish failure")
	}
}

func TestGetTask_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockTaskRepo{getErr: repository.ErrNotFound}
	svc := NewImportService(repo, &mockPublisher{})

	_, err := svc.GetTask(context.Background(), uuid.New().String())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
