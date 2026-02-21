package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
)

var openCmd = &cobra.Command{
	Use:          "open <thought>",
	Short:        "Open a thought's directory in the file manager",
	Long:         "Open the data directory for an installed thought, local file, or URL in the system file manager.",
	Args:         cobra.ExactArgs(1),
	RunE:         runOpen,
	SilenceUsage: true,
}

func runOpen(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "open")
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
		fmt.Println(thoughtDir)
		return nil
	}

	var opener string
	switch goruntime.GOOS {
	case "darwin":
		opener = "open"
	case "linux":
		opener = "xdg-open"
	case "windows":
		opener = "explorer"
	default:
		// Fallback: just print the path
		fmt.Println(thoughtDir)
		return nil
	}

	if err := exec.Command(opener, thoughtDir).Run(); err != nil {
		// If open fails, just print the path
		fmt.Println(thoughtDir)
	}
	return nil
}
