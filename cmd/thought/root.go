package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "thought",
	Short:        "Manage thinkingscript thoughts",
	SilenceUsage: true,
}

func execute(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(workspaceCmd)
}
