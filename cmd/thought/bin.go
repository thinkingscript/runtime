package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var binCmd = &cobra.Command{
	Use:          "bin <name>",
	Short:        "Print the path to an installed thought's binary",
	Long:         "Print the full path to a thought binary for use in scripts.",
	Args:         cobra.ExactArgs(1),
	RunE:         runBin,
	SilenceUsage: true,
}

func runBin(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "bin")
	if err != nil {
		return err
	}

	if resolved.Target == TargetFile {
		return fmt.Errorf("'%s' is a file, not an installed thought.", args[0])
	}

	fmt.Println(resolved.Path)
	return nil
}
