package processor

import (
	"testing"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/go-logr/logr/testr"
)

func TestDefaultProcessor_Process(t *testing.T) {
	// Table-driven test pattern - common in Kubernetes codebase
	// Defines multiple test cases in a single test function
	testCases := []struct {
		name        string
		task        *models.Task
		expectError bool
		checkResult func(t *testing.T, task *models.Task)
	}{
		{
			name: "successful processing",
			task: &models.Task{
				ID:   "test-task-1",
				Name: "Successful Task",
				Data: map[string]interface{}{"key": "value"},
			},
			expectError: false,
			checkResult: func(t *testing.T, task *models.Task) {
				if task.Result == nil {
					t.Error("Expected result to be set")
				}
				// Check that the result contains our processed data
				resultStr, ok := task.Result.(string)
				if !ok {
					t.Error("Expected result to be a string")
				}
				if resultStr == "" {
					t.Error("Expected non-empty result")
				}
			},
		},
		{
			name: "empty data processing",
			task: &models.Task{
				ID:   "test-task-2",
				Name: "Empty Data Task",
				Data: nil,
			},
			expectError: false,
			checkResult: func(t *testing.T, task *models.Task) {
				if task.Result == nil {
					t.Error("Expected result to be set even with nil data")
				}
			},
		},
		// Additional test cases would go here
	}

	// Run each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Using logr/testr - an implementation of logr interface for testing
			// This is common in Kubernetes test code
			logger := testr.New(t)

			// Create the processor
			processor := NewDefaultProcessor(logger)

			// Execute the operation
			err := processor.Process(tc.task)

			// Check error expectation
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Run custom result validation
			if tc.checkResult != nil {
				tc.checkResult(t, tc.task)
			}
		})
	}
}
