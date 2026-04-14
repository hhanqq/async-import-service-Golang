package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"async-import-service/internal/model"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, taskType string, payload model.ImportPayload) (model.Task, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return model.Task{}, fmt.Errorf("marshal payload: %w", err)
	}

	const query = `
		INSERT INTO tasks (type, status, payload)
		VALUES ($1, $2, $3)
		RETURNING id, type, status, created_at, updated_at
	`

	var task model.Task
	if err := r.pool.QueryRow(ctx, query, taskType, model.TaskStatusQueued, payloadBytes).
		Scan(&task.ID, &task.Type, &task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return model.Task{}, fmt.Errorf("insert task: %w", err)
	}

	task.Payload = payload
	return task, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (model.Task, error) {
	const query = `
		SELECT id, type, status, payload, result, error_message, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	var (
		task         model.Task
		payloadBytes []byte
		resultBytes  []byte
		errorMessage *string
	)

	err := r.pool.QueryRow(ctx, query, id).
		Scan(&task.ID, &task.Type, &task.Status, &payloadBytes, &resultBytes, &errorMessage, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Task{}, ErrNotFound
		}
		return model.Task{}, fmt.Errorf("select task by id: %w", err)
	}

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &task.Payload); err != nil {
			return model.Task{}, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	if len(resultBytes) > 0 {
		var result model.ImportResult
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			return model.Task{}, fmt.Errorf("unmarshal result: %w", err)
		}
		task.Result = &result
	}

	task.ErrorMessage = errorMessage
	return task, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]model.Task, error) {
	const query = `
		SELECT id, type, status, result, error_message, created_at, updated_at
		FROM tasks
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		var (
			task         model.Task
			resultBytes  []byte
			errorMessage *string
		)

		if err := rows.Scan(&task.ID, &task.Type, &task.Status, &resultBytes, &errorMessage, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		if len(resultBytes) > 0 {
			var result model.ImportResult
			if err := json.Unmarshal(resultBytes, &result); err != nil {
				return nil, fmt.Errorf("unmarshal task result: %w", err)
			}
			task.Result = &result
		}

		task.ErrorMessage = errorMessage
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus, result *model.ImportResult, errorMessage *string) error {
	var resultBytes []byte
	if result != nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
		resultBytes = encoded
	}

	const query = `
		UPDATE tasks
		SET status = $2,
			result = COALESCE($3, result),
			error_message = $4,
			updated_at = NOW()
		WHERE id = $1
	`

	commandTag, err := r.pool.Exec(ctx, query, id, status, resultBytes, errorMessage)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) CreateIfNotExists(ctx context.Context, input model.ClientInput) (bool, error) {
	var phone *string
	if input.Phone != "" {
		phone = &input.Phone
	}

	const query = `
		INSERT INTO clients (name, email, phone)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING
	`

	commandTag, err := r.pool.Exec(ctx, query, input.Name, input.Email, phone)
	if err != nil {
		return false, fmt.Errorf("insert client: %w", err)
	}

	return commandTag.RowsAffected() > 0, nil
}

var ErrNotFound = errors.New("not found")
