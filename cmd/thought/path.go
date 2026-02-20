package main

import (
	"fmt"

	"github.com/spf13/cobra"
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
	resolved, err := ResolveThought(args[0], "path")
	if err != nil {
		return err
	}

	if resolved.Target == TargetFile {
		return fmt.Errorf("'%s' is a file, not an installed thought.", args[0])
	}

	fmt.Println(resolved.Path)
	return nil
}
