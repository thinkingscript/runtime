# thinkingscript

## Project Overview

A Go CLI with two binaries:
- **`think`** — shebang interpreter for natural language `.thought` scripts. Users write files with `#!/usr/bin/env think`, and the CLI sends the prompt to an LLM which uses tools to accomplish the task.
- **`thought`** — management tool for cache operations and building `.thought` scripts.

Repo: `thinkingscript/cli`.

## Quick Setup

```bash
# 1. Set your API key
export THINKINGSCRIPT__ANTHROPIC__API_KEY=sk-ant-...

# 2. Build
make build

# 3. Run a script
./bin/think examples/weather.md "San Francisco"
```

For development, set `THINKINGSCRIPT_HOME=./thinkingscript` to avoid writing to `~/.thinkingscript/`.

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
internal/config/         → Home dir, config.json, agents, fingerprinting
internal/script/         → Script parser (shebang + frontmatter + prompt)
internal/tools/          → Tool registry + implementations (stdio, script)
internal/sandbox/        → Sandboxed JS runtime (goja) with fs/net/env/sys bridges
internal/approval/       → Charm huh approval prompts + persistence
```

### Wiring (cmd/think/root.go)

Everything is wired in `runScript()`:
1. `approval.NewApprover(thoughtDir, globalPolicyPath)` — policy-based approval system
2. `tools.NewRegistry(approver, workDir, workspaceDir)` — tool registry with two tools

Stdin data and CLI arguments are injected directly into the prompt (no tool call needed).

### Security Model: The Sandbox Boundary

**The sandbox (goja JS runtime) is the security boundary.** CWD is read-only — reads are unrestricted but writes to CWD require user approval. Workspace and memories directories are fully read-write. Accessing paths outside these directories prompts the user for approval. Environment variable reads prompt the user for approval. Network access requires user approval. There is no shell access — system introspection (CPU, memory, uptime, load) is provided through the `sys` bridge.

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
- `bridge_fs.go` — `fs.readFile`, `fs.writeFile`, `fs.appendFile`, `fs.readDir`, `fs.stat`, `fs.exists`, `fs.delete`, `fs.mkdir`, `fs.copy`, `fs.move`, `fs.glob` (CWD read-only; workspace + memories read-write; other paths prompt for approval)
- `bridge_net.go` — `net.fetch(url, options?)` (requires user approval)
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

Three approval flows in `approval.go`:
- **`ApproveNet(host)`** — for network access to specific hosts
- **`ApprovePath(op, path)`** — for filesystem access (op is "read", "write", or "delete")
- **`ApproveEnvRead(name)`** — for environment variable reads

Order of checks: global protected entries → thought policy → global policy → prompt.

Policies are JSON files:
- `~/.thinkingscript/policy.json` — global defaults (read-only)
- `~/.thinkingscript/thoughts/<name>/policy.json` — per-thought overrides (read-write)

```json
{
  "version": 1,
  "paths": {
    "default": "prompt",
    "entries": [
      {"path": "/Users/brad/projects", "mode": "rwd", "approval": "allow", "source": "prompt"},
      {"path": "/etc", "mode": "r", "approval": "allow", "source": "config"}
    ],
    "protected": []
  },
  "env": {
    "default": "prompt",
    "entries": [
      {"name": "HOME", "approval": "allow", "source": "config"},
      {"name": "AWS_*", "approval": "deny", "source": "config"}
    ]
  },
  "net": {
    "hosts": {
      "default": "prompt",
      "entries": [
        {"host": "*.github.com", "approval": "allow", "source": "prompt"}
      ]
    },
    "listen": {
      "default": "deny",
      "entries": []
    }
  }
}
```

**Path modes:** `r` (read/list), `w` (write), `d` (delete). Combined like chmod: `rwd` for full access.

**Approval values:** `allow`, `deny`, `prompt`. Default is `prompt` for most things, `deny` for listen.

**Sources:** `default` (auto-generated), `prompt` (user answered), `config` (manually edited), `cli` (via `thought policy` command).

**Wildcards:** Env names support suffix wildcards (`AWS_*`). Hosts support prefix wildcards (`*.github.com`).

**Protected entries:** Global policy can have `protected` path entries that thought policies cannot override.

**Security:** Thoughts cannot modify their own `policy.json` — hardcoded deny.

**Bootstrap:** On first run, workspace/memories get `rwd` and CWD gets `r` with `source: default`.

### System Prompt (internal/agent/agent.go)

The LLM system prompt is a template string in `agent.go` (formatted with workspace dir path at runtime). It documents all tools, sandbox globals, workspace, and rules.

When modifying the system prompt, be direct and explicit — especially for smaller models like Haiku. "You MUST do X when Y" works better than "consider doing X when appropriate."

## Key Conventions

- **stdout is sacred**: Only `write_stdout` tool writes to stdout. All debug/UI → stderr.
- **Sandbox is the boundary**: CWD is read-only; workspace + memories are read-write. Network access, writes to CWD, paths outside, and env reads require user approval. No shell access. `require()` available for CommonJS modules.
- **Provider interface**: Agent loop is decoupled from any specific LLM SDK.
- **Keep primitives simple**: Small, focused tools that stack on each other. Don't over-architect.
- **`think` is the interpreter**, **`thought` is the management tool**. Scripts are `.thought` files, shebangs are `#!/usr/bin/env think`.
- Config precedence: env vars (`THINKINGSCRIPT__*`) > frontmatter > `~/.thinkingscript/` > defaults.

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

## Configuration

Home dir: `~/.thinkingscript/` (overridable via `THINKINGSCRIPT_HOME`).

```
~/.thinkingscript/
  config.json              # Global settings (agent, max_tokens, max_iterations)
  policy.json              # Global default policy (net, env, paths)
  agents/                  # Provider configs (anthropic.json, local.json, etc.)
  bin/                     # Installed thought binaries (added to PATH)
  thoughts/
    <name>/
      policy.json          # Per-thought policy (overrides global)
      workspace/           # Per-thought working directory
      memories/            # Per-thought persistent memories
  cache/<hash>/            # Fingerprint-gated, per-script-path
    fingerprint
```

### Environment Variables

Config env vars use the `THINKINGSCRIPT__` prefix (double underscore):

| Variable | Description | Default |
|----------|-------------|---------|
| `THINKINGSCRIPT_HOME` | Override home directory | `~/.thinkingscript` |
| `THINKINGSCRIPT__AGENT` | Agent name to use | `anthropic` |
| `THINKINGSCRIPT__MODEL` | Model override | Agent's model |
| `THINKINGSCRIPT__MAX_TOKENS` | Max tokens per response | `4096` |
| `THINKINGSCRIPT__CACHE` | Cache mode: `persist`, `ephemeral`, `off` | `persist` |
| `THINKINGSCRIPT__ANTHROPIC__API_KEY` | Anthropic API key | — |
| `THINKINGSCRIPT__OPENAI__API_KEY` | OpenAI-compatible API key | — |
| `THINKINGSCRIPT__OPENAI__API_BASE` | OpenAI-compatible base URL | — |

Note: `THINKINGSCRIPT_HOME` uses a single underscore (it's not a config override, it's a path).

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/anthropics/anthropic-sdk-go` — Anthropic API
- `github.com/dop251/goja` — Pure-Go JavaScript engine (ES6)
- `github.com/dop251/goja_nodejs` — CommonJS require() for goja
- `github.com/charmbracelet/huh` — TUI confirmation prompts
- `github.com/charmbracelet/lipgloss` — Styled terminal output
- `golang.org/x/term` — PTY detection
- `gopkg.in/yaml.v3` — YAML parsing (frontmatter only)

## Policy Management

Use `thought policy` to manage policy entries:

```bash
# List policy for a thought
thought policy list weather

# Add entries
thought policy add path weather /Users/brad/data --mode rwd
thought policy add env weather HOME
thought policy add host weather "*.github.com"

# Remove entries
thought policy remove path weather /Users/brad/data
thought policy remove env weather HOME
thought policy remove host weather "*.github.com"

# List global policy
thought policy list
```
