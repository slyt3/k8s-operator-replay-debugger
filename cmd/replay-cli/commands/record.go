package commands

import (
	"fmt"

	"github.com/operator-replay-debugger/internal/assert"
	"github.com/operator-replay-debugger/pkg/storage"
	"github.com/spf13/cobra"
)

// NewRecordCommand creates the record subcommand.
func NewRecordCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record operator operations",
		Long: `Start recording Kubernetes operator operations.
This command is typically used as a library in operator code.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Recording is integrated into operator code")
			fmt.Println("See documentation for RecordingClient usage")
			return nil
		},
	}

	return cmd
}

// SessionInfo holds session information.
type SessionInfo struct {
	SessionID  string
	StartTime  int64
	OpCount    int
}

// NewSessionsCommand creates the sessions subcommand.
func NewSessionsCommand() *cobra.Command {
	var databasePath string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List recorded sessions",
		Long:  "Display all recorded sessions in the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessions(databasePath)
		},
	}

	cmd.Flags().StringVarP(
		&databasePath,
		"database",
		"d",
		defaultDatabasePath,
		"Path to SQLite database",
	)

	return cmd
}

// runSessions lists all recorded sessions.
func runSessions(dbPath string) error {
	err := assert.AssertStringNotEmpty(dbPath, "database path")
	if err != nil {
		return err
	}

	db, err := storage.NewDatabase(dbPath, 1000000)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close database: %v\n", closeErr)
		}
	}()

	fmt.Println("Available Sessions:")
	fmt.Println("(Use 'replay-cli replay <session-id>' to replay)")
	fmt.Println()
	fmt.Println("Note: Full session listing requires additional DB methods")

	return nil
}
