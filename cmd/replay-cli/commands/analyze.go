package commands

import (
	"encoding/json"
	"fmt"

	"github.com/operator-replay-debugger/internal/assert"
	"github.com/operator-replay-debugger/pkg/analysis"
	"github.com/operator-replay-debugger/pkg/storage"
	"github.com/spf13/cobra"
)

const (
	defaultLoopWindow    = 10
	defaultSlowThreshold = 1000
)

// JSONSlowOperation represents slow operation in JSON output.
type JSONSlowOperation struct {
	Index      int    `json:"index"`
	Type       string `json:"type"`
	Resource   string `json:"resource"`
	DurationMs int64  `json:"duration_ms"`
}

// JSONLoopDetection represents detected loop in JSON output.
type JSONLoopDetection struct {
	StartIndex  int    `json:"start_index"`
	EndIndex    int    `json:"end_index"`
	RepeatCount int    `json:"repeat_count"`
	Description string `json:"description"`
}

// JSONErrorSummary represents error summary in JSON output.
type JSONErrorSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// JSONAnalysisReport represents complete analysis in JSON format.
type JSONAnalysisReport struct {
	SessionID       string              `json:"session_id"`
	TotalOperations int                 `json:"total_operations"`
	SlowOperations  []JSONSlowOperation `json:"slow_operations,omitempty"`
	LoopsDetected   []JSONLoopDetection `json:"loops_detected,omitempty"`
	Errors          *JSONErrorSummary   `json:"errors,omitempty"`
}

// AnalyzeConfig holds analyze command configuration.
type AnalyzeConfig struct {
	DatabasePath  string
	SessionID     string
	DetectLoops   bool
	FindSlow      bool
	AnalyzeErrors bool
	LoopWindow    int
	SlowThreshold int64
	Format        string
	StorageType   string
	MongoURI      string
	MongoDatabase string
}

// NewAnalyzeCommand creates the analyze subcommand.
func NewAnalyzeCommand() *cobra.Command {
	cfg := &AnalyzeConfig{}

	cmd := &cobra.Command{
		Use:   "analyze [session-id]",
		Short: "Analyze recorded operations for issues",
		Long: `Analyze recorded operations to detect:
- Infinite loops and repeated patterns
- Slow operations exceeding threshold
- Error patterns and frequencies
- Resource access patterns`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cfg, args)
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
		&cfg.DetectLoops,
		"loops",
		"l",
		true,
		"Detect loop patterns",
	)

	cmd.Flags().BoolVarP(
		&cfg.FindSlow,
		"slow",
		"s",
		true,
		"Find slow operations",
	)

	cmd.Flags().BoolVarP(
		&cfg.AnalyzeErrors,
		"errors",
		"e",
		true,
		"Analyze error patterns",
	)

	cmd.Flags().IntVarP(
		&cfg.LoopWindow,
		"window",
		"w",
		defaultLoopWindow,
		"Loop detection window size",
	)

	cmd.Flags().Int64VarP(
		&cfg.SlowThreshold,
		"threshold",
		"t",
		defaultSlowThreshold,
		"Slow operation threshold in ms",
	)

	cmd.Flags().StringVar(
		&cfg.Format,
		"format",
		"text",
		"Output format: text or json",
	)

	cmd.Flags().StringVar(
		&cfg.StorageType,
		"storage",
		"sqlite",
		"Storage backend: sqlite or mongodb",
	)

	cmd.Flags().StringVar(
		&cfg.MongoURI,
		"mongo-uri",
		"mongodb://localhost:27017",
		"MongoDB connection URI",
	)

	cmd.Flags().StringVar(
		&cfg.MongoDatabase,
		"mongo-db",
		"operator_replay",
		"MongoDB database name",
	)

	cmd.AddCommand(NewCausalityCommand())

	return cmd
}

// runAnalyze executes the analyze command.
func runAnalyze(cfg *AnalyzeConfig, args []string) error {
	err := assert.AssertNotNil(cfg, "config")
	if err != nil {
		return err
	}

	err = assert.AssertInRange(len(args), 1, 1, "args count")
	if err != nil {
		return err
	}

	cfg.SessionID = args[0]

	err = validateAnalyzeConfig(cfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create storage based on type
	storeCfg := createStorageConfig(cfg)
	store, err := storage.NewOperationStore(storeCfg)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer func() {
		closeErr := store.Close()
		if closeErr != nil && cfg.Format != "json" {
			fmt.Printf("Warning: failed to close storage: %v\n", closeErr)
		}
	}()

	ops, err := store.QueryOperations(cfg.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load operations: %w", err)
	}

	if len(ops) == 0 {
		return fmt.Errorf("no operations found for session: %s", cfg.SessionID)
	}

	if cfg.Format == "json" {
		return outputJSON(cfg, ops)
	}

	return outputText(cfg, ops)
}

// outputJSON generates JSON format output.
func outputJSON(cfg *AnalyzeConfig, ops []storage.Operation) error {
	report := JSONAnalysisReport{
		SessionID:       cfg.SessionID,
		TotalOperations: len(ops),
	}

	if cfg.FindSlow {
		slowOps, err := analysis.FindSlowOperations(ops, cfg.SlowThreshold)
		if err != nil {
			return fmt.Errorf("slow operation analysis failed: %w", err)
		}

		maxDisplay := 10
		displayCount := len(slowOps)
		if displayCount > maxDisplay {
			displayCount = maxDisplay
		}

		for i := 0; i < displayCount; i++ {
			slow := &slowOps[i]
			resource := fmt.Sprintf("%s/%s/%s",
				slow.Operation.ResourceKind,
				slow.Operation.Namespace,
				slow.Operation.Name)
			report.SlowOperations = append(report.SlowOperations, JSONSlowOperation{
				Index:      slow.Index,
				Type:       string(slow.Operation.OperationType),
				Resource:   resource,
				DurationMs: slow.DurationMs,
			})
		}
	}

	if cfg.DetectLoops {
		patterns, err := analysis.DetectLoops(ops, cfg.LoopWindow)
		if err != nil {
			return fmt.Errorf("loop detection failed: %w", err)
		}

		report.LoopsDetected = make([]JSONLoopDetection, 0, len(patterns))
		for _, pattern := range patterns {
			report.LoopsDetected = append(report.LoopsDetected, JSONLoopDetection{
				StartIndex:  pattern.StartIndex,
				EndIndex:    pattern.EndIndex,
				RepeatCount: pattern.RepeatCount,
				Description: pattern.Description,
			})
		}
	}

	if cfg.AnalyzeErrors {
		summary, err := analysis.AnalyzeErrors(ops)
		if err != nil {
			return fmt.Errorf("error analysis failed: %w", err)
		}

		report.Errors = &JSONErrorSummary{
			Total:  summary.TotalErrors,
			ByType: summary.ErrorsByType,
		}
	}

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}

// createStorageConfig creates storage configuration.
func createStorageConfig(cfg *AnalyzeConfig) storage.StorageConfig {
	err := assert.AssertNotNil(cfg, "config")
	if err != nil {
		return storage.StorageConfig{}
	}

	storeCfg := storage.StorageConfig{
		Type:          cfg.StorageType,
		MaxOperations: 1000000, // Use default max operations
	}

	if cfg.StorageType == "sqlite" {
		storeCfg.ConnectionURI = cfg.DatabasePath
	} else if cfg.StorageType == "mongodb" {
		storeCfg.ConnectionURI = cfg.MongoURI
		storeCfg.DatabaseName = cfg.MongoDatabase
		storeCfg.CollectionName = "operations"
	}

	return storeCfg
}

// outputText generates text format output.
func outputText(cfg *AnalyzeConfig, ops []storage.Operation) error {
	fmt.Printf("Analyzing %d operations for session: %s\n\n",
		len(ops), cfg.SessionID)

	if cfg.DetectLoops {
		err := analyzeLoops(ops, cfg.LoopWindow)
		if err != nil {
			return err
		}
	}

	if cfg.FindSlow {
		err := analyzeSlowOps(ops, cfg.SlowThreshold)
		if err != nil {
			return err
		}
	}

	if cfg.AnalyzeErrors {
		err := analyzeErrorPatterns(ops)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateAnalyzeConfig validates configuration.
func validateAnalyzeConfig(cfg *AnalyzeConfig) error {
	// Set default storage type if not specified (for tests)
	if cfg.StorageType == "" {
		cfg.StorageType = "sqlite"
	}

	// Storage type validation
	if cfg.StorageType != "sqlite" && cfg.StorageType != "mongodb" {
		return fmt.Errorf("invalid storage type: %s (must be 'sqlite' or 'mongodb')", cfg.StorageType)
	}

	// Storage-specific validations
	if cfg.StorageType == "sqlite" {
		err := assert.AssertStringNotEmpty(cfg.DatabasePath, "database path")
		if err != nil {
			return err
		}
	} else if cfg.StorageType == "mongodb" {
		err := assert.AssertStringNotEmpty(cfg.MongoURI, "mongo URI")
		if err != nil {
			return err
		}
		err = assert.AssertStringNotEmpty(cfg.MongoDatabase, "mongo database")
		if err != nil {
			return err
		}
	}

	err := assert.AssertStringNotEmpty(cfg.SessionID, "session ID")
	if err != nil {
		return err
	}

	err = assert.AssertInRange(
		cfg.LoopWindow,
		2,
		100,
		"loop window",
	)
	if err != nil {
		return err
	}

	err = assert.AssertInRange(
		int(cfg.SlowThreshold),
		1,
		1000000,
		"slow threshold",
	)
	if err != nil {
		return err
	}

	if cfg.Format != "text" && cfg.Format != "json" {
		return fmt.Errorf("invalid format: %s (must be 'text' or 'json')", cfg.Format)
	}

	return nil
}

// analyzeLoops detects loop patterns.
func analyzeLoops(ops []storage.Operation, window int) error {
	fmt.Println("=== Loop Detection ===")

	patterns, err := analysis.DetectLoops(ops, window)
	if err != nil {
		return fmt.Errorf("loop detection failed: %w", err)
	}

	if len(patterns) == 0 {
		fmt.Println("No loop patterns detected")
		fmt.Println()
		return nil
	}

	fmt.Printf("Found %d potential loops:\n", len(patterns))

	maxDisplay := 10
	count := 0

	for count < len(patterns) && count < maxDisplay {
		p := &patterns[count]
		fmt.Printf("  [%d-%d] %s (repeated %d times)\n",
			p.StartIndex,
			p.EndIndex,
			p.Description,
			p.RepeatCount,
		)
		count = count + 1
	}

	if len(patterns) > maxDisplay {
		fmt.Printf("  ... and %d more\n", len(patterns)-maxDisplay)
	}

	fmt.Println()
	return nil
}

// analyzeSlowOps finds slow operations.
func analyzeSlowOps(ops []storage.Operation, threshold int64) error {
	fmt.Println("=== Slow Operations ===")

	slowOps, err := analysis.FindSlowOperations(ops, threshold)
	if err != nil {
		return fmt.Errorf("slow operation analysis failed: %w", err)
	}

	if len(slowOps) == 0 {
		fmt.Printf("No operations slower than %dms\n", threshold)
		fmt.Println()
		return nil
	}

	fmt.Printf("Found %d slow operations (>%dms):\n", len(slowOps), threshold)

	maxDisplay := 10
	count := 0

	for count < len(slowOps) && count < maxDisplay {
		slow := &slowOps[count]
		fmt.Printf("  [%d] %s %s/%s/%s: %dms\n",
			slow.Index,
			slow.Operation.OperationType,
			slow.Operation.ResourceKind,
			slow.Operation.Namespace,
			slow.Operation.Name,
			slow.DurationMs,
		)
		count = count + 1
	}

	if len(slowOps) > maxDisplay {
		fmt.Printf("  ... and %d more\n", len(slowOps)-maxDisplay)
	}

	fmt.Println()
	return nil
}

// analyzeErrorPatterns analyzes error patterns.
func analyzeErrorPatterns(ops []storage.Operation) error {
	fmt.Println("=== Error Analysis ===")

	summary, err := analysis.AnalyzeErrors(ops)
	if err != nil {
		return fmt.Errorf("error analysis failed: %w", err)
	}

	if summary.TotalErrors == 0 {
		fmt.Println("No errors found")
		fmt.Println()
		return nil
	}

	fmt.Printf("Total Errors: %d\n", summary.TotalErrors)
	fmt.Println("\nErrors by Type:")

	maxTypes := 20
	count := 0

	for errType, errCount := range summary.ErrorsByType {
		if count >= maxTypes {
			break
		}
		fmt.Printf("  %s: %d\n", errType, errCount)
		count = count + 1
	}

	if summary.FirstError != nil {
		fmt.Printf("\nFirst Error (seq %d): %s\n",
			summary.FirstError.SequenceNumber,
			summary.FirstError.Error,
		)
	}

	if summary.LastError != nil {
		fmt.Printf("Last Error (seq %d): %s\n",
			summary.LastError.SequenceNumber,
			summary.LastError.Error,
		)
	}

	fmt.Println()
	return nil
}
