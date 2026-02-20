package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
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
	resolved, err := ResolveThought(args[0], "run")
	if err != nil {
		return err
	}

	if resolved.Target == TargetFile {
		return fmt.Errorf("'%s' is a file, not an installed thought.\nUse 'think %s' to run the script.", args[0], args[0])
	}

	thoughtArgs := args[1:]
	binPath := resolved.Path

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
