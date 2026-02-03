package commands

import (
	"fmt"

	"github.com/slyt3/kubestep/internal/assert"
	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/spf13/cobra"
)

// VerifyConfig holds verify command configuration.
type VerifyConfig struct {
	DatabasePath string
	Strict       bool
}

// NewVerifyCommand creates the verify subcommand.
func NewVerifyCommand() *cobra.Command {
	cfg := &VerifyConfig{}

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify database integrity",
		Long: `Verify database schema and data consistency.
Reports missing columns, sequence gaps, and span anomalies.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(cfg)
		},
	}

	cmd.Flags().StringVarP(
		&cfg.DatabasePath,
		"database",
		"d",
		defaultDatabasePath,
		"Path to SQLite database",
	)

	cmd.Flags().BoolVar(
		&cfg.Strict,
		"strict",
		false,
		"Treat missing optional columns as errors",
	)

	return cmd
}

func runVerify(cfg *VerifyConfig) error {
	err := assert.AssertNotNil(cfg, "config")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(cfg.DatabasePath, "database path")
	if err != nil {
		return err
	}

	result, err := storage.VerifySQLite(cfg.DatabasePath, cfg.Strict)
	if err != nil {
		return err
	}

	fmt.Printf("Database: %s\n", cfg.DatabasePath)
	fmt.Printf("Sessions: %d\n", result.Stats.Sessions)
	fmt.Printf("Operations: %d\n", result.Stats.Operations)
	if result.Stats.Spans > 0 {
		fmt.Printf("Spans: %d\n", result.Stats.Spans)
	}

	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for i := 0; i < len(result.Warnings); i++ {
			fmt.Printf("  - %s\n", result.Warnings[i])
		}
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for i := 0; i < len(result.Errors); i++ {
			fmt.Printf("  - %s\n", result.Errors[i])
		}
		return fmt.Errorf("verification failed: %d error(s)", len(result.Errors))
	}

	fmt.Println("\nVerify OK")
	return nil
}
