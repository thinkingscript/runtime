package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bradgessler/agent-exec/internal/approval"
	"github.com/bradgessler/agent-exec/internal/provider"
)

type readEnvInput struct {
	Name string `json:"name"`
}

func (r *Registry) registerEnv(approver *approval.Approver) {
	r.register(provider.ToolDefinition{
		Name:        "read_env",
		Description: "Read an environment variable by name. Requires user approval.",
		InputSchema: provider.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The environment variable name to read",
				},
			},
			Required: []string{"name"},
		},
	}, func(_ context.Context, input json.RawMessage) (string, error) {
		var args readEnvInput
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("parsing read_env input: %w", err)
		}

		approved, err := approver.ApproveEnvRead(args.Name)
		if err != nil {
			return "", fmt.Errorf("approval error: %w", err)
		}
		if !approved {
			return "", fmt.Errorf("denied: read_env %s", args.Name)
		}

		return os.Getenv(args.Name), nil
	})
}
