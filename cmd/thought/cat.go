package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/script"
)

var catCmd = &cobra.Command{
	Use:          "cat <name>",
	Short:        "Display a thought's script content",
	Long:         "Print the embedded script content of an installed thought.",
	Args:         cobra.ExactArgs(1),
	RunE:         runCat,
	SilenceUsage: true,
}

func runCat(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "show")
	if err != nil {
		return err
	}

	if resolved.Target == TargetFile {
		return fmt.Errorf("'%s' is a file. Use 'cat %s' directly.", args[0], args[0])
	}

	parsed, err := script.Parse(resolved.Path)
	if err != nil {
		return fmt.Errorf("parsing script: %w", err)
	}

	fmt.Print(parsed.Prompt)
	return nil
}
