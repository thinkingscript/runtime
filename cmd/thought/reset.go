package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var (
	resetAllFlag      bool
	resetMemoriesFlag bool
	resetPolicyFlag   bool
)

var resetCmd = &cobra.Command{
	Use:   "reset <name|file>",
	Short: "Reset a thought's state",
	Long: `Reset a thought's memory.js, lib/, and tmp/ directories.

By default, this command removes:
  - memory.js (the static script)
  - lib/ (persistent modules)
  - tmp/ (scratch files)

Use --memories to also clear the memories/ directory.
Use --policy to also reset policy.json to defaults.
Use --all to clear everything.

Examples:
  thought reset weather          # Reset installed thought
  thought reset ./weather.md     # Reset thought for a file
  thought reset weather --all    # Reset everything including memories and policy`,
	Args:         cobra.ExactArgs(1),
	RunE:         runReset,
	SilenceUsage: true,
}

func init() {
	resetCmd.Flags().BoolVarP(&resetAllFlag, "all", "a", false, "Reset everything (memory.js, lib, tmp, memories, policy)")
	resetCmd.Flags().BoolVar(&resetMemoriesFlag, "memories", false, "Also clear memories/")
	resetCmd.Flags().BoolVar(&resetPolicyFlag, "policy", false, "Also reset policy.json")
}

func runReset(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "reset")
	if err != nil {
		return err
	}

	name := resolved.Name
	thoughtDir := filepath.Join(config.HomeDir(), "thoughts", name)

	// Check if thought directory exists
	if _, err := os.Stat(thoughtDir); os.IsNotExist(err) {
		return fmt.Errorf("no thought data found for '%s'", name)
	}

	// Define what to clear
	memoryJS := filepath.Join(thoughtDir, "memory.js")
	libDir := filepath.Join(thoughtDir, "lib")
	tmpDir := filepath.Join(thoughtDir, "tmp")
	memoriesDir := filepath.Join(thoughtDir, "memories")
	policyJSON := filepath.Join(thoughtDir, "policy.json")

	// Always clear: memory.js, lib/, tmp/
	cleared := []string{}

	if _, err := os.Stat(memoryJS); err == nil {
		if err := os.Remove(memoryJS); err != nil {
			return fmt.Errorf("removing memory.js: %w", err)
		}
		cleared = append(cleared, "memory.js")
	}

	if _, err := os.Stat(libDir); err == nil {
		if err := os.RemoveAll(libDir); err != nil {
			return fmt.Errorf("removing lib/: %w", err)
		}
		cleared = append(cleared, "lib/")
	}

	if _, err := os.Stat(tmpDir); err == nil {
		if err := os.RemoveAll(tmpDir); err != nil {
			return fmt.Errorf("removing tmp/: %w", err)
		}
		cleared = append(cleared, "tmp/")
	}

	// Optionally clear memories/
	if resetAllFlag || resetMemoriesFlag {
		if _, err := os.Stat(memoriesDir); err == nil {
			if err := os.RemoveAll(memoriesDir); err != nil {
				return fmt.Errorf("removing memories/: %w", err)
			}
			cleared = append(cleared, "memories/")
		}
	}

	// Optionally clear policy.json
	if resetAllFlag || resetPolicyFlag {
		if _, err := os.Stat(policyJSON); err == nil {
			if err := os.Remove(policyJSON); err != nil {
				return fmt.Errorf("removing policy.json: %w", err)
			}
			cleared = append(cleared, "policy.json")
		}
	}

	if len(cleared) == 0 {
		fmt.Fprintf(os.Stderr, "Nothing to reset for '%s'\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Reset '%s': %v\n", name, cleared)
	}

	return nil
}
