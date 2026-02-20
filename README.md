# thinkingscript

A shebang interpreter for natural language scripts. Write what you want in plain English, make it executable, and run it.

```
#!/usr/bin/env think
Print "hello world" and exit
```

```bash
chmod +x hello.thought
./hello.thought
# => hello world
```

`think` sends your script to an LLM, which executes JavaScript in a sandboxed runtime to accomplish the task. Output goes to stdout, debug goes to stderr — scripts are fully pipeable.

## Quick Start

1. **Build**

```bash
make build
```

2. **Set up your API key**

```bash
./bin/thought setup
```

Or set the environment variable:

```bash
export THINKINGSCRIPT__ANTHROPIC__API_KEY=sk-ant-...
```

3. **Run a script**

```bash
./bin/think examples/weather.md "San Francisco"
```

You can also run thoughts directly from a URL:

```bash
./bin/think https://example.com/weather.thought "NYC"
```

When running from a URL, `think` displays the thought content and asks for confirmation before executing.

## The Shebang

The first line `#!/usr/bin/env think` tells your OS to use think as the interpreter. Everything after the shebang (minus optional frontmatter) becomes the prompt sent to the LLM.

## Frontmatter

Scripts can include optional YAML frontmatter between `---` delimiters to configure behavior:

```
#!/usr/bin/env think
---
agent: anthropic
model: claude-haiku-4-5-20251001
max_tokens: 8192
---

List all Go files in the current directory and count them.
```

All frontmatter fields are optional:

| Field | Description | Default |
|-------|-------------|---------|
| `agent` | Which agent definition to use | `anthropic` |
| `model` | Override the agent's default model | Agent's model |
| `max_tokens` | Maximum tokens for LLM response | `4096` |

## Configuration

Configuration is resolved with three layers (highest precedence first):

### 1. Environment Variables

ENV vars use `THINKINGSCRIPT__` prefix with `__` as separator:

| ENV var | Description | Example |
|---------|-------------|---------|
| `THINKINGSCRIPT__AGENT` | Agent to use | `anthropic` |
| `THINKINGSCRIPT__MODEL` | Model override | `claude-opus-4-20250514` |
| `THINKINGSCRIPT__MAX_TOKENS` | Max tokens | `8192` |
| `THINKINGSCRIPT__ANTHROPIC__API_KEY` | Anthropic API key | `sk-ant-...` |
| `THINKINGSCRIPT__OPENAI__API_KEY` | OpenAI API key | `sk-...` |
| `THINKINGSCRIPT__OPENAI__API_BASE` | OpenAI base URL | `http://localhost:11434/v1` |
| `THINKINGSCRIPT__CACHE` | Cache mode (see below) | `off` |
| `THINKINGSCRIPT_HOME` | Override home directory | `~/.mythinkingscript` |

Note: `THINKINGSCRIPT_HOME` uses a single underscore (it's a path, not a config override).

### 2. Script Frontmatter

See [Frontmatter](#frontmatter) above.

### 3. Home Folder (`~/.thinkingscript/`)

```
~/.thinkingscript/
├── config.json           # Global settings
├── policy.json           # Global default policy
├── agents/
│   └── anthropic.json    # Anthropic agent definition
├── bin/                  # Installed thought binaries
├── thoughts/
│   └── <name>/
│       ├── policy.json   # Per-thought policy
│       ├── workspace/    # Per-thought working directory
│       └── memories/     # Per-thought persistent memories
└── cache/<fingerprint>/  # Per-script cache (content-addressed)
    └── fingerprint
```

**`config.json`** — Global defaults:

```json
{
  "version": 1,
  "agent": "anthropic",
  "max_tokens": 4096,
  "max_iterations": 50
}
```

**`agents/anthropic.json`** — Agent definition:

```json
{
  "version": 1,
  "provider": "anthropic",
  "api_key": "sk-ant-...",
  "model": "claude-sonnet-4-5-20250929"
}
```

## Tools

The LLM has two tools available:

| Tool | Description |
|------|-------------|
| `write_stdout` | Write text to stdout (the only way to produce output) |
| `run_script` | Execute JavaScript in a sandboxed runtime |

The LLM's text responses go to stderr (debug). Only `write_stdout` produces actual output.

## Sandbox

The `run_script` tool executes JavaScript in a sandboxed [goja](https://github.com/dop251/goja) runtime with these globals:

| Global | Description |
|--------|-------------|
| `fs.readFile`, `fs.writeFile`, `fs.readDir`, etc. | Filesystem access |
| `net.fetch(url, options?)` | HTTP requests |
| `env.get(name)` | Read environment variables |
| `sys.platform()`, `sys.arch()`, `sys.cpus()`, etc. | System info |
| `console.log`, `console.error` | Debug output (to stderr) |
| `process.cwd()`, `process.args`, `process.exit(code)` | Process info |
| `require(path)` | CommonJS module loading |

All JS is synchronous — no async/await/Promises.

## Policy System

When the LLM wants to access something sensitive, a prompt appears:

```
  ◆ NET  api.github.com
      ❯ Once     allow this time
        Session  allow all this run
        Always   save to policy
        Deny     reject
```

- **Once**: allow this specific action, this run only
- **Session**: allow all actions of this type for the rest of this run
- **Always**: persist the decision to the thought's `policy.json`
- **Deny**: reject the action; the LLM adapts and tries another approach
- **Non-interactive**: all sensitive actions are denied by default (safe for CI/pipes)

### Policy Files

Policies are JSON files that control what a thought can access:

```json
{
  "version": 1,
  "paths": {
    "default": "prompt",
    "entries": [
      {"path": "/Users/brad/projects", "mode": "rwd", "approval": "allow"}
    ]
  },
  "env": {
    "default": "prompt",
    "entries": [
      {"name": "HOME", "approval": "allow"},
      {"name": "AWS_*", "approval": "deny"}
    ]
  },
  "net": {
    "hosts": {
      "default": "prompt",
      "entries": [
        {"host": "*.github.com", "approval": "allow"}
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

**Wildcards:** Env names support suffix wildcards (`AWS_*`). Hosts support prefix wildcards (`*.github.com`).

### Default Permissions

On first run, a thought automatically gets:
- **Workspace** (`~/.thinkingscript/thoughts/<name>/workspace/`): `rwd`
- **Memories** (`~/.thinkingscript/thoughts/<name>/memories/`): `rwd`
- **CWD**: `r` (read-only)

### Managing Policies

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

## Cache Modes

Controls how per-script cache is managed between runs. Caches are automatically invalidated when either the script content or the `think` binary changes.

| Mode | Description |
|------|-------------|
| `persist` (default) | Cache survives between runs |
| `ephemeral` | Cache works during the run but is wiped on exit |
| `off` | No cache at all |

```bash
THINKINGSCRIPT__CACHE=off think examples/weather.md "NYC"
```

## Cache Management

```bash
# Print cache directory for a script
thought cache ./hello.thought

# Clear cache for a specific script
thought cache --clear ./hello.thought

# Clear all caches
thought cache --clear-all
```

## Piping

stdout is sacred — only `write_stdout` tool output goes there. Scripts compose naturally with Unix pipes:

```bash
# Pipe data through an AI script
cat data.csv | ./summarize.thought > summary.txt

# Chain scripts
./generate-report.thought | ./format-output.thought
```

## Examples

### Hello World

```
#!/usr/bin/env think
Print "hello world" and exit
```

### Fetch Weather

```
#!/usr/bin/env think
Fetch the current weather for the city provided as an argument.
Print a brief summary including temperature and conditions.
```

```bash
./weather.thought "San Francisco"
```

### Process JSON

```
#!/usr/bin/env think
Read stdin as JSON, extract all "name" fields, and print them one per line.
```

```bash
cat users.json | ./extract-names.thought
```

### File Operations

```
#!/usr/bin/env think
List all .go files in the current directory, count them,
and print "Found N Go files" where N is the count.
```

## Building Standalone Binaries

```bash
# Build a thought into a standalone binary
thought build weather.thought -o weather

# Install to ~/.thinkingscript/bin/
thought install weather.thought
```
