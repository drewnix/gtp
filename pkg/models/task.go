package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// Current state of a task
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

// Task represents a processing job
type Task struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Status      TaskStatus  `json:"status"`
	CreatedAt   time.Time   `json:"createdAt"`
	StartedAt   *time.Time  `json:"startedAt,omitempty"`
	CompletedAt *time.Time  `json:"completedAt,omitempty"`
	Data        interface{} `json:"data"`
	Result      interface{} `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// TaskConfig contains configuration options for task processing
type TaskConfig struct {
	Timeout int `json:"timeoutSeconds,omitempty"` // Zero means no timeout
}

// TaskRequest is the input format for creating a new task
type TaskRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Data        interface{} `json:"data"`

	// Type embedding: TaskRequest embeds TaskConfig
	// This promotes all TaskConfig fields to TaskRequest
	// In Kubernetes, this pattern is used extensively (PodSpec in Pod)
	TaskConfig
}

// TaskProcessor defines the interface for processing tasks
type TaskProcessor interface {
	// Process handles a single task and returns an error if processing fails
	Process(task *Task) error
}

// TaskStore defines an interface for task persistence
type TaskStore interface {
	// SaveTask persists a task
	SaveTask(task *Task) error

	// GetTask retrieves a task by ID
	GetTask(id string) (*Task, error)

	// ListTasks returns all tasks
	ListTasks() ([]*Task, error)

	// DeleteTask removes a task
	DeleteTask(id string) error
}

// FullTaskManager combines processing and storage capabilities
type FullTaskManager interface {
	TaskProcessor
	TaskStore
}

// Marshal converts a task to JSON
func (t *Task) Marshal() ([]byte, error) {
	return json.Marshal(t)
}

// Custom JSON marshaling for Task
// This demonstrates techniques used in Kubernetes API types
func (t *Task) MarshalJSON() ([]byte, error) {
	// Define a shadow type to avoid infinite recursion
	// This pattern is used throughout Kubernetes API types
	type TaskAlias Task

	// Create a custom representation for serialization
	return json.Marshal(&struct {
		*TaskAlias
		// Add custom formatting for specific fields
		FormattedCreatedAt string `json:"formattedCreatedAt,omitempty"`
		// Add computed fields
		Duration string `json:"duration,omitempty"`
	}{
		TaskAlias:          (*TaskAlias)(t),
		FormattedCreatedAt: t.CreatedAt.Format(time.RFC3339),
		Duration:           calculateDuration(t),
	})
}

// Custom JSON unmarshaling for TaskRequest
// This demonstrates handling of custom formats and validation
func (tr *TaskRequest) UnmarshalJSON(data []byte) error {
	// Shadow type to avoid recursive unmarshaling
	type Alias TaskRequest
	aux := &struct {
		*Alias

		// Add fields with custom formats
		CustomTimeout string `json:"customTimeoutStr,omitempty"`
	}{
		Alias: (*Alias)(tr),
	}

	// Unmarshal into the auxiliary type
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("invalid task request format: %w", err)
	}

	// Process custom formats if present
	if aux.CustomTimeout != "" {
		// Parse custom timeout format (e.g., "30s", "1m")
		duration, err := time.ParseDuration(aux.CustomTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
		tr.Timeout = int(duration.Seconds())
	}

	// Validate the unmarshaled data
	if tr.Name == "" {
		return fmt.Errorf("task name is required")
	}

	return nil
}

// Helper function to calculate task duration
func calculateDuration(t *Task) string {
	// If the task hasn't started yet
	if t.StartedAt == nil {
		return ""
	}

	// Determine end time (completed or current time)
	endTime := time.Now()
	if t.CompletedAt != nil {
		endTime = *t.CompletedAt
	}

	// Calculate and format duration
	duration := endTime.Sub(*t.StartedAt)
	return duration.String()
}

// Enhanced JSON utilities

// ToJSON converts any struct to a JSON string
func ToJSON(v interface{}) (string, error) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// FromJSON parses JSON into a struct, with proper error handling
func FromJSON(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return nil
}
