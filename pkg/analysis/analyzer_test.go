package analysis

import (
	"testing"
	"time"

	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestDetectLoops(t *testing.T) {
	ops := make([]storage.Operation, 0, 6)
	for i := 0; i < 3; i++ {
		ops = append(ops, storage.Operation{
			OperationType: storage.OperationGet,
			ResourceKind:  "Pod",
			Namespace:     "default",
			Name:          "demo",
		})
		ops = append(ops, storage.Operation{
			OperationType: storage.OperationGet,
			ResourceKind:  "Pod",
			Namespace:     "default",
			Name:          "demo-2",
		})
	}

	patterns, err := DetectLoops(ops, 2)
	require.NoError(t, err)
	require.NotEmpty(t, patterns)
}

func TestDetectLoopsInvalidWindow(t *testing.T) {
	_, err := DetectLoops([]storage.Operation{}, 1)
	require.Error(t, err)
}

func TestFindSlowOperations(t *testing.T) {
	ops := []storage.Operation{
		{DurationMs: 5},
		{DurationMs: 15},
		{DurationMs: 50},
	}

	slow, err := FindSlowOperations(ops, 10)
	require.NoError(t, err)
	require.Len(t, slow, 2)
}

func TestFindSlowOperationsInvalidThreshold(t *testing.T) {
	_, err := FindSlowOperations([]storage.Operation{}, 0)
	require.Error(t, err)
}

func TestAnalyzeErrors(t *testing.T) {
	ops := []storage.Operation{
		{OperationType: storage.OperationGet, Error: "boom"},
		{OperationType: storage.OperationGet, Error: ""},
		{OperationType: storage.OperationUpdate, Error: "oops"},
	}

	summary, err := AnalyzeErrors(ops)
	require.NoError(t, err)
	require.Equal(t, 2, summary.TotalErrors)
	require.Equal(t, 1, summary.ErrorsByType[string(storage.OperationGet)])
	require.Equal(t, 1, summary.ErrorsByType[string(storage.OperationUpdate)])
	require.NotNil(t, summary.FirstError)
	require.NotNil(t, summary.LastError)
}

func TestAnalyzeResourceAccess(t *testing.T) {
	now := time.Now()
	ops := []storage.Operation{
		{OperationType: storage.OperationGet, ResourceKind: "Pod", Namespace: "default", Name: "a", Timestamp: now},
		{OperationType: storage.OperationUpdate, ResourceKind: "Pod", Namespace: "default", Name: "a", Timestamp: now.Add(time.Second)},
		{OperationType: storage.OperationList, ResourceKind: "Service", Namespace: "default", Name: "b", Timestamp: now},
	}

	patterns, err := AnalyzeResourceAccess(ops)
	require.NoError(t, err)
	require.Len(t, patterns, 2)

	pod := patterns["Pod/default/a"]
	require.NotNil(t, pod)
	require.Equal(t, 1, pod.ReadCount)
	require.Equal(t, 1, pod.WriteCount)
}
