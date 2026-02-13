package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thinkingscript/cli/internal/config"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:          "install <script>",
	Short:        "Build and install a script to the thoughts directory",
	Long:         "Builds a script with shebang and executable permissions, then installs it to ~/.thinkingscript/thoughts/ (or THINKINGSCRIPT_THOUGHTS_HOME). Add that directory to your PATH to run scripts by name.",
	Args:         cobra.ExactArgs(1),
	RunE:         runInstall,
	SilenceUsage: true,
}

func thoughtsHome() string {
	if v := os.Getenv("THINKINGSCRIPT_THOUGHTS_HOME"); v != "" {
		return v
	}
	return filepath.Join(config.HomeDir(), "thoughts")
}

func runInstall(cmd *cobra.Command, args []string) error {
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

	dir := thoughtsHome()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating thoughts directory: %w", err)
	}

	// Strip known extensions for the binary name
	base := filepath.Base(inputPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	outPath := filepath.Join(dir, name)
	if err := os.WriteFile(outPath, []byte(body), 0755); err != nil {
		return fmt.Errorf("writing script: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Installed %s → %s\n", inputPath, outPath)
	fmt.Fprintln(os.Stderr, "Make sure this is in your PATH:")
	fmt.Fprintf(os.Stderr, "  export PATH=\"%s:$PATH\"\n", dir)
	return nil
}
