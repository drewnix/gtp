# Go Task Processor (GTP)

A concurrent task processing service demonstrating Go concurrency patterns relevant to Kubernetes controller development.

## Overview

GTP is a HTTP service that:

- Accepts HTTP requests to create processing tasks
- Manages task execution concurrently using worker pools
- Allows checking status and cancellation of tasks
- Implements proper timeout and context handling
- Uses structured logging and idiomatic error handling

## Features

- **Concurrent Task Processing**: Worker pool pattern for controlled concurrency
- **Context & Cancellation**: Robust handling of timeouts and cancellation
- **RESTful API**: Clean HTTP interface following Kubernetes-like patterns
- **Structured Logging**: Using logr and zap for production-grade logging
- **Configuration Management**: Layered config from files, environment, and flags
- **Error Handling**: Domain-specific error types with proper wrapping
- **Graceful Shutdown**: Clean termination of in-flight requests
- **Unit & Integration Tests**: Comprehensive test coverage

## Getting Started

### Prerequisites

- Go 1.18+

### Installation

```bash
git clone https://github.com/drewnix/gtp.git
cd gtp
go mod download
```

### Running

```bash
# Run with default configuration
go run cmd/server/main.go

# Run with custom port and log level
go run cmd/server/main.go --port 9090 --v 2

# Run with config file
go run cmd/server/main.go --config config.yaml

# Run with higher concurrency
go run cmd/server/main.go --concurrency 20
```

### API Usage

Create a task:
```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sample Task",
    "description": "A sample task for demonstration",
    "data": {"input": "sample data"},
    "timeoutSeconds": 30
  }'
```

Get task status:
```bash
curl http://localhost:8080/api/v1/tasks/YOUR_TASK_ID
```

List all tasks:
```bash
curl http://localhost:8080/api/v1/tasks
```

Cancel a task:
```bash
curl -X POST http://localhost:8080/api/v1/tasks/YOUR_TASK_ID/cancel
```

## Configuration

GTP can be configured via:

- Command-line flags (highest priority)
- Environment variables
- YAML configuration file
- Default values

Example configuration file:
```yaml
server:
  port: 9090
  host: "127.0.0.1"
  readTimeoutSec: 30
  writeTimeoutSec: 30
  idleTimeoutSec: 60
processing:
  maxConcurrent: 20
  defaultTimeoutSec: 60
  maxTimeoutSec: 300
logging:
  level: 2
  format: "json"
```

Environment variables:
```bash
SERVER_PORT=9090 PROCESSING_MAX_CONCURRENT=20 go run cmd/server/main.go
```

## Documentation

For detailed documentation, see [doc.md](doc.md).

## Testing

Run the tests:
```bash
go test ./...
```

## License

MIT