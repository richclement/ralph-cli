# Ralph CLI Architecture

## Purpose
Ralph is a deterministic outer loop that repeatedly runs a CLI LLM agent until
it emits a completion response. Each iteration runs guardrails (build/lint/test)
and optionally executes SCM tasks (commit/push) after guardrails pass.

## High-Level Flow
1. Build the prompt (inline text or re-read prompt file).
2. Prepend iteration summary when configured.
3. Run the agent command and capture full output (optionally streaming).
4. Run guardrails; on failure, feed truncated output into the next prompt
   according to each guardrail's fail action.
5. If guardrails pass, optionally run SCM tasks.
6. Check completion response; stop when matched or exit after max iterations.

## Key Packages
- `cmd/ralph/main.go` sets up CLI subcommands (`init` and `run`), signal
  handling, and routes to the appropriate handler.
- `internal/initcmd` implements the interactive `ralph init` command which
  walks users through creating `.ralph/settings.json` with prompts for
  agent command, flags, guardrails, and SCM configuration.
- `internal/config` loads `.ralph/settings.json`, merges
  `.ralph/settings.local.json`, applies CLI overrides, and validates settings.
- `internal/agent` executes the agent command, supports streaming output, and
  infers non-REPL flags (`claude -p`, `codex e`, `amp -x`). Also handles
  text mode for simple requests (commit messages) with agent-specific flags.
- `internal/guardrail` runs guardrail commands, writes full logs to `.ralph/`,
  truncates output for prompt feedback, and applies fail actions.
- `internal/loop` orchestrates iterations, prompt construction, guardrails,
  SCM tasks, and completion detection.
- `internal/scm` runs SCM tasks and uses the agent to generate commit messages.
- `internal/response` extracts `<response>...</response>` and determines
  completion.
- `internal/stream` provides parsers for agent streaming JSON formats (Claude,
  Amp) and formats events for display.

## Configuration and Precedence
Settings are loaded in this order:
1. `.ralph/settings.json` (base; defaults applied if missing).
2. `.ralph/settings.local.json` (deep merge: scalars override, arrays replace,
   objects merge).
3. CLI overrides (highest priority for `maximumIterations`,
   `completionResponse`, and `streamAgentOutput`).

`agent.command` is required; validation fails otherwise.

## Prompt Construction
- Positional argument or `--prompt`/`-p` uses the inline value.
- `--prompt-file`/`-f` is re-read every iteration to allow live edits.
- Exactly one prompt source required (positional, `--prompt`, or `--prompt-file`).
- Guardrail failures feed into the next prompt using `APPEND`, `PREPEND`, or
  `REPLACE`, with a two-newline separator.
- If `includeIterationCountInPrompt` is enabled, prepend `Iteration X of Y, Z remaining.` with two newlines.

For Codex, the prompt is written to `.ralph/prompt_###.txt` and passed as a file
argument when the `e` subcommand is used.

## Agent Execution
Agent commands run via the user's shell (`$SHELL -ic`) so aliases/functions are
available. Output is always captured; if streaming is enabled:
- On non-Windows, a PTY is used for live output when possible.
- A pipe-based fallback is used if PTY setup fails.

### Streaming Modes

| Agent | Streaming Flags | Text Mode Flags |
|-------|-----------------|-----------------|
| `claude` | `--output-format stream-json --verbose` | `--output-format text` |
| `amp` | `--stream-json --dangerously-allow-all` | `--dangerously-allow-all` |
| `codex` | (none) | (none) |

The `--dangerously-allow-all` flag for Amp enables autonomous tool execution
without approval prompts, matching the autonomous workflow ralph provides.

## Guardrails
Guardrails run after each agent response. Results include:
- Full output saved to `.ralph/guardrail_<iter>_<slug>.log`.
- A truncated output snippet (default 5000 chars) used in prompt feedback.
- Exit codes and fail actions for logging and prompt updates.

Slugs replace non-alphanumeric characters with `_`, trim edges, and truncate to
50 chars; duplicate slugs get numeric suffixes.

## Completion Detection
Completion is detected only after guardrails pass. The response parser searches
for the first `<response>...</response>` tag (case-insensitive) and compares the
contents to `completionResponse` (case-insensitive).

## SCM Tasks
If configured, SCM tasks run after guardrails pass. Supported tasks:
- `commit`: prompt the agent (using text output mode) for a short imperative
  commit message and run `git commit -am "<message>"`.
- `push`: run `git push`.
- Any other task is run as `<scm.command> <task>`.

SCM errors are logged and do not stop the loop.

## Signals and Exit Codes
SIGINT/SIGTERM cancel the loop and exit with code 130 after the current work
completes. Other exit codes:
- `0`: completion matched
- `1`: max iterations reached
- `2`: configuration or validation error

## Runtime Files
- `.ralph/` stores guardrail logs and temporary prompt files.
- `.ralph/settings.json` and `.ralph/settings.local.json` drive configuration.

## Extension Points
- Add guardrails in settings to enforce repo-specific checks.
- Add agent flags to control model/provider behavior.
- Add SCM tasks for post-success automation (e.g., tag, push, custom scripts).
