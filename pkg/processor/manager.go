package processor

import (
	"context"
	"sync"
	"time"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Common error types for domain-specific errors
var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskNotCancellable = errors.New("task is not in a cancellable state")
)

// TaskManager handles task lifecycle and orchestrates concurrent processing.
// This resembles controller managers in Kubernetes that coordinate multiple control loops.
type TaskManager struct {
	// In-memory task storage (in Kubernetes, this would use the API server)
	tasks map[string]*models.Task
	// Task processor for actual processing logic
	processor models.TaskProcessor
	// Structured logger
	logger logr.Logger
	// Mutex for protecting concurrent access to task map
	mu sync.RWMutex
	// Map of channels for task cancellation signals
	// Similar to how Kubernetes uses contexts for cancellation
	cancelChans map[string]chan struct{}

	workerPool *WorkerPool
}

// NewTaskManager creates a new task manager with the given processor and logger.
func NewTaskManager(processor models.TaskProcessor, logger logr.Logger, concurrency int) *TaskManager {
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}

	tm := &TaskManager{
		tasks:       make(map[string]*models.Task),
		processor:   processor,
		logger:      logger.WithName("task-manager"),
		cancelChans: make(map[string]chan struct{}),
	}

	// Create and start the worker pool
	tm.workerPool = NewWorkerPool(concurrency, processor, logger)
	tm.workerPool.Start()

	return tm
}

// CreateTask creates a new task and starts its asynchronous processing.
// This is similar to how Kubernetes controllers enqueue work items.
func (tm *TaskManager) CreateTask(req *models.TaskRequest) (*models.Task, error) {
	// Create task with a unique ID (similar to Kubernetes resources)
	task := &models.Task{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Status:      models.StatusPending,
		CreatedAt:   time.Now(),
		Data:        req.Data,
	}

	// Thread-safe task storage using mutex
	// Critical sections should be as small as possible
	tm.mu.Lock()
	tm.tasks[task.ID] = task
	// Create cancellation channel for this task
	cancelChan := make(chan struct{})
	tm.cancelChans[task.ID] = cancelChan
	tm.mu.Unlock()

	// Start asynchronous processing with a goroutine
	// This mimics how Kubernetes controllers process resources asynchronously
	go tm.processTask(task, req.Timeout, cancelChan)

	return task, nil
}

// processTask handles the asynchronous processing of a task.
// This method demonstrates Go's concurrency patterns with goroutines, channels, and select.
// These patterns are extensively used in Kubernetes controllers for handling asynchronous operations.
func (tm *TaskManager) processTask(task *models.Task, timeoutSec int, cancelChan <-chan struct{}) {
	// Create a context with timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc

	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// Update task status to running
	tm.mu.Lock()
	now := time.Now()
	task.Status = models.StatusRunning
	task.StartedAt = &now
	tm.mu.Unlock()

	// Monitor for cancellation in a separate goroutine
	go func() {
		select {
		case <-cancelChan:
			// Task was cancelled manually
			cancel() // Cancel the context
		case <-ctx.Done():
			// Context was cancelled or timed out - nothing to do
		}
	}()

	// Submit the task to the worker pool
	resultChan := tm.workerPool.SubmitTask(ctx, task)

	// Wait for task completion
	err := <-resultChan

	// Update task based on result
	tm.mu.Lock()
	completedAt := time.Now()
	task.CompletedAt = &completedAt

	if ctx.Err() == context.Canceled {
		// Task was cancelled
		task.Status = models.StatusCancelled
		task.Error = "Task was cancelled"
		tm.logger.Info("Task was cancelled", "taskID", task.ID)
	} else if ctx.Err() == context.DeadlineExceeded {
		// Task timed out
		task.Status = models.StatusFailed
		task.Error = "Task timed out"
		tm.logger.Info("Task timed out", "taskID", task.ID, "timeout", timeoutSec)
	} else if err != nil {
		// Task failed
		task.Status = models.StatusFailed
		task.Error = err.Error()
		tm.logger.Error(err, "Task processing failed", "taskID", task.ID)
	} else {
		// Task completed successfully
		task.Status = models.StatusCompleted
		tm.logger.Info("Task completed successfully", "taskID", task.ID)
	}
	tm.mu.Unlock()

	// Clean up the cancel channel
	tm.mu.Lock()
	delete(tm.cancelChans, task.ID)
	tm.mu.Unlock()
}

// GetTask retrieves a task by ID with thread safety.
// This demonstrates proper mutex usage for concurrent read access.
func (tm *TaskManager) GetTask(id string) (*models.Task, error) {
	// RLock allows multiple readers but no writers
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks returns all tasks with thread safety.
// Similar to how Kubernetes listers provide access to cached resources.
func (tm *TaskManager) ListTasks() []*models.Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Create a new slice to avoid returning direct access to the map
	tasks := make([]*models.Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CancelTask cancels a running task by closing its cancellation channel.
// This demonstrates how Go channels can be used for signaling between goroutines.
func (tm *TaskManager) CancelTask(id string) error {
	// First check if the task exists and is cancellable
	tm.mu.RLock()
	cancelChan, exists := tm.cancelChans[id]
	task, taskExists := tm.tasks[id]
	tm.mu.RUnlock()

	if !taskExists {
		return ErrTaskNotFound
	}

	if task.Status != models.StatusRunning && task.Status != models.StatusPending {
		return ErrTaskNotCancellable
	}

	if !exists {
		return errors.New("task cannot be cancelled")
	}

	// Signal cancellation by closing the channel
	// In Go, a closed channel will always be ready to receive with the zero value
	close(cancelChan)
	return nil
}

func (tm *TaskManager) Shutdown(ctx context.Context) error {
	tm.logger.Info("Shutting down task manager")

	// Cancel all in-progress tasks
	tm.mu.Lock()
	for id, cancelChan := range tm.cancelChans {
		tm.logger.Info("Cancelling task during shutdown", "taskID", id)
		close(cancelChan)
	}
	tm.mu.Unlock()

	// Shut down the worker pool
	tm.workerPool.Shutdown()

	tm.logger.Info("Task manager shutdown complete")
	return nil
}
