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
	Long:         "Remove a thought's binary from ~/.thinkingscript/bin/ and optionally its data (workspace, memories, policy).",
	Args:         cobra.ExactArgs(1),
	RunE:         runRm,
	SilenceUsage: true,
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForceFlag, "force", "f", false, "Also remove thought data (workspace, memories, policy)")
}

func runRm(cmd *cobra.Command, args []string) error {
	name := args[0]

	binPath := filepath.Join(config.BinDir(), name)
	thoughtDir := filepath.Join(config.HomeDir(), "thoughts", name)

	binExists := false
	if _, err := os.Stat(binPath); err == nil {
		binExists = true
	}

	dataExists := false
	if _, err := os.Stat(thoughtDir); err == nil {
		dataExists = true
	}

	if !binExists && !dataExists {
		return fmt.Errorf("thought '%s' not found", name)
	}

	// Remove binary
	if binExists {
		if err := os.Remove(binPath); err != nil {
			return fmt.Errorf("removing binary: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Removed %s\n", binPath)
	}

	// Remove data if --force
	if dataExists {
		if rmForceFlag {
			if err := os.RemoveAll(thoughtDir); err != nil {
				return fmt.Errorf("removing thought data: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Removed %s\n", thoughtDir)
		} else if binExists {
			fmt.Fprintf(os.Stderr, "Note: thought data remains at %s (use --force to remove)\n", thoughtDir)
		}
	}

	return nil
}
