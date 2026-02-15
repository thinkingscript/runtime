package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/provider"
	"github.com/thinkingscript/cli/internal/sandbox"
	"github.com/thinkingscript/cli/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

type runScriptInput struct {
	Code string `json:"code"`
}

func (r *Registry) registerScript(approver *approval.Approver, workDir, workspaceDir, memoriesDir, scriptName string) {
	r.register(provider.ToolDefinition{
		Name:        "run_script",
		Description: "Execute JavaScript code in a sandboxed runtime. Has access to the filesystem (current directory and workspace freely; other paths require user approval), HTTP, environment variables, and system info. Use this for all tasks: file I/O, data processing, HTTP requests, and transformations.",
		InputSchema: provider.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "JavaScript code to execute. The last expression value is returned as the result.",
				},
			},
			Required: []string{"code"},
		},
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		var args runScriptInput
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("parsing run_script input: %w", err)
		}

		memoriesPrefix := memoriesDir + string(filepath.Separator)
		dotStyle := ui.Renderer.NewStyle().Foreground(lipgloss.Color("213"))
		detailStyle := ui.Renderer.NewStyle().Foreground(lipgloss.Color("245"))

		sb, err := sandbox.New(sandbox.Config{
			AllowedPaths:  []string{workDir, workspaceDir, memoriesDir},
			WritablePaths: []string{workspaceDir, memoriesDir},
			WorkDir:       workDir,
			Stderr:       os.Stderr,
			ApprovePath:  approver.ApprovePath,
			ApproveEnv:   approver.ApproveEnvRead,
			ApproveNet:   approver.ApproveNet,
			OnWrite: func(path, content string) {
				if strings.HasPrefix(path, memoriesPrefix) {
					name := filepath.Base(path)
					fmt.Fprintf(os.Stderr, "\n%s %s %s\n\n", dotStyle.Render("●"), scriptName, detailStyle.Render("memorizing "+name))
					for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
						fmt.Fprintf(os.Stderr, "  %s\n", detailStyle.Render(line))
					}
					fmt.Fprintf(os.Stderr, "\n  %s\n", detailStyle.Render(path))
				}
			},
		})
		if err != nil {
			return "", fmt.Errorf("creating sandbox: %w", err)
		}

		result, err := sb.Run(ctx, args.Code)
		if err != nil {
			return "", err
		}
		return result, nil
	}, nil)
}
