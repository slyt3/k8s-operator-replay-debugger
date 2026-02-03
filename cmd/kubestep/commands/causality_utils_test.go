package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/slyt3/kubestep/pkg/analysis"
	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestParseWindow(t *testing.T) {
	start, end, err := parseWindow("")
	require.NoError(t, err)
	require.Nil(t, start)
	require.Nil(t, end)

	start, end, err = parseWindow("2024-01-01T00:00:00Z,2024-01-01T00:01:00Z")
	require.NoError(t, err)
	require.NotNil(t, start)
	require.NotNil(t, end)

	start, end, err = parseWindow("1700000000,1700000060")
	require.NoError(t, err)
	require.NotNil(t, start)
	require.NotNil(t, end)

	_, _, err = parseWindow("bad")
	require.Error(t, err)

	_, _, err = parseWindow("2024-01-02T00:00:00Z,2024-01-01T00:00:00Z")
	require.Error(t, err)
}

func TestFormatResourceRef(t *testing.T) {
	require.Equal(t, "name", formatResourceRef("", "name"))
	require.Equal(t, "ns", formatResourceRef("ns", ""))
	require.Equal(t, "ns/name", formatResourceRef("ns", "name"))
}

func TestFormatCausalityNode(t *testing.T) {
	now := time.Now()
	opNode := analysis.CausalityNode{
		ID:          "op-1",
		Type:        analysis.NodeTypeOperation,
		ActorID:     "actor",
		Kind:        "Pod",
		Namespace:   "default",
		Name:        "demo",
		ResourceVer: "10",
		UID:         "uid",
		Timestamp:   now,
	}

	out := formatCausalityNode(opNode)
	require.True(t, strings.Contains(out, "op["))

	spanNode := analysis.CausalityNode{
		ID:          "span-1",
		Type:        analysis.NodeTypeSpan,
		ActorID:     "actor",
		Kind:        "Pod",
		Namespace:   "default",
		Name:        "demo",
		ResourceVer: "10",
		UID:         "uid",
		DurationMs:  5,
		Timestamp:   now,
	}

	out = formatCausalityNode(spanNode)
	require.True(t, strings.Contains(out, "span["))
}

func TestFilterByWindow(t *testing.T) {
	now := time.Now()
	ops := []storage.Operation{
		{Timestamp: now.Add(-2 * time.Minute)},
		{Timestamp: now},
		{Timestamp: now.Add(2 * time.Minute)},
	}

	start := now.Add(-time.Minute)
	end := now.Add(time.Minute)
	filtered := filterOperationsByWindow(ops, &start, &end)
	require.Len(t, filtered, 1)

	spans := []storage.ReconcileSpan{
		{StartTime: now.Add(-2 * time.Minute)},
		{StartTime: now},
		{StartTime: now.Add(2 * time.Minute)},
	}

	filteredSpans := filterSpansByWindow(spans, &start, &end)
	require.Len(t, filteredSpans, 1)
}

func TestValidateCausalityConfig(t *testing.T) {
	cfg := &CausalityConfig{
		SessionID:    "s",
		Format:       "text",
		StorageType:  "sqlite",
		DatabasePath: "db",
		MaxDepth:     defaultCausalityDepth,
	}
	require.NoError(t, validateCausalityConfig(cfg))

	cfg.Format = "bad"
	require.Error(t, validateCausalityConfig(cfg))
}
