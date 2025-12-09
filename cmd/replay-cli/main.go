package main

import (
	"fmt"
	"os"

	"github.com/operator-replay-debugger/cmd/replay-cli/commands"
	"github.com/spf13/cobra"
)

const (
	maxCommandDepth = 5
)

var (
	version = "1.0.0"
)

func main() {
	exitCode := runCLI()
	os.Exit(exitCode)
}

// runCLI executes the command line interface.
// Rule 1: No recursion, cobra handles dispatch iteratively.
func runCLI() int {
	rootCmd := buildRootCommand()

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}

// buildRootCommand constructs the root command with subcommands.
// Rule 4: Function under 60 lines.
func buildRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "replay-cli",
		Short: "Kubernetes Operator Replay Debugger",
		Long: `Record, replay, and analyze Kubernetes operator reconciliation loops.
Helps debug operator behavior by capturing all API interactions.`,
		Version: version,
	}

	rootCmd.AddCommand(commands.NewRecordCommand())
	rootCmd.AddCommand(commands.NewReplayCommand())
	rootCmd.AddCommand(commands.NewAnalyzeCommand())
	rootCmd.AddCommand(commands.NewSessionsCommand())

	return rootCmd
}
