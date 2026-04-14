package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"async-import-service/internal/model"
	"async-import-service/internal/service"
)

type ImportHandler struct {
	service *service.ImportService
}

func NewImportHandler(service *service.ImportService) *ImportHandler {
	return &ImportHandler{service: service}
}

func (h *ImportHandler) RegisterRoutes(r chi.Router) {
	r.Get("/health", h.Health)
	r.Post("/imports", h.CreateImport)
	r.Get("/imports", h.ListImports)
	r.Get("/imports/{id}", h.GetImport)
}

func (h *ImportHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type createImportRequest struct {
	Clients []model.ClientInput `json:"clients"`
}

type createImportResponse struct {
	TaskID string           `json:"task_id"`
	Status model.TaskStatus `json:"status"`
}

func (h *ImportHandler) CreateImport(w http.ResponseWriter, r *http.Request) {
	var req createImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := h.service.CreateImportTask(r.Context(), model.ImportPayload{Clients: req.Clients})
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create import task")
		return
	}

	writeJSON(w, http.StatusAccepted, createImportResponse{TaskID: task.ID.String(), Status: task.Status})
}

func (h *ImportHandler) GetImport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.service.GetTask(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch task")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (h *ImportHandler) ListImports(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.ListTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
