# Ralph Loop Harness (Go) PRD

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

---

## Implementation Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.25 |
| Module | `github.com/richclement/ralph-cli` |
| Binary | `ralph` |
| CLI Parser | [kong](https://github.com/alecthomas/kong) |
| JSON | `encoding/json` (stdlib) |
| Testing | `testing` (stdlib) |
| Linting | `golangci-lint` |
| Build | `Makefile` |
| Release | goreleaser (future), Homebrew tap (future) |

## Project Structure

```
ralph-cli/
├── cmd/
│   └── ralph/
│       └── main.go           # Entry point, kong CLI setup
├── internal/
│   ├── config/
│   │   ├── config.go         # Settings structs, loading, merging
│   │   └── config_test.go
│   ├── initcmd/
│   │   ├── initcmd.go        # Interactive init command
│   │   └── initcmd_test.go
│   ├── agent/
│   │   ├── agent.go          # Agent invocation, output streaming
│   │   └── agent_test.go
│   ├── guardrail/
│   │   ├── guardrail.go      # Guardrail execution, fail actions
│   │   └── guardrail_test.go
│   ├── loop/
│   │   ├── loop.go           # Main loop orchestration
│   │   └── loop_test.go
│   └── scm/
│       ├── scm.go            # SCM task execution
│       └── scm_test.go
├── .ralph/                   # Runtime directory (gitignored)
├── .golangci.yml             # Linter configuration
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── specs/
    └── PRD.md                # This file
```

---

## CLI

Using kong for argument parsing with subcommands:

```go
type CLI struct {
    Init    InitCmd          `cmd:"" help:"Initialize settings file interactively."`
    Run     RunCmd           `cmd:"" default:"1" help:"Run the agent loop."`
    Version kong.VersionFlag `name:"version" short:"v" help:"Print version and exit."`
}

type InitCmd struct{}

type RunCmd struct {
    Prompt             string `name:"prompt" short:"p" help:"Prompt string to send to agent."`
    PromptFile         string `name:"prompt-file" short:"f" help:"Path to file containing prompt text."`
    MaximumIterations  int    `name:"maximum-iterations" short:"m" help:"Maximum iterations before stopping."`
    CompletionResponse string `name:"completion-response" short:"c" help:"Completion response text."`
    StreamAgentOutput  *bool  `name:"stream-agent-output" help:"Stream agent output to console." negatable:""`
    Verbose            bool   `name:"verbose" short:"V" help:"Enable verbose/debug output."`
}
```

**Subcommands:**
- `ralph init` - Initialize settings file interactively (see Init Command section)
- `ralph run` - Run the agent loop (default command)
- `ralph --version` / `ralph -v` - Print version and exit

The `run` command is marked as default (`default:"1"`), so `ralph -p "prompt"` works without explicitly typing `run`.

**Version Handling:**
Kong provides built-in version support. Set version at build time via `-ldflags`:
```bash
go build -ldflags "-X main.version=1.0.0" -o ralph ./cmd/ralph
```

In main.go:
```go
var version = "dev"

func main() {
    ctx := kong.Parse(&cli, kong.Vars{"version": version})
    // ...
}
```

**Run Command Flags:**
- `--prompt`, `-p`: prompt string
- `--prompt-file`, `-f`: path to file contents used as prompt (mutually exclusive with `--prompt`)
- `--maximum-iterations`, `-m`: integer
- `--completion-response`, `-c`: string (default `DONE`)
- `--stream-agent-output` / `--no-stream-agent-output`: boolean (default `true`)
- `--verbose`, `-V`: enable verbose/debug output

**Settings Path:**
The settings file path is hardcoded to `.ralph/settings.json` (not configurable via CLI).

**Validation:**
- For the `run` command, exactly one of `--prompt` or `--prompt-file` must be provided (mutually exclusive).

---

## Init Command

The `ralph init` command interactively creates `.ralph/settings.json` by walking the user through required and optional settings.

**Requirements:**
- Interactive-only (requires TTY); exits with error if stdin is not a terminal
- Signal handling: Ctrl+C aborts gracefully without writing partial file (exit code 130)

**Behavior when settings exist:**
1. Load effective config by merging `.ralph/settings.json` with `.ralph/settings.local.json` (if present)
2. Pretty-print merged config to stdout using `json.MarshalIndent`
3. Display which files were loaded:
   - `"Loaded from .ralph/settings.json"` or
   - `"Loaded from .ralph/settings.json (with local overlay from settings.local.json)"`
4. Prompt for overwrite: `"Overwrite? (y/N): "` (default No)
5. If declined, exit with code 0

**Prompts:**
- `agent.command` (required, re-prompt if blank)
- `agent.flags` (optional, comma-separated, whitespace trimmed)
- `maximumIterations` (default `10` when blank, validates positive integer)
- `completionResponse` (default `DONE` when blank)
- Guardrails loop (0+):
  - `command` (empty input exits loop)
  - `failAction` (APPEND|PREPEND|REPLACE, case-normalized to uppercase)
- SCM setup (optional):
  - Prompt `"Configure SCM? (y/N):"` first
  - If yes: `scm.command`, `scm.tasks` (comma-separated)

**Defaults applied:**
- `outputTruncateChars`: `5000`
- `streamAgentOutput`: `true`

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success or declined overwrite |
| 2 | Validation error, non-TTY, or write failure |
| 130 | Interrupted by signal (Ctrl+C) or EOF |

**Example Flow:**
```
$ ralph init

Agent command (e.g., claude, codex, amp, or other LLM CLI): claude
Agent flags (comma-separated, optional): --model,opus
Maximum iterations [10]:
Completion response [DONE]:
Add guardrail command (leave blank to finish): make lint
  Fail action (APPEND|PREPEND|REPLACE): APPEND
Add guardrail command (leave blank to finish): make test
  Fail action (APPEND|PREPEND|REPLACE): APPEND
Add guardrail command (leave blank to finish):
Configure SCM? (y/N): y
  SCM command (e.g., git): git
  SCM tasks (comma-separated, e.g., commit,push): commit

Settings written to .ralph/settings.json
```

---

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
  "includeIterationCountInPrompt": false,
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

**Settings Structs:**
```go
type Settings struct {
    MaximumIterations  int          `json:"maximumIterations"`
    CompletionResponse string       `json:"completionResponse"`
    OutputTruncateChars int         `json:"outputTruncateChars"`
    StreamAgentOutput  bool         `json:"streamAgentOutput"`
    IncludeIterationCountInPrompt bool `json:"includeIterationCountInPrompt"`
    Agent              AgentConfig  `json:"agent"`
    Guardrails         []Guardrail  `json:"guardrails"`
    SCM                *SCMConfig   `json:"scm,omitempty"`
}

type AgentConfig struct {
    Command string   `json:"command"`
    Flags   []string `json:"flags"`
}

type Guardrail struct {
    Command    string `json:"command"`
    FailAction string `json:"failAction"` // APPEND, PREPEND, REPLACE
}

type SCMConfig struct {
    Command string   `json:"command"`
    Tasks   []string `json:"tasks"`
}
```

**Loading Priority:**
1. Load `./.ralph/settings.json` (base)
2. Deep merge `./.ralph/settings.local.json` if present
3. Override with CLI flags

**Deep Merge Behavior:**
Settings from `settings.local.json` are recursively merged into `settings.json`:
- Scalar values (string, int, bool) in local override base
- Arrays in local replace arrays in base (not concatenated)
- Objects are recursively merged (preserves keys not present in local)

Example:
```
base:  {"agent": {"command": "claude", "flags": ["--model opus"]}}
local: {"agent": {"flags": ["--verbose"]}}
result: {"agent": {"command": "claude", "flags": ["--verbose"]}}
```

**Agent Command Inference:**
- `claude` -> non-REPL flag `-p`
- `codex` -> non-REPL flag `e`
- `amp` -> non-REPL flag `-x`

**Streaming Output Flags:**
Agents that support structured JSON output receive additional flags when `streamAgentOutput` is enabled:
- `claude` -> `--output-format stream-json --verbose`
- `amp` -> `--stream-json --dangerously-allow-all`

**Text Mode Flags:**
For simple text operations (e.g., commit message generation), agents use text mode:
- `claude` -> `--output-format text`
- `amp` -> `--dangerously-allow-all` (omits `--stream-json`)

---

## Loop Behavior

For each iteration:
1. Build the base prompt:
   - If `--prompt` is provided, use it.
   - If `--prompt-file` is provided, re-read the file on each iteration.
2. If any guardrail failed in the previous iteration and its `failAction` is `APPEND` or `PREPEND`, add the failed guardrail output to the base prompt.
   - Separate new text from existing text with two newlines.
3. If `includeIterationCountInPrompt` is enabled, prepend `Iteration X of Y, Z remaining.` with two newlines separating it from the prompt.
4. Invoke the agent with configured command and flags.
   - Always capture full agent output for completion detection.
   - If `streamAgentOutput` is enabled, tee the agent output to the console as it arrives (use `io.TeeReader` or similar).
5. Run guardrails after the agent response.
6. Save each guardrail output to `./.ralph/guardrail_<iter>_<slug>.log`.
7. If guardrails failed, apply their `failAction` to the next prompt and continue the loop.
8. If all guardrails pass, check for completion response and stop if matched.

If max iterations is reached without completion, exit non-zero (exit code 1).

**Prompt Construction:**
- `--prompt` uses the value as-is.
- `--prompt-file` is re-read each iteration to construct the base prompt.
- If any guardrail failed in the previous iteration and its `failAction` is `APPEND` or `PREPEND`, include the failed guardrail output in the next prompt.
- Separate appended/prepended guardrail text from existing prompt text with two newlines.
- If `includeIterationCountInPrompt` is enabled, prepend `Iteration X of Y, Z remaining.` with two newlines before the prompt content.

---

## Completion Detection

- Look for `<response>message</response>` in agent output.
- Case-insensitive exact match against `completionResponse`.
- If multiple tags, the first match wins.
- Use `regexp.MustCompile(`(?i)<response>(.*?)</response>`)` for extraction.
- Completion detection occurs only after guardrails pass for that iteration.

---

## Guardrails

Each guardrail has:
- `command`: shell command to run.
- `failAction`: one of `APPEND`, `PREPEND`, `REPLACE`.

**Shell Execution:**
- Unix (Linux, macOS): `sh -c "<command>"`
- Windows: `cmd /c "<command>"`

Detect OS at runtime using `runtime.GOOS`.

On failure:
- A failure is a non-zero exit code.
- Always write full output (stdout+stderr) to a file under `./.ralph/`.
- Truncate output sent to the agent to `outputTruncateChars` (default 5000).
  - Truncation keeps the first N characters and appends `... [truncated]`.
  - The indicator is only added when truncation actually occurs.
- Print guardrail start/end, exit status, and fail action used.

**Log File Naming:**
- Log files are named `./.ralph/guardrail_<iter>_<slug>.log`.
- Slug is derived from command: replace non-alphanumeric chars with `_`, truncate to 50 chars.
- Example: `./mvnw clean install -T 2C` → `mvnw_clean_install_T_2C`.

---

## SCM Tasks

Optional. If configured and guardrails pass:
1. Invoke the agent (using the same runner as the main loop) with a fixed prompt:
   `"Provide a short imperative commit message for the changes. Output only the message, no explanation."`
2. Parse the agent response to extract the commit message:
   - If response contains `<response>` tag, use its contents.
   - Otherwise, use the first non-empty line of output.
   - If no valid message is extracted, abort SCM tasks with an error.
3. Run `scm.command` with each task in order (e.g., `commit`, `push`).
   - For commit, use `-am` with the agent-provided message (e.g., `git commit -am "<message>"`).

If guardrails fail, SCM tasks do not run.

---

## Defaults

| Setting | Default |
|---------|---------|
| `maximumIterations` | `10` |
| `completionResponse` | `DONE` |
| `outputTruncateChars` | `5000` |
| `streamAgentOutput` | `true` |

---

## Repository Setup Requirements

### 1. Initialize Go Module
```bash
go mod init github.com/richclement/ralph-cli
```

### 2. Create Directory Structure
```bash
mkdir -p cmd/ralph internal/{config,agent,guardrail,loop,scm} specs
```

### 3. Add Dependencies
```bash
go get github.com/alecthomas/kong
```

### 4. Create .gitignore
```gitignore
# Binaries
ralph
*.exe
dist/

# Runtime
.ralph/*.log

# IDE
.idea/
.vscode/
*.swp

# Go
vendor/

# OS
.DS_Store
```

### 5. Create .golangci.yml
Configure linters: `gofmt`, `govet`, `errcheck`, `staticcheck`, `unused`, `gosimple`, `ineffassign`.

### 6. Create Makefile
Targets:
- `build`: compile binary to `./ralph`
- `test`: run `go test ./...`
- `lint`: run `golangci-lint run`
- `fmt`: run `go fmt ./...`
- `clean`: remove binary and dist
- `install`: install to `$GOPATH/bin` via `go install ./cmd/ralph`
- `all`: fmt, lint, test, build

### 7. Update README.md
Must include:
- Project description
- Prerequisites (Go 1.25+, golangci-lint)
- Installation instructions (go install, build from source)
- Local development setup instructions
- Build commands
- Usage examples
- Configuration reference
- Architecture overview

---

## Installation

### From Source (go install)
```bash
go install github.com/richclement/ralph-cli/cmd/ralph@latest
```

### Clone and Build
```bash
git clone https://github.com/richclement/ralph-cli.git
cd ralph-cli
make build
# Binary at ./ralph
```

### Future: Homebrew (not yet implemented)
```bash
brew install richclement/tap/ralph
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (completion response matched) |
| 1 | Max iterations reached without completion |
| 2 | Configuration/validation error |
| 130 | Interrupted by signal (SIGINT/SIGTERM) |

---

## Signal Handling

Ralph gracefully handles termination signals:

**Signals:**
- `SIGINT` (Ctrl+C)
- `SIGTERM`

**Behavior:**
1. On first signal: set shutdown flag, wait for current operation to complete
2. If agent is running: allow it to finish current response, then exit
3. If guardrail is running: allow it to finish, then exit
4. Print message: `Received signal, shutting down...`
5. Exit with code 130

**Implementation:**
```go
ctx, cancel := context.WithCancel(context.Background())
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    fmt.Fprintln(os.Stderr, "Received signal, shutting down...")
    cancel()
}()
```

Pass `ctx` to agent and guardrail runners; check `ctx.Done()` between iterations.

---

## Verbose Output

When `--verbose` is enabled, print additional debug information:

- Settings loaded from each file
- Merged configuration (redacted if sensitive)
- Agent command being executed
- Prompt being sent (truncated to first 200 chars)
- Guardrail commands and exit codes
- Completion detection attempts
- Timing information for each operation

**Format:**
```
[ralph] Loading settings from .ralph/settings.json
[ralph] Loading settings from .ralph/settings.local.json
[ralph] Agent command: claude -p --model opus
[ralph] Iteration 1/10 starting
[ralph] Running guardrail: ./mvnw clean install
[ralph] Guardrail exited with code 0 (1.2s)
```

Use `log` package or simple `fmt.Fprintf(os.Stderr, "[ralph] ...")` for verbose output.

---

## Testing Requirements

- Unit tests for each internal package
- Test settings loading and merging
- Test CLI flag parsing and validation
- Test completion response detection regex
- Test guardrail fail action application
- Test output truncation

---

## Agent Streaming Formats

### Amp Streaming JSON

Amp outputs 5 message types as NDJSON when `--stream-json` is enabled:

**System Init** (first message):
```json
{"type": "system", "subtype": "init", "session_id": "...", "tools": [...]}
```

**Assistant** (text and tool use):
```json
{"type": "assistant", "message": {"content": [{"type": "text", "text": "..."}], "usage": {...}}}
{"type": "assistant", "message": {"content": [{"type": "tool_use", "id": "...", "name": "...", "input": {...}}]}}
```

**User** (tool results):
```json
{"type": "user", "message": {"content": [{"type": "tool_result", "tool_use_id": "...", "content": "..."}]}}
```

**Result Success**:
```json
{"type": "result", "subtype": "success", "result": "...", "duration_ms": 1234, "num_turns": 3, "usage": {"input_tokens": 100, "output_tokens": 50}}
```

**Result Error**:
```json
{"type": "result", "subtype": "error_during_execution", "error": "...", "is_error": true}
```

---

## Future Considerations (Out of Scope)

- goreleaser configuration for cross-platform builds
- Homebrew formula
- Shell completions (bash, zsh, fish via kong)
- Dry-run mode
- Timeout per agent/guardrail invocation
- Retry logic for transient failures
