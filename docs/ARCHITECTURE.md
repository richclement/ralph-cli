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
5. If guardrails pass and review threshold reached, run review cycle.
6. If guardrails pass, optionally run SCM tasks.
7. Check completion response; stop when matched or exit after max iterations.

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
  review cycles, SCM tasks, and completion detection.
- `internal/review` runs review cycles with configurable prompts and
  guardrail retries.
- `internal/scm` runs SCM tasks and uses the agent to generate commit messages.
- `internal/response` extracts `<response>...</response>` and determines
  completion.
- `internal/stream` provides parsers for agent streaming JSON formats (Claude,
  Codex, Amp), normalizes output into a common event model, and formats events
  for display with tool correlation and statistics tracking.

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

### Agent-Specific Flags

Each supported agent has inferred flags for non-REPL mode and output handling:

| Agent | Non-REPL | Streaming Flags | Text Mode Flags |
|-------|----------|-----------------|-----------------|
| `claude` | `-p` | `--output-format stream-json --verbose` | `--output-format text` |
| `amp` | `-x` | `--stream-json --dangerously-allow-all` | `--dangerously-allow-all` |
| `codex` | `e` | `--json --full-auto` | `--full-auto -o <file>` |

### Amp Integration

The `--dangerously-allow-all` flag for Amp enables autonomous tool execution
without approval prompts, matching the autonomous workflow ralph provides.

Note: Amp requires `-x <prompt>` to appear together at the end of the command,
so ralph orders flags accordingly: `amp [flags...] -x <prompt>`.

### Codex Integration

Codex has special handling:

**Streaming Mode:** `codex e --json --full-auto <prompt-file>`
- `--json` for structured JSON output (enables parsing)
- `--full-auto` for autonomous mode (no approval prompts)
- Prompt written to `.ralph/prompt_###.txt` to avoid shell escaping

**Text Mode:** `codex e --full-auto -o <output-file> <prompt-file>`
- Used for commit messages where plain text is needed
- Omits `--json` for plain text output
- `-o` writes response to file, which is read and cleaned up after execution

## Stream Processing

When streaming is enabled, agent output is parsed and formatted for display.
The `internal/stream` package handles this with three components:

### Event Model

Agent-specific JSON formats are normalized into a common `Event` struct:

| Event Type | Description |
|------------|-------------|
| `EventToolStart` | Tool invocation began (name, ID, input summary) |
| `EventToolEnd` | Tool completed with output or error |
| `EventText` | Assistant text output |
| `EventResult` | Final completion with cost and token statistics |
| `EventTodo` | Task list update from TodoWrite tool calls |
| `EventProgress` | Status updates (session info, progress) |

Events include timestamps, tool correlation IDs, and for `EventResult`:
- `Cost`: cumulative cost in USD
- `InputTokens`, `OutputTokens`: token counts
- `CacheReadTokens`, `CacheWriteTokens`: cache statistics

### Agent Parsers

Each agent has a dedicated parser implementing the `Parser` interface:

- **ClaudeParser**: Parses Claude Code `stream-json` format with tool_use,
  tool_result, and result messages. Extracts TodoWrite calls as `EventTodo`.
- **CodexParser**: Parses Codex `--json` format with thread/turn/item events.
  Maps command_execution to tool events, reasoning and agent_message to text.
- **AmpParser**: Parses Amp `--stream-json` format with system, assistant,
  user, and result messages.

### Formatter and Display

The `Formatter` renders events with visual indicators and tool correlation:

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

Configuration options (via `FormatterConfig`):
- `UseEmoji`: emoji indicators vs text markers (`[OK]`, `[ERR]`)
- `UseColor`: ANSI color output (auto-detected via termenv)
- `ShowTimestamp`: `[HH:MM:SS]` prefix on each line
- `MaxOutputLines`, `MaxOutputChars`: truncation limits for tool output
- `ShowText`: display assistant text output
- `Verbose`: show extra details and empty results

Tool correlation tracks pending tool calls by ID and displays the correlation
when results arrive (`Result ← Read`). Statistics (tool count, error count,
elapsed time) accumulate across the iteration.

## Guardrails
Guardrails run after each agent response. Each guardrail has:
- `command`: shell command to run
- `failAction`: `APPEND`, `PREPEND`, or `REPLACE`
- `hint` (optional): guidance text injected into the prompt on failure

Results include:
- Full output saved to `.ralph/guardrail_<iter>_<slug>.log`.
- A truncated output snippet (default 5000 chars) used in prompt feedback.
- Exit codes and fail actions for logging and prompt updates.

On failure, the message sent to the agent includes the command, exit code, log
file path, optional hint (if configured), and truncated output. Hints are literal
strings and are never truncated.

Slugs replace non-alphanumeric characters with `_`, trim edges, and truncate to
50 chars; duplicate slugs get numeric suffixes.

## Review Cycles

Review cycles implement the "Rule of 5" concept: forcing agents to review their
work multiple times from different angles leads to significantly better output.

### Configuration

```json
{
  "reviews": {
    "reviewAfter": 10,
    "guardrailRetryLimit": 3,
    "prompts": [
      {"name": "detailed", "prompt": "Review for correctness..."},
      {"name": "architecture", "prompt": "Review the overall design..."}
    ]
  }
}
```

| Field | Description |
|-------|-------------|
| `reviewAfter` | Iterations between review cycles (0 = disabled) |
| `guardrailRetryLimit` | Max retries per review prompt when guardrails fail |
| `prompts` | Array of review prompts (omit for defaults, empty `[]` disables) |

### Default Prompts

When `prompts` is omitted, these 4 defaults are used:
- **detailed**: Review for correctness, edge cases, and error handling
- **architecture**: Review overall design and problem approach
- **security**: Review for vulnerabilities (injection, auth, data exposure)
- **codeHealth**: Review naming, structure, duplication, simplicity

### Review Cycle Flow

```
for each reviewPrompt in prompts:
    retries = 0
    currentPrompt = reviewPrompt.prompt
    loop:
        run agent with currentPrompt
        run guardrails
        if guardrails pass:
            break  // next review prompt
        retries++
        if retries >= guardrailRetryLimit:
            log warning, break  // next review prompt
        inject guardrail failure into currentPrompt
```

### Key Design Decisions

1. **Review iterations do NOT count toward maxIterations** - Reviews happen
   within outer loop iterations, not as separate iterations.

2. **Reviews run after guardrails pass** - Only trigger when guardrails are
   green and the loop count threshold is met.

3. **Guardrails run after each review prompt** - If agent makes changes during
   review, guardrails validate before proceeding.

4. **Guardrail failures inject into review prompt** - Not the main prompt.

5. **Deterministic ordering** - Prompts array ensures consistent execution.

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
