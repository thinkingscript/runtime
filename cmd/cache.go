package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bradgessler/agent-exec/internal/config"
	"github.com/spf13/cobra"
)

var (
	clearFlag    bool
	clearAllFlag bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache [script]",
	Short: "Manage per-script cache directories",
	Long:  "Print or clear cache directories used to store approval decisions and script metadata.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCache,
	SilenceUsage: true,
}

func init() {
	cacheCmd.Flags().BoolVar(&clearFlag, "clear", false, "Clear cache for the specified script")
	cacheCmd.Flags().BoolVar(&clearAllFlag, "clear-all", false, "Clear all script caches")
}

func runCache(cmd *cobra.Command, args []string) error {
	if clearAllFlag {
		cacheBase := filepath.Join(config.HomeDir(), "cache")
		if err := os.RemoveAll(cacheBase); err != nil {
			return fmt.Errorf("clearing all caches: %w", err)
		}
		if err := os.MkdirAll(cacheBase, 0755); err != nil {
			return fmt.Errorf("recreating cache dir: %w", err)
		}
		fmt.Fprintln(os.Stderr, "All caches cleared.")
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("script path required (or use --clear-all)")
	}

	scriptPath := args[0]
	cacheDir, err := config.CacheDir(scriptPath)
	if err != nil {
		return fmt.Errorf("computing cache dir: %w", err)
	}

	if clearFlag {
		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("clearing cache: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Cache cleared.")
		return nil
	}

	// Default: print the cache directory path
	fmt.Println(cacheDir)
	return nil
}
