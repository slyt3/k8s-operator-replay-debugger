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
	testSessionID     = "test-session-001"
)

// TestAnalyzeJSONOutput tests JSON format output.
func TestAnalyzeJSONOutput(t *testing.T) {
	cleanupAnalyzeTestDB(t)
	defer cleanupAnalyzeTestDB(t)

	db, err := storage.NewDatabase(testAnalyzeDBPath, 1000)
	require.NoError(t, err)

	err = createTestData(db)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	cfg := &AnalyzeConfig{
		DatabasePath:  testAnalyzeDBPath,
		SessionID:     testSessionID,
		DetectLoops:   true,
		FindSlow:      true,
		AnalyzeErrors: true,
		LoopWindow:    3,
		SlowThreshold: 100,
		Format:        "json",
	}

	err = validateAnalyzeConfig(cfg)
	assert.NoError(t, err, "config validation should succeed")

	db, err = storage.NewDatabase(testAnalyzeDBPath, 1000)
	require.NoError(t, err)
	defer func() {
		closeErr := db.Close()
		assert.NoError(t, closeErr)
	}()

	ops, err := db.QueryOperations(testSessionID)
	require.NoError(t, err)
	require.NotEmpty(t, ops, "should have test operations")

	err = outputJSON(cfg, ops)
	assert.NoError(t, err, "JSON output should succeed")
}

// TestAnalyzeValidation tests configuration validation.
func TestAnalyzeValidation(t *testing.T) {
	testCases := []struct {
		name   string
		cfg    AnalyzeConfig
		hasErr bool
	}{
		{
			name: "valid config",
			cfg: AnalyzeConfig{
				DatabasePath:  testAnalyzeDBPath,
				SessionID:     testSessionID,
				LoopWindow:    5,
				SlowThreshold: 1000,
				Format:        "json",
			},
			hasErr: false,
		},
		{
			name: "invalid format",
			cfg: AnalyzeConfig{
				DatabasePath:  testAnalyzeDBPath,
				SessionID:     testSessionID,
				LoopWindow:    5,
				SlowThreshold: 1000,
				Format:        "xml",
			},
			hasErr: true,
		},
		{
			name: "invalid threshold",
			cfg: AnalyzeConfig{
				DatabasePath:  testAnalyzeDBPath,
				SessionID:     testSessionID,
				LoopWindow:    5,
				SlowThreshold: -1,
				Format:        "text",
			},
			hasErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAnalyzeConfig(&tc.cfg)
			if tc.hasErr {
				assert.Error(t, err, "validation should fail")
			} else {
				assert.NoError(t, err, "validation should succeed")
			}
		})
	}
}

// TestJSONStructure tests JSON output structure.
func TestJSONStructure(t *testing.T) {
	report := JSONAnalysisReport{
		SessionID:       "test-001",
		TotalOperations: 10,
		SlowOperations: []JSONSlowOperation{
			{
				Index:      5,
				Type:       "UPDATE",
				Resource:   "Deployment/production/app",
				DurationMs: 2000,
			},
		},
		LoopsDetected: []JSONLoopDetection{
			{
				StartIndex:  10,
				EndIndex:    30,
				RepeatCount: 3,
				Description: "Repeated Pod operations",
			},
		},
		Errors: &JSONErrorSummary{
			Total: 3,
			ByType: map[string]int{
				"GET":    2,
				"UPDATE": 1,
			},
		},
	}

	jsonBytes, err := json.Marshal(report)
	require.NoError(t, err)

	var parsed JSONAnalysisReport
	err = json.Unmarshal(jsonBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, report.SessionID, parsed.SessionID)
	assert.Equal(t, report.TotalOperations, parsed.TotalOperations)
	assert.Len(t, parsed.SlowOperations, 1)
	assert.Len(t, parsed.LoopsDetected, 1)
	assert.NotNil(t, parsed.Errors)
	assert.Equal(t, 3, parsed.Errors.Total)
}

// TestJSONOmitsDisabledAnalysis tests that disabled analysis sections are omitted.
func TestJSONOmitsDisabledAnalysis(t *testing.T) {
	cleanupAnalyzeTestDB(t)
	defer cleanupAnalyzeTestDB(t)

	db, err := storage.NewDatabase(testAnalyzeDBPath, 1000)
	require.NoError(t, err)

	err = createTestData(db)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	cfg := &AnalyzeConfig{
		DatabasePath:  testAnalyzeDBPath,
		SessionID:     testSessionID,
		DetectLoops:   false,
		FindSlow:      false,
		AnalyzeErrors: false,
		LoopWindow:    3,
		SlowThreshold: 100,
		Format:        "json",
	}

	db, err = storage.NewDatabase(testAnalyzeDBPath, 1000)
	require.NoError(t, err)
	defer func() {
		closeErr := db.Close()
		assert.NoError(t, closeErr)
	}()

	ops, err := db.QueryOperations(testSessionID)
	require.NoError(t, err)

	err = outputJSON(cfg, ops)
	assert.NoError(t, err)
}

// createTestData creates test operations in database.
func createTestData(db *storage.Database) error {
	ops := []storage.Operation{
		{
			SessionID:      testSessionID,
			SequenceNumber: 1,
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "test-pod",
			DurationMs:     50,
		},
		{
			SessionID:      testSessionID,
			SequenceNumber: 2,
			Timestamp:      time.Now(),
			OperationType:  storage.OperationUpdate,
			ResourceKind:   "Deployment",
			Namespace:      "production",
			Name:           "app",
			DurationMs:     2000,
		},
		{
			SessionID:      testSessionID,
			SequenceNumber: 3,
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Service",
			Namespace:      "default",
			Name:           "test-svc",
			DurationMs:     25,
			Error:          "not found",
		},
	}

	for _, op := range ops {
		err := db.InsertOperation(&op)
		if err != nil {
			return err
		}
	}

	return nil
}

// cleanupAnalyzeTestDB removes test database file.
func cleanupAnalyzeTestDB(t *testing.T) {
	err := os.Remove(testAnalyzeDBPath)
	if err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: failed to cleanup test DB: %v", err)
	}
}
