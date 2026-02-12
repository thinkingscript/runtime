# agent-exec

## Project Overview

A Go CLI that acts as a shebang interpreter for natural language scripts. Users write `.ai` files with `#!/usr/bin/env agent-exec`, and agent-exec sends the prompt to an LLM which uses tools to accomplish the task.

## Architecture

```
main.go              → cmd.Execute()
cmd/root.go          → Cobra root: parse script, resolve config, run agent loop
cmd/cache.go         → `agent-exec cache` subcommand
internal/agent/      → Core agent loop (provider-agnostic)
internal/provider/   → Provider interface + Anthropic adapter
internal/config/     → Home dir, config.yaml, agents, fingerprinting
internal/script/     → Script parser (shebang + frontmatter + prompt)
internal/tools/      → Tool registry + implementations (command, env, stdio)
internal/approval/   → Charm huh approval prompts + persistence
```

## Key Conventions

- **stdout is sacred**: Only `write_stdout` tool writes to stdout. All debug/UI → stderr.
- **Provider interface**: Agent loop is decoupled from any specific LLM SDK.
- Config precedence: env vars (`AGENTEXEC__*`) > frontmatter > `~/.agent-exec/` > defaults.
- Home dir: `~/.agent-exec/` (overridable via `AGENT_EXEC_HOME`).

## Build & Run

```bash
go build -o agent-exec .
./agent-exec hello.ai
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/anthropics/anthropic-sdk-go` — Anthropic API
- `github.com/charmbracelet/huh` — TUI confirmation prompts
- `github.com/charmbracelet/lipgloss` — Styled terminal output
- `golang.org/x/term` — PTY detection
- `gopkg.in/yaml.v3` — YAML parsing
