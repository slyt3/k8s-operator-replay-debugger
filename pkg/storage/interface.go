package storage

import (
	"context"
	"fmt"

	"github.com/operator-replay-debugger/internal/assert"
)

// OperationStore defines the interface for storing and retrieving operations.
type OperationStore interface {
	InsertOperation(op *Operation) error
	QueryOperations(sessionID string) ([]Operation, error)
	QueryOperationsByRange(sessionID string, start, end int64) ([]Operation, error)
	ListSessions() ([]SessionInfo, error)
	Close() error
}

// SessionInfo holds basic session metadata.
type SessionInfo struct {
	SessionID   string
	StartTime   int64
	EndTime     int64
	OpCount     int64
	Description string
}

// StorageConfig holds configuration for storage backends.
type StorageConfig struct {
	Type           string // "sqlite" or "mongodb"
	ConnectionURI  string
	DatabaseName   string
	CollectionName string
	MaxOperations  int
	Context        context.Context
}

// NewOperationStore creates a new storage implementation based on config.
// Multiple assertions for validation.
func NewOperationStore(cfg StorageConfig) (OperationStore, error) {
	err := validateStorageConfig(&cfg)
	if err != nil {
		return nil, err
	}

	switch cfg.Type {
	case "sqlite":
		return NewSQLiteStore(cfg)
	case "mongodb":
		return NewMongoStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}

// validateStorageConfig validates storage configuration.
// Multiple assertions for validation.
func validateStorageConfig(cfg *StorageConfig) error {
	err := assert.AssertNotNil(cfg, "storage config")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(cfg.Type, "storage type")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(cfg.ConnectionURI, "connection URI")
	if err != nil {
		return err
	}

	err = assert.AssertInRange(
		cfg.MaxOperations,
		1,
		1000000, // defaultMaxOperations
		"max operations",
	)
	if err != nil {
		return err
	}

	return nil
}