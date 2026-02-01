package analysis

import (
	"testing"
	"time"

	"github.com/operator-replay-debugger/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestCausalityExactMatch(t *testing.T) {
	start := time.Now()

	ops := []storage.Operation{
		{
			SessionID:       "session-1",
			SequenceNumber:  1,
			Timestamp:       start,
			OperationType:   storage.OperationUpdate,
			ResourceKind:    "ConfigMap",
			Namespace:       "default",
			Name:            "cm",
			UID:             "uid-1",
			ResourceVersion: "5",
			ActorID:         "controller-a",
		},
	}

	spans := []storage.ReconcileSpan{
		{
			ID:                     "span-1",
			SessionID:              "session-1",
			ActorID:                "controller-b",
			StartTime:              start.Add(2 * time.Second),
			EndTime:                start.Add(4 * time.Second),
			Kind:                   "ConfigMap",
			Namespace:              "default",
			Name:                   "cm",
			TriggerUID:             "uid-1",
			TriggerResourceVersion: "5",
		},
	}

	graph, _, err := BuildCausalityGraph(ops, spans, CausalityOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, graph)

	found := hasEdge(graph.Edges, "op:1", "span:span-1", EdgeTypeOpToSpan)
	assert.True(t, found, "expected op->span edge for exact uid+rv match")
}

func TestCausalitySpanToWriteEdge(t *testing.T) {
	start := time.Now()

	ops := []storage.Operation{
		{
			SessionID:      "session-2",
			SequenceNumber: 2,
			Timestamp:      start.Add(3 * time.Second),
			OperationType:  storage.OperationCreate,
			ResourceKind:   "Secret",
			Namespace:      "default",
			Name:           "secret",
			ActorID:        "controller-b",
		},
	}

	spans := []storage.ReconcileSpan{
		{
			ID:        "span-2",
			SessionID: "session-2",
			ActorID:   "controller-b",
			StartTime: start.Add(2 * time.Second),
			EndTime:   start.Add(6 * time.Second),
			Kind:      "Secret",
			Namespace: "default",
			Name:      "secret",
		},
	}

	graph, _, err := BuildCausalityGraph(ops, spans, CausalityOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, graph)

	found := hasEdge(graph.Edges, "span:span-2", "op:2", EdgeTypeSpanToOp)
	assert.True(t, found, "expected span->op edge for write within span window")
}

func hasEdge(edges []CausalityEdge, from, to string, edgeType CausalityEdgeType) bool {
	for i := 0; i < len(edges); i++ {
		edge := edges[i]
		if edge.From == from && edge.To == to && edge.Type == edgeType {
			return true
		}
	}
	return false
}
