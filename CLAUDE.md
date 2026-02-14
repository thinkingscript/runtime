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
internal/tools/          → Tool registry + implementations (stdio, script)
internal/sandbox/        → Sandboxed JS runtime (goja) with fs/net/env/sys bridges
internal/approval/       → Charm huh approval prompts + persistence
```

### Wiring (cmd/think/root.go)

Everything is wired in `runScript()`:
1. `approval.NewApprover(cacheDir)` — approval system for env reads and path access
2. `tools.NewRegistry(approver, workDir, workspaceDir)` — tool registry with two tools

Stdin data and CLI arguments are injected directly into the prompt (no tool call needed).

### Security Model: The Sandbox Boundary

**The sandbox (goja JS runtime) is the security boundary.** Filesystem access within CWD and workspace is unrestricted. Accessing paths outside these directories prompts the user for approval. Environment variable reads prompt the user for approval. Network access is unrestricted. There is no shell access — system introspection (CPU, memory, uptime, load) is provided through the `sys` bridge.

CommonJS `require()` is available for loading modules. Modules are loaded through the same sandbox path checks — paths inside CWD/workspace load freely, paths outside require approval.

If you add a new bridge function that touches the host system beyond the sandbox's allowed paths, network, or env — it needs approval. No exceptions.

### Tool Registration Pattern

Each tool lives in its own file under `internal/tools/` and registers via `(r *Registry) registerXxx()`. Pattern:
- Define an input struct with json tags
- Call `r.register(ToolDefinition, handlerFunc)`
- Handler unmarshals input, does work, returns string result

Tools: `write_stdout`, `run_script`.

### Sandbox (internal/sandbox/)

The JS runtime uses `github.com/dop251/goja` (pure Go, no CGo) with `goja_nodejs` for CommonJS `require()` support. Bridge files:
- `bridge_fs.go` — `fs.readFile`, `fs.writeFile`, `fs.appendFile`, `fs.readDir`, `fs.stat`, `fs.exists`, `fs.delete`, `fs.mkdir`, `fs.copy`, `fs.move`, `fs.glob` (CWD + workspace free; other paths prompt for approval)
- `bridge_net.go` — `net.fetch(url, options?)` (unrestricted)
- `bridge_env.go` — `env.get(name)` (prompts user for approval)
- `bridge_sys.go` — `sys.platform()`, `sys.arch()`, `sys.cpus()`, `sys.totalmem()`, `sys.freemem()`, `sys.uptime()`, `sys.loadavg()` (system introspection)
- `bridge_console.go` — `console.log`, `console.error` → stderr
- `bridge_process.go` — `process.cwd()`, `process.args`, `process.exit(code)`

Key details:
- All JS is synchronous. No async/await/Promises.
- Objects returned from run_script or logged via console.log are auto-JSON.stringified (so the LLM sees real data, not `[object Object]`).
- Errors use `throwError()` for clean messages (no Go stack traces leaking to the LLM).
- Context cancellation flows through to HTTP requests (Ctrl+C works).
- 30-second default timeout per execution.

### Approval System

Two approval flows via `approveSimple()` in `approval.go`:
- **`ApprovePath`** — for filesystem access outside CWD/workspace
- **`ApproveEnvRead`** — for environment variable reads

Both follow the same pattern: stored lookup → prompt with Yes/Always/No options.

Approvals persist to `<cacheDir>/approvals.json` with maps for: env_vars, paths.

### System Prompt (internal/agent/agent.go)

The LLM system prompt is a template string in `agent.go` (formatted with workspace dir path at runtime). It documents all tools, sandbox globals, workspace, and rules.

When modifying the system prompt, be direct and explicit — especially for smaller models like Haiku. "You MUST do X when Y" works better than "consider doing X when appropriate."

## Key Conventions

- **stdout is sacred**: Only `write_stdout` tool writes to stdout. All debug/UI → stderr.
- **Sandbox is the boundary**: fs/net/sys run free inside CWD + workspace. Paths outside and env reads require user approval. No shell access. `require()` available for CommonJS modules.
- **Provider interface**: Agent loop is decoupled from any specific LLM SDK.
- **Keep primitives simple**: Small, focused tools that stack on each other. Don't over-architect.
- **`think` is the interpreter**, **`thought` is the management tool**. Scripts are `.thought` files, shebangs are `#!/usr/bin/env think`.
- Config precedence: env vars (`THINKINGSCRIPT__*`) > frontmatter > `~/.thinkingscript/` > defaults.
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
./bin/think examples/weather.md "San Francisco"
./bin/thought cache examples/weather.md
./bin/thought build input.thought -o output.thought
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/anthropics/anthropic-sdk-go` — Anthropic API
- `github.com/dop251/goja` — Pure-Go JavaScript engine (ES6)
- `github.com/dop251/goja_nodejs` — CommonJS require() for goja
- `github.com/charmbracelet/huh` — TUI confirmation prompts
- `github.com/charmbracelet/lipgloss` — Styled terminal output
- `golang.org/x/term` — PTY detection
- `gopkg.in/yaml.v3` — YAML parsing
