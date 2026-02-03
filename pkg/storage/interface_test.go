package storage

import "testing"

func TestValidateStorageConfig(t *testing.T) {
	cfg := &StorageConfig{}
	if err := validateStorageConfig(cfg); err == nil {
		t.Fatalf("expected error for empty config")
	}

	cfg = &StorageConfig{Type: "sqlite", ConnectionURI: "", MaxOperations: 1}
	if err := validateStorageConfig(cfg); err == nil {
		t.Fatalf("expected error for empty connection URI")
	}

	cfg = &StorageConfig{Type: "sqlite", ConnectionURI: "db", MaxOperations: 0}
	if err := validateStorageConfig(cfg); err == nil {
		t.Fatalf("expected error for max operations")
	}
}

func TestNewOperationStoreUnsupportedType(t *testing.T) {
	_, err := NewOperationStore(StorageConfig{
		Type:          "unknown",
		ConnectionURI: "db",
		MaxOperations: 1,
	})
	if err == nil {
		t.Fatalf("expected error for unsupported storage type")
	}
}
