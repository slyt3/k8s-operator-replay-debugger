package commands

import (
	"testing"
	"time"

	"github.com/slyt3/kubestep/pkg/replay"
	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestValidateReplayConfig(t *testing.T) {
	cfg := &ReplayConfig{DatabasePath: "db", SessionID: "s"}
	require.NoError(t, validateReplayConfig(cfg))

	cfg.SessionID = ""
	require.Error(t, validateReplayConfig(cfg))
}

func TestHandleReplayCommands(t *testing.T) {
	ops := []storage.Operation{
		{
			SessionID:      "s",
			SequenceNumber: 1,
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "demo",
			DurationMs:     5,
		},
		{
			SessionID:      "s",
			SequenceNumber: 2,
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "demo-2",
			DurationMs:     5,
		},
	}

	engine, err := replay.NewReplayEngine(replay.Config{
		Operations:   ops,
		SessionID:    "s",
		MaxCacheSize: 10,
	})
	require.NoError(t, err)

	_, err = handleReplayCommand(engine, "n")
	require.NoError(t, err)

	_, err = handleReplayCommand(engine, "b")
	require.NoError(t, err)

	_, err = handleReplayCommand(engine, "r")
	require.NoError(t, err)

	_, err = handleReplayCommand(engine, "s")
	require.NoError(t, err)

	shouldExit, err := handleReplayCommand(engine, "q")
	require.NoError(t, err)
	require.True(t, shouldExit)

	_, err = handleReplayCommand(engine, "bad")
	require.Error(t, err)
}

func TestRunAutomaticReplayQuiet(t *testing.T) {
	ops := []storage.Operation{
		{
			SessionID:      "s",
			SequenceNumber: 1,
			Timestamp:      time.Now(),
			OperationType:  storage.OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "demo",
			DurationMs:     5,
			Error:          "boom",
		},
	}

	engine, err := replay.NewReplayEngine(replay.Config{
		Operations:   ops,
		SessionID:    "s",
		MaxCacheSize: 10,
	})
	require.NoError(t, err)

	require.NoError(t, runAutomaticReplay(engine, true))
}
