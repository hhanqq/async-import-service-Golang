package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"async-import-service/internal/model"
	"async-import-service/internal/repository"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
)

type TaskPublisher interface {
	PublishTask(ctx context.Context, msg model.TaskMessage) error
}

type ImportService struct {
	taskRepo  repository.TaskRepository
	publisher TaskPublisher
}

func NewImportService(taskRepo repository.TaskRepository, publisher TaskPublisher) *ImportService {
	return &ImportService{
		taskRepo:  taskRepo,
		publisher: publisher,
	}
}

func (s *ImportService) CreateImportTask(ctx context.Context, payload model.ImportPayload) (model.Task, error) {
	if err := ValidateCreateImportPayload(payload); err != nil {
		return model.Task{}, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	task, err := s.taskRepo.Create(ctx, model.TaskTypeClientImport, payload)
	if err != nil {
		return model.Task{}, fmt.Errorf("create task: %w", err)
	}

	message := model.TaskMessage{
		TaskID: task.ID.String(),
		Type:   task.Type,
	}

	if err := s.publisher.PublishTask(ctx, message); err != nil {
		errMsg := err.Error()
		_ = s.taskRepo.UpdateStatus(ctx, task.ID, model.TaskStatusFailed, nil, &errMsg)
		return model.Task{}, fmt.Errorf("publish task message: %w", err)
	}

	return task, nil
}

func (s *ImportService) GetTask(ctx context.Context, taskID string) (model.Task, error) {
	id, err := uuid.Parse(taskID)
	if err != nil {
		return model.Task{}, fmt.Errorf("%w: invalid task id", ErrBadRequest)
	}

	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Task{}, ErrNotFound
		}
		return model.Task{}, fmt.Errorf("get task: %w", err)
	}

	return task, nil
}

func (s *ImportService) ListTasks(ctx context.Context) ([]model.Task, error) {
	tasks, err := s.taskRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}
