package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slyt3/kubestep/internal/assert"
	"github.com/slyt3/kubestep/pkg/analysis"
	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/spf13/cobra"
)

const (
	defaultCausalityDepth = 6
	defaultMaxChains      = 10
	maxWindowItems        = 100000
	maxWarningItems       = 50
)

// CausalityConfig holds causality command configuration.
type CausalityConfig struct {
	DatabasePath   string
	SessionID      string
	Format         string
	Window         string
	MaxDepth       int
	IncludePayload bool
	StorageType    string
	MongoURI       string
	MongoDatabase  string
}

// NewCausalityCommand creates the analyze causality subcommand.
func NewCausalityCommand() *cobra.Command {
	cfg := &CausalityConfig{}

	cmd := &cobra.Command{
		Use:   "causality",
		Short: "Analyze cross-controller causality chains",
		Long: `Infer causal chains between writes and reconciles:
controller A WRITE -> controller B RECONCILE -> controller B WRITE -> ...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCausality(cfg)
		},
	}

	cmd.Flags().StringVarP(
		&cfg.DatabasePath,
		"database",
		"d",
		defaultDatabasePath,
		"Path to SQLite database",
	)

	cmd.Flags().StringVar(
		&cfg.SessionID,
		"session",
		"",
		"Session ID to analyze (required)",
	)

	cmd.Flags().StringVar(
		&cfg.Format,
		"format",
		"text",
		"Output format: text or json",
	)

	cmd.Flags().StringVar(
		&cfg.Window,
		"window",
		"",
		"Time window filter: <start,end> (RFC3339 or unix seconds)",
	)

	cmd.Flags().IntVar(
		&cfg.MaxDepth,
		"max-depth",
		defaultCausalityDepth,
		"Maximum depth for causal chains",
	)

	cmd.Flags().BoolVar(
		&cfg.IncludePayload,
		"include-payloads",
		false,
		"Include resource payloads in JSON output",
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
		"kubestep",
		"MongoDB database name",
	)

	return cmd
}

func runCausality(cfg *CausalityConfig) error {
	err := validateCausalityConfig(cfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	storeCfg := createStorageConfig(&AnalyzeConfig{
		DatabasePath:  cfg.DatabasePath,
		StorageType:   cfg.StorageType,
		MongoURI:      cfg.MongoURI,
		MongoDatabase: cfg.MongoDatabase,
	})

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

	spans, err := store.QueryReconcileSpans(cfg.SessionID)
	if err != nil {
		return fmt.Errorf("failed to load reconcile spans: %w", err)
	}

	start, end, err := parseWindow(cfg.Window)
	if err != nil {
		return fmt.Errorf("invalid window: %w", err)
	}

	if start != nil || end != nil {
		ops = filterOperationsByWindow(ops, start, end)
		spans = filterSpansByWindow(spans, start, end)
	}

	graph, warnings, err := analysis.BuildCausalityGraph(
		ops,
		spans,
		analysis.CausalityOptions{IncludePayloads: cfg.IncludePayload},
	)
	if err != nil {
		return fmt.Errorf("causality analysis failed: %w", err)
	}

	if cfg.Format == "json" {
		return outputCausalityJSON(cfg, graph, warnings)
	}

	return outputCausalityText(cfg, graph, warnings)
}

func validateCausalityConfig(cfg *CausalityConfig) error {
	err := assert.AssertNotNil(cfg, "config")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(cfg.SessionID, "session ID")
	if err != nil {
		return err
	}

	if cfg.Format != "text" && cfg.Format != "json" {
		return fmt.Errorf("invalid format: %s (must be 'text' or 'json')", cfg.Format)
	}

	if cfg.MaxDepth < 2 || cfg.MaxDepth > 50 {
		return fmt.Errorf("invalid max-depth: %d (must be 2-50)", cfg.MaxDepth)
	}

	if cfg.StorageType != "sqlite" && cfg.StorageType != "mongodb" {
		return fmt.Errorf("invalid storage type: %s (must be 'sqlite' or 'mongodb')", cfg.StorageType)
	}

	if cfg.StorageType == "sqlite" {
		err = assert.AssertStringNotEmpty(cfg.DatabasePath, "database path")
		if err != nil {
			return err
		}
	}

	if cfg.StorageType == "mongodb" {
		err = assert.AssertStringNotEmpty(cfg.MongoURI, "mongo URI")
		if err != nil {
			return err
		}
		err = assert.AssertStringNotEmpty(cfg.MongoDatabase, "mongo database")
		if err != nil {
			return err
		}
	}

	return nil
}

func outputCausalityJSON(
	cfg *CausalityConfig,
	graph *analysis.CausalityGraph,
	warnings []string,
) error {
	report := struct {
		Nodes    []analysis.CausalityNode `json:"nodes"`
		Edges    []analysis.CausalityEdge `json:"edges"`
		Warnings []string                 `json:"warnings,omitempty"`
	}{
		Nodes:    graph.Nodes,
		Edges:    graph.Edges,
		Warnings: warnings,
	}

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, string(jsonBytes))
	return nil
}

func outputCausalityText(
	cfg *CausalityConfig,
	graph *analysis.CausalityGraph,
	warnings []string,
) error {
	fmt.Printf("Causality Graph for session: %s\n\n", cfg.SessionID)

	if len(warnings) > 0 {
		fmt.Println("Warnings:")
		maxWarnings := len(warnings)
		if maxWarnings > maxWarningItems {
			maxWarnings = maxWarningItems
		}
		for i := 0; i < maxWarnings; i++ {
			fmt.Printf("  - %s\n", warnings[i])
		}
		fmt.Println()
	}

	chains := analysis.BuildCausalityChains(graph, cfg.MaxDepth, defaultMaxChains)
	if len(chains) == 0 {
		fmt.Println("No causal chains found.")
		return nil
	}

	nodeByID := make(map[string]analysis.CausalityNode, len(graph.Nodes))
	maxNodes := len(graph.Nodes)
	if maxNodes > maxWindowItems {
		maxNodes = maxWindowItems
	}
	for i := 0; i < maxNodes; i++ {
		nodeByID[graph.Nodes[i].ID] = graph.Nodes[i]
	}

	fmt.Printf("Top causal chains (max depth %d):\n", cfg.MaxDepth)
	for i := 0; i < len(chains) && i < defaultMaxChains; i++ {
		chain := chains[i]
		parts := make([]string, 0, len(chain.NodeIDs))
		for j := 0; j < len(chain.NodeIDs); j++ {
			node := nodeByID[chain.NodeIDs[j]]
			parts = append(parts, formatCausalityNode(node))
		}

		fmt.Printf("%2d) len=%d fanout=%d %s\n",
			i+1,
			chain.Length,
			chain.FanOut,
			strings.Join(parts, " -> "),
		)
	}

	return nil
}

func formatCausalityNode(node analysis.CausalityNode) string {
	ref := formatResourceRef(node.Namespace, node.Name)
	ts := ""
	if !node.Timestamp.IsZero() {
		ts = node.Timestamp.Format(time.RFC3339)
	}

	switch node.Type {
	case analysis.NodeTypeOperation:
		return fmt.Sprintf("op[%s %s %s rv=%s uid=%s ts=%s]",
			node.ActorID,
			node.Kind,
			ref,
			node.ResourceVer,
			node.UID,
			ts,
		)
	case analysis.NodeTypeSpan:
		return fmt.Sprintf("span[%s %s %s rv=%s uid=%s dur=%dms ts=%s]",
			node.ActorID,
			node.Kind,
			ref,
			node.ResourceVer,
			node.UID,
			node.DurationMs,
			ts,
		)
	default:
		return fmt.Sprintf("node[%s]", node.ID)
	}
}

func formatResourceRef(namespace, name string) string {
	if len(namespace) == 0 {
		return name
	}
	if len(name) == 0 {
		return namespace
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}

func parseWindow(window string) (*time.Time, *time.Time, error) {
	if len(window) == 0 {
		return nil, nil, nil
	}

	parts := strings.Split(window, ",")
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("window must be in <start,end> format")
	}

	start, err := parseTimeValue(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid start time: %w", err)
	}

	end, err := parseTimeValue(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid end time: %w", err)
	}

	if start != nil && end != nil && start.After(*end) {
		return nil, nil, fmt.Errorf("start time must be before end time")
	}

	return start, end, nil
}

func parseTimeValue(value string) (*time.Time, error) {
	if len(value) == 0 {
		return nil, nil
	}

	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}

	epoch, err := strconvParseInt(value)
	if err != nil {
		return nil, err
	}

	parsed := time.Unix(epoch, 0)
	return &parsed, nil
}

func strconvParseInt(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func filterOperationsByWindow(
	ops []storage.Operation,
	start *time.Time,
	end *time.Time,
) []storage.Operation {
	filtered := make([]storage.Operation, 0, len(ops))
	maxOps := len(ops)
	if maxOps > maxWindowItems {
		maxOps = maxWindowItems
	}
	for i := 0; i < maxOps; i++ {
		op := ops[i]
		if start != nil && op.Timestamp.Before(*start) {
			continue
		}
		if end != nil && op.Timestamp.After(*end) {
			continue
		}
		filtered = append(filtered, op)
	}
	return filtered
}

func filterSpansByWindow(
	spans []storage.ReconcileSpan,
	start *time.Time,
	end *time.Time,
) []storage.ReconcileSpan {
	filtered := make([]storage.ReconcileSpan, 0, len(spans))
	maxSpans := len(spans)
	if maxSpans > maxWindowItems {
		maxSpans = maxWindowItems
	}
	for i := 0; i < maxSpans; i++ {
		span := spans[i]
		if start != nil && span.StartTime.Before(*start) {
			continue
		}
		if end != nil && span.StartTime.After(*end) {
			continue
		}
		filtered = append(filtered, span)
	}
	return filtered
}
