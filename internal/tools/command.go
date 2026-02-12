package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/bradgessler/agent-exec/internal/approval"
	"github.com/bradgessler/agent-exec/internal/provider"
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
	}, func(input json.RawMessage) (string, error) {
		var args runCommandInput
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("parsing run_command input: %w", err)
		}

		approved, err := approver.ApproveCommand(args.Command)
		if err != nil {
			return "", fmt.Errorf("approval error: %w", err)
		}
		if !approved {
			return "", fmt.Errorf("denied: run_command %s", args.Command)
		}

		cmd := exec.Command("sh", "-c", args.Command)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		exitCode := 0
		if err := cmd.Run(); err != nil {
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
	})
}
