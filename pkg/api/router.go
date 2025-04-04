package api

import (
	"net/http"

	"github.com/drewnix/gtp/pkg/processor"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
)

// NewRouter creates and configures a new HTTP router.
// This function sets up routing similar to how Kubernetes API servers
// define their API structure with versioning.
func NewRouter(manager *processor.TaskManager, logger logr.Logger) *mux.Router {
	router := mux.NewRouter()

	// Create handler instance
	handler := NewHandler(manager, logger)

	// Configure middleware - applies to all routes
	// Middleware ordering matters, similar to admission webhooks in Kubernetes
	router.Use(LoggingMiddleware(logger))  // Logs all requests
	router.Use(RecoveryMiddleware(logger)) // Handles panics

	// API versioning with a subrouter
	// Kubernetes follows this pattern with its /apis/GROUP/VERSION/ structure
	api := router.PathPrefix("/api/v1").Subrouter()

	// RESTful resource endpoints for tasks
	// These follow the Kubernetes resource API pattern:
	// - POST for creation
	// - GET for retrieval (single or collection)
	// - Custom verbs as subresources (e.g., /cancel)
	api.HandleFunc("/tasks", handler.CreateTask).Methods("POST")
	api.HandleFunc("/tasks", handler.ListTasks).Methods("GET")
	api.HandleFunc("/tasks/{id}", handler.GetTask).Methods("GET")
	api.HandleFunc("/tasks/{id}/cancel", handler.CancelTask).Methods("POST")

	// Health check endpoint for liveness/readiness probes
	// Standard in Kubernetes for container health checking
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	return router
}
