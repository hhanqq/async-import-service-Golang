package model

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusFailed     TaskStatus = "failed"
)

const TaskTypeClientImport = "client_import"

type Task struct {
	ID           uuid.UUID     `json:"task_id"`
	Type         string        `json:"type"`
	Status       TaskStatus    `json:"status"`
	Payload      ImportPayload `json:"-"`
	Result       *ImportResult `json:"result,omitempty"`
	ErrorMessage *string       `json:"error_message,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type TaskMessage struct {
	TaskID string `json:"task_id"`
	Type   string `json:"type"`
}

type ImportPayload struct {
	Clients []ClientInput `json:"clients"`
}

type ImportResult struct {
	TotalRows   int        `json:"total_rows"`
	SuccessRows int        `json:"success_rows"`
	FailedRows  int        `json:"failed_rows"`
	Errors      []RowError `json:"errors,omitempty"`
}

type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}
