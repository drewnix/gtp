package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/drewnix/gtp/pkg/api"
	"github.com/drewnix/gtp/pkg/processor"
	"github.com/go-logr/logr/testr"
)

// Integration test for the API handlers
// This tests the full request processing flow through multiple components
func TestTaskAPI_Integration(t *testing.T) {
	// Create test logger
	logger := testr.New(t)

	// Create real processor and manager (not mocks)
	// This is a true integration test as it uses real implementations
	taskProcessor := processor.NewDefaultProcessor(logger)
	taskManager := processor.NewTaskManager(taskProcessor, logger, 5)

	// Create router with real components
	router := api.NewRouter(taskManager, logger)

	// Create test server
	server := httptest.NewServer(router)
	defer server.Close()

	// Test task creation
	t.Run("Create and retrieve task", func(t *testing.T) {
		// Create task request
		taskReq := models.TaskRequest{
			Name:        "Integration Test Task",
			Description: "Testing the API integration",
			Data:        map[string]string{"test": "data"},
		}

		// Marshal to JSON
		reqBody, err := json.Marshal(taskReq)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		// Make POST request to create task
		resp, err := http.Post(
			server.URL+"/api/v1/tasks",
			"application/json",
			bytes.NewBuffer(reqBody),
		)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		// Parse response
		var task models.Task
		if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify task was created correctly
		if task.ID == "" {
			t.Error("Expected task ID to be set")
		}
		if task.Name != taskReq.Name {
			t.Errorf("Expected name %q, got %q", taskReq.Name, task.Name)
		}

		// Now retrieve the task to verify it exists
		getResp, err := http.Get(server.URL + "/api/v1/tasks/" + task.ID)
		if err != nil {
			t.Fatalf("Failed to get task: %v", err)
		}
		defer getResp.Body.Close()

		// Check status code
		if getResp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", getResp.StatusCode)
		}

		// Parse response
		var retrievedTask models.Task
		if err := json.NewDecoder(getResp.Body).Decode(&retrievedTask); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify retrieved task matches created task
		if retrievedTask.ID != task.ID {
			t.Errorf("Expected ID %q, got %q", task.ID, retrievedTask.ID)
		}
	})
}
