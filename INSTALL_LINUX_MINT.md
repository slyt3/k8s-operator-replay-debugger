# Installation Guide for Linux Mint

Complete step-by-step guide to install and run the Kubernetes Operator Replay Debugger on Linux Mint.

## Prerequisites Installation

### 1. Install Go

```bash
# Download Go 1.21 or later
cd /tmp
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz

# Remove old installation if exists
sudo rm -rf /usr/local/go

# Extract new installation
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc for permanent)
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Verify installation
go version
```

### 2. Install Build Tools

```bash
# Install GCC and build essentials
sudo apt-get update
sudo apt-get install -y build-essential

# Install SQLite development libraries
sudo apt-get install -y libsqlite3-dev

# Verify installations
gcc --version
pkg-config --cflags sqlite3
```

### 3. Install Git (if not installed)

```bash
sudo apt-get install -y git
git --version
```

## Project Setup

### 1. Clone or Download Project

If you have the project files:

```bash
cd ~
# Project files should be in: ~/k8s-operator-replay-debugger
```

If downloading from repository:

```bash
cd ~
git clone https://github.com/your-org/k8s-operator-replay-debugger
cd k8s-operator-replay-debugger
```

### 2. Run Automated Setup

```bash
cd ~/k8s-operator-replay-debugger

# Make setup script executable
chmod +x setup.sh

# Run setup
./setup.sh
```

The setup script will:
- Check all prerequisites
- Download Go dependencies
- Build the CLI binary
- Run tests
- Create a sample database
- Display usage instructions

### 3. Manual Build (Alternative)

If you prefer manual steps:

```bash
cd ~/k8s-operator-replay-debugger

# Download dependencies
go mod download

# Build the binary
go build -o replay-cli ./cmd/replay-cli

# Make it executable
chmod +x replay-cli

# Verify build
./replay-cli --version
```

### 4. Run Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./pkg/storage
go test -v ./pkg/replay
```

## Quick Start

### 1. Create Sample Database

```bash
# Create sample recordings
go run examples/create_sample.go
```

This creates `sample_recordings.db` with three sessions:
- sample-session-001: Normal operations
- sample-session-002: Operations with slow queries
- sample-session-003: Operations with errors

### 2. List Sessions

```bash
./replay-cli sessions -d sample_recordings.db
```

### 3. Replay Operations

```bash
# Automatic replay (runs all operations)
./replay-cli replay sample-session-001 -d sample_recordings.db

# Interactive replay (step through operations)
./replay-cli replay sample-session-001 -d sample_recordings.db -i
```

Interactive commands:
- `n` - step forward to next operation
- `b` - step backward to previous operation
- `r` - reset to beginning
- `s` - show statistics
- `q` - quit

### 4. Analyze Operations

```bash
# Full analysis (loops, slow ops, errors)
./replay-cli analyze sample-session-002 -d sample_recordings.db

# Detect only loops
./replay-cli analyze sample-session-001 -d sample_recordings.db --loops

# Find slow operations with custom threshold
./replay-cli analyze sample-session-002 -d sample_recordings.db --slow --threshold 1500
```

## Makefile Commands

The project includes a Makefile for common tasks:

```bash
# Show all available commands
make help

# Build the binary
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detector
make test-race

# Clean build artifacts
make clean

# Format code
make fmt

# Run linting
make lint

# Verify safety-critical compliance
make verify

# Install to GOPATH/bin
make install
```

## Troubleshooting

### Go Not Found

```bash
# Check Go installation
which go

# If not found, add to PATH
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### CGO Errors

```bash
# Install SQLite development libraries
sudo apt-get install -y libsqlite3-dev

# Verify installation
pkg-config --cflags sqlite3
```

### Build Fails

```bash
# Clean and rebuild
make clean
go mod tidy
make build
```

### Permission Denied

```bash
# Make scripts executable
chmod +x setup.sh
chmod +x replay-cli
```

## Integrating with Your Operator

### 1. Add as Dependency

```bash
# In your operator project
go get github.com/operator-replay-debugger
```

### 2. Import Packages

```go
import (
    "github.com/operator-replay-debugger/pkg/recorder"
    "github.com/operator-replay-debugger/pkg/storage"
)
```

### 3. Wrap Client

See `examples/operator_integration.go` for complete example:

```go
// Create database
db, err := storage.NewDatabase("recordings.db", 1000000)
if err != nil {
    panic(err)
}
defer db.Close()

// Wrap your Kubernetes client
recordingClient, err := recorder.NewRecordingClient(recorder.Config{
    Client:      k8sClient,
    Database:    db,
    SessionID:   "my-operator-session",
    MaxSequence: 1000000,
})

// Use recording client in reconciliation
pod, err := recordingClient.RecordGet(ctx, "Pod", "default", "my-pod", metav1.GetOptions{})
```

## File Locations

After installation, you will have:

```
~/k8s-operator-replay-debugger/
├── replay-cli                     # Main binary
├── sample_recordings.db           # Sample database
├── pkg/                          # Source packages
├── cmd/                          # CLI application
├── examples/                     # Example programs
├── README.md                     # Documentation
└── Makefile                      # Build commands
```

## Environment Variables

Optional configuration:

```bash
# Set default database path
export REPLAY_DB_PATH="recordings.db"

# Set maximum operations
export REPLAY_MAX_OPS=1000000

# Set slow operation threshold
export REPLAY_SLOW_THRESHOLD=1000
```

Add to ~/.bashrc for persistence:

```bash
echo 'export REPLAY_DB_PATH="recordings.db"' >> ~/.bashrc
source ~/.bashrc
```

## Uninstallation

```bash
# Remove binary
rm ~/k8s-operator-replay-debugger/replay-cli

# Remove databases
rm ~/k8s-operator-replay-debugger/*.db

# Remove entire project
rm -rf ~/k8s-operator-replay-debugger
```

## Next Steps

1. Read the full documentation in README.md
2. Explore examples in examples/
3. Integrate with your Kubernetes operator
4. Record production operations
5. Replay and analyze locally

## Support

For issues or questions:
1. Check README.md
2. Review examples/
3. Run `./replay-cli --help`
4. Check logs for error details
