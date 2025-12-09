package replay

import (
	"encoding/json"
	"fmt"

	"github.com/operator-replay-debugger/internal/assert"
	"github.com/operator-replay-debugger/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	maxOperationsInMemory = 100000
	maxStepSize           = 1000
)

// ReplayEngine manages playback of recorded operations.
// Rule 6: Minimal scope for all state.
type ReplayEngine struct {
	operations    []storage.Operation
	currentIndex  int
	maxIndex      int
	sessionID     string
	stateCache    map[string]runtime.Object
	maxCacheSize  int
}

// Config holds replay configuration.
type Config struct {
	Operations   []storage.Operation
	SessionID    string
	MaxCacheSize int
}

// NewReplayEngine creates a replay engine from recorded operations.
// Rule 5: Multiple assertions for validation.
func NewReplayEngine(cfg Config) (*ReplayEngine, error) {
	err := assert.AssertNotNil(cfg.Operations, "operations")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(cfg.SessionID, "session_id")
	if err != nil {
		return nil, err
	}

	opCount := len(cfg.Operations)
	err = assert.AssertInRange(
		opCount,
		0,
		maxOperationsInMemory,
		"operation count",
	)
	if err != nil {
		return nil, err
	}

	if cfg.MaxCacheSize <= 0 {
		cfg.MaxCacheSize = 1000
	}

	stateCache := make(map[string]runtime.Object, cfg.MaxCacheSize)

	return &ReplayEngine{
		operations:   cfg.Operations,
		currentIndex: 0,
		maxIndex:     opCount,
		sessionID:    cfg.SessionID,
		stateCache:   stateCache,
		maxCacheSize: cfg.MaxCacheSize,
	}, nil
}

// StepForward advances replay by one operation.
// Rule 2: Bounded by maxIndex check.
func (r *ReplayEngine) StepForward() (*storage.Operation, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return nil, err
	}

	if r.currentIndex >= r.maxIndex {
		return nil, fmt.Errorf("at end of replay: index %d", r.currentIndex)
	}

	op := &r.operations[r.currentIndex]
	r.currentIndex = r.currentIndex + 1

	err = r.updateCache(op)
	if err != nil {
		return op, fmt.Errorf("cache update failed: %w", err)
	}

	return op, nil
}

// StepBackward moves replay back by one operation.
// Rule 2: Bounded by zero check.
func (r *ReplayEngine) StepBackward() (*storage.Operation, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return nil, err
	}

	if r.currentIndex <= 0 {
		return nil, fmt.Errorf("at beginning of replay")
	}

	r.currentIndex = r.currentIndex - 1
	op := &r.operations[r.currentIndex]

	return op, nil
}

// StepN advances or rewinds by N operations.
// Rule 2: Bounded steps with explicit limit.
func (r *ReplayEngine) StepN(n int) error {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return err
	}

	err = assert.AssertInRange(n, -maxStepSize, maxStepSize, "step count")
	if err != nil {
		return err
	}

	targetIndex := r.currentIndex + n
	if targetIndex < 0 {
		targetIndex = 0
	}
	if targetIndex > r.maxIndex {
		targetIndex = r.maxIndex
	}

	r.currentIndex = targetIndex
	return nil
}

// GetCurrentOperation returns operation at current index.
func (r *ReplayEngine) GetCurrentOperation() (*storage.Operation, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return nil, err
	}

	if r.currentIndex < 0 || r.currentIndex >= r.maxIndex {
		return nil, fmt.Errorf("invalid index: %d", r.currentIndex)
	}

	return &r.operations[r.currentIndex], nil
}

// GetProgress returns current position and total operations.
func (r *ReplayEngine) GetProgress() (int, int, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return 0, 0, err
	}

	return r.currentIndex, r.maxIndex, nil
}

// Reset moves replay to beginning.
func (r *ReplayEngine) Reset() error {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return err
	}

	r.currentIndex = 0

	if r.stateCache != nil {
		for k := range r.stateCache {
			delete(r.stateCache, k)
		}
	}

	return nil
}

// updateCache updates the state cache with operation result.
// Rule 4: Function under 60 lines.
func (r *ReplayEngine) updateCache(op *storage.Operation) error {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return err
	}

	err = assert.AssertNotNil(op, "operation")
	if err != nil {
		return err
	}

	if len(op.ResourceData) == 0 {
		return nil
	}

	key := fmt.Sprintf("%s/%s/%s", op.ResourceKind, op.Namespace, op.Name)

	if len(r.stateCache) >= r.maxCacheSize {
		return fmt.Errorf("cache size limit reached: %d", r.maxCacheSize)
	}

	var obj runtime.Object
	err = json.Unmarshal([]byte(op.ResourceData), &obj)
	if err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	r.stateCache[key] = obj
	return nil
}

// GetCachedObject retrieves object from state cache.
func (r *ReplayEngine) GetCachedObject(
	kind string,
	namespace string,
	name string,
) (runtime.Object, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(kind, "kind")
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s/%s/%s", kind, namespace, name)
	obj, found := r.stateCache[key]
	if !found {
		return nil, fmt.Errorf("object not found in cache: %s", key)
	}

	return obj, nil
}

// OperationStats holds statistics about operations.
type OperationStats struct {
	TotalOps       int
	GetOps         int
	UpdateOps      int
	CreateOps      int
	DeleteOps      int
	ErrorCount     int
	AvgDurationMs  int64
	MaxDurationMs  int64
	MinDurationMs  int64
}

// CalculateStats computes statistics for recorded operations.
// Rule 2: Bounded loop with explicit limit.
func (r *ReplayEngine) CalculateStats() (*OperationStats, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return nil, err
	}

	stats := &OperationStats{
		TotalOps:      len(r.operations),
		MinDurationMs: 999999999,
	}

	var totalDuration int64
	count := 0
	maxIterations := len(r.operations)

	for count < maxIterations {
		op := &r.operations[count]

		switch op.OperationType {
		case storage.OperationGet:
			stats.GetOps = stats.GetOps + 1
		case storage.OperationUpdate:
			stats.UpdateOps = stats.UpdateOps + 1
		case storage.OperationCreate:
			stats.CreateOps = stats.CreateOps + 1
		case storage.OperationDelete:
			stats.DeleteOps = stats.DeleteOps + 1
		}

		if len(op.Error) > 0 {
			stats.ErrorCount = stats.ErrorCount + 1
		}

		totalDuration = totalDuration + op.DurationMs

		if op.DurationMs > stats.MaxDurationMs {
			stats.MaxDurationMs = op.DurationMs
		}

		if op.DurationMs < stats.MinDurationMs {
			stats.MinDurationMs = op.DurationMs
		}

		count = count + 1
	}

	if stats.TotalOps > 0 {
		stats.AvgDurationMs = totalDuration / int64(stats.TotalOps)
	}

	return stats, nil
}

// GetOperationAt returns operation at specific index.
func (r *ReplayEngine) GetOperationAt(index int) (*storage.Operation, error) {
	err := assert.AssertNotNil(r, "replay engine")
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(index, 0, r.maxIndex-1, "index")
	if err != nil {
		return nil, err
	}

	return &r.operations[index], nil
}

// MockClient provides a mock Kubernetes client for replay.
type MockClient struct {
	engine *ReplayEngine
}

// NewMockClient creates a mock client backed by replay engine.
func NewMockClient(engine *ReplayEngine) (*MockClient, error) {
	err := assert.AssertNotNil(engine, "replay engine")
	if err != nil {
		return nil, err
	}

	return &MockClient{
		engine: engine,
	}, nil
}

// Get simulates a Kubernetes GET operation from replay.
func (m *MockClient) Get(
	kind string,
	namespace string,
	name string,
	opts metav1.GetOptions,
) (runtime.Object, error) {
	err := assert.AssertNotNil(m, "mock client")
	if err != nil {
		return nil, err
	}

	err = assert.AssertNotNil(m.engine, "engine")
	if err != nil {
		return nil, err
	}

	obj, getErr := m.engine.GetCachedObject(kind, namespace, name)
	if getErr != nil {
		return nil, getErr
	}

	return obj, nil
}
