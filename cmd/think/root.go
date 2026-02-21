package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/agent"
	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/config"
	"github.com/thinkingscript/cli/internal/provider"
	"github.com/thinkingscript/cli/internal/sandbox"
	"github.com/thinkingscript/cli/internal/script"
	"github.com/thinkingscript/cli/internal/tools"
	"github.com/thinkingscript/cli/internal/ui"
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

// cacheMode returns the cache behavior: "persist" (default), "ephemeral", or "off".
func cacheMode() string {
	switch strings.ToLower(os.Getenv("THINKINGSCRIPT__CACHE")) {
	case "off", "none", "disable":
		return "off"
	case "ephemeral", "tmp":
		return "ephemeral"
	default:
		return "persist"
	}
}

func runScript(cmd *cobra.Command, args []string) error {
	scriptPath := args[0]
	mode := cacheMode()

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
	cacheDir := config.CacheDir(parsed.Fingerprint)

	if mode == "off" {
		// No persistent cache — always start fresh, clean up on exit
		os.RemoveAll(cacheDir)
		defer os.RemoveAll(cacheDir)
	}

	// Check fingerprint and manage cache
	if config.CheckFingerprint(cacheDir, parsed.Fingerprint) {
		// Cache is valid, reuse approvals
	} else {
		// Cache is stale or doesn't exist — wipe and recreate
		os.RemoveAll(cacheDir)
		if err := os.MkdirAll(cacheDir, 0700); err != nil {
			return fmt.Errorf("creating cache dir: %w", err)
		}
		if err := config.WriteFingerprint(cacheDir, parsed.Fingerprint); err != nil {
			return fmt.Errorf("writing fingerprint: %w", err)
		}
	}

	if mode == "ephemeral" {
		defer os.RemoveAll(cacheDir)
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

	// Set up sandbox paths — resolve to absolute so the LLM sees full paths
	workDir, _ := os.Getwd()
	thoughtDir, _ := filepath.Abs(config.ThoughtDir(scriptPath))
	workspaceDir, _ := filepath.Abs(config.WorkspaceDir(scriptPath))
	memoriesDir, _ := filepath.Abs(config.MemoriesDir(scriptPath))
	memoryJSPath, _ := filepath.Abs(config.MemoryJSPath(scriptPath))
	os.MkdirAll(workspaceDir, 0700)
	os.MkdirAll(memoriesDir, 0700)

	// Set up approval system
	globalPolicyPath, _ := filepath.Abs(filepath.Join(config.HomeDir(), "policy.json"))
	approver := approval.NewApprover(thoughtDir, globalPolicyPath)
	defer approver.Close()

	// Bootstrap default policy entries for workspace, memories, and CWD
	approver.BootstrapDefaults(workspaceDir, memoriesDir, workDir)

	// Try memory.js first (static execution without agent)
	resumeContext := ""
	if _, err := os.Stat(memoryJSPath); err == nil {
		code, err := os.ReadFile(memoryJSPath)
		if err != nil {
			resumeContext = fmt.Sprintf("failed to read memory.js: %s", err)
		} else {
			// SECURITY: ThoughtDir is readable but NOT writable (protects policy.json)
			// Only memory.js, workspace, and memories are writable
			sb, err := sandbox.New(sandbox.Config{
				AllowedPaths:  []string{workDir, thoughtDir, workspaceDir, memoriesDir},
				WritablePaths: []string{workspaceDir, memoriesDir, memoryJSPath},
				WorkDir:       workDir,
				Stderr:        os.Stderr,
				Args:          args[1:],
				ApprovePath:   approver.ApprovePath,
				ApproveEnv:    approver.ApproveEnvRead,
				ApproveNet:    approver.ApproveNet,
			})
			if err != nil {
				resumeContext = fmt.Sprintf("failed to create sandbox: %s", err)
			} else {
				// Show memory.js execution
				scriptName := config.ThoughtName(scriptPath)
				dotStyle := ui.Renderer.NewStyle().Foreground(lipgloss.Color("213"))
				nameStyle := ui.Renderer.NewStyle().Foreground(lipgloss.Color("255"))
				fileStyle := ui.Renderer.NewStyle().Foreground(lipgloss.Color("245"))
				fmt.Fprintf(os.Stderr, "%s %s %s\n", dotStyle.Render("●"), nameStyle.Render(scriptName), fileStyle.Render("memory.js"))

				stopSpinner := ui.Spinner("Working...")
				result, err := sb.Run(cmd.Context(), string(code))
				stopSpinner()

				if err == nil {
					// Success! memory.js handled everything
					if result != "" {
						fmt.Fprint(os.Stdout, result)
					}
					return nil
				}

				// Check if it's a resume request or an error
				var resumeErr *sandbox.ResumeError
				if errors.As(err, &resumeErr) {
					resumeContext = resumeErr.Context
				} else {
					resumeContext = fmt.Sprintf("memory.js error: %s", err)
				}
			}
		}
	} else {
		resumeContext = "no memory.js exists, first run"
	}

	// Set up tool registry
	registry := tools.NewRegistry(approver, workDir, thoughtDir, workspaceDir, memoriesDir, memoryJSPath, scriptPath)

	// Create provider
	p, err := createProvider(resolved)
	if err != nil {
		return err
	}

	// Build prompt: script content + stdin + CLI arguments
	prompt := parsed.Prompt
	if stdinData != "" {
		prompt += "\n\nStdin:\n" + stdinData
	}
	if len(args) > 1 {
		prompt += "\n\nArguments: " + strings.Join(args[1:], " ")
	}

	// Run agent loop
	a := agent.New(p, registry, resolved.Model, resolved.MaxTokens, resolved.MaxIterations, scriptPath, thoughtDir, workspaceDir, memoriesDir, memoryJSPath, mode, resumeContext)
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
