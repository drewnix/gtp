package api

import (
	"encoding/json"
	"net/http"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/drewnix/gtp/pkg/processor"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// Handler contains HTTP request handlers that process API requests.
// Similar to how Kubernetes API server handlers process resource operations.
type Handler struct {
	manager *processor.TaskManager
	logger  logr.Logger
}

// NewHandler creates a new handler instance.
func NewHandler(manager *processor.TaskManager, logger logr.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger.WithName("api-handler"),
	}
}

// CreateTask handles POST /api/v1/tasks
// This is similar to how Kubernetes handles resource creation.
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	// Get request-scoped logger from context
	logger := r.Context().Value("logger").(logr.Logger)

	// Parse request body (JSON unmarshal)
	// Kubernetes API server does similar parsing of request bodies
	var req models.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Validate request - similar to Kubernetes admission validation
	if req.Name == "" {
		h.writeError(w, errors.New("task name is required"), http.StatusBadRequest)
		return
	}

	// Create task - similar to how controllers handle resource creation
	task, err := h.manager.CreateTask(&req)
	if err != nil {
		h.writeError(w, errors.Wrap(err, "failed to create task"), http.StatusInternalServerError)
		return
	}

	// Log creation and return the created resource
	// Kubernetes returns the created resource with UID and resourceVersion
	logger.Info("Task created", "taskID", task.ID)
	h.writeJSON(w, task, http.StatusCreated)
}

// GetTask handles GET /api/v1/tasks/{id}
// Similar to how Kubernetes handles GET requests for specific resources.
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	// Extract path parameters
	vars := mux.Vars(r)
	taskID := vars["id"]

	// Get task by ID - similar to Get operations in Kubernetes
	task, err := h.manager.GetTask(taskID)
	if err != nil {
		// Use errors.Is to check error types
		// This pattern is common in Kubernetes for specific error handling
		if errors.Is(err, processor.ErrTaskNotFound) {
			h.writeError(w, err, http.StatusNotFound)
		} else {
			h.writeError(w, errors.Wrap(err, "failed to get task"), http.StatusInternalServerError)
		}
		return
	}

	// Return the requested resource
	h.writeJSON(w, task, http.StatusOK)
}

// ListTasks handles GET /api/v1/tasks
// Similar to how Kubernetes handles LIST operations.
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// Get all tasks - similar to List operations in Kubernetes
	tasks := h.manager.ListTasks()

	// Return the collection - Kubernetes would include metadata like resourceVersion
	h.writeJSON(w, tasks, http.StatusOK)
}

// CancelTask handles POST /api/v1/tasks/{id}/cancel
// Similar to Kubernetes custom resource subresources or actions.
func (h *Handler) CancelTask(w http.ResponseWriter, r *http.Request) {
	// Extract path parameters
	vars := mux.Vars(r)
	taskID := vars["id"]

	// Cancel the task
	err := h.manager.CancelTask(taskID)
	if err != nil {
		// Error type handling pattern - check specific error types
		if errors.Is(err, processor.ErrTaskNotFound) {
			h.writeError(w, err, http.StatusNotFound)
		} else if errors.Is(err, processor.ErrTaskNotCancellable) {
			h.writeError(w, err, http.StatusBadRequest)
		} else {
			h.writeError(w, errors.Wrap(err, "failed to cancel task"), http.StatusInternalServerError)
		}
		return
	}

	// Get the updated task for the response
	task, err := h.manager.GetTask(taskID)
	if err != nil {
		h.writeError(w, errors.Wrap(err, "failed to get updated task"), http.StatusInternalServerError)
		return
	}

	// Return the updated resource
	h.writeJSON(w, task, http.StatusOK)
}

// writeJSON writes a JSON response with proper headers.
// This is a helper function for consistent JSON formatting.
func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error(err, "Failed to encode JSON response")
	}
}

// writeError writes a standardized error response.
// This creates consistent error formatting similar to Kubernetes error responses.
func (h *Handler) writeError(w http.ResponseWriter, err error, status int) {
	// Log the error with context
	h.logger.Error(err, "API error", "status", status)

	// Create a structured error response
	// Kubernetes uses a similar Status object for errors
	response := struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	}

	// Write the error as JSON with appropriate status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error(err, "Failed to encode error response")
	}
}
