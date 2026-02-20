package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/thinkingscript/cli/internal/config"
	"github.com/spf13/cobra"
)

var workspaceOpenFlag bool

var workspaceCmd = &cobra.Command{
	Use:          "workspace <script>",
	Aliases:      []string{"ws"},
	Short:        "Show or open a thought's directory (lib, tmp, memory.js, etc.)",
	Args:         cobra.ExactArgs(1),
	RunE:         runWorkspace,
	SilenceUsage: true,
}

func init() {
	workspaceCmd.Flags().BoolVar(&workspaceOpenFlag, "open", false, "Open the thought directory in Finder/file manager")
}

func runWorkspace(cmd *cobra.Command, args []string) error {
	scriptPath := args[0]
	thoughtDir := config.ThoughtDir(scriptPath)
	if _, err := os.Stat(thoughtDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No thought data yet.")
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
		return exec.Command(open, thoughtDir).Run()
	}

	fmt.Println(thoughtDir)
	return nil
}
