# agent-exec

A shebang interpreter for natural language scripts. Write what you want in plain English, make it executable, and run it.

```
#!/usr/bin/env agent-exec
Print "hello world" and exit
```

```bash
chmod +x hello.ai
./hello.ai
# => hello world
```

agent-exec sends your script's text to an LLM, which uses tools (shell commands, stdout, stdin, env vars) to accomplish the task. Output goes to stdout, debug goes to stderr — so scripts are fully pipeable.

## Quick Start

1. **Install**

```bash
go install github.com/bradgessler/agent-exec@latest
```

2. **Set up your API key**

```bash
mkdir -p ~/.agent-exec/agents
cat > ~/.agent-exec/agents/anthropic.yaml << 'EOF'
version: 1
provider: anthropic
api_key: sk-ant-your-key-here
model: claude-sonnet-4-5-20250929
EOF
```

Or set the environment variable:

```bash
export THINKINGSCRIPT__ANTHROPIC__API_KEY=sk-ant-...
```

3. **Write a script**

```bash
cat > hello.ai << 'EOF'
#!/usr/bin/env agent-exec
Print "hello world" and exit
EOF
chmod +x hello.ai
```

4. **Run it**

```bash
./hello.ai
```

You can also run thoughts directly from a URL:

```bash
think https://raw.githubusercontent.com/thinkingscript/thoughts/main/weather.md "NYC"
```

When running from a URL, `think` displays the thought content and asks for confirmation before executing.

## The Shebang

The first line `#!/usr/bin/env agent-exec` tells your OS to use agent-exec as the interpreter. Everything after the shebang (minus optional frontmatter) becomes the prompt sent to the LLM.

## Frontmatter

Scripts can include optional YAML frontmatter between `---` delimiters to configure behavior:

```
#!/usr/bin/env agent-exec
---
agent: local
model: llama3
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
| `THINKINGSCRIPT__MODEL` | Model override | `claude-opus-4-6` |
| `THINKINGSCRIPT__MAX_TOKENS` | Max tokens | `8192` |
| `THINKINGSCRIPT__ANTHROPIC__API_KEY` | Anthropic API key | `sk-ant-...` |
| `THINKINGSCRIPT__OPENAI__API_KEY` | OpenAI API key | `sk-...` |
| `THINKINGSCRIPT__OPENAI__API_BASE` | OpenAI base URL | `http://localhost:11434/v1` |
| `THINKINGSCRIPT__CACHE` | Cache mode (see below) | `off` |
| `THINKINGSCRIPT_HOME` | Override home directory | `~/.mythinkingscript` |

#### Cache modes (`THINKINGSCRIPT__CACHE`)

Controls how per-script cache (approvals, scratch notes) is managed between runs. Note that caches are automatically invalidated when either the script content or the `think` binary changes — so upgrading `think` or editing a script always starts fresh.

| Mode | Description |
|------|-------------|
| `persist` (default) | Cache survives between runs. Scratch notes accumulate so scripts improve over time. |
| `ephemeral` | Cache works during the run but is wiped on exit. Useful for one-off runs where you don't want stale notes influencing behavior. |
| `off` | No cache at all. Cache is wiped before and after the run, and scratch notes instructions are removed from the system prompt. |

```bash
# Fresh run, no notes influencing behavior
THINKINGSCRIPT__CACHE=off think examples/images.md

# Cache works during the run but doesn't persist
THINKINGSCRIPT__CACHE=ephemeral think examples/weather.md
```

### 2. Script Frontmatter

See [Frontmatter](#frontmatter) above.

### 3. Home Folder (`~/.agent-exec/`)

```
~/.agent-exec/
├── config.yaml          # Global settings
├── agents/
│   ├── anthropic.yaml   # Anthropic agent definition
│   └── local.yaml       # Local/Ollama agent definition
└── cache/
    └── <fingerprint>/   # Per-script cache (content-addressed)
        ├── fingerprint
        ├── approvals.json
        └── meta.json
```

**`config.yaml`** — Global defaults:

```yaml
version: 1
agent: anthropic
max_tokens: 4096
max_iterations: 50
```

**`agents/anthropic.yaml`** — Agent definition:

```yaml
version: 1
provider: anthropic
api_key: sk-ant-...
model: claude-sonnet-4-5-20250929
```

**`agents/local.yaml`** — Ollama (speaks OpenAI protocol):

```yaml
version: 1
provider: openai
api_base: http://localhost:11434/v1
api_key: ollama
model: llama3
```

## Tools

The LLM has four tools available:

| Tool | Description | Approval Required |
|------|-------------|:-:|
| `write_stdout` | Write text to stdout | No |
| `read_stdin` | Read piped stdin data | No |
| `run_command` | Execute shell command (`sh -c`) | Yes |
| `read_env` | Read an environment variable | Yes |

The LLM's text responses go to stderr (debug). Only `write_stdout` produces actual output.

## Approval System

When the LLM wants to access something sensitive (network, env vars, files outside the workspace), a prompt appears:

```
  ◆ NET  network access
      ❯ Once     allow this time
        Session  allow all this run
        Always   save to policy
        Deny     reject
```

- **Once**: allow this specific action, this run only
- **Session**: allow all actions of this type for the rest of this run
- **Always**: persist the decision to the thought's `policy.yaml`
- **Deny**: reject the action; the LLM adapts and tries another approach
- **Non-interactive**: all sensitive actions are denied by default (safe for CI/pipes)

## Piping

stdout is sacred — only `write_stdout` tool output goes there. This means scripts compose naturally with Unix pipes:

```bash
# Pipe data through an AI script
cat data.csv | ./summarize.ai > summary.txt

# Chain scripts
./generate-report.ai | ./format-output.ai
```

## Cache Management

Each script gets a cache directory keyed by its content fingerprint. This means the same script content produces the same cache whether run from a local file or a URL. The cache is automatically invalidated when script content or the `think` binary changes.

```bash
# Print cache directory for a script
thought cache ./hello.ai

# List cache contents
ls $(thought cache ./hello.ai)

# Clear cache for a specific script
thought cache --clear ./hello.ai

# Clear all caches
thought cache --clear-all
```

## Examples

### Hello World

```
#!/usr/bin/env agent-exec
Print "hello world" and exit
```

### Process Piped Input

```
#!/usr/bin/env agent-exec
Read stdin, count the number of lines, and print the count.
```

```bash
cat /etc/hosts | ./count-lines.ai
```

### Run Commands

```
#!/usr/bin/env agent-exec
List all .go files in the current directory, then count them and print
"Found N Go files" where N is the count.
```

### Use Environment Variables

```
#!/usr/bin/env agent-exec
Read the HOME environment variable and print the path.
```

### Run Commands with Approval

```
#!/usr/bin/env agent-exec
Run "uname -a" and print the output.
```
