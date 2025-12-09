# Project Index - Quick Navigation

Welcome to the Kubernetes Operator Replay Debugger. This index helps you find what you need quickly.

## Start Here

**New to this project?**
→ Read [GETTING_STARTED.md](GETTING_STARTED.md) - 5-minute quickstart

**Ready to install?**
→ Follow [INSTALL_LINUX_MINT.md](INSTALL_LINUX_MINT.md) - Step-by-step setup

**Want the full picture?**
→ Check [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md) - Complete overview

## Documentation Files

### User Documentation

| File | Purpose | Read Time |
|------|---------|-----------|
| [GETTING_STARTED.md](GETTING_STARTED.md) | 5-minute quickstart guide | 5 min |
| [README.md](README.md) | Complete user manual | 15 min |
| [INSTALL_LINUX_MINT.md](INSTALL_LINUX_MINT.md) | Detailed installation | 10 min |
| [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md) | What was built and why | 10 min |

### Technical Documentation

| File | Purpose | Audience |
|------|---------|----------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | System design | Developers |
| [SAFETY_COMPLIANCE.md](SAFETY_COMPLIANCE.md) | Code quality verification | Technical leads |

### Build Files

| File | Purpose |
|------|---------|
| [Makefile](Makefile) | Build automation commands |
| [setup.sh](setup.sh) | Automated installation script |
| [go.mod](go.mod) | Go dependencies |

## Source Code Files

### Core Packages (pkg/)

**Recording**
- `pkg/recorder/client.go` - Records Kubernetes API operations

**Replay**
- `pkg/replay/engine.go` - Replays recorded operations with time-travel
- `pkg/replay/engine_test.go` - Tests for replay engine

**Storage**
- `pkg/storage/types.go` - Data structures and schema
- `pkg/storage/database.go` - SQLite database operations
- `pkg/storage/database_test.go` - Database tests

**Analysis**
- `pkg/analysis/analyzer.go` - Detects loops, errors, slow operations

**Utilities**
- `internal/assert/assert.go` - Safety assertions

### CLI Application (cmd/)

- `cmd/replay-cli/main.go` - Entry point
- `cmd/replay-cli/commands/replay.go` - Replay command
- `cmd/replay-cli/commands/analyze.go` - Analyze command
- `cmd/replay-cli/commands/record.go` - Record info command

### Examples (examples/)

- `examples/create_sample.go` - Generate sample database
- `examples/operator_integration.go` - Integration example

## Quick Command Reference

### Installation
```bash
chmod +x setup.sh
./setup.sh
```

### Build
```bash
make build          # Build binary
make test           # Run tests
make clean          # Clean up
```

### Usage
```bash
# Create sample data
go run examples/create_sample.go

# List sessions
./replay-cli sessions -d sample_recordings.db

# Replay
./replay-cli replay sample-session-001 -d sample_recordings.db

# Interactive
./replay-cli replay sample-session-001 -d sample_recordings.db -i

# Analyze
./replay-cli analyze sample-session-002 -d sample_recordings.db
```

## File Tree

```
k8s-operator-replay-debugger/
│
├── Documentation (Start here)
│   ├── GETTING_STARTED.md         ← Read this first
│   ├── INSTALL_LINUX_MINT.md      ← Installation guide
│   ├── README.md                  ← Full documentation
│   ├── PROJECT_SUMMARY.md         ← Overview
│   ├── ARCHITECTURE.md            ← System design
│   └── SAFETY_COMPLIANCE.md       ← Code quality
│
├── Build System
│   ├── Makefile                   ← Build commands
│   ├── setup.sh                   ← Automated setup
│   └── go.mod                     ← Dependencies
│
├── Source Code
│   ├── cmd/replay-cli/            ← CLI application
│   │   ├── main.go
│   │   └── commands/
│   │       ├── replay.go
│   │       ├── analyze.go
│   │       └── record.go
│   │
│   ├── pkg/                       ← Core packages
│   │   ├── recorder/              ← Recording
│   │   ├── replay/                ← Replay engine
│   │   ├── storage/               ← Database
│   │   └── analysis/              ← Analysis tools
│   │
│   └── internal/assert/           ← Utilities
│
└── Examples
    ├── create_sample.go           ← Generate test data
    └── operator_integration.go    ← Integration example
```

## Reading Path for Different Roles

### For End Users
1. GETTING_STARTED.md - Understand what this does
2. INSTALL_LINUX_MINT.md - Install it
3. Try the examples - Get hands-on
4. README.md - Learn all features

### For Developers
1. PROJECT_SUMMARY.md - Understand scope
2. ARCHITECTURE.md - Learn design
3. Source code in pkg/ - See implementation
4. SAFETY_COMPLIANCE.md - Understand constraints

### For Operators Engineers
1. GETTING_STARTED.md - Quick intro
2. examples/operator_integration.go - Integration pattern
3. README.md (Integration section) - Detailed usage
4. Start recording your operator

### For Technical Leads
1. PROJECT_SUMMARY.md - Complete overview
2. SAFETY_COMPLIANCE.md - Quality verification
3. ARCHITECTURE.md - Design decisions
4. Test files - Coverage and quality

## Statistics

- **Total files**: 22 (Go, Markdown, Shell, Make)
- **Go source code**: ~2,500 lines (excluding tests)
- **Test code**: ~700 lines
- **Documentation**: ~50 KB
- **Dependencies**: 12 (all standard Go/K8s libraries)
- **Build time**: ~30 seconds
- **Test time**: ~5 seconds

## Key Features

- [x] Record K8s API operations
- [x] Replay with time-travel (forward/backward)
- [x] Detect infinite loops
- [x] Find slow operations
- [x] Analyze error patterns
- [x] Interactive CLI
- [x] SQLite storage
- [x] Safety-critical code (JPL Power of 10)
- [x] Zero runtime dependencies
- [x] Comprehensive tests
- [x] Complete documentation

## Support Flow

1. **Problem?** → Check GETTING_STARTED.md
2. **Installation issue?** → See INSTALL_LINUX_MINT.md
3. **Usage question?** → Read README.md
4. **Want examples?** → Run examples/create_sample.go
5. **Integration help?** → See examples/operator_integration.go

## Version Information

- **Version**: 1.0.0
- **Go Version**: 1.21+
- **Status**: Production Ready
- **License**: MIT
- **Safety Standard**: JPL Power of 10 Compliant

## Quick Links

- Main documentation: [README.md](README.md)
- Quickstart: [GETTING_STARTED.md](GETTING_STARTED.md)
- Installation: [INSTALL_LINUX_MINT.md](INSTALL_LINUX_MINT.md)
- Summary: [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md)
- Architecture: [ARCHITECTURE.md](ARCHITECTURE.md)
- Safety: [SAFETY_COMPLIANCE.md](SAFETY_COMPLIANCE.md)

---

**Next Step**: Open [GETTING_STARTED.md](GETTING_STARTED.md) and follow the 5-minute setup.
