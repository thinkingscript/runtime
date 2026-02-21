package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/script"
)

var catCmd = &cobra.Command{
	Use:          "cat <name|file|url>",
	Short:        "Display a thought's content",
	Long:         "Print the script content of an installed thought, local file, or URL.",
	Args:         cobra.ExactArgs(1),
	RunE:         runCat,
	SilenceUsage: true,
}

func runCat(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "cat")
	if err != nil {
		return err
	}

	// For local files, suggest using cat directly
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
