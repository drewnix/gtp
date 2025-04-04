package processor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/drewnix/gtp/pkg/models"
	"github.com/go-logr/logr/testr"
)

func TestWorkerPool_Start(t *testing.T) {
	logger := testr.New(t)
	processor := &MockProcessor{}

	// Create worker pool with 3 workers
	pool := NewWorkerPool(3, processor, logger)

	// Start the pool
	pool.Start()

	// Check metrics after starting
	metrics := pool.GetMetrics()
	workerMetrics := metrics["workers"].(map[string]interface{})

	// Verify number of workers
	if workerMetrics["total"].(int) != 3 {
		t.Errorf("Expected 3 total workers, got %v", workerMetrics["total"])
	}

	// Verify active workers
	if workerMetrics["active"].(int64) != 3 {
		t.Errorf("Expected 3 active workers, got %v", workerMetrics["active"])
	}

	// Clean up
	pool.Shutdown()
}

func TestWorkerPool_SubmitTask(t *testing.T) {
	logger := testr.New(t)

	// Create a processor with controlled completion
	taskCompleted := make(chan struct{})
	processor := &MockProcessor{
		ProcessFunc: func(task *models.Task) error {
			// Signal when processing completes
			defer close(taskCompleted)
			return nil
		},
	}

	// Create worker pool with 1 worker for predictable behavior
	pool := NewWorkerPool(1, processor, logger)
	pool.Start()
	defer pool.Shutdown()

	// Create a task
	task := &models.Task{
		ID:   "test-task",
		Name: "Test Task",
		Data: "test data",
	}

	// Submit the task
	ctx := context.Background()
	resultChan := pool.SubmitTask(ctx, task)

	// Wait for completion or timeout
	select {
	case err := <-resultChan:
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for task to complete")
	}

	// Verify metrics
	metrics := pool.GetMetrics()
	taskMetrics := metrics["tasks"].(map[string]interface{})

	if taskMetrics["queued"].(int64) != 1 {
		t.Errorf("Expected 1 queued task, got %v", taskMetrics["queued"])
	}
	if taskMetrics["processed"].(int64) != 1 {
		t.Errorf("Expected 1 processed task, got %v", taskMetrics["processed"])
	}
	if taskMetrics["succeeded"].(int64) != 1 {
		t.Errorf("Expected 1 succeeded task, got %v", taskMetrics["succeeded"])
	}
}

func TestWorkerPool_TaskCancellation(t *testing.T) {
	logger := testr.New(t)

	// Create a processor that blocks until told to proceed
	blockChan := make(chan struct{})
	processor := &MockProcessor{
		ProcessFunc: func(task *models.Task) error {
			// Block until test allows it to proceed or context is cancelled
			select {
			case <-blockChan:
				return nil
			case <-time.After(5 * time.Second): // Guard against test hangs
				return errors.New("timeout waiting for test")
			}
		},
	}

	// Create worker pool
	pool := NewWorkerPool(1, processor, logger)
	pool.Start()
	defer pool.Shutdown()

	// Create a task
	task := &models.Task{
		ID:   "cancel-task",
		Name: "Task to Cancel",
		Data: "test data",
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Submit the task
	resultChan := pool.SubmitTask(ctx, task)

	// Give the worker time to start processing
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	// Check that we get a cancellation error
	select {
	case err := <-resultChan:
		if err == nil {
			t.Error("Expected error but got nil")
		}
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for cancellation")
	}

	// Always unblock the processor to avoid leaking goroutines
	close(blockChan)

	// Verify metrics
	metrics := pool.GetMetrics()
	taskMetrics := metrics["tasks"].(map[string]interface{})

	if taskMetrics["failed"].(int64) != 1 {
		t.Errorf("Expected 1 failed task, got %v", taskMetrics["failed"])
	}
}

func TestWorkerPool_Shutdown(t *testing.T) {
	logger := testr.New(t)
	processor := &MockProcessor{}

	// Create worker pool
	pool := NewWorkerPool(3, processor, logger)
	pool.Start()

	// Create a WaitGroup to track shutdown completion
	var wg sync.WaitGroup
	wg.Add(1)

	// Shutdown in a separate goroutine
	go func() {
		defer wg.Done()
		pool.Shutdown()
	}()

	// Wait for shutdown to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Shutdown completed successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for worker pool shutdown")
	}

	// Verify metrics after shutdown
	metrics := pool.GetMetrics()
	workerMetrics := metrics["workers"].(map[string]interface{})

	if workerMetrics["active"].(int64) != 0 {
		t.Errorf("Expected 0 active workers after shutdown, got %v", workerMetrics["active"])
	}
}

func TestWorkerPool_ConcurrentTasks(t *testing.T) {
	logger := testr.New(t)

	// Use a mutex to protect the completed count
	var mu sync.Mutex
	completed := 0

	// Create a processor that simulates work with a small delay
	processor := &MockProcessor{
		ProcessFunc: func(task *models.Task) error {
			// Simulate work
			time.Sleep(100 * time.Millisecond)
			
			// Record completion
			mu.Lock()
			completed++
			mu.Unlock()
			
			return nil
		},
	}

	// Create worker pool with multiple workers
	numWorkers := 5
	pool := NewWorkerPool(numWorkers, processor, logger)
	pool.Start()
	defer pool.Shutdown()

	// Create multiple tasks
	numTasks := 10
	resultChans := make([]<-chan error, numTasks)

	// Submit all tasks
	ctx := context.Background()
	for i := 0; i < numTasks; i++ {
		task := &models.Task{
			ID:   fmt.Sprintf("task-%d", i),
			Name: fmt.Sprintf("Task %d", i),
			Data: fmt.Sprintf("data %d", i),
		}
		resultChans[i] = pool.SubmitTask(ctx, task)
	}

	// Wait for all tasks to complete
	for i, ch := range resultChans {
		select {
		case err := <-ch:
			if err != nil {
				t.Errorf("Task %d failed: %v", i, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("Timed out waiting for task %d", i)
		}
	}

	// Verify that all tasks completed
	mu.Lock()
	if completed != numTasks {
		t.Errorf("Expected %d completed tasks, got %d", numTasks, completed)
	}
	mu.Unlock()

	// Verify metrics
	metrics := pool.GetMetrics()
	taskMetrics := metrics["tasks"].(map[string]interface{})

	if taskMetrics["processed"].(int64) != int64(numTasks) {
		t.Errorf("Expected %d processed tasks, got %v", numTasks, taskMetrics["processed"])
	}
	if taskMetrics["succeeded"].(int64) != int64(numTasks) {
		t.Errorf("Expected %d succeeded tasks, got %v", numTasks, taskMetrics["succeeded"])
	}
}