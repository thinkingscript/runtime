package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/provider"
)

type runCommandInput struct {
	Command string `json:"command"`
}

type runCommandOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func (r *Registry) registerCommand(approver *approval.Approver) {
	r.register(provider.ToolDefinition{
		Name:        "run_command",
		Description: "Execute a shell command via sh -c. Returns stdout, stderr, and exit code. Requires user approval.",
		InputSchema: provider.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			Required: []string{"command"},
		},
	}, func(ctx context.Context, input json.RawMessage) (string, error) {
		var args runCommandInput
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("parsing run_command input: %w", err)
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = io.MultiWriter(&stdout, os.Stderr)
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

		exitCode := 0
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return "", fmt.Errorf("executing command: %w", err)
			}
		}

		out := runCommandOutput{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		}
		result, _ := json.Marshal(out)
		return string(result), nil
	}, func(input json.RawMessage) (bool, error) {
		var args runCommandInput
		if err := json.Unmarshal(input, &args); err != nil {
			return false, fmt.Errorf("parsing run_command input: %w", err)
		}
		return approver.ApproveCommand(args.Command)
	})
}
