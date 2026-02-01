# Kubernetes Operator Replay Debugger

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Safety Critical](https://img.shields.io/badge/Safety-JPL%20Power%20of%2010-green)](SAFETY_COMPLIANCE.md)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()
[![CI](https://github.com/slyt3/k8s-operator-replay-debugger/actions/workflows/ci.yml/badge.svg)](https://github.com/slyt3/k8s-operator-replay-debugger/actions/workflows/ci.yml)

> Record, replay, and debug Kubernetes operator reconciliation loops with time-travel debugging

# Kubernetes Operator Replay Debugger

A production-grade tool for recording, replaying, and analyzing Kubernetes operator reconciliation loops. Helps debug operator behavior by capturing all API interactions and enabling time-travel debugging.

## Features

- **Recording Mode**: Transparently record all Kubernetes API operations
- **Replay Mode**: Step through recorded operations forward/backward
- **Analysis Mode**: Detect loops, slow operations, and error patterns
- **Flexible Storage**: SQLite (embedded) or MongoDB (scalable) backends
- **Safety-Critical**: Follows JPL Power of 10 coding rules
- **JSON Export**: Machine-readable output for CI/CD integration
- **Time Travel**: Navigate through operation history

## Architecture

```
┌─────────────────────────────────────┐
│ 1. Recording Mode                   │
│ - Intercept all K8s API calls       │
│ - Record: events, state, timing     │
│ - Store in SQLite or MongoDB        │
└─────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────┐
│ 2. Replay Mode                      │
│ - Mock K8s API server               │
│ - Feed recorded events              │
│ - Step through reconciliation       │
│ - Time travel (rewind/forward)      │
└─────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────┐
│ 3. Analysis Mode                    │
│ - Show state diff at each step      │
│ - Identify infinite loops           │
│ - Find race conditions              │
│ - Performance bottlenecks           │
│ - JSON output for automation        │
└─────────────────────────────────────┘
```

## Installation

### Prerequisites

- Go 1.21 or later
- GCC (for SQLite CGO)
- Linux Mint or similar distribution

### Build from Source

```bash
git clone https://github.com/your-org/k8s-operator-replay-debugger
cd k8s-operator-replay-debugger

# Install dependencies
go mod download

# Build the CLI
go build -o replay-cli ./cmd/replay-cli

# Run tests
go test ./...
```

## Quick Start

### 1. Record Operations (Library Usage)

Integrate the recording client into your operator:

```go
package main

import (
    "context"
    "github.com/operator-replay-debugger/pkg/recorder"
    "github.com/operator-replay-debugger/pkg/storage"
    "k8s.io/client-go/kubernetes"
)

func main() {
    // Create your normal Kubernetes client
    k8sClient := kubernetes.NewForConfigOrDie(config)
    
    // Open recording database
    db, err := storage.NewDatabase("recordings.db", 1000000)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // Wrap client with recorder
    recordingClient, err := recorder.NewRecordingClient(recorder.Config{
        Client:      k8sClient,
        Database:    db,
        SessionID:   "prod-deployment-001",
        MaxSequence: 1000000,
        ActorID:     "my-operator/controller-a",
    })
    if err != nil {
        panic(err)
    }
    
    // Use recording client for operations
    pod, err := recordingClient.RecordGet(
        context.Background(),
        "Pod",
        "default",
        "my-pod",
        metav1.GetOptions{},
    )
}
```

### 2. Replay Operations

```bash
# List available sessions
./replay-cli sessions -d recordings.db

# Replay a session automatically
./replay-cli replay prod-deployment-001 -d recordings.db

# Interactive replay with step controls
./replay-cli replay prod-deployment-001 -d recordings.db -i

# Interactive commands:
#   n - step forward
#   b - step backward
#   r - reset to beginning
#   s - show statistics
#   q - quit
```

### 3. Analyze Operations

**SQLite Storage (default):**
```bash
# Detect loops, slow operations, and errors
./replay-cli analyze prod-deployment-001 -d recordings.db

# Only detect loops
./replay-cli analyze prod-deployment-001 -d recordings.db --loops --no-slow --no-errors

# JSON output for automation and CI/CD pipelines
./replay-cli analyze prod-deployment-001 -d recordings.db --format json > report.json
```

**MongoDB Storage:**
```bash
# Analyze with MongoDB backend
./replay-cli analyze prod-deployment-001 \
    --storage mongodb \
    --mongo-uri "mongodb://localhost:27017" \
    --mongo-db "operator_replay"

# MongoDB with custom settings
./replay-cli analyze prod-deployment-001 \
    --storage mongodb \
    --mongo-uri "mongodb://user:pass@cluster.mongodb.net" \
    --mongo-db "production_debugging" \
    --threshold 2000 \
    --format json
```

## Causality Graph (A→B→C)

Infer cross-controller chains like:
`controller A WRITE -> controller B RECONCILE -> controller B WRITE -> controller C RECONCILE`.

**Text output:**
```bash
./replay-cli analyze causality --session prod-deployment-001 -d recordings.db
```

**JSON output (graph nodes/edges):**
```bash
./replay-cli analyze causality --session prod-deployment-001 \
  -d recordings.db \
  --format json \
  --max-depth 6
```

**Optional window + payloads:**
```bash
./replay-cli analyze causality --session prod-deployment-001 \
  -d recordings.db \
  --window "2024-12-08T10:00:00Z,2024-12-08T10:10:00Z" \
  --format json \
  --include-payloads
```

## Database Schema

```sql
CREATE TABLE operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    sequence_number INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    operation_type TEXT NOT NULL,
    resource_kind TEXT NOT NULL,
    namespace TEXT,
    name TEXT,
    resource_data TEXT,
    error TEXT,
    duration_ms INTEGER NOT NULL,
    actor_id TEXT,
    uid TEXT,
    resource_version TEXT,
    generation INTEGER,
    verb TEXT
);

CREATE TABLE reconcile_spans (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    start_ts INTEGER NOT NULL,
    end_ts INTEGER,
    duration_ms INTEGER,
    kind TEXT NOT NULL,
    namespace TEXT,
    name TEXT,
    trigger_uid TEXT,
    trigger_resource_version TEXT,
    trigger_reason TEXT,
    error TEXT
);
```

## Safety-Critical Compliance

This project follows the JPL Power of 10 rules for safety-critical code:

1. **No recursion** - All algorithms use iteration
2. **Bounded loops** - All loops have explicit upper bounds
3. **No dynamic allocation after init** - Memory allocated during setup only
4. **Functions under 60 lines** - Each function is a logical unit
5. **Minimum 2 assertions per function** - Defensive programming
6. **Minimal scope** - Variables declared at smallest scope
7. **Return values checked** - All errors propagated
8. **Limited preprocessor** - No token pasting or recursion
9. **Single-level pointers** - No multiple indirection
10. **Zero warnings** - Compiles cleanly with all warnings enabled

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./pkg/storage
go test -v ./pkg/replay
go test -v ./pkg/analysis

# Run with race detector
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Project Structure

```
k8s-operator-replay-debugger/
├── cmd/
│   └── replay-cli/
│       ├── main.go           # CLI entry point
│       └── commands/         # Subcommands
│           ├── replay.go     # Replay operations
│           ├── analyze.go    # Analysis tools
│           └── record.go     # Recording info
├── pkg/
│   ├── recorder/             # Recording client
│   │   └── client.go
│   ├── replay/               # Replay engine
│   │   └── engine.go
│   ├── storage/              # Database layer
│   │   ├── types.go
│   │   └── database.go
│   └── analysis/             # Analysis tools
│       └── analyzer.go
├── internal/
│   └── assert/               # Assertion utilities
│       └── assert.go
├── go.mod
├── go.sum
├── README.md
└── ARCHITECTURE.md
```

## Configuration

### Environment Variables

```bash
# Database path
export REPLAY_DB_PATH="recordings.db"

# Maximum operations per session
export REPLAY_MAX_OPS=1000000

# Slow operation threshold (ms)
export REPLAY_SLOW_THRESHOLD=1000
```

## Use Cases

### Debug Production Issues

Record operations in production, then replay locally to investigate:

```bash
# In production
recordingClient.Enable()
# ... issue occurs ...
recordingClient.Disable()

# Copy recordings.db to local machine
# Replay locally
./replay-cli replay prod-issue-123 -i
```

### Performance Analysis

Find slow operations causing bottlenecks:

```bash
./replay-cli analyze session-001 --slow --threshold 500
```

### Loop Detection

Identify infinite reconciliation loops:

```bash
./replay-cli analyze session-001 --loops --window 10
```

### Error Pattern Analysis

Understand error frequency and types:

```bash
./replay-cli analyze session-001 --errors
```

### JSON Export for Automation

Generate machine-readable analysis reports for CI/CD pipelines:

```bash
# Export all analysis as JSON
./replay-cli analyze session-001 --format json > analysis.json

# Only export slow operations analysis
./replay-cli analyze session-001 --slow --no-loops --no-errors --format json
```

Example JSON output:
```json
{
  "session_id": "session-001",
  "total_operations": 100,
  "slow_operations": [
    {
      "index": 5,
      "type": "UPDATE",
      "resource": "Deployment/production/app",
      "duration_ms": 2000
    }
  ],
  "loops_detected": [
    {
      "start_index": 10,
      "end_index": 30,
      "repeat_count": 3,
      "description": "Repeated Pod operations"
    }
  ],
  "errors": {
    "total": 3,
    "by_type": {
      "GET": 2,
      "UPDATE": 1
    }
  }
}
```

## Limitations

- SQLite-based storage (single file, not distributed)
- Maximum 1M operations per session by default
- No real-time streaming (batch recording)
- Resource data limited to 1MB per operation
- Requires CGO for SQLite (not pure Go)

## Contributing

Contributions must follow the safety-critical coding standards:

1. All functions under 60 lines
2. Minimum 2 assertions per function
3. All loops explicitly bounded
4. No recursion
5. Zero compiler warnings
6. Tests for all new functionality

## License

MIT License - See LICENSE file for details

## References

- [JPL Power of 10 Rules](https://en.wikipedia.org/wiki/The_Power_of_10:_Rules_for_Developing_Safety-Critical_Code)
- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
