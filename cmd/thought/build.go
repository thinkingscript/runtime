package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var outputFlag string

var buildCmd = &cobra.Command{
	Use:          "build <input>",
	Short:        "Build a .thought script with shebang and executable permissions",
	Args:         cobra.ExactArgs(1),
	RunE:         runBuild,
	SilenceUsage: true,
}

func init() {
	buildCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path (required)")
	buildCmd.MarkFlagRequired("output")
}

func runBuild(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading input file: %w", err)
	}

	shebang := "#!/usr/bin/env think\n"
	body := string(content)
	if !strings.HasPrefix(body, shebang) {
		body = shebang + body
	}

	if err := os.WriteFile(outputFlag, []byte(body), 0755); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}

	if err := os.Chmod(outputFlag, 0755); err != nil {
		return fmt.Errorf("setting executable permission: %w", err)
	}

	fmt.Println(outputFlag)
	return nil
}
