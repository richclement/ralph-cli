# Ralph CLI

A deterministic outer loop that repeatedly runs a AI agent until it returns a completion response. Ralph enforces guardrails (build/lint/test) and optionally runs SCM commands when guardrails pass.

Based on the [Ralph Wiggum](https://ghuntley.com/ralph/)  loop by Geoffrey Huntley.

## Prerequisites

- Go 1.25+
- golangci-lint (for development)

## Installation

### Homebrew

```bash
brew install richclement/tap/ralph-cli
```

### Using go install

```bash
go install github.com/richclement/ralph-cli/cmd/ralph@latest
```

This downloads and installs the binary to `$GOPATH/bin` (or `$HOME/go/bin` if `GOPATH` is unset).

### Build From Source

```bash
git clone https://github.com/richclement/ralph-cli.git
cd ralph-cli
make build
# Binary at ./bin/ralph

# Optionally install to $GOPATH/bin
make install
```

## Usage

### Interactive Setup

The easiest way to get started is with the interactive `init` command:

```bash
ralph init
```

This walks you through setting up your configuration interactively:

```
$ ralph init

Agent command (e.g., claude, codex, amp, or other LLM CLI): claude
Agent flags (comma-separated, optional): --model,opus
Maximum iterations [10]:
Completion response [DONE]:
Add guardrail command (leave blank to finish): make lint
  Fail action (APPEND|PREPEND|REPLACE): APPEND
  Hint (optional, guidance for agent on failure): Fix lint errors only. Do not change behavior.
Add guardrail command (leave blank to finish): make test
  Fail action (APPEND|PREPEND|REPLACE): APPEND
  Hint (optional, guidance for agent on failure):
Add guardrail command (leave blank to finish):
Configure SCM? (y/N): y
  SCM command (e.g., git): git
  SCM tasks (comma-separated, e.g., commit,push): commit

Settings written to .ralph/settings.json
```

If a settings file already exists, it will show the current configuration and ask whether to overwrite.

### Running the Agent

```bash
# Run with inline prompt
ralph run -p "Fix the failing tests"

# Run with prompt file
ralph run -f prompt.txt

# With options
ralph run -p "Implement the feature" -m 5 -c "COMPLETE" -V
```

### CLI Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--prompt` | `-p` | Prompt string to send to agent |
| `--prompt-file` | `-f` | Path to file containing prompt text |
| `--maximum-iterations` | `-m` | Maximum iterations before stopping |
| `--completion-response` | `-c` | Completion response text (default: `DONE`) |
| `--stream-agent-output` | | Stream agent output to console (default: true) |
| `--no-stream-agent-output` | | Disable streaming agent output |
| `--verbose` | `-V` | Enable verbose/debug output |
| `--version` | `-v` | Print version and exit |

One of `--prompt` or `--prompt-file` is required (mutually exclusive).

## Configuration

Ralph uses JSON configuration files located in `.ralph/`:

- `.ralph/settings.json` - Base configuration
- `.ralph/settings.local.json` - Local overrides (optional, gitignored)

### Supported Agents

Ralph supports the following CLI LLM agents with automatic flag detection:

| Agent | Subcommand | Streaming Flags | Text Mode Flags | Prompt Handling |
|-------|------------|-----------------|-----------------|-----------------|
| `claude` | `-p` | `--output-format stream-json --verbose` | `--output-format text` | Inline argument |
| `amp` | `-x` | `--stream-json --dangerously-allow-all` | `--dangerously-allow-all` | Inline argument |
| `codex` | `e` | `--json --full-auto` | `--full-auto -o <file>` | Written to `.ralph/prompt_###.txt` |

**Amp Integration:**
- `--dangerously-allow-all` enables autonomous tool execution without approval prompts
- Amp requires `-x <prompt>` at the end of the command, so ralph orders flags accordingly

**Codex Integration:**
- Streaming mode uses `--json` for structured output and `--full-auto` for autonomous operation
- Text mode (for commit messages) omits `--json` and uses `-o <file>` to capture output
- Prompts are written to temporary files to avoid shell escaping issues

### Example Settings

```json
{
  "maximumIterations": 10,
  "completionResponse": "DONE",
  "outputTruncateChars": 5000,
  "streamAgentOutput": true,
  "includeIterationCountInPrompt": false,
  "agent": {
    "command": "claude",
    "flags": ["--model", "opus", "--no-auto-compact"]
  },
  "guardrails": [
    {
      "command": "make lint",
      "failAction": "APPEND",
      "hint": "Fix lint errors only. Do not change behavior."
    },
    {
      "command": "make test",
      "failAction": "APPEND"
    }
  ],
  "scm": {
    "command": "git",
    "tasks": ["commit", "push"]
  }
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `maximumIterations` | `10` | Max iterations before stopping |
| `completionResponse` | `DONE` | Response text to detect completion |
| `outputTruncateChars` | `5000` | Max chars of guardrail output sent to agent |
| `streamAgentOutput` | `true` | Stream agent output to console |
| `includeIterationCountInPrompt` | `false` | Prepend iteration summary to each prompt |
| `agent.command` | (required) | Agent CLI command (e.g., `claude`, `codex`, `amp`) |
| `agent.flags` | `[]` | Additional flags for agent command |
| `guardrails` | `[]` | Array of guardrail commands |
| `scm.command` | | SCM command (e.g., `git`) |
| `scm.tasks` | `[]` | SCM tasks to run (e.g., `["commit", "push"]`) |

### Guardrail Configuration

Each guardrail has:
- `command` - Shell command to run
- `failAction` - One of `APPEND`, `PREPEND`, or `REPLACE`
- `hint` (optional) - Guidance text injected into the prompt when the guardrail fails

**Fail Actions:**
- `APPEND` - Append failed output to the prompt
- `PREPEND` - Prepend failed output to the prompt
- `REPLACE` - Replace the prompt with failed output

**Hints:**

When a guardrail fails and has a `hint` configured, the hint is included in the failure message sent to the agent:

```
Guardrail "make lint" failed with exit code 1.
Hint: Fix lint errors only. Do not change behavior.
Output file: .ralph/guardrail_001_make_lint.log
Output (truncated):
<output...>
```

Hints are literal strings (no templating) and are never truncated.

## Completion Detection

Ralph detects completion from the JSON result in stream-json mode. A match occurs if the result ends with `completionResponse` (case-insensitive).

Examples that match with default `completionResponse: "DONE"`:
- `DONE`
- `Task completed successfully. DONE`

Completion is only checked when all guardrails pass.

## Streaming Output

When `streamAgentOutput` is enabled, Ralph parses agent JSON output and displays it with visual indicators:

```
⏺ Read(/path/to/file.go)
✅ Result ← Read (45 lines, 1234 chars)
  ⎿  package main
      import "fmt"
  ⎿  ... 43 more lines

📋 Todo List
  ✅ Read the file
  🔄 Edit the code ← ACTIVE
  ⏸️ Run tests
  📊 Progress: 1/3 (33%)

✅ Complete (cost: $5.76, tokens: 1.2M in (850K cached) / 45K out, tools: 58, errors: 4, time: 6m7s)
```

Features:
- **Tool correlation**: Results are linked back to their originating tool calls
- **Token tracking**: Input, output, and cache token counts with K/M suffixes
- **Statistics**: Tool count, error count, and elapsed time per iteration
- **Todo list display**: Task progress from TodoWrite tool calls
- **Auto-detected color**: Uses termenv for terminal capability detection

## Architecture

```
ralph-cli/
├── cmd/ralph/          # Entry point, CLI setup with subcommands
├── internal/
│   ├── config/         # Settings loading and merging
│   ├── initcmd/        # Interactive init command
│   ├── agent/          # Agent command execution
│   ├── guardrail/      # Guardrail execution and logging
│   ├── loop/           # Main loop orchestration
│   ├── response/       # Completion response extraction
│   ├── scm/            # SCM task execution
│   └── stream/         # Agent output parsing and formatting
├── .ralph/             # Runtime directory
└── specs/              # PRD and documentation
```

## Development

```bash
# Build
make build

# Run tests
make test

# Format code
make fmt

# Lint (requires golangci-lint)
make lint

# All checks
make all

# Install to $GOPATH/bin
make install
```

## License

See [LICENSE](LICENSE) file.
