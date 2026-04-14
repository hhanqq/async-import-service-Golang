package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"async-import-service/internal/model"
)

type fakeTaskRepo struct {
	task     model.Task
	statuses []model.TaskStatus
	final    *model.ImportResult
	errorMsg *string
}

func (f *fakeTaskRepo) Create(ctx context.Context, taskType string, payload model.ImportPayload) (model.Task, error) {
	panic("not used")
}

func (f *fakeTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (model.Task, error) {
	return f.task, nil
}

func (f *fakeTaskRepo) List(ctx context.Context) ([]model.Task, error) {
	panic("not used")
}

func (f *fakeTaskRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus, result *model.ImportResult, errorMessage *string) error {
	f.statuses = append(f.statuses, status)
	if result != nil {
		copied := *result
		f.final = &copied
	}
	f.errorMsg = errorMessage
	return nil
}

type fakeClientRepo struct {
	emails map[string]struct{}
}

func (f *fakeClientRepo) CreateIfNotExists(ctx context.Context, input model.ClientInput) (bool, error) {
	if _, exists := f.emails[input.Email]; exists {
		return false, nil
	}
	f.emails[input.Email] = struct{}{}
	return true, nil
}

func TestProcessor_HappyPath(t *testing.T) {
	t.Parallel()

	taskID := uuid.New()
	taskRepo := &fakeTaskRepo{task: model.Task{ID: taskID, Type: model.TaskTypeClientImport, Payload: model.ImportPayload{Clients: []model.ClientInput{
		{Name: "Ivan", Email: "ivan@example.com", Phone: "+79990000000"},
		{Name: "Petr", Email: "bad-email"},
		{Name: "Ivan Duplicate", Email: "ivan@example.com"},
	}}}}
	clientRepo := &fakeClientRepo{emails: make(map[string]struct{})}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	processor := NewProcessor(taskRepo, clientRepo, logger)

	messageBody, err := json.Marshal(model.TaskMessage{TaskID: taskID.String(), Type: model.TaskTypeClientImport})
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	if err := processor.ProcessMessage(context.Background(), messageBody); err != nil {
		t.Fatalf("unexpected processing error: %v", err)
	}

	if len(taskRepo.statuses) != 2 {
		t.Fatalf("expected two status updates, got %d", len(taskRepo.statuses))
	}
	if taskRepo.statuses[0] != model.TaskStatusProcessing {
		t.Fatalf("first status should be processing, got %s", taskRepo.statuses[0])
	}
	if taskRepo.statuses[1] != model.TaskStatusDone {
		t.Fatalf("second status should be done, got %s", taskRepo.statuses[1])
	}

	if taskRepo.final == nil {
		t.Fatal("expected final result")
	}

	if taskRepo.final.TotalRows != 3 || taskRepo.final.SuccessRows != 1 || taskRepo.final.FailedRows != 2 {
		t.Fatalf("unexpected result counters: %+v", *taskRepo.final)
	}

	if len(taskRepo.final.Errors) != 2 {
		t.Fatalf("expected 2 row errors, got %d", len(taskRepo.final.Errors))
	}
}
