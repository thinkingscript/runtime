package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
	"github.com/thinkingscript/cli/internal/script"
)

var (
	clearFlag    bool
	clearAllFlag bool
)

var cacheCmd = &cobra.Command{
	Use:          "cache [script]",
	Short:        "Manage per-script cache directories",
	Long:         "Print or clear cache directories used to store approval decisions and script metadata.\n\nIf the argument is not a file, looks for an installed binary in ~/.thinkingscript/bin/.",
	Args:         cobra.MaximumNArgs(1),
	RunE:         runCache,
	SilenceUsage: true,
}

func init() {
	cacheCmd.Flags().BoolVar(&clearFlag, "clear", false, "Clear cache for the specified script")
	cacheCmd.Flags().BoolVar(&clearAllFlag, "clear-all", false, "Clear all script caches")
}

// resolveScriptPath resolves a script reference to an actual path.
// If the arg is an existing file, returns it directly.
// If not, checks for an installed binary in ~/.thinkingscript/bin/.
// Returns the resolved path and whether both file and binary exist (ambiguous).
func resolveScriptPath(arg string) (path string, ambiguous bool, err error) {
	// Check if it's an existing file
	fileExists := false
	if _, err := os.Stat(arg); err == nil {
		fileExists = true
	}

	// Check if there's a binary with this name
	binPath := filepath.Join(config.BinDir(), arg)
	binExists := false
	if _, err := os.Stat(binPath); err == nil {
		binExists = true
	}

	// Both exist - ambiguous
	if fileExists && binExists {
		return arg, true, nil // prefer file, but warn
	}

	// File exists
	if fileExists {
		return arg, false, nil
	}

	// Binary exists
	if binExists {
		return binPath, false, nil
	}

	// Neither exists - return original arg, let script.Parse handle the error
	return arg, false, nil
}

func runCache(cmd *cobra.Command, args []string) error {
	if clearAllFlag {
		cacheBase := filepath.Join(config.HomeDir(), "cache")
		if err := os.RemoveAll(cacheBase); err != nil {
			return fmt.Errorf("clearing all caches: %w", err)
		}
		if err := os.MkdirAll(cacheBase, 0700); err != nil {
			return fmt.Errorf("recreating cache dir: %w", err)
		}
		fmt.Fprintln(os.Stderr, "All caches cleared.")
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("script path required (or use --clear-all)")
	}

	scriptPath, ambiguous, err := resolveScriptPath(args[0])
	if err != nil {
		return err
	}
	if ambiguous {
		binPath := filepath.Join(config.BinDir(), args[0])
		fmt.Fprintf(os.Stderr, "Note: both file '%s' and binary '%s' exist. Using file.\n", args[0], binPath)
	}

	parsed, err := script.Parse(scriptPath)
	if err != nil {
		return fmt.Errorf("parsing script: %w", err)
	}
	cacheDir := config.CacheDir(parsed.Fingerprint)

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
