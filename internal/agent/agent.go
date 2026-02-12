package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/bradgessler/agent-exec/internal/approval"
	"github.com/bradgessler/agent-exec/internal/provider"
	"github.com/bradgessler/agent-exec/internal/tools"
	"github.com/charmbracelet/lipgloss"
)

const systemPrompt = `You are agent-exec, a script interpreter that executes natural language scripts.

The user's message contains the contents of a script file. Your job is to
accomplish exactly what the script describes by using the tools available to you.
You do NOT generate code — you ARE the runtime. Use tools to produce results.

## Your tools

- write_stdout: Write text to the script's standard output. This is the ONLY
  way to produce output visible to the user or pipeable to other programs.
  Call this for every piece of output the script should produce.

- run_command: Execute a shell command (via sh -c). Returns stdout, stderr,
  and exit code. Use this when the script requires running programs, file
  operations, installations, or any system task. The user must approve each
  command unless running in wreckless mode.

- read_env: Read an environment variable by name. Use when the script needs
  configuration from the environment. Requires user approval.

- read_stdin: Read all data piped into this script via stdin. Use when the
  script is expected to process piped input (e.g., "cat file | agent-exec
  transform.ai"). Returns empty if nothing was piped.

## Rules

1. ONLY use write_stdout to produce output. Any text you generate outside
   of tool calls is debug info on stderr — the user won't see it as output.
2. Be literal and precise. If the script says "print hello world", call
   write_stdout with exactly "hello world\n". Don't embellish.
3. Be efficient. Accomplish the task in as few tool calls as possible.
4. If a tool call is denied, explain what you needed and stop gracefully.
   Do not retry denied operations.
5. When done, stop calling tools. Do not call write_stdout with status
   messages like "Done!" unless the script asked for that.
6. If the script is ambiguous, prefer the simplest interpretation.
7. For multi-step tasks, execute steps in order. Check command exit codes
   and stop on failure unless the script says otherwise.`

var (
	debugStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	toolStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

type Agent struct {
	provider      provider.Provider
	registry      *tools.Registry
	model         string
	maxTokens     int
	maxIterations int
}

func New(p provider.Provider, r *tools.Registry, model string, maxTokens, maxIterations int) *Agent {
	return &Agent{
		provider:      p,
		registry:      r,
		model:         model,
		maxTokens:     maxTokens,
		maxIterations: maxIterations,
	}
}

func (a *Agent) Run(ctx context.Context, prompt string) error {
	messages := []provider.Message{
		provider.NewUserMessage(provider.NewTextBlock(prompt)),
	}

	for i := 0; i < a.maxIterations; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		resp, err := a.provider.Chat(ctx, provider.ChatParams{
			Model:     a.model,
			System:    systemPrompt,
			Messages:  messages,
			Tools:     a.registry.Definitions(),
			MaxTokens: a.maxTokens,
		})
		if err != nil {
			return fmt.Errorf("API call failed: %w", err)
		}

		// Process response blocks
		var toolUses []provider.ContentBlock
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					fmt.Fprintln(os.Stderr, debugStyle.Render(block.Text))
				}
			case "tool_use":
				toolUses = append(toolUses, block)
			}
		}

		// If no tool calls, we're done
		if len(toolUses) == 0 {
			return nil
		}

		// Add assistant message with all content blocks
		messages = append(messages, provider.NewAssistantMessage(resp.Content...))

		// Execute each tool call and collect results
		var resultBlocks []provider.ContentBlock
		for _, tu := range toolUses {
			fmt.Fprintf(os.Stderr, "%s %s\n", toolStyle.Render("tool:"), tu.ToolName)

			result, err := a.registry.Execute(ctx, tu.ToolName, json.RawMessage(tu.Input))
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, approval.ErrInterrupted) {
					return err
				}
				fmt.Fprintf(os.Stderr, "%s %s\n", errorStyle.Render("error:"), err.Error())
				resultBlocks = append(resultBlocks, provider.NewToolResultBlock(tu.ToolUseID, err.Error(), true))
			} else {
				resultBlocks = append(resultBlocks, provider.NewToolResultBlock(tu.ToolUseID, result, false))
			}
		}

		// Send tool results back
		messages = append(messages, provider.NewUserMessage(resultBlocks...))

		// If stop reason is end_turn (not tool_use), we're done
		if resp.StopReason == "end_turn" {
			return nil
		}
	}

	return fmt.Errorf("agent loop exceeded maximum iterations (%d)", a.maxIterations)
}
