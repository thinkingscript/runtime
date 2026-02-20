package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var pathCmd = &cobra.Command{
	Use:          "path <name>",
	Short:        "Print the path to an installed thought",
	Long:         "Print the full path to a thought binary for use in scripts.",
	Args:         cobra.ExactArgs(1),
	RunE:         runPath,
	SilenceUsage: true,
}

func runPath(cmd *cobra.Command, args []string) error {
	name := args[0]

	binPath := filepath.Join(config.BinDir(), name)

	if info, err := os.Stat(binPath); err != nil || info.IsDir() {
		return fmt.Errorf("thought '%s' not found", name)
	}

	fmt.Println(binPath)
	return nil
}
