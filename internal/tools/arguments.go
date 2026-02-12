package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradgessler/agent-exec/internal/approval"
	"github.com/bradgessler/agent-exec/internal/arguments"
	"github.com/bradgessler/agent-exec/internal/provider"
)

type setArgumentInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (r *Registry) registerArgument(approver *approval.Approver, argStore *arguments.Store) {
	r.register(provider.ToolDefinition{
		Name:        "set_argument",
		Description: "Register a named argument for the current session. Named arguments are used to create approval patterns — when a command contains an argument's value, the user can approve a pattern that auto-matches future runs with different values.",
		InputSchema: provider.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The argument name (e.g. Location, URL, Filename)",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "The argument value (e.g. San Francisco, https://example.com)",
				},
			},
			Required: []string{"name", "value"},
		},
	}, func(_ context.Context, input json.RawMessage) (string, error) {
		var args setArgumentInput
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("parsing set_argument input: %w", err)
		}

		if args.Name == "" {
			return "", errors.New("argument name must not be empty")
		}
		if args.Value == "" {
			return "", errors.New("argument value must not be empty")
		}

		detail := fmt.Sprintf("%s = %s", args.Name, args.Value)
		approved, err := approver.ApproveArgument(detail)
		if err != nil {
			return "", fmt.Errorf("approval error: %w", err)
		}
		if !approved {
			return "", fmt.Errorf("denied: set_argument %s", detail)
		}

		argStore.Set(args.Name, args.Value)
		return fmt.Sprintf("Argument %s set to %q", args.Name, args.Value), nil
	})
}
