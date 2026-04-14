package repository

import (
	"context"

	"github.com/google/uuid"

	"async-import-service/internal/model"
)

type TaskRepository interface {
	Create(ctx context.Context, taskType string, payload model.ImportPayload) (model.Task, error)
	GetByID(ctx context.Context, id uuid.UUID) (model.Task, error)
	List(ctx context.Context) ([]model.Task, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus, result *model.ImportResult, errorMessage *string) error
}

type ClientRepository interface {
	CreateIfNotExists(ctx context.Context, input model.ClientInput) (bool, error)
}
