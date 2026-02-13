package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/arguments"
	"github.com/thinkingscript/cli/internal/provider"
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

		argStore.Set(args.Name, args.Value)
		return fmt.Sprintf("Argument %s set to %q", args.Name, args.Value), nil
	}, func(input json.RawMessage) (bool, error) {
		var args setArgumentInput
		if err := json.Unmarshal(input, &args); err != nil {
			return false, fmt.Errorf("parsing set_argument input: %w", err)
		}
		if args.Name == "" {
			return false, errors.New("argument name must not be empty")
		}
		if args.Value == "" {
			return false, errors.New("argument value must not be empty")
		}
		detail := fmt.Sprintf("%s = %s", args.Name, args.Value)
		return approver.ApproveArgument(detail)
	})
}
