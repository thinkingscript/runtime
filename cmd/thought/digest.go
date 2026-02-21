package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/script"
)

var digestCmd = &cobra.Command{
	Use:          "digest <thought>",
	Aliases:      []string{"fingerprint", "hash"},
	Short:        "Print the content fingerprint of a thought",
	Long:         "Print the SHA256 fingerprint of a thought's content. This is used for cache invalidation.\nAccepts an installed thought name, local file path, or URL.",
	Args:         cobra.ExactArgs(1),
	RunE:         runDigest,
	SilenceUsage: true,
}

func runDigest(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "digest")
	if err != nil {
		return err
	}

	parsed, err := script.Parse(resolved.Path)
	if err != nil {
		return fmt.Errorf("parsing script: %w", err)
	}

	fmt.Println(parsed.Fingerprint)
	return nil
}
