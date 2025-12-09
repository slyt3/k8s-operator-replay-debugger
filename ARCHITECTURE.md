# Kubernetes Operator Replay Debugger - Architecture

## Overview
Tool for recording, replaying, and analyzing Kubernetes operator reconciliation loops.
Helps debug operator behavior by capturing all API interactions and replaying them.

## Three-Phase Architecture

### Phase 1: Recording Mode
- Intercepts all Kubernetes API calls (GET, UPDATE, PATCH, DELETE, CREATE)
- Records events, resource states, and timing information
- Stores in SQLite database for persistence
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
- Storage: SQLite (embedded, no external dependencies)
- K8s Client: client-go library
- Testing: Go standard testing + testify

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
