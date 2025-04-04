package processor

import (
	"fmt"
	"time"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/go-logr/logr"
)

// DefaultProcessor implements the TaskProcessor interface for basic task processing.
// This resembles the reconciliation logic in Kubernetes controllers.
type DefaultProcessor struct {
	logger logr.Logger
}

// NewDefaultProcessor creates a new processor with the given logger.
// This follows the factory pattern commonly used in Kubernetes components.
func NewDefaultProcessor(logger logr.Logger) *DefaultProcessor {
	return &DefaultProcessor{
		// Use WithName to create a hierarchical logger, common in Kubernetes
		logger: logger.WithName("processor"),
	}
}

// Process handles the actual business logic of task processing.
// Similar to Reconcile methods in Kubernetes controllers.
func (p *DefaultProcessor) Process(task *models.Task) error {
	// Structured logging with key-value pairs, the Kubernetes standard
	p.logger.Info("Processing task", "taskID", task.ID, "name", task.Name)

	// In real applications, this would be actual business logic
	// For demonstration, we're simulating work with a sleep
	time.Sleep(2 * time.Second)

	// Modify the task with results, similar to how controllers update resources
	p.logger.Info("Task processed successfully", "taskID", task.ID)
	task.Result = fmt.Sprintf("Processed: %v", task.Data)

	return nil
}
