package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var runCmd = &cobra.Command{
	Use:          "run <name> [args...]",
	Short:        "Run an installed thought",
	Long:         "Execute a thought binary from ~/.thinkingscript/bin/ with the given arguments.",
	Args:         cobra.MinimumNArgs(1),
	RunE:         runRun,
	SilenceUsage: true,
}

func runRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	thoughtArgs := args[1:]

	binPath := filepath.Join(config.BinDir(), name)

	if info, err := os.Stat(binPath); err != nil || info.IsDir() {
		return fmt.Errorf("thought '%s' not found in %s", name, config.BinDir())
	}

	// Execute the thought binary
	thoughtCmd := exec.Command(binPath, thoughtArgs...)
	thoughtCmd.Stdin = os.Stdin
	thoughtCmd.Stdout = os.Stdout
	thoughtCmd.Stderr = os.Stderr

	if err := thoughtCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("running thought: %w", err)
	}

	return nil
}
