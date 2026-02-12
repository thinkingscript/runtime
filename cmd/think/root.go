package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/thinkingscript/cli/internal/agent"
	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/arguments"
	"github.com/thinkingscript/cli/internal/config"
	"github.com/thinkingscript/cli/internal/provider"
	"github.com/thinkingscript/cli/internal/script"
	"github.com/thinkingscript/cli/internal/tools"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var rootCmd = &cobra.Command{
	Use:          "think <script> [args...]",
	Short:        "A shebang interpreter for natural language scripts",
	Long:         "think runs natural language .thought scripts by sending them to an LLM that uses tools to accomplish the described task.",
	Args:         cobra.MinimumNArgs(1),
	RunE:         runScript,
	SilenceUsage: true,
}

func execute(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().SetInterspersed(false)
}

func runScript(cmd *cobra.Command, args []string) error {
	scriptPath := args[0]

	// Parse script
	parsed, err := script.Parse(scriptPath)
	if err != nil {
		return err
	}

	// Resolve configuration
	resolved := config.Resolve(parsed.Config)

	// Ensure home directory exists
	if err := config.EnsureHomeDir(); err != nil {
		return fmt.Errorf("setting up home directory: %w", err)
	}

	// Set up cache directory
	cacheDir, err := config.CacheDir(scriptPath)
	if err != nil {
		return fmt.Errorf("computing cache dir: %w", err)
	}

	// Check fingerprint and manage cache
	if config.CheckFingerprint(cacheDir, parsed.Fingerprint) {
		// Cache is valid, reuse approvals
	} else {
		// Cache is stale or doesn't exist — wipe and recreate
		os.RemoveAll(cacheDir)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return fmt.Errorf("creating cache dir: %w", err)
		}
		if err := config.WriteFingerprint(cacheDir, parsed.Fingerprint); err != nil {
			return fmt.Errorf("writing fingerprint: %w", err)
		}
		if err := config.WriteMeta(cacheDir, scriptPath); err != nil {
			return fmt.Errorf("writing meta: %w", err)
		}
	}

	// Read stdin if piped
	stdinData := ""
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		stdinData = string(data)
	}

	// Set up named arguments store and approval system
	argStore := arguments.NewStore()
	approver := approval.NewApprover(resolved.Wreckless, cacheDir, argStore)

	// Set up tool registry
	registry := tools.NewRegistry(approver, stdinData, argStore)

	// Create provider
	p, err := createProvider(resolved)
	if err != nil {
		return err
	}

	// Build prompt: script content + any CLI arguments
	prompt := parsed.Prompt
	if len(args) > 1 {
		prompt += "\n\nArguments: " + strings.Join(args[1:], " ")
	}

	// Run agent loop
	a := agent.New(p, registry, resolved.Model, resolved.MaxTokens, resolved.MaxIterations)
	return a.Run(cmd.Context(), prompt)
}

func createProvider(cfg *config.ResolvedConfig) (provider.Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return provider.NewAnthropicProvider(cfg.APIKey), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}
