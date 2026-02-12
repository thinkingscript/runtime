# thinkingscript

## Project Overview

A Go CLI with two binaries:
- **`think`** — shebang interpreter for natural language `.thought` scripts. Users write files with `#!/usr/bin/env think`, and the CLI sends the prompt to an LLM which uses tools to accomplish the task.
- **`thought`** — management tool for cache operations and building `.thought` scripts.

Repo: `thinkingscript/cli`.

## Architecture

```
cmd/think/main.go        → Signal handling, calls execute()
cmd/think/root.go        → Cobra root: parse script, resolve config, run agent loop
cmd/thought/main.go      → Signal handling, calls execute()
cmd/thought/root.go      → Cobra root: container for subcommands
cmd/thought/cache.go     → `thought cache` subcommand
cmd/thought/build.go     → `thought build` subcommand
internal/agent/          → Core agent loop (provider-agnostic)
internal/provider/       → Provider interface + Anthropic adapter
internal/config/         → Home dir, config.yaml, agents, fingerprinting
internal/script/         → Script parser (shebang + frontmatter + prompt)
internal/tools/          → Tool registry + implementations (command, env, stdio, arguments)
internal/approval/       → Charm huh approval prompts + persistence + pattern matching
internal/arguments/      → Named argument store (session-scoped, used by tools + approval)
```

### Wiring (cmd/think/root.go)

Everything is wired in `runScript()`:
1. `arguments.NewStore()` — shared mutable state for named arguments
2. `approval.NewApprover(wreckless, cacheDir, argStore)` — approval system gets the arg store for pattern matching
3. `tools.NewRegistry(approver, stdinData, argStore)` — tool registry gets both

The `argStore` is the bridge between tools and approval — tools write to it, approval reads from it.

### Tool Registration Pattern

Each tool lives in its own file under `internal/tools/` and registers via `(r *Registry) registerXxx()`. Pattern:
- Define an input struct with json tags
- Call `r.register(ToolDefinition, handlerFunc)`
- Handler unmarshals input, calls approver if needed, does work, returns string result

Tools: `write_stdout`, `read_stdin`, `set_argument`, `read_env`, `run_command` (registered in this order).

### Approval System

Three approval flows, all in `approval.go`:
- **`ApproveCommand`** — most complex: exact match → pattern match → prompt with 3 or 4 options
- **`ApproveEnvRead`** / **`ApproveArgument`** — simple flow via `approveSimple()` helper: stored lookup → prompt with 3 options

Approvals persist to `<cacheDir>/approvals.json` with maps for: commands, env_vars, arguments, command_patterns.

Pattern-based approval: `set_argument` registers named values → when a command contains those values, user can approve a pattern like `curl {Location}` → future runs with different values auto-match.

### System Prompt (internal/agent/agent.go)

The LLM system prompt is a const string in `agent.go`. It documents all tools and rules. Rule 8 is critical: it tells the LLM to call `set_argument` for CLI arguments and dynamic values before using them in commands.

When modifying the system prompt, be direct and explicit — especially for smaller models like Haiku. "You MUST do X when Y" works better than "consider doing X when appropriate."

## Key Conventions

- **stdout is sacred**: Only `write_stdout` tool writes to stdout. All debug/UI → stderr.
- **Provider interface**: Agent loop is decoupled from any specific LLM SDK.
- **Keep primitives simple**: Small, focused tools that stack on each other. Don't over-architect.
- **Approval is king**: Every tool that has side effects or reads sensitive data goes through the approver. Even `set_argument` requires approval so the user sees every assignment.
- **`think` is the interpreter**, **`thought` is the management tool**. Scripts are `.thought` files, shebangs are `#!/usr/bin/env think`.
- Config precedence: env vars (`THINK__*`) > frontmatter > `~/.thinkingscript/` > defaults.
- Home dir: `~/.thinkingscript/` (overridable via `THINKINGSCRIPT_HOME`).

## Code Style

- Standard Go: `gofmt`, stdlib imports grouped before third-party, alphabetized within groups.
- `errors.New` for static error strings, `fmt.Errorf` with `%w` only when wrapping.
- No unnecessary abstractions — three similar lines > a premature helper.
- Extract helpers only when there's real duplication (like `approveSimple`).
- Comments on exported types/functions, skip on obvious internal code.
- Never co-sign commits.

## Build & Run

```bash
make build
./bin/think examples/weather.thought "San Francisco"
./bin/thought cache examples/weather.thought
./bin/thought build input.thought -o output.thought
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/anthropics/anthropic-sdk-go` — Anthropic API
- `github.com/charmbracelet/huh` — TUI confirmation prompts
- `github.com/charmbracelet/lipgloss` — Styled terminal output
- `golang.org/x/term` — PTY detection
- `gopkg.in/yaml.v3` — YAML parsing
