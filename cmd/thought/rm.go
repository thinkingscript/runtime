package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var rmForceFlag bool

var rmCmd = &cobra.Command{
	Use:          "rm <name>",
	Aliases:      []string{"remove"},
	Short:        "Remove an installed thought",
	Long:         "Remove a thought's binary from ~/.thinkingscript/bin/ and optionally its data (workspace, memories, policy).\n\nUse the thought name (not a file path). For files, use 'rm' directly.",
	Args:         cobra.ExactArgs(1),
	RunE:         runRm,
	SilenceUsage: true,
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForceFlag, "force", "f", false, "Also remove thought data (workspace, memories, policy)")
}

func runRm(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "rm")
	if err != nil {
		return err
	}

	if resolved.Target == TargetFile {
		return fmt.Errorf("'%s' is a file, not an installed thought.\nTo remove the file: rm %s", args[0], args[0])
	}

	name := resolved.Name
	binPath := resolved.Path
	thoughtDir := filepath.Join(config.HomeDir(), "thoughts", name)

	// Remove binary (resolver already verified it exists)
	if err := os.Remove(binPath); err != nil {
		return fmt.Errorf("removing binary: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Removed %s\n", binPath)

	// Remove data if --force
	if _, err := os.Stat(thoughtDir); err == nil {
		if rmForceFlag {
			if err := os.RemoveAll(thoughtDir); err != nil {
				return fmt.Errorf("removing thought data: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Removed %s\n", thoughtDir)
		} else {
			fmt.Fprintf(os.Stderr, "Note: thought data remains at %s (use --force to remove)\n", thoughtDir)
		}
	}

	return nil
}
