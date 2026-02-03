# KubeStep

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Safety Critical](https://img.shields.io/badge/Safety-JPL%20Power%20of%2010-green)](SAFETY_COMPLIANCE.md)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()
[![CI](https://github.com/slyt3/kubestep/actions/workflows/ci.yml/badge.svg)](https://github.com/slyt3/kubestep/actions/workflows/ci.yml)

KubeStep records what your operator did in Kubernetes so you can replay it and see what happened.

## What you can do

- Record Kubernetes API calls made by your controllers
- Replay those calls step by step
- Analyze for loops, slow calls, and errors
- Build simple cross-controller causality chains

## Quick start

```bash
git clone https://github.com/slyt3/kubestep
cd kubestep
go mod download
go build -o kubestep ./cmd/kubestep
```

Create a sample database and try the CLI:

```bash
go run examples/create_sample.go
./kubestep sessions -d sample_recordings.db
./kubestep replay sample-session-001 -d sample_recordings.db
./kubestep analyze sample-session-002 -d sample_recordings.db
```

## Causality (A -> B -> C)

```bash
./kubestep analyze causality --session prod-deployment-001 -d recordings.db
```

JSON graph output:

```bash
./kubestep analyze causality --session prod-deployment-001 -d recordings.db --format json
```

## Architecture

```
┌───────────────────────────────┐        ┌──────────────────────┐
│  Your Operator / Controllers  │        │   Replay + Analysis  │
│  (client-go calls + reconcile)│        │  (CLI: kubestep)      │
└───────────────┬───────────────┘        └──────────┬───────────┘
                │                                   │
                │ record (operations + spans)       │ read + analyze
                ▼                                   ▼
       ┌──────────────────────┐           ┌──────────────────────┐
       │   Recorder Wrapper   │           │   Analyzer / Graph   │
       │  (lightweight layer) │           │  loops + causality   │
       └──────────┬───────────┘           └──────────┬───────────┘
                  │                                   │
                  ▼                                   ▼
           ┌───────────────────────────────────────────────────┐
           │          Storage (SQLite or MongoDB)               │
           │  operations: who/what/when/object/version          │
           │  spans: reconcile triggers + writes                │
           └───────────────────────────────────────────────────┘
```

## Use in your operator

```go
db, _ := storage.NewDatabase("recordings.db", 1000000)
client, _ := recorder.NewRecordingClient(recorder.Config{
    Client:    k8sClient,
    Database:  db,
    SessionID: "prod-deployment-001",
    ActorID:   "my-operator/controller-a",
})
_ = client
```

## Docs

- `GETTING_STARTED.md`
- `ARCHITECTURE.md`

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
./kubestep replay prod-issue-123 -i
```

### Performance Analysis

Find slow operations causing bottlenecks:

```bash
./kubestep analyze session-001 --slow --threshold 500
```

### Loop Detection

Identify infinite reconciliation loops:

```bash
./kubestep analyze session-001 --loops --window 10
```

### Error Pattern Analysis

Understand error frequency and types:

```bash
./kubestep analyze session-001 --errors
```

### JSON Export for Automation

Generate machine-readable analysis reports for CI/CD pipelines:

```bash
# Export all analysis as JSON
./kubestep analyze session-001 --format json > analysis.json

# Only export slow operations analysis
./kubestep analyze session-001 --slow --no-loops --no-errors --format json
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

## Limitations (working on this one xd)

- SQLite-based storage (single file, not distributed)
- Maximum 1M operations per session by default
- No real-time streaming (batch recording)
- Resource data limited to 1MB per operation
- Requires CGO for SQLite (not pure Go)


## License

MIT License - See LICENSE file for details

## References

- [JPL Power of 10 Rules](https://en.wikipedia.org/wiki/The_Power_of_10:_Rules_for_Developing_Safety-Critical_Code)
- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
