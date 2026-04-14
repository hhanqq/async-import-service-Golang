package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"async-import-service/internal/model"
	"async-import-service/internal/repository"
	"async-import-service/internal/service"
)

type Processor struct {
	taskRepo   repository.TaskRepository
	clientRepo repository.ClientRepository
	logger     *slog.Logger
}

func NewProcessor(taskRepo repository.TaskRepository, clientRepo repository.ClientRepository, logger *slog.Logger) *Processor {
	return &Processor{
		taskRepo:   taskRepo,
		clientRepo: clientRepo,
		logger:     logger,
	}
}

func (p *Processor) ProcessMessage(ctx context.Context, body []byte) error {
	var message model.TaskMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return fmt.Errorf("unmarshal task message: %w", err)
	}

	taskID, err := uuid.Parse(message.TaskID)
	if err != nil {
		return fmt.Errorf("parse task_id: %w", err)
	}

	if message.Type != model.TaskTypeClientImport {
		errMsg := fmt.Sprintf("unsupported task type: %s", message.Type)
		_ = p.taskRepo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, nil, &errMsg)
		return errors.New(errMsg)
	}

	if err := p.taskRepo.UpdateStatus(ctx, taskID, model.TaskStatusProcessing, nil, nil); err != nil {
		return fmt.Errorf("set processing status: %w", err)
	}

	p.logger.Info("task processing started", "task_id", taskID.String())

	task, err := p.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		errMsg := err.Error()
		_ = p.taskRepo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, nil, &errMsg)
		return fmt.Errorf("load task payload: %w", err)
	}

	result, err := p.processClients(ctx, task.Payload)
	if err != nil {
		errMsg := err.Error()
		_ = p.taskRepo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, nil, &errMsg)
		return fmt.Errorf("process clients: %w", err)
	}

	if err := p.taskRepo.UpdateStatus(ctx, taskID, model.TaskStatusDone, &result, nil); err != nil {
		return fmt.Errorf("set done status: %w", err)
	}

	p.logger.Info(
		"task processing completed",
		"task_id", taskID.String(),
		"total_rows", result.TotalRows,
		"success_rows", result.SuccessRows,
		"failed_rows", result.FailedRows,
	)

	return nil
}

func (p *Processor) processClients(ctx context.Context, payload model.ImportPayload) (model.ImportResult, error) {
	result := model.ImportResult{TotalRows: len(payload.Clients)}

	for i, client := range payload.Clients {
		rowNumber := i + 1

		if err := service.ValidateClientInput(client); err != nil {
			result.FailedRows++
			result.Errors = append(result.Errors, model.RowError{Row: rowNumber, Message: err.Error()})
			continue
		}

		inserted, err := p.clientRepo.CreateIfNotExists(ctx, client)
		if err != nil {
			return model.ImportResult{}, fmt.Errorf("insert client row %d: %w", rowNumber, err)
		}

		if !inserted {
			result.FailedRows++
			result.Errors = append(result.Errors, model.RowError{Row: rowNumber, Message: "duplicate email, skipped"})
			continue
		}

		result.SuccessRows++
	}

	return result, nil
}
