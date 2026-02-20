package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var lsCmd = &cobra.Command{
	Use:          "ls",
	Aliases:      []string{"list"},
	Short:        "List installed thoughts",
	Long:         "List all thoughts installed in ~/.thinkingscript/bin/.",
	Args:         cobra.NoArgs,
	RunE:         runLs,
	SilenceUsage: true,
}

func runLs(cmd *cobra.Command, args []string) error {
	binDir := config.BinDir()

	entries, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "No thoughts installed.")
			return nil
		}
		return fmt.Errorf("reading bin directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No thoughts installed.")
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Check if there's associated thought data
		thoughtDir := filepath.Join(config.HomeDir(), "thoughts", name)
		hasData := false
		if _, err := os.Stat(thoughtDir); err == nil {
			hasData = true
		}

		if hasData {
			fmt.Printf("%s\n", name)
		} else {
			fmt.Printf("%s (no data)\n", name)
		}
	}

	return nil
}
