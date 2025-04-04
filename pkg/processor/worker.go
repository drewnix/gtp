package processor

import (
	"context"
	"sync"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/go-logr/logr"
)

// WorkerPool manages a pool of worker goroutines that process tasks.
// This pattern is common in Kubernetes controllers to limit concurrent
// operations and control resource usage.
type WorkerPool struct {
	// Number of worker goroutines to start
	workers int
	// Channel for sending tasks to workers
	taskChan chan *workerTask
	// Channel for signaling shutdown
	shutdown chan struct{}
	// WaitGroup to track when all workers have exited
	wg sync.WaitGroup
	// Logger for worker operations
	logger logr.Logger
	// Processor for actual task processing
	processor models.TaskProcessor
	// Metrics for monitoring worker pool activity
	metrics *workerMetrics
}

// workerTask represents a task to be processed by a worker,
// including the callbacks for success and failure.
type workerTask struct {
	// The task to process
	task *models.Task
	// Context for cancellation and timeout
	ctx context.Context
	// Channel to report completion
	resultChan chan<- error
}

// workerMetrics tracks statistics about worker pool operations.
// This mimics how Kubernetes controllers track metrics.
type workerMetrics struct {
	tasksQueued     int64
	tasksProcessed  int64
	tasksSucceeded  int64
	tasksFailed     int64
	activeWorkers   int64
	queueDepth      int64
	processingTimes []float64
	mu              sync.Mutex
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
func NewWorkerPool(workers int, processor models.TaskProcessor, logger logr.Logger) *WorkerPool {
	if workers <= 0 {
		workers = 1 // Ensure at least one worker
	}

	// Create the worker pool with buffered channels
	// Buffer size is a trade-off between memory usage and handling bursts of tasks
	bufferSize := workers * 10
	return &WorkerPool{
		workers:   workers,
		taskChan:  make(chan *workerTask, bufferSize),
		shutdown:  make(chan struct{}),
		logger:    logger.WithName("worker-pool"),
		processor: processor,
		metrics:   &workerMetrics{},
	}
}

// Start launches the worker goroutines and begins processing tasks.
// This is similar to how Kubernetes controllers start worker goroutines
// when the controller manager starts.
func (wp *WorkerPool) Start() {
	wp.logger.Info("Starting worker pool", "workers", wp.workers)

	// Launch the specified number of worker goroutines
	wp.wg.Add(wp.workers)
	for i := 0; i < wp.workers; i++ {
		workerID := i
		// Each worker runs in its own goroutine
		go wp.runWorker(workerID)
	}
}

// runWorker runs a single worker goroutine that processes tasks.
// This pattern is used in Kubernetes controller workers that
// process items from a work queue.
func (wp *WorkerPool) runWorker(id int) {
	// Log worker startup
	wp.logger.V(1).Info("Worker started", "workerID", id)

	// Mark this worker as active
	wp.metrics.mu.Lock()
	wp.metrics.activeWorkers++
	wp.metrics.mu.Unlock()

	// Ensure cleanup when the worker exits
	defer func() {
		wp.logger.V(1).Info("Worker shutting down", "workerID", id)
		wp.metrics.mu.Lock()
		wp.metrics.activeWorkers--
		wp.metrics.mu.Unlock()
		wp.wg.Done()
	}()

	// Worker processing loop
	for {
		select {
		// Check for shutdown signal
		case <-wp.shutdown:
			return

		// Process tasks from the task channel
		case workerTask := <-wp.taskChan:
			// Update metrics
			wp.metrics.mu.Lock()
			wp.metrics.queueDepth--
			wp.metrics.mu.Unlock()

			// Process the task, handling cancellation via context
			wp.processTask(id, workerTask)
		}
	}
}

// processTask handles the processing of a single task by a worker.
// This pattern is similar to how Kubernetes controllers process
// individual reconciliation requests.
func (wp *WorkerPool) processTask(workerID int, wt *workerTask) {
	// Get a logger with task context
	logger := wp.logger.WithValues(
		"workerID", workerID,
		"taskID", wt.task.ID,
		"taskName", wt.task.Name,
	)

	logger.V(1).Info("Worker processing task")

	// Check if the task context is already cancelled
	select {
	case <-wt.ctx.Done():
		logger.Info("Task context already cancelled or timed out before processing")
		wt.resultChan <- wt.ctx.Err()
		// Update metrics
		wp.metrics.mu.Lock()
		wp.metrics.tasksProcessed++
		wp.metrics.tasksFailed++
		wp.metrics.mu.Unlock()
		return
	default:
		// Context not cancelled, proceed with processing
	}

	// Update metrics for task processing start
	wp.metrics.mu.Lock()
	wp.metrics.tasksProcessed++
	wp.metrics.mu.Unlock()

	// Create a processing goroutine to handle the task
	// This allows us to handle task processing with context cancellation
	processDone := make(chan error, 1)
	go func() {
		// Perform the actual task processing
		err := wp.processor.Process(wt.task)
		processDone <- err
	}()

	// Wait for processing to complete or context to be cancelled
	select {
	case err := <-processDone:
		// Task processing completed
		if err != nil {
			logger.Error(err, "Task processing failed")
			// Update failure metrics
			wp.metrics.mu.Lock()
			wp.metrics.tasksFailed++
			wp.metrics.mu.Unlock()
		} else {
			logger.V(1).Info("Task processing succeeded")
			// Update success metrics
			wp.metrics.mu.Lock()
			wp.metrics.tasksSucceeded++
			wp.metrics.mu.Unlock()
		}
		// Send result to the result channel
		wt.resultChan <- err

	case <-wt.ctx.Done():
		// Task was cancelled or timed out
		logger.Info("Task cancelled or timed out during processing",
			"error", wt.ctx.Err())
		// Send cancellation error to the result channel
		wt.resultChan <- wt.ctx.Err()
		// Update metrics
		wp.metrics.mu.Lock()
		wp.metrics.tasksFailed++
		wp.metrics.mu.Unlock()
	}
}

// SubmitTask adds a task to the worker pool for processing.
// Returns a channel that will receive the processing result.
// This pattern is used in Kubernetes controllers to enqueue
// reconciliation requests.
func (wp *WorkerPool) SubmitTask(ctx context.Context, task *models.Task) <-chan error {
	resultChan := make(chan error, 1)

	// Create a worker task
	wt := &workerTask{
		task:       task,
		ctx:        ctx,
		resultChan: resultChan,
	}

	// Update queue metrics
	wp.metrics.mu.Lock()
	wp.metrics.tasksQueued++
	wp.metrics.queueDepth++
	wp.metrics.mu.Unlock()

	// Try to send the task to the worker pool
	select {
	case wp.taskChan <- wt:
		// Task successfully queued
		wp.logger.V(1).Info("Task queued for processing", "taskID", task.ID)

	case <-ctx.Done():
		// Context cancelled before task could be queued
		wp.logger.Info("Task context cancelled before queuing",
			"taskID", task.ID,
			"error", ctx.Err())
		// Send error to result channel
		resultChan <- ctx.Err()
		// Update metrics
		wp.metrics.mu.Lock()
		wp.metrics.queueDepth--
		wp.metrics.tasksFailed++
		wp.metrics.mu.Unlock()
	}

	return resultChan
}

// Shutdown stops the worker pool and waits for all workers to exit.
// This is similar to how Kubernetes controllers shut down gracefully.
func (wp *WorkerPool) Shutdown() {
	wp.logger.Info("Shutting down worker pool")

	// Signal all workers to stop
	close(wp.shutdown)

	// Wait for all workers to finish
	wp.wg.Wait()

	wp.logger.Info("Worker pool shutdown complete")
}

// GetMetrics returns current worker pool metrics.
// This is similar to how Kubernetes controllers expose metrics.
func (wp *WorkerPool) GetMetrics() map[string]interface{} {
	wp.metrics.mu.Lock()
	defer wp.metrics.mu.Unlock()

	return map[string]interface{}{
		"workers": map[string]interface{}{
			"total":  wp.workers,
			"active": wp.metrics.activeWorkers,
		},
		"tasks": map[string]interface{}{
			"queued":     wp.metrics.tasksQueued,
			"processed":  wp.metrics.tasksProcessed,
			"succeeded":  wp.metrics.tasksSucceeded,
			"failed":     wp.metrics.tasksFailed,
			"queueDepth": wp.metrics.queueDepth,
		},
	}
}
