package replay

import (
	"testing"
	"time"

	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestOperations creates a set of test operations.
func createTestOperations(count int) []storage.Operation {
	ops := make([]storage.Operation, 0, count)

	maxOps := count
	if maxOps > 100 {
		maxOps = 100
	}

	for i := 0; i < maxOps; i = i + 1 {
		op := storage.Operation{
			SessionID:      "test-session",
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

// TestNewReplayEngine tests engine creation.
func TestNewReplayEngine(t *testing.T) {
	ops := createTestOperations(10)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})

	require.NoError(t, err, "engine creation should succeed")
	require.NotNil(t, engine, "engine should not be nil")
	assert.Equal(t, 0, engine.currentIndex, "should start at index 0")
	assert.Equal(t, 10, engine.maxIndex, "should have correct max index")
}

// TestStepForward tests forward stepping.
func TestStepForward(t *testing.T) {
	ops := createTestOperations(5)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	maxSteps := 5
	for i := 0; i < maxSteps; i = i + 1 {
		op, stepErr := engine.StepForward()
		assert.NoError(t, stepErr, "step should succeed")
		assert.NotNil(t, op, "operation should not be nil")
		assert.Equal(t, int64(i+1), op.SequenceNumber)
	}

	_, err = engine.StepForward()
	assert.Error(t, err, "step beyond end should fail")
}

// TestStepBackward tests backward stepping.
func TestStepBackward(t *testing.T) {
	ops := createTestOperations(5)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	maxForward := 3
	for i := 0; i < maxForward; i = i + 1 {
		_, stepErr := engine.StepForward()
		require.NoError(t, stepErr)
	}

	maxBackward := 2
	for i := 0; i < maxBackward; i = i + 1 {
		op, stepErr := engine.StepBackward()
		assert.NoError(t, stepErr, "backward step should succeed")
		assert.NotNil(t, op)
	}

	current, total, err := engine.GetProgress()
	require.NoError(t, err)
	assert.Equal(t, 1, current, "should be at index 1")
	assert.Equal(t, 5, total, "total should be 5")
}

// TestStepN tests multi-step operations.
func TestStepN(t *testing.T) {
	ops := createTestOperations(10)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	err = engine.StepN(5)
	assert.NoError(t, err, "step forward 5 should succeed")

	current, _, err := engine.GetProgress()
	require.NoError(t, err)
	assert.Equal(t, 5, current, "should be at index 5")

	err = engine.StepN(-3)
	assert.NoError(t, err, "step backward 3 should succeed")

	current, _, err = engine.GetProgress()
	require.NoError(t, err)
	assert.Equal(t, 2, current, "should be at index 2")
}

// TestReset tests engine reset.
func TestReset(t *testing.T) {
	ops := createTestOperations(5)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	maxSteps := 3
	for i := 0; i < maxSteps; i = i + 1 {
		_, stepErr := engine.StepForward()
		require.NoError(t, stepErr)
	}

	err = engine.Reset()
	assert.NoError(t, err, "reset should succeed")

	current, _, err := engine.GetProgress()
	require.NoError(t, err)
	assert.Equal(t, 0, current, "should be at index 0 after reset")
}

// TestCalculateStats tests statistics calculation.
func TestCalculateStats(t *testing.T) {
	ops := make([]storage.Operation, 0, 10)

	maxOps := 10
	for i := 0; i < maxOps; i = i + 1 {
		op := storage.Operation{
			SessionID:      "test-session",
			SequenceNumber: int64(i + 1),
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "test-pod",
			DurationMs:     int64(100 + i*10),
		}

		if i == 5 {
			op.Error = "test error"
		}

		ops = append(ops, op)
	}

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	stats, err := engine.CalculateStats()
	require.NoError(t, err, "calculate stats should succeed")
	require.NotNil(t, stats, "stats should not be nil")

	assert.Equal(t, 10, stats.TotalOps, "should count all operations")
	assert.Equal(t, 10, stats.GetOps, "should count GET operations")
	assert.Equal(t, 1, stats.ErrorCount, "should count errors")
	assert.Greater(t, stats.AvgDurationMs, int64(0), "should have average duration")
}

// TestGetOperationAt tests operation retrieval by index.
func TestGetOperationAt(t *testing.T) {
	ops := createTestOperations(5)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	op, err := engine.GetOperationAt(2)
	assert.NoError(t, err, "should retrieve operation at index 2")
	assert.NotNil(t, op)
	assert.Equal(t, int64(3), op.SequenceNumber)

	_, err = engine.GetOperationAt(10)
	assert.Error(t, err, "should fail for out of bounds index")
}

// TestMockClient tests mock client creation.
func TestMockClient(t *testing.T) {
	ops := createTestOperations(5)

	engine, err := NewReplayEngine(Config{
		Operations:   ops,
		SessionID:    "test-session",
		MaxCacheSize: 100,
	})
	require.NoError(t, err)

	client, err := NewMockClient(engine)
	assert.NoError(t, err, "mock client creation should succeed")
	assert.NotNil(t, client, "mock client should not be nil")
}
