# Ralph Loop Harness (Python + UV) PRD

## Overview
Ralph is a deterministic outer loop that repeatedly runs a CLI LLM agent until
it returns a completion response. It enforces guardrails (build/lint/test) and
optionally runs SCM commands when guardrails pass.

## Goals
- Loop an agent until a completion response or max iterations.
- Support per-repo defaults via `./.ralph/settings.json`, overridden by CLI.
- Support optional local overrides via `./.ralph/settings.local.json`, with CLI still taking highest priority.
- Enforce guardrails and feed failures back into the next prompt.
- Optionally run SCM tasks (e.g., commit, push) when guardrails pass.
- Stream agent output to the console while still capturing it for completion detection (configurable).

## Non-Goals
- Full agent framework or CI replacement.
- Auth/credential management for SCM or model providers.

## CLI
- `--prompt` string
- `--prompt-file` path to file contents used as prompt
- `--maximum-iterations`, `-m` integer
- `--completion-response`, `-c` string (default `DONE`)
- `--settings` path (default `./.ralph/settings.json`)
- `--stream-agent-output` (default `true`)
- `--no-stream-agent-output` to disable streaming

## Settings File
Default path: `./.ralph/settings.json`
Optional local overrides: `./.ralph/settings.local.json`

Example:
```json
{
  "maximumIterations": 10,
  "completionResponse": "DONE",
  "outputTruncateChars": 5000,
  "streamAgentOutput": true,
  "agent": {
    "command": "claude",
    "flags": ["--model opus", "--no-auto-compact"]
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

Notes:
- Load `./.ralph/settings.json` first, then `./.ralph/settings.local.json` if present.
- Settings from `settings.local.json` override `settings.json`.
- CLI values override settings values.
- `agent.command` determines non-REPL invocation flags:
  - `claude` -> `-p`
  - `codex` -> `e`
  - `amp` -> `-x`
- `agent.flags` are appended after the non-REPL invocation flags.
- Streaming behavior is controlled by `streamAgentOutput`; provider-specific streaming flags (if desired) can be included in `agent.flags`.

## Loop Behavior
For each iteration:
1. Build prompt (base prompt plus any guardrail feedback).
2. Invoke the agent with configured command and flags.
   - Always capture full agent output for completion detection.
   - If `streamAgentOutput` is enabled, tee the agent output to the console as it arrives.
3. Run guardrails after the agent response.
4. Save each guardrail output to `./.ralph/guardrail_<iter>_<slug>.log`.
5. If a guardrail fails, apply its `failAction` to the next prompt.
6. Check for completion response and stop if matched.

If max iterations is reached without completion, exit non-zero.

## Completion Detection
- Look for `<response>message</response>` in agent output.
- Case-insensitive exact match against `completionResponse`.
- If multiple tags, the first match wins.

## Guardrails
Each guardrail has:
- `command`: shell command to run.
- `failAction`: one of `APPEND`, `PREPEND`, `REPLACE`.

On failure:
- Always write full output to a file under `./.ralph/`.
- Truncate output sent to the agent to `outputTruncateChars` (default 5000).
- Print guardrail start/end, exit status, and fail action used.

## SCM Tasks
Optional. If configured and guardrails pass:
1. Ask agent for a commit message (short, imperative).
2. Run `scm.command` with each task in order (e.g., `commit`, `push`).

If guardrails fail, SCM tasks do not run.

## Defaults
- `completionResponse`: `DONE`
- `outputTruncateChars`: `5000`
