//go:build ignore

package main

import (
	"fmt"
	"time"

	"github.com/slyt3/kubestep/pkg/storage"
)

const (
	sampleDBPath = "sample_recordings.db"
	maxSampleOps = 20
)

func main() {
	fmt.Println("Creating sample database...")

	db, err := storage.NewDatabase(sampleDBPath, 1000000)
	if err != nil {
		fmt.Printf("Failed to create database: %v\n", err)
		return
	}
	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close database: %v\n", closeErr)
		}
	}()

	err = createNormalSession(db)
	if err != nil {
		fmt.Printf("Failed to create normal session: %v\n", err)
		return
	}

	err = createSlowSession(db)
	if err != nil {
		fmt.Printf("Failed to create slow session: %v\n", err)
		return
	}

	err = createErrorSession(db)
	if err != nil {
		fmt.Printf("Failed to create error session: %v\n", err)
		return
	}

	fmt.Println("\nSample database created: sample_recordings.db")
	fmt.Println("\nTry these commands:")
	fmt.Println("  ./kubestep sessions -d sample_recordings.db")
	fmt.Println("  ./kubestep replay sample-session-001 -d sample_recordings.db")
	fmt.Println("  ./kubestep replay sample-session-001 -d sample_recordings.db -i")
	fmt.Println("  ./kubestep analyze sample-session-002 -d sample_recordings.db")
}

func createNormalSession(db *storage.Database) error {
	sessionID := "sample-session-001"
	fmt.Printf("Creating session: %s (normal operations)\n", sessionID)

	count := 0
	for count < maxSampleOps && count < 10 {
		op := &storage.Operation{
			SessionID:      sessionID,
			SequenceNumber: int64(count + 1),
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           fmt.Sprintf("test-pod-%d", count),
			ResourceData:   fmt.Sprintf(`{"kind":"Pod","metadata":{"name":"test-pod-%d"}}`, count),
			DurationMs:     int64(100 + count*10),
		}

		err := db.InsertOperation(op)
		if err != nil {
			return fmt.Errorf("failed to insert operation: %w", err)
		}

		count = count + 1
	}

	fmt.Printf("  Created %d operations\n", count)
	return nil
}

func createSlowSession(db *storage.Database) error {
	sessionID := "sample-session-002"
	fmt.Printf("Creating session: %s (with slow operations)\n", sessionID)

	count := 0
	for count < maxSampleOps && count < 15 {
		duration := int64(100)
		if count%3 == 0 {
			duration = 2000
		}

		op := &storage.Operation{
			SessionID:      sessionID,
			SequenceNumber: int64(count + 1),
			Timestamp:      time.Now(),
			OperationType:  storage.OperationUpdate,
			ResourceKind:   "Deployment",
			Namespace:      "production",
			Name:           fmt.Sprintf("app-deploy-%d", count),
			ResourceData:   fmt.Sprintf(`{"kind":"Deployment","metadata":{"name":"app-deploy-%d"}}`, count),
			DurationMs:     duration,
		}

		err := db.InsertOperation(op)
		if err != nil {
			return fmt.Errorf("failed to insert operation: %w", err)
		}

		count = count + 1
	}

	fmt.Printf("  Created %d operations (5 slow)\n", count)
	return nil
}

func createErrorSession(db *storage.Database) error {
	sessionID := "sample-session-003"
	fmt.Printf("Creating session: %s (with errors)\n", sessionID)

	count := 0
	for count < maxSampleOps && count < 12 {
		errorMsg := ""
		if count%4 == 0 {
			errorMsg = "resource not found"
		}

		op := &storage.Operation{
			SessionID:      sessionID,
			SequenceNumber: int64(count + 1),
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "ConfigMap",
			Namespace:      "default",
			Name:           fmt.Sprintf("config-%d", count),
			ResourceData:   fmt.Sprintf(`{"kind":"ConfigMap","metadata":{"name":"config-%d"}}`, count),
			Error:          errorMsg,
			DurationMs:     int64(150),
		}

		err := db.InsertOperation(op)
		if err != nil {
			return fmt.Errorf("failed to insert operation: %w", err)
		}

		count = count + 1
	}

	fmt.Printf("  Created %d operations (3 errors)\n", count)
	return nil
}
