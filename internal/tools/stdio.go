package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/thinkingscript/cli/internal/provider"
	"golang.org/x/term"
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
	})
}

func (r *Registry) registerStdin(stdinData string) {
	pipedData := stdinData
	pipeConsumed := false

	r.register(provider.ToolDefinition{
		Name:        "read_stdin",
		Description: "Read input from stdin. If data was piped in, returns all piped data. If stdin is a terminal, waits for the user to type a line and press Enter.",
		InputSchema: provider.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}, func(_ context.Context, input json.RawMessage) (string, error) {
		// If we have pre-read piped data, return it (once)
		if pipedData != "" && !pipeConsumed {
			pipeConsumed = true
			return pipedData, nil
		}

		// If stdin is a TTY, read a line interactively
		if term.IsTerminal(int(os.Stdin.Fd())) {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				return scanner.Text(), nil
			}
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("reading stdin: %w", err)
			}
			return "", nil
		}

		// Non-TTY, already consumed — return empty
		return "", nil
	})
}
