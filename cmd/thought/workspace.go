package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var pathCmd = &cobra.Command{
	Use:          "path <thought>",
	Aliases:      []string{"dir"},
	Short:        "Print the path to a thought's directory",
	Long:         "Print the path to a thought's data directory containing workspace/, memories/, and memory.js.\nAccepts an installed thought name, local file path, or URL.",
	Args:         cobra.ExactArgs(1),
	RunE:         runPath,
	SilenceUsage: true,
}

func runPath(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "path")
	if err != nil {
		return err
	}

	var thoughtDir string
	if resolved.Target == TargetInstalled {
		thoughtDir = filepath.Join(config.HomeDir(), "thoughts", resolved.Name)
	} else {
		thoughtDir = config.ThoughtDir(resolved.Path)
	}

	if _, err := os.Stat(thoughtDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No thought data yet.")
	}
	fmt.Println(thoughtDir)
	return nil
}
