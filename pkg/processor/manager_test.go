package processor

import (
	"errors"
	"testing"
	"time"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/go-logr/logr/testr"
)

// MockProcessor implements the TaskProcessor interface for testing.
// This pattern is standard in Kubernetes controller tests, where
// dependencies are mocked to isolate the component under test.
type MockProcessor struct {
	// Function fields allow tests to customize behavior
	ProcessFunc func(task *models.Task) error
	// We could add fields to track calls for verification
	ProcessCalls int
}

// Process implements the TaskProcessor interface with the mock behavior.
func (mp *MockProcessor) Process(task *models.Task) error {
	// Track calls for verification
	mp.ProcessCalls++

	// Execute the custom function if provided
	if mp.ProcessFunc != nil {
		return mp.ProcessFunc(task)
	}
	// Default implementation
	return nil
}

func TestTaskManager_CreateTask(t *testing.T) {
	// Create test logger using testr
	logger := testr.New(t)

	// Create a mock processor with a quick, successful implementation
	mockProcessor := &MockProcessor{
		ProcessFunc: func(task *models.Task) error {
			// Simulate successful processing
			task.Result = "Processed in test"
			return nil
		},
	}

	// Create the task manager with the mock
	manager := NewTaskManager(mockProcessor, logger, 2)

	// Create a sample task request
	req := &models.TaskRequest{
		Name:        "Test Task",
		Description: "Task for testing",
		Data:        "test data",
	}

	// Execute the operation being tested
	task, err := manager.CreateTask(req)

	// Assert on basic expectations
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task.ID == "" {
		t.Fatal("Expected task ID to be set")
	}

	if task.Status != models.StatusPending {
		t.Errorf("Expected initial status to be pending, got %v", task.Status)
	}

	// Wait for asynchronous processing to complete
	// In real Kubernetes controller tests, we'd use a test environment
	// that allows deterministic execution without real waits
	time.Sleep(100 * time.Millisecond)

	// Retrieve the updated task and verify its state
	updatedTask, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get updated task: %v", err)
	}

	// Verify the task was processed correctly
	if updatedTask.Status != models.StatusCompleted {
		t.Errorf("Expected final status to be completed, got %v", updatedTask.Status)
	}

	if updatedTask.Result != "Processed in test" {
		t.Errorf("Expected result from mock, got %v", updatedTask.Result)
	}

	// Verify the mock was called as expected
	if mockProcessor.ProcessCalls != 1 {
		t.Errorf("Expected Process to be called once, got %d calls", mockProcessor.ProcessCalls)
	}
}

func TestTaskManager_CancelTask(t *testing.T) {
	// Create test logger
	logger := testr.New(t)

	// Create a mock processor that blocks until cancelled
	blockingChan := make(chan struct{})
	mockProcessor := &MockProcessor{
		ProcessFunc: func(task *models.Task) error {
			// Block until test signals completion or long timeout
			select {
			case <-blockingChan:
				return errors.New("processing was interrupted")
			case <-time.After(5 * time.Second): // Guard against test hangs
				return nil
			}
		},
	}

	// Create task manager with the blocking mock
	manager := NewTaskManager(mockProcessor, logger, 2)

	// Create task request
	req := &models.TaskRequest{
		Name: "Cancellable Task",
		Data: "test data",
	}

	// Create the task
	task, err := manager.CreateTask(req)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Wait for task to start processing
	time.Sleep(100 * time.Millisecond)

	// Verify initial running state
	runningTask, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if runningTask.Status != models.StatusRunning {
		t.Errorf("Expected task to be running, got status: %v", runningTask.Status)
	}

	// Cancel the task
	err = manager.CancelTask(task.ID)
	if err != nil {
		t.Errorf("Failed to cancel task: %v", err)
	}

	// Allow the mock processor to finish after cancellation
	close(blockingChan)

	// Wait for cancellation to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify the task was properly cancelled
	cancelledTask, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get cancelled task: %v", err)
	}

	if cancelledTask.Status != models.StatusCancelled {
		t.Errorf("Expected status to be cancelled, got %v", cancelledTask.Status)
	}

	if cancelledTask.Error == "" {
		t.Error("Expected error message in cancelled task")
	}
}

func TestTaskManager_TaskTimeout(t *testing.T) {
	// Create test logger
	logger := testr.New(t)

	// Create mock processor that takes longer than the specified timeout
	mockProcessor := &MockProcessor{
		ProcessFunc: func(task *models.Task) error {
			// Sleep longer than the timeout to trigger timeout handling
			time.Sleep(2 * time.Second)
			return nil
		},
	}

	// Create task manager with the slow mock
	manager := NewTaskManager(mockProcessor, logger, 2)

	// Create task request with a short timeout
	req := &models.TaskRequest{
		Name: "Timeout Task",
		Data: "timeout test data",
		TaskConfig: models.TaskConfig{
			Timeout: 1, // 1 second timeout
		},
	}

	// Create the task
	task, err := manager.CreateTask(req)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Wait for the timeout to occur and be processed
	// This is slightly longer than the timeout to ensure processing completes
	time.Sleep(1500 * time.Millisecond)

	// Verify the task timed out correctly
	timedOutTask, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get timed out task: %v", err)
	}

	// Check status and error message
	if timedOutTask.Status != models.StatusFailed {
		t.Errorf("Expected status to be failed, got %v", timedOutTask.Status)
	}

	if timedOutTask.Error != "Task timed out" {
		t.Errorf("Expected timeout error message, got: %v", timedOutTask.Error)
	}

	// Verify CompletedAt timestamp was set
	if timedOutTask.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set for timed out task")
	}
}
