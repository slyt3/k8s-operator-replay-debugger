# KubeStep - Architecture

## Overview
Tool for recording, replaying, and analyzing Kubernetes operator reconciliation loops.
Helps debug operator behavior by capturing all API interactions and replaying them.

## Three-Phase Architecture

### Phase 1: Recording Mode
- Intercepts all Kubernetes API calls (GET, UPDATE, PATCH, DELETE, CREATE)
- Records events, resource states, and timing information
- Stores in SQLite database (embedded) or MongoDB (scalable)
- Transparent wrapper around k8s client library

### Phase 2: Replay Mode
- Mocked Kubernetes API server
- Feeds recorded events back to operator
- Step-by-step execution control
- Time travel capabilities (forward/backward)
- Breakpoint support

### Phase 3: Analysis Mode
- State diff visualization at each step
- Infinite loop detection
- Race condition identification
- Performance bottleneck analysis
- Statistical summaries

## Technology Stack
- Language: Go (primary k8s operator language)
- Storage: SQLite (embedded, zero dependencies) or MongoDB (scalable, distributed)
- K8s Client: client-go library
- Testing: Go standard testing + testify
- CLI: Cobra framework for command-line interface

## Storage Architecture

### Interface Abstraction
- `OperationStore` interface enables pluggable storage backends
- Unified API: `InsertOperation`, `QueryOperations`, `ListSessions`
- Factory pattern for backend selection based on configuration

### SQLite Backend
- **Use Case**: Development, testing, single-node deployments
- **Benefits**: Zero dependencies, embedded, file-based
- **Performance**: Excellent for moderate workloads (< 1M operations)
- **Deployment**: Simple binary distribution

### MongoDB Backend  
- **Use Case**: Production, multi-node, large-scale deployments
- **Benefits**: Horizontal scaling, replication, clustering
- **Performance**: Optimized for high-throughput workloads
- **Indexing**: Automatic indexing on session_id and sequence_number
- **Features**: BSON document storage, aggregation pipelines

### Backend Selection
```bash
# SQLite (default)
./kubestep analyze session-001 --storage sqlite -d recordings.db

# MongoDB
./kubestep analyze session-001 --storage mongodb \
    --mongo-uri "mongodb://localhost:27017" \
    --mongo-db "kubestep"
```

## Safety-Critical Compliance
Following JPL Power of 10 rules:
- No recursion
- All loops bounded
- No dynamic allocation after init
- Functions under 60 lines
- Minimum 2 assertions per function
- Minimal scope for variables
- All return values checked
- Limited preprocessor use
- Single-level pointer dereferencing
- Zero compiler warnings
