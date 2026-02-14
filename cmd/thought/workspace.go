package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/thinkingscript/cli/internal/config"
	"github.com/spf13/cobra"
)

var workspaceOpenFlag bool

var workspaceCmd = &cobra.Command{
	Use:          "workspace <script>",
	Aliases:      []string{"ws"},
	Short:        "Show or open a script's workspace directory",
	Args:         cobra.ExactArgs(1),
	RunE:         runWorkspace,
	SilenceUsage: true,
}

func init() {
	workspaceCmd.Flags().BoolVar(&workspaceOpenFlag, "open", false, "Open the workspace directory in Finder/file manager")
}

func runWorkspace(cmd *cobra.Command, args []string) error {
	scriptPath := args[0]
	cacheDir, err := config.CacheDir(scriptPath)
	if err != nil {
		return fmt.Errorf("computing cache dir: %w", err)
	}

	workspaceDir := filepath.Join(cacheDir, "workspace")
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No workspace yet.")
		return nil
	}

	if workspaceOpenFlag {
		var open string
		switch runtime.GOOS {
		case "darwin":
			open = "open"
		case "linux":
			open = "xdg-open"
		default:
			return fmt.Errorf("open not supported on %s", runtime.GOOS)
		}
		return exec.Command(open, workspaceDir).Run()
	}

	fmt.Println(workspaceDir)
	return nil
}
