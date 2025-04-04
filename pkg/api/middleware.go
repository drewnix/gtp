package api

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// LoggingMiddleware logs request details, similar to how Kubernetes API server
// logs incoming requests with correlation IDs for traceability.
func LoggingMiddleware(logger logr.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Record start time for duration calculation
			start := time.Now()

			// Create a request ID for tracing
			// Kubernetes uses UID for resource objects and request IDs for API requests
			requestID := uuid.New().String()

			// Add request context with values
			// Context values are used in Kubernetes for request metadata
			ctx := context.WithValue(r.Context(), "requestID", requestID)

			// Create a request-scoped logger with additional context
			// This is the same pattern used in Kubernetes API server
			reqLogger := logger.WithValues(
				"requestID", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"remoteAddr", r.RemoteAddr,
				"userAgent", r.UserAgent(),
			)
			ctx = context.WithValue(ctx, "logger", reqLogger)

			// Use a custom response writer to capture status and size
			rw := NewResponseWriter(w)

			// Log request start (verbose)
			reqLogger.V(1).Info("Request started")

			// Process request with the enhanced context
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Log request completion with metadata
			reqLogger.Info("Request completed",
				"status", rw.Status(),
				"duration", time.Since(start).String(),
				"bytes", rw.BytesWritten(),
			)
		})
	}
}

// RecoveryMiddleware recovers from panics in request handlers.
// This is similar to how Kubernetes API server handles panics
// to prevent crashes from taking down the entire server.
func RecoveryMiddleware(logger logr.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Defer panic recovery
			defer func() {
				if err := recover(); err != nil {
					// Get request-scoped logger if available
					reqLogger, ok := r.Context().Value("logger").(logr.Logger)
					if !ok {
						reqLogger = logger
					}

					// Log panic with stack trace for debugging
					reqLogger.Error(nil, "Panic in HTTP handler",
						"error", err,
						"stack", string(debug.Stack()),
					)

					// Return 500 error to client
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ResponseWriter wraps http.ResponseWriter to capture metrics.
// This pattern is common in monitoring middleware for HTTP services.
type ResponseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

// NewResponseWriter creates a new ResponseWriter.
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	// Default status is 200 OK
	return &ResponseWriter{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader captures the status code before delegating.
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size before delegating.
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Status returns the HTTP status code.
func (rw *ResponseWriter) Status() int {
	return rw.status
}

// BytesWritten returns the number of bytes written.
func (rw *ResponseWriter) BytesWritten() int {
	return rw.bytesWritten
}
