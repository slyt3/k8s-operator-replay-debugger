package commands

import (
	"fmt"

	"github.com/schollz/progressbar/v3"
	"github.com/slyt3/kubestep/internal/assert"
	"github.com/slyt3/kubestep/pkg/replay"
	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/spf13/cobra"
)

const (
	defaultDatabasePath = "recordings.db"
	maxSessionIDInput   = 100
)

// ReplayConfig holds replay command configuration.
type ReplayConfig struct {
	DatabasePath string
	SessionID    string
	Interactive  bool
	Quiet        bool
}

// NewReplayCommand creates the replay subcommand.
func NewReplayCommand() *cobra.Command {
	cfg := &ReplayConfig{}

	cmd := &cobra.Command{
		Use:   "replay [session-id]",
		Short: "Replay recorded operations",
		Long: `Load and replay recorded Kubernetes operations.
Allows stepping forward/backward through the operation sequence.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReplay(cfg, args)
		},
	}

	cmd.Flags().StringVarP(
		&cfg.DatabasePath,
		"database",
		"d",
		defaultDatabasePath,
		"Path to SQLite database",
	)

	cmd.Flags().BoolVarP(
		&cfg.Interactive,
		"interactive",
		"i",
		false,
		"Enable interactive stepping mode",
	)

	cmd.Flags().BoolVarP(
		&cfg.Quiet,
		"quiet",
		"q",
		false,
		"Disable progress bar during replay",
	)

	return cmd
}

// runReplay executes the replay command.
// Rule 4: Function under 60 lines with clear flow.
func runReplay(cfg *ReplayConfig, args []string) error {
	err := assert.AssertNotNil(cfg, "config")
	if err != nil {
		return err
	}

	err = assert.AssertInRange(len(args), 1, 1, "args count")
	if err != nil {
		return err
	}

	cfg.SessionID = args[0]

	err = validateReplayConfig(cfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	db, err := storage.NewDatabase(cfg.DatabasePath, 1000000)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close database: %v\n", closeErr)
		}
	}()

	ops, err := db.QueryOperations(cfg.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load operations: %w", err)
	}

	if len(ops) == 0 {
		return fmt.Errorf("no operations found for session: %s", cfg.SessionID)
	}

	engine, err := replay.NewReplayEngine(replay.Config{
		Operations:   ops,
		SessionID:    cfg.SessionID,
		MaxCacheSize: 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	if cfg.Interactive {
		return runInteractiveReplay(engine)
	}

	return runAutomaticReplay(engine, cfg.Quiet)
}

// validateReplayConfig validates configuration.
func validateReplayConfig(cfg *ReplayConfig) error {
	err := assert.AssertStringNotEmpty(cfg.DatabasePath, "database path")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(cfg.SessionID, "session ID")
	if err != nil {
		return err
	}

	err = assert.AssertInRange(
		len(cfg.SessionID),
		1,
		maxSessionIDInput,
		"session ID length",
	)
	if err != nil {
		return err
	}

	return nil
}

// runInteractiveReplay runs replay with user interaction.
// Rule 2: Bounded loop with explicit exit conditions.
func runInteractiveReplay(engine *replay.ReplayEngine) error {
	err := assert.AssertNotNil(engine, "engine")
	if err != nil {
		return err
	}

	fmt.Println("Interactive Replay Mode")
	fmt.Println("Commands: n=next, b=back, r=reset, s=stats, q=quit")
	fmt.Println()

	maxIterations := 10000
	iteration := 0

	for iteration < maxIterations {
		current, total, err := engine.GetProgress()
		if err != nil {
			return err
		}

		fmt.Printf("[%d/%d] > ", current, total)

		var input string
		_, err = fmt.Scanln(&input)
		if err != nil {
			return fmt.Errorf("input error: %w", err)
		}

		if len(input) == 0 {
			continue
		}

		shouldExit, cmdErr := handleReplayCommand(engine, input)
		if cmdErr != nil {
			fmt.Printf("Error: %v\n", cmdErr)
		}

		if shouldExit {
			break
		}

		iteration = iteration + 1
	}

	if iteration >= maxIterations {
		fmt.Println("Maximum iterations reached")
	}

	return nil
}

// handleReplayCommand processes a single replay command.
func handleReplayCommand(engine *replay.ReplayEngine, input string) (bool, error) {
	err := assert.AssertNotNil(engine, "engine")
	if err != nil {
		return false, err
	}

	switch input {
	case "n":
		return handleNextCommand(engine)
	case "b":
		return handleBackCommand(engine)
	case "r":
		return handleResetCommand(engine)
	case "s":
		return handleStatsCommand(engine)
	case "q":
		return true, nil
	default:
		return false, fmt.Errorf("unknown command: %s", input)
	}
}

// handleNextCommand moves to next operation.
func handleNextCommand(engine *replay.ReplayEngine) (bool, error) {
	op, err := engine.StepForward()
	if err != nil {
		return false, err
	}

	displayOperation(op)
	return false, nil
}

// handleBackCommand moves to previous operation.
func handleBackCommand(engine *replay.ReplayEngine) (bool, error) {
	op, err := engine.StepBackward()
	if err != nil {
		return false, err
	}

	displayOperation(op)
	return false, nil
}

// handleResetCommand resets to beginning.
func handleResetCommand(engine *replay.ReplayEngine) (bool, error) {
	err := engine.Reset()
	if err != nil {
		return false, err
	}

	fmt.Println("Reset to beginning")
	return false, nil
}

// handleStatsCommand displays statistics.
func handleStatsCommand(engine *replay.ReplayEngine) (bool, error) {
	stats, err := engine.CalculateStats()
	if err != nil {
		return false, err
	}

	displayStats(stats)
	return false, nil
}

// displayOperation shows operation details.
func displayOperation(op *storage.Operation) {
	if op == nil {
		return
	}

	fmt.Printf("Seq: %d | Type: %s | Kind: %s | NS: %s | Name: %s | Duration: %dms\n",
		op.SequenceNumber,
		op.OperationType,
		op.ResourceKind,
		op.Namespace,
		op.Name,
		op.DurationMs,
	)

	if len(op.Error) > 0 {
		fmt.Printf("  Error: %s\n", op.Error)
	}
}

// displayStats shows operation statistics.
func displayStats(stats *replay.OperationStats) {
	if stats == nil {
		return
	}

	fmt.Println("\nOperation Statistics:")
	fmt.Printf("  Total Operations: %d\n", stats.TotalOps)
	fmt.Printf("  GET: %d\n", stats.GetOps)
	fmt.Printf("  UPDATE: %d\n", stats.UpdateOps)
	fmt.Printf("  CREATE: %d\n", stats.CreateOps)
	fmt.Printf("  DELETE: %d\n", stats.DeleteOps)
	fmt.Printf("  Errors: %d\n", stats.ErrorCount)
	fmt.Printf("  Avg Duration: %dms\n", stats.AvgDurationMs)
	fmt.Printf("  Max Duration: %dms\n", stats.MaxDurationMs)
	fmt.Printf("  Min Duration: %dms\n", stats.MinDurationMs)
	fmt.Println()
}

// runAutomaticReplay runs through all operations automatically.
// Rule 2: Bounded by operation count.
func runAutomaticReplay(engine *replay.ReplayEngine, quiet bool) error {
	err := assert.AssertNotNil(engine, "engine")
	if err != nil {
		return err
	}

	_, total, err := engine.GetProgress()
	if err != nil {
		return err
	}

	fmt.Printf("Replaying %d operations...\n", total)

	// Rule 6: Declare in smallest scope
	var bar *progressbar.ProgressBar
	if !quiet {
		bar = progressbar.NewOptions(total,
			progressbar.OptionSetDescription("Progress"),
			progressbar.OptionSetWidth(20),
			progressbar.OptionShowCount(),
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionSetElapsedTime(true),
		)
	}

	// Rule 1,2: Simple loop with fixed bound
	count := 0
	for count < total {
		op, err := engine.StepForward()
		if err != nil {
			return fmt.Errorf("step failed at %d: %w", count, err)
		}

		// Update progress bar if not quiet
		if !quiet && bar != nil {
			// Rule 7: Check return value
			updateErr := bar.Add(1)
			if updateErr != nil {
				fmt.Printf("Warning: progress update failed: %v\n", updateErr)
			}
		}

		// Show errors and periodic updates for quiet mode
		if len(op.Error) > 0 {
			displayOperation(op)
		} else if quiet && count%100 == 0 {
			fmt.Printf("Progress: %d/%d\n", count+1, total)
		}

		count = count + 1
	}

	// Rule 5: Assert completion
	if count != total {
		return fmt.Errorf("replay incomplete: processed %d of %d", count, total)
	}

	if !quiet && bar != nil {
		// Rule 7: Explicitly ignore return value
		_ = bar.Finish()
	}

	fmt.Println("\nReplay complete")

	stats, err := engine.CalculateStats()
	if err != nil {
		return err
	}

	displayStats(stats)

	return nil
}
