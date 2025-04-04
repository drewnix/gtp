package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"github.com/drewnix/gtp/pkg/api"
	"github.com/drewnix/gtp/pkg/config"
	"github.com/drewnix/gtp/pkg/processor"
)

func main() {
	// Parse command line flags
	// Kubernetes components typically use flags for configuration
	// They also support loading from environment variables and/or config files
	var (
		port        int
		logLevel    int
		configPath  string
		concurrency int
	)
	flag.IntVar(&port, "port", 8080, "HTTP server port")
	flag.IntVar(&logLevel, "v", 0, "Log verbosity level (0-10)")
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.IntVar(&concurrency, "concurrency", 10, "Maximum number of concurrent tasks")
	flag.Parse()
	
	// Load configuration from file and/or environment variables
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}
	
	// Command line flags take precedence over config file/env vars
	if flag.CommandLine.Changed("port") {
		cfg.Server.Port = port
	} else {
		port = cfg.Server.Port
	}
	
	if flag.CommandLine.Changed("v") {
		cfg.Logging.Level = logLevel
	} else {
		logLevel = cfg.Logging.Level
	}
	
	if flag.CommandLine.Changed("concurrency") {
		cfg.Processing.MaxConcurrent = concurrency
	} else {
		concurrency = cfg.Processing.MaxConcurrent
	}

	// Initialize structured logging
	// This mirrors how Kubernetes components configure logging
	zapConfig := zap.NewProductionConfig()
	// Production configuration includes:
	// - JSON formatting (machine-readable)
	// - Timestamp
	// - Log level
	// - Caller information
	zapLog, err := zapConfig.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer zapLog.Sync()

	// Create a logr.Logger using zap
	// logr is the logging interface used throughout Kubernetes
	// zapr adapts zap to the logr interface
	logger := zapr.NewLogger(zapLog).WithName("task-processor")
	logger.V(logLevel).Info("Starting task processor service",
		"version", "1.0.0",
		"port", port,
		"logLevel", logLevel)

	// Component initialization and wiring
	// This pattern of explicit dependency injection is common in Kubernetes components

	// Initialize the task processor - core business logic
	taskProcessor := processor.NewDefaultProcessor(logger)

	// Initialize the task manager - coordinates processing
	// Use concurrency value from config
	taskManager := processor.NewTaskManager(taskProcessor, logger, concurrency)

	// Create HTTP router with all routes configured
	router := api.NewRouter(taskManager, logger)

	// Configure the HTTP server with timeouts from config
	// Proper timeout configuration is critical for production services
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
		// These timeouts help prevent resource exhaustion under load
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,  // Max time to read request
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second, // Max time to write response
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,  // Max time for idle connections
	}

	// Start HTTP server in a goroutine to allow graceful shutdown
	// This pattern is used in all Kubernetes components that serve HTTP
	go func() {
		logger.Info("Starting HTTP server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Only log errors other than server closed
			logger.Error(err, "HTTP server critical error")
			os.Exit(1)
		}
	}()

	// Set up signal handling for graceful shutdown
	// This pattern is universal in Kubernetes components
	sigChan := make(chan os.Signal, 1)
	// SIGINT is sent by Ctrl+C, SIGTERM is sent by Kubernetes during pod termination
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("Received termination signal, initiating shutdown", "signal", sig)

	// Graceful shutdown with timeout context
	// Kubernetes components use this pattern to ensure clean shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shut down the HTTP server
	logger.Info("Shutting down HTTP server")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "HTTP server shutdown error")
	}

	// At this point, we could shut down other components like:
	// - Closing database connections
	// - Stopping worker pools
	// - Flushing caches or queues

	logger.Info("Shutdown complete, exiting")
}
