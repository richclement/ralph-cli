# Ralph CLI

A deterministic outer loop that repeatedly runs a AI agent until it returns a completion response. Ralph enforces guardrails (build/lint/test) and optionally runs SCM commands when guardrails pass.

## Prerequisites

- Go 1.25+
- golangci-lint (for development)

## Installation

### Homebre

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

```bash
# Run with inline prompt
ralph "Fix the failing tests"

# Run with prompt file
ralph -f prompt.txt

# With options
ralph "Implement the feature. Responde with <response>COMPLETE</response> when done." -m 5 -c "<response>COMPLETE</response>" -V
```

### CLI Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--prompt-file` | `-f` | Path to file containing prompt text |
| `--maximum-iterations` | `-m` | Maximum iterations before stopping |
| `--completion-response` | `-c` | Completion response text (default: `DONE`) |
| `--settings` | | Path to settings file (default: `.ralph/settings.json`) |
| `--stream-agent-output` | | Stream agent output to console (default: true) |
| `--no-stream-agent-output` | | Disable streaming agent output |
| `--verbose` | `-V` | Enable verbose/debug output |
| `--version` | `-v` | Print version and exit |

## Configuration

Ralph uses JSON configuration files located in `.ralph/`:

- `.ralph/settings.json` - Base configuration
- `.ralph/settings.local.json` - Local overrides (optional, gitignored)

### Example Settings

```json
{
  "maximumIterations": 10,
  "completionResponse": "DONE",
  "outputTruncateChars": 5000,
  "streamAgentOutput": true,
  "agent": {
    "command": "claude",
    "flags": ["--model", "opus", "--no-auto-compact"]
  },
  "guardrails": [
    {
      "command": "./mvnw clean install -T 2C -q -e",
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
| `agent.command` | (required) | Agent CLI command (e.g., `claude`, `codex`, `amp`) |
| `agent.flags` | `[]` | Additional flags for agent command |
| `guardrails` | `[]` | Array of guardrail commands |
| `scm.command` | | SCM command (e.g., `git`) |
| `scm.tasks` | `[]` | SCM tasks to run (e.g., `["commit", "push"]`) |

### Guardrail Fail Actions

- `APPEND` - Append failed output to the prompt
- `PREPEND` - Prepend failed output to the prompt
- `REPLACE` - Replace the prompt with failed output

## Completion Detection

Ralph looks for `<response>message</response>` tags in agent output:

```
<response>DONE</response>
```

The match is case-insensitive. Completion is only checked when all guardrails pass.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (completion response matched) |
| 1 | Max iterations reached without completion |
| 2 | Configuration/validation error |
| 130 | Interrupted by signal (SIGINT/SIGTERM) |

## Architecture

```
ralph-cli/
├── cmd/ralph/          # Entry point, CLI setup
├── internal/
│   ├── config/         # Settings loading and merging
│   ├── agent/          # Agent command execution
│   ├── guardrail/      # Guardrail execution and logging
│   ├── loop/           # Main loop orchestration
│   └── scm/            # SCM task execution
├── .ralph/             # Runtime directory
└── specs/              # PRD and implementation tasks
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
