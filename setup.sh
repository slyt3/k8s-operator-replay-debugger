#!/bin/bash

set -e

echo "=================================================="
echo "K8s Operator Replay Debugger - Setup Script"
echo "=================================================="
echo ""

check_prerequisites() {
    echo "Checking prerequisites..."
    
    if ! command -v go &> /dev/null; then
        echo "Error: Go is not installed"
        echo "Install Go 1.21 or later from https://go.dev/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Found Go version: $GO_VERSION"
    
    if ! command -v gcc &> /dev/null; then
        echo "Error: GCC is not installed"
        echo "Run: sudo apt-get install build-essential"
        exit 1
    fi
    
    echo "GCC found: $(gcc --version | head -n1)"
    
    if ! pkg-config --exists sqlite3 2>/dev/null; then
        echo "Warning: SQLite3 development libraries not found"
        echo "Installing: sudo apt-get install libsqlite3-dev"
        sudo apt-get update
        sudo apt-get install -y libsqlite3-dev
    fi
    
    echo "Prerequisites OK"
    echo ""
}

download_dependencies() {
    echo "Downloading Go dependencies..."
    go mod download
    
    if [ $? -ne 0 ]; then
        echo "Error: Failed to download dependencies"
        exit 1
    fi
    
    echo "Dependencies downloaded"
    echo ""
}

build_cli() {
    echo "Building replay-cli binary..."
    
    go build -v -o replay-cli ./cmd/replay-cli
    
    if [ $? -ne 0 ]; then
        echo "Error: Build failed"
        exit 1
    fi
    
    chmod +x replay-cli
    echo "Build successful: ./replay-cli"
    echo ""
}

run_tests() {
    echo "Running tests..."
    
    go test ./... -v
    
    if [ $? -ne 0 ]; then
        echo "Warning: Some tests failed"
        echo "This may be expected if k8s dependencies are not configured"
    else
        echo "All tests passed"
    fi
    echo ""
}

verify_build() {
    echo "Verifying build..."
    
    if [ ! -f "./replay-cli" ]; then
        echo "Error: replay-cli binary not found"
        exit 1
    fi
    
    ./replay-cli --version
    
    if [ $? -ne 0 ]; then
        echo "Error: Binary verification failed"
        exit 1
    fi
    
    echo "Build verified"
    echo ""
}

create_sample_database() {
    echo "Creating sample database..."
    
    cat > create_sample.go << 'EOF'
package main

import (
    "fmt"
    "time"
    "github.com/operator-replay-debugger/pkg/storage"
)

func main() {
    db, err := storage.NewDatabase("sample_recordings.db", 1000000)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    sessionID := "sample-session-001"
    
    for i := 0; i < 10; i++ {
        op := &storage.Operation{
            SessionID:      sessionID,
            SequenceNumber: int64(i + 1),
            Timestamp:      time.Now(),
            OperationType:  storage.OperationGet,
            ResourceKind:   "Pod",
            Namespace:      "default",
            Name:           fmt.Sprintf("test-pod-%d", i),
            ResourceData:   fmt.Sprintf(`{"kind":"Pod","metadata":{"name":"test-pod-%d"}}`, i),
            DurationMs:     int64(100 + i*10),
        }
        
        err = db.InsertOperation(op)
        if err != nil {
            panic(err)
        }
    }
    
    fmt.Println("Sample database created: sample_recordings.db")
    fmt.Println("Try: ./replay-cli sessions -d sample_recordings.db")
    fmt.Println("     ./replay-cli replay sample-session-001 -d sample_recordings.db")
}
EOF

    go run create_sample.go
    rm create_sample.go
    echo ""
}

print_usage() {
    echo "=================================================="
    echo "Setup Complete!"
    echo "=================================================="
    echo ""
    echo "Quick Start:"
    echo ""
    echo "1. View available sessions:"
    echo "   ./replay-cli sessions -d sample_recordings.db"
    echo ""
    echo "2. Replay operations:"
    echo "   ./replay-cli replay sample-session-001 -d sample_recordings.db"
    echo ""
    echo "3. Interactive replay:"
    echo "   ./replay-cli replay sample-session-001 -d sample_recordings.db -i"
    echo ""
    echo "4. Analyze operations:"
    echo "   ./replay-cli analyze sample-session-001 -d sample_recordings.db"
    echo ""
    echo "5. View help:"
    echo "   ./replay-cli --help"
    echo ""
    echo "Documentation: See README.md"
    echo "=================================================="
}

main() {
    check_prerequisites
    download_dependencies
    build_cli
    run_tests
    verify_build
    create_sample_database
    print_usage
}

main
