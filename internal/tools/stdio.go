package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/thinkingscript/cli/internal/provider"
)

type writeStdoutInput struct {
	Content string `json:"content"`
}

func (r *Registry) registerStdio() {
	r.register(provider.ToolDefinition{
		Name:        "write_stdout",
		Description: "Write text to the script's standard output. This is the ONLY way to produce output visible to the user or pipeable to other programs.",
		InputSchema: provider.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "The text to write to stdout",
				},
			},
			Required: []string{"content"},
		},
	}, func(_ context.Context, input json.RawMessage) (string, error) {
		var args writeStdoutInput
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("parsing write_stdout input: %w", err)
		}
		_, err := fmt.Fprint(os.Stdout, args.Content)
		if err != nil {
			return "", fmt.Errorf("writing to stdout: %w", err)
		}
		return "ok", nil
	}, nil) // no approval needed
}
