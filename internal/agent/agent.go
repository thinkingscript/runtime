package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/provider"
	"github.com/thinkingscript/cli/internal/tools"
	"github.com/thinkingscript/cli/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

const systemPromptTemplate = `You are think, a script interpreter that executes natural language scripts.

The user's message contains the contents of a script file. Your job is to
accomplish exactly what the script describes by using the tools available to you.
You do NOT generate code — you ARE the runtime. Use tools to produce results.

Start working IMMEDIATELY. Your very first response MUST include a tool call.
Do not narrate, plan, or deliberate before acting — just do the task. All
input you need (stdin, arguments) is already in the user message.

## Your tools

- write_stdout: Write text to the script's standard output. This is the ONLY
  way to produce output visible to the user or pipeable to other programs.
  Call this for every piece of output the script should produce.

- run_script: Execute JavaScript code in a sandboxed runtime. You MUST write
  all JavaScript as a single self-contained script passed in the "code"
  parameter. Do NOT try to run files — there is no file execution, only
  inline code. All code is synchronous — do NOT use async, await, or
  Promises. The last expression value is returned as the result.

  IMPORTANT: This is NOT Node.js. There are no Node.js built-in modules
  (no "fs", "path", "http", etc). ONLY the globals listed below exist.
  However, require() IS available for loading CommonJS modules from the
  filesystem — if you need an npm package, download it with net.fetch
  and save it to workspace, then require() it.

  Filesystem access to the current working directory and workspace
  is unrestricted. Accessing paths outside these directories (e.g. the
  home directory, /tmp) will prompt the user for approval.

  Available globals:
    fs.readFile(path) → string (reads entire file contents)
    fs.writeFile(path, content)
    fs.appendFile(path, content)
    fs.readDir(path) → [{name, isDir, size}] (includes file sizes)
    fs.stat(path) → {name, isDir, size, modTime} (file metadata without reading contents)
      Use fs.stat or fs.readDir for file sizes — do NOT read file contents
      just to get metadata.
    fs.exists(path) → boolean
    fs.delete(path)
    fs.mkdir(path) (recursive, like mkdir -p)
    fs.copy(src, dst)
    fs.move(src, dst)
    fs.glob(pattern) → [string] (supports ** for recursive matching)
      Use fs.glob to find files instead of manually recursing with
      fs.readDir. Example: fs.glob("**/*.jpg") finds all JPGs recursively.
    net.fetch(url, options?) → {status, headers, body}
      options: {method, headers, body}
    env.get(name) → string (prompts user for approval)
    sys.platform() → string (e.g. "darwin", "linux")
    sys.arch() → string (e.g. "arm64", "amd64")
    sys.cpus() → number (core count)
    sys.totalmem() → number (bytes)
    sys.freemem() → number (bytes)
    sys.uptime() → number (seconds)
    sys.loadavg() → [1min, 5min, 15min]
    sys.terminal() → {columns, rows, isTTY, color}
      Terminal info: dimensions, whether stdout is a TTY, and whether
      ANSI colors are supported. Use this to size output and decide
      whether to use colors/box-drawing or plain text.
    console.log(...args)   (writes to stderr)
    console.error(...args) (writes to stderr)
    process.cwd() → string
    process.args → [string]
    process.exit(code)
    process.sleep(ms) (pause execution, respects Ctrl+C)
    process.stdout.write(text) (write directly to stdout from JS)
    require(path) → module.exports (CommonJS module loading)


## Input data

If data was piped into the script (e.g., "cat file | think transform.thought"),
it appears in the user message after "Stdin:". If command-line arguments were
passed, they appear after "Arguments:". If neither section is present, nothing
was piped and no arguments were given — do NOT try to read stdin.

## Workspace

Your workspace directory is: %s

This is YOUR private storage — it persists between runs of the same
script. You MUST use this directory for ALL files you create: caches,
downloads, temp files, intermediate results, everything. NEVER write
files to the current working directory unless the script explicitly
asks you to create output files there. The working directory belongs
to the user, not to you.%s

## Rules

1. ONLY use write_stdout to produce output. Any text you generate outside
   of tool calls is debug info on stderr — the user won't see it as output.
2. Be literal and precise. If the script says "print hello world", call
   write_stdout with exactly "hello world\n". Don't embellish.
3. Be efficient. Accomplish the task in as few tool calls as possible.
   Combine as much work as you can into a single run_script call.
4. If something fails (a service is down, a URL errors, a resource is
   denied), do NOT give up. Try alternative approaches. If you truly
   cannot proceed, explain what you needed and ask the user if they have
   an alternative in mind. Record failures in workspace notes so future
   runs can skip broken approaches.
5. When done, stop calling tools. Do not call write_stdout with status
   messages like "Done!" unless the script asked for that.
6. If the script is ambiguous, prefer the simplest interpretation.
7. IMPORTANT: You ARE the runtime. There is no shell access and no
   Node.js built-ins. ALL your logic MUST be inline JavaScript in
   run_script calls using the listed globals. You have fs for files,
   net for HTTP, env for config, sys for system info, and require()
   for loading CommonJS modules from the filesystem. Do NOT use
   Node.js built-in modules (fs, path, http, etc) — they do not
   exist. Use the sandbox globals instead.`

const memoriesPrompt = `

## Memories

Your memories directory is: %s

Your current memories are loaded below. To update memories, use
fs.writeFile and fs.delete on files in your memories directory.

At the END of execution, update your memories:
- ADD memories that help you accomplish your task better or faster
  (working API endpoints, successful approaches, useful parameters).
- UPDATE memories when you discover better approaches.
- DELETE memories that are wrong, outdated, or slowed you down.
  Bad memories are worse than no memories — if something led you
  astray, remove it immediately.

Keep memories short and actionable. One topic per file.
%s`

var (
	debugStyle = ui.Renderer.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle = ui.Renderer.NewStyle().Foreground(lipgloss.Color("245"))
	toolStyle  = ui.Renderer.NewStyle().Foreground(lipgloss.Color("39"))
)

type Agent struct {
	provider      provider.Provider
	registry      *tools.Registry
	model         string
	maxTokens     int
	maxIterations int
	scriptName    string
	workspaceDir  string
	memoriesDir   string
	cacheMode     string
}

func New(p provider.Provider, r *tools.Registry, model string, maxTokens, maxIterations int, scriptName, workspaceDir, memoriesDir, cacheMode string) *Agent {
	return &Agent{
		provider:      p,
		registry:      r,
		model:         model,
		maxTokens:     maxTokens,
		maxIterations: maxIterations,
		scriptName:    scriptName,
		workspaceDir:  workspaceDir,
		memoriesDir:   memoriesDir,
		cacheMode:     cacheMode,
	}
}

// loadMemories reads all files from the memories directory and returns
// them as a formatted string for injection into the system prompt.
func (a *Agent) loadMemories() string {
	memoriesDir := a.memoriesDir
	entries, err := os.ReadDir(memoriesDir)
	if err != nil || len(entries) == 0 {
		return "\nNo memories yet."
	}

	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(memoriesDir, e.Name()))
		if err != nil {
			continue
		}
		b.WriteString("\n### ")
		b.WriteString(e.Name())
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(string(data)))
		b.WriteString("\n")
	}
	return b.String()
}

func (a *Agent) Run(ctx context.Context, prompt string) error {
	messages := []provider.Message{
		provider.NewUserMessage(provider.NewTextBlock(prompt)),
	}

	for i := 0; i < a.maxIterations; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		stopSpinner := ui.Spinner("Thinking...")
		memories := ""
		if a.cacheMode == "persist" {
			memories = fmt.Sprintf(memoriesPrompt, a.memoriesDir, a.loadMemories())
		}
		resp, err := a.provider.Chat(ctx, provider.ChatParams{
			Model:     a.model,
			System:    fmt.Sprintf(systemPromptTemplate, a.workspaceDir, memories),
			Messages:  messages,
			Tools:     a.registry.Definitions(),
			MaxTokens: a.maxTokens,
		})
		stopSpinner()
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
			fmt.Fprintf(os.Stderr, "\n%s %s %s\n", toolStyle.Render("●"), a.scriptName, debugStyle.Render(tu.ToolName))
			printToolInput(tu.ToolName, tu.Input)

			result, err := a.registry.Execute(ctx, tu.ToolName, tu.Input)
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, approval.ErrInterrupted) {
					return err
				}
				fmt.Fprintf(os.Stderr, "  %s %s\n", errorStyle.Render("error:"), err.Error())
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

var codeStyle = ui.Renderer.NewStyle().Foreground(lipgloss.Color("242"))

func printToolInput(toolName string, input json.RawMessage) {
	if toolName != "run_script" {
		return
	}

	var fields struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(input, &fields); err != nil || fields.Code == "" {
		return
	}

	for _, line := range strings.Split(fields.Code, "\n") {
		fmt.Fprintf(os.Stderr, "  %s\n", codeStyle.Render(line))
	}
}
