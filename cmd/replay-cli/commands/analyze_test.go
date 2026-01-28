package commands

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/operator-replay-debugger/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAnalyzeDBPath = "test_analyze.db"
	testMaxOps        = 1000
)

// cleanupAnalyzeTestDB removes test database.
func cleanupAnalyzeTestDB(t *testing.T) {
	err := os.Remove(testAnalyzeDBPath)
	if err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: failed to remove test db: %v", err)
	}
}

// createTestDB creates a test database with sample operations.
func createTestDB(t *testing.T, sessionID string, ops []storage.Operation) *storage.Database {
	cleanupAnalyzeTestDB(t)

	db, err := storage.NewDatabase(testAnalyzeDBPath, testMaxOps)
	require.NoError(t, err, "database creation should succeed")

	for i := 0; i < len(ops); i = i + 1 {
		err = db.InsertOperation(&ops[i])
		require.NoError(t, err, "insert should succeed")
	}

	return db
}

// createTestOperations creates sample operations for testing.
func createTestOperations(sessionID string, count int) []storage.Operation {
	ops := make([]storage.Operation, 0, count)

	maxOps := count
	if maxOps > 100 {
		maxOps = 100
	}

	for i := 0; i < maxOps; i = i + 1 {
		op := storage.Operation{
			SessionID:      sessionID,
			SequenceNumber: int64(i + 1),
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "test-pod",
			ResourceData:   `{"kind":"Pod"}`,
			DurationMs:     100,
		}
		ops = append(ops, op)
	}

	return ops
}

// TestValidateAnalyzeConfig tests configuration validation.
func TestValidateAnalyzeConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *AnalyzeConfig
		wantErr bool
	}{
		{
			name: "valid text format",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    10,
				SlowThreshold: 1000,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: false,
		},
		{
			name: "valid json format",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    10,
				SlowThreshold: 1000,
				Format:        "json",
				StorageType:   "sqlite",
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    10,
				SlowThreshold: 1000,
				Format:        "xml",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
		{
			name: "empty database path",
			cfg: &AnalyzeConfig{
				DatabasePath:  "",
				SessionID:     "test-session",
				LoopWindow:    10,
				SlowThreshold: 1000,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
		{
			name: "empty session id",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "",
				LoopWindow:    10,
				SlowThreshold: 1000,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
		{
			name: "invalid loop window too small",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    1,
				SlowThreshold: 1000,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
		{
			name: "invalid loop window too large",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    101,
				SlowThreshold: 1000,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
		{
			name: "invalid threshold too small",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    10,
				SlowThreshold: 0,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
		{
			name: "invalid threshold too large",
			cfg: &AnalyzeConfig{
				DatabasePath:  "/path/to/db",
				SessionID:     "test-session",
				LoopWindow:    10,
				SlowThreshold: 1000001,
				Format:        "text",
				StorageType:   "sqlite",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAnalyzeConfig(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestOutputJSONBasic tests basic JSON output.
func TestOutputJSONBasic(t *testing.T) {
	sessionID := "test-session-json-basic"
	ops := createTestOperations(sessionID, 5)

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 200,
		LoopWindow:    10,
		Format:        "json",
	}

	err := outputJSON(cfg, ops)
	assert.NoError(t, err, "outputJSON should succeed")
}

// TestOutputJSONSlowOperations tests JSON output with slow operations.
func TestOutputJSONSlowOperations(t *testing.T) {
	sessionID := "test-session-slow"
	ops := createTestOperations(sessionID, 10)

	ops[2].DurationMs = 1500
	ops[5].DurationMs = 2000
	ops[8].DurationMs = 1800

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      true,
		DetectLoops:   false,
		AnalyzeErrors: false,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := outputJSON(cfg, ops)
	assert.NoError(t, err, "outputJSON with slow ops should succeed")
}

// TestOutputJSONLoopDetection tests JSON output with loop detection.
func TestOutputJSONLoopDetection(t *testing.T) {
	sessionID := "test-session-loops"
	ops := make([]storage.Operation, 0, 30)

	for i := 0; i < 3; i = i + 1 {
		for j := 0; j < 10; j = j + 1 {
			op := storage.Operation{
				SessionID:      sessionID,
				SequenceNumber: int64(i*10 + j + 1),
				Timestamp:      time.Now(),
				OperationType:  storage.OperationGet,
				ResourceKind:   "Pod",
				Namespace:      "default",
				Name:           "loop-pod",
				ResourceData:   `{"kind":"Pod"}`,
				DurationMs:     100,
			}
			ops = append(ops, op)
		}
	}

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      false,
		DetectLoops:   true,
		AnalyzeErrors: false,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := outputJSON(cfg, ops)
	assert.NoError(t, err, "outputJSON with loops should succeed")
}

// TestOutputJSONErrorAnalysis tests JSON output with error analysis.
func TestOutputJSONErrorAnalysis(t *testing.T) {
	sessionID := "test-session-errors"
	ops := createTestOperations(sessionID, 10)

	ops[1].Error = "NotFound"
	ops[3].Error = "Forbidden"
	ops[6].Error = "NotFound"
	ops[8].Error = "Timeout"

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      false,
		DetectLoops:   false,
		AnalyzeErrors: true,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := outputJSON(cfg, ops)
	assert.NoError(t, err, "outputJSON with errors should succeed")
}

// TestOutputJSONAllDisabled tests JSON output with all analysis disabled.
func TestOutputJSONAllDisabled(t *testing.T) {
	sessionID := "test-session-disabled"
	ops := createTestOperations(sessionID, 5)

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      false,
		DetectLoops:   false,
		AnalyzeErrors: false,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := outputJSON(cfg, ops)
	assert.NoError(t, err, "outputJSON with all disabled should succeed")
}

// TestOutputJSONEmptyOperations tests JSON output with empty operations.
func TestOutputJSONEmptyOperations(t *testing.T) {
	sessionID := "test-session-empty"
	ops := make([]storage.Operation, 0)

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := outputJSON(cfg, ops)
	assert.NoError(t, err, "outputJSON with empty ops should succeed")
}

// TestJSONStructureValid tests that JSON output has valid structure.
func TestJSONStructureValid(t *testing.T) {
	sessionID := "test-session-structure"
	ops := createTestOperations(sessionID, 10)

	ops[2].DurationMs = 1500
	ops[5].Error = "NotFound"

	report := JSONAnalysisReport{
		SessionID:       sessionID,
		TotalOperations: len(ops),
		SlowOperations:  make([]JSONSlowOperation, 0),
		LoopsDetected:   make([]JSONLoopDetection, 0),
		Errors:          &JSONErrorSummary{ByType: make(map[string]int)},
	}

	report.SlowOperations = append(report.SlowOperations, JSONSlowOperation{
		Index:      2,
		Type:       "GET",
		Resource:   "Pod/default/test-pod",
		DurationMs: 1500,
	})

	report.Errors.Total = 1
	report.Errors.ByType["GET"] = 1

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err, "JSON marshaling should succeed")

	var decoded JSONAnalysisReport
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err, "JSON unmarshaling should succeed")

	assert.Equal(t, sessionID, decoded.SessionID)
	assert.Equal(t, len(ops), decoded.TotalOperations)
	assert.Equal(t, 1, len(decoded.SlowOperations))
	assert.Equal(t, int64(1500), decoded.SlowOperations[0].DurationMs)
	assert.Equal(t, 1, decoded.Errors.Total)
	assert.Equal(t, 1, decoded.Errors.ByType["GET"])
}

// TestOutputTextBasic tests basic text output.
func TestOutputTextBasic(t *testing.T) {
	sessionID := "test-session-text"
	ops := createTestOperations(sessionID, 5)

	cfg := &AnalyzeConfig{
		SessionID:     sessionID,
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 200,
		LoopWindow:    10,
		Format:        "text",
	}

	err := outputText(cfg, ops)
	assert.NoError(t, err, "outputText should succeed")
}

// TestRunAnalyzeWithDatabase tests full analyze command with database.
func TestRunAnalyzeWithDatabase(t *testing.T) {
	cleanupAnalyzeTestDB(t)
	defer cleanupAnalyzeTestDB(t)

	sessionID := "test-session-full"
	ops := createTestOperations(sessionID, 10)
	ops[2].DurationMs = 1500
	ops[5].Error = "NotFound"

	db := createTestDB(t, sessionID, ops)
	err := db.Close()
	require.NoError(t, err)

	cfg := &AnalyzeConfig{
		DatabasePath:  testAnalyzeDBPath,
		SessionID:     sessionID,
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
		StorageType:   "sqlite",
	}

	err = runAnalyze(cfg, []string{sessionID})
	assert.NoError(t, err, "runAnalyze should succeed")
}

// TestRunAnalyzeNonExistentSession tests analyze with non-existent session.
func TestRunAnalyzeNonExistentSession(t *testing.T) {
	cleanupAnalyzeTestDB(t)
	defer cleanupAnalyzeTestDB(t)

	sessionID := "existing-session"
	ops := createTestOperations(sessionID, 5)

	db := createTestDB(t, sessionID, ops)
	err := db.Close()
	require.NoError(t, err)

	cfg := &AnalyzeConfig{
		DatabasePath:  testAnalyzeDBPath,
		SessionID:     "non-existent-session",
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err = runAnalyze(cfg, []string{"non-existent-session"})
	assert.Error(t, err, "runAnalyze should fail for non-existent session")
}

// TestRunAnalyzeInvalidDatabase tests analyze with invalid database path.
func TestRunAnalyzeInvalidDatabase(t *testing.T) {
	cfg := &AnalyzeConfig{
		DatabasePath:  "/invalid/path/to/database.db",
		SessionID:     "test-session",
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := runAnalyze(cfg, []string{"test-session"})
	assert.Error(t, err, "runAnalyze should fail for invalid database")
}

// TestRunAnalyzeWithNilConfig tests analyze with nil configuration.
func TestRunAnalyzeWithNilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// Expected panic for nil config
			t.Log("Recovered from expected panic with nil config")
		}
	}()

	err := runAnalyze(nil, []string{"test-session"})
	if err == nil {
		t.Error("runAnalyze should fail with nil config")
	}
}

// TestRunAnalyzeWithInvalidArgs tests analyze with invalid arguments.
func TestRunAnalyzeWithInvalidArgs(t *testing.T) {
	cfg := &AnalyzeConfig{
		DatabasePath:  testAnalyzeDBPath,
		SessionID:     "test-session",
		FindSlow:      true,
		DetectLoops:   true,
		AnalyzeErrors: true,
		SlowThreshold: 1000,
		LoopWindow:    10,
		Format:        "json",
	}

	err := runAnalyze(cfg, []string{})
	assert.Error(t, err, "runAnalyze should fail with empty args")

	err = runAnalyze(cfg, []string{"session1", "session2"})
	assert.Error(t, err, "runAnalyze should fail with too many args")
}
