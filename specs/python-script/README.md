# Ralph

A deterministic outer loop harness that repeatedly runs CLI-based LLM agents until task completion, enforcing quality guardrails between iterations.

## Overview

Ralph orchestrates LLM coding agents (Claude, Codex, AMP) in a feedback loop:

1. Run the agent with a prompt
2. Execute guardrails (build, lint, test)
3. If guardrails fail, feed the failure back to the agent
4. Repeat until the agent signals completion or max iterations reached

This enables autonomous task completion with quality enforcement.

## Installation

Requires Python 3.10+ and [uv](https://docs.astral.sh/uv/).

```bash
git clone <repo-url>
cd ralph-script
```

## Usage

```bash
# Run with an inline prompt
./ralph --prompt "Fix the failing tests"

# Run with a prompt file
./ralph --prompt-file PROMPT.md

# Override settings via CLI
./ralph --prompt "task" --maximum-iterations 5
./ralph --prompt "task" --no-stream-agent-output
```

### CLI Options

| Option | Description |
|--------|-------------|
| `--prompt` | Inline task prompt |
| `--prompt-file` | Path to prompt file (mutually exclusive with `--prompt`) |
| `--maximum-iterations`, `-m` | Max loop iterations (default: 10) |
| `--completion-response`, `-c` | Completion signal text (default: "DONE") |
| `--settings` | Path to settings JSON file |
| `--stream-agent-output` / `--no-stream-agent-output` | Toggle real-time output streaming |

## Configuration

Settings are loaded from `.ralph/settings.json` with optional overrides from `.ralph/settings.local.json`. CLI arguments take highest priority.

```json
{
  "maximumIterations": 10,
  "completionResponse": "DONE",
  "outputTruncateChars": 5000,
  "streamAgentOutput": true,
  "agent": {
    "command": "codex",
    "flags": ["--sandbox workspace-write"]
  },
  "guardrails": [
    {
      "command": "uv run ruff check ralph",
      "failAction": "APPEND"
    },
    {
      "command": "uv run pytest -q",
      "failAction": "APPEND"
    }
  ],
  "scm": {
    "command": "git",
    "tasks": ["commit"]
  }
}
```

### Settings Reference

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `maximumIterations` | int | 10 | Max iterations before failure |
| `completionResponse` | string | "DONE" | Text to match in `<response>` tags |
| `outputTruncateChars` | int | 5000 | Max chars of guardrail output sent to agent |
| `streamAgentOutput` | bool | true | Stream agent output to console |
| `agent.command` | string | required | Agent CLI command (`claude`, `codex`, `amp`) |
| `agent.flags` | array | [] | Additional flags for the agent |
| `guardrails` | array | [] | List of guardrail commands |
| `scm.command` | string | - | SCM command (e.g., `git`) |
| `scm.tasks` | array | [] | Post-completion tasks (e.g., `["commit"]`) |

### Guardrail Fail Actions

When a guardrail fails, its output is fed back to the agent using one of:

- **APPEND**: Add failure output after the base prompt
- **PREPEND**: Add failure output before the base prompt
- **REPLACE**: Replace the prompt entirely with failure output

## Supported Agents

Ralph automatically infers the correct non-REPL flags:

| Agent | Inferred Flag | Prompt Method |
|-------|---------------|---------------|
| `claude` | `-p` | stdin |
| `codex` | `e` | file |
| `amp` | `-x` | stdin |

## Completion Detection

The agent signals task completion by emitting:

```
<response>DONE</response>
```

The tag is case-insensitive and the inner text must match `completionResponse`.

## Debugging

Guardrail output is saved to `.ralph/guardrail_<iteration>_<slug>.log` for debugging failed runs.

## Development

```bash
# Run tests
uv run pytest -q

# Run linter
uv run ruff check ralph

# Run a single test
uv run pytest tests/test_settings.py::test_merge_settings_defaults -v
```

## Architecture

```
ralph              # Entry point script
ralph.py           # Main module (loop logic, settings, guardrails)
.ralph/
  settings.json    # Default configuration
  *.log            # Guardrail output logs
tests/
  test_settings.py # Unit tests
```

## License

See LICENSE file.
