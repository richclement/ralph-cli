# Ralph Go Implementation Review Tasks

Comparison of Go implementation against Python reference (`specs/python-script/ralph.py`).

## Critical

### 1. [DONE] failAction is IGNORED
**File:** `internal/loop/loop.go:162`

~~The `failAction` from guardrail config is **never used** - hardcoded to "APPEND". Must pass the actual action from failed guardrails.~~

**Fixed:** Now stores failed results and applies each guardrail's `failAction` sequentially.

### 2. [DONE] Multiple guardrail failures handled incorrectly
**Files:** `internal/loop/loop.go:111`, `internal/guardrail/guardrail.go:134-144`

~~Go combines all failures into one string and always APPENDs. Each guardrail's `failAction` should be applied individually in order.~~

**Fixed:** Now iterates through failed results and applies each one's action sequentially (matching Python behavior).

### 3. [DONE] Codex Prompt File Handling
**File:** `internal/agent/agent.go:57-80`

~~Go currently passes prompt as CLI argument for all agents. Need to add file-based prompt passing for Codex.~~

**Fixed:** Now detects Codex with "e" subcommand and writes prompt to `.ralph/prompt_XXX.txt`, passing file path instead of prompt text. File is cleaned up after execution.

### 4. [DONE] Regex Missing DOTALL Flag
**File:** `internal/loop/completion.go:8`

~~Missing `(?s)` flag for DOTALL mode where `.` matches newlines.~~

**Fixed:** Added `(?s)` flag to regex: `(?is)<response>(.*?)</response>`

### 5. [DONE] Missing failure context in prompt
**File:** `internal/guardrail/guardrail.go:134-144`

~~Python's `format_failure_message` includes exit code, log file path, command name. Go's `GetFailedOutputForPrompt` only includes command name and output.~~

**Fixed:** Added `FormatFailureMessage` function that includes exit code, log file path, and truncated output. Added `ExitCode` field to `Result` struct.

### 6. [DONE] SCM tasks should run every iteration when guardrails pass
**Files:** `internal/loop/loop.go`, `cmd/ralph/main.go:152-162`

~~SCM tasks run only once, after the loop exits. Should run on every iteration where guardrails pass.~~

**Fixed:** SCM now runs inside the loop after guardrails pass, before checking completion. Created `internal/response` package to resolve import cycle.

### 7. [DONE] Loop continues after agent failure
**File:** `internal/loop/loop.go:93-102`

~~Agent failures were logged but loop continued. Should exit with error.~~

**Fixed:** Agent failure now prints error to stderr and returns `ExitConfigError` immediately.

### 8. [DONE] No user feedback without verbose mode
**Files:** `internal/loop/loop.go`, `internal/guardrail/guardrail.go`

~~Go only outputs status via `r.log()` which requires `-V` flag.~~

**Fixed:** Added `print()` method that always outputs. Now shows:
- Iteration banner: `=== Ralph iteration N/M ===`
- Guardrail start/end with exit codes
- All guardrails passed
- Completion response matched
- Maximum iterations reached

### 9. [DONE] Agent command doesn't support shell aliases/functions
**File:** `internal/agent/agent.go:33-54`

~~Go uses `exec.Command` directly which bypasses the shell. This means shell aliases (e.g., `alias cco="claude"`) and functions defined in `.bashrc`/`.zshrc` don't work.~~

**Fixed:** Now runs agent through user's interactive shell with `-ic` flag. Added `shellQuote()` function to safely escape arguments for shell execution.

### 10. [DONE] Prompt passed as CLI argument instead of stdin
**File:** `internal/agent/agent.go:50,77`

~~Python sends prompt via stdin. Go passes prompt as CLI argument.~~

**Resolved:** This is not an issue because:
- Claude's `-p` flag expects the prompt as an argument, not stdin
- Codex uses file-based prompts (task #3)
- Task #9's shell quoting handles special characters
- Modern shells support command lines of 128KB-2MB, sufficient for prompts
- Shell execution (task #9) is required for alias support, making direct stdin piping complex

## Optional

### 11. [DONE] Log file naming format
**File:** `internal/guardrail/guardrail.go:66`

~~Python: `guardrail_001_slug.log` (zero-padded iteration), Go: `guardrail_1_slug.log` (no padding)~~

**Fixed:** Now uses `%03d` format for zero-padded iteration numbers matching Python.

### 12. [DONE] Config Validation
**File:** `internal/config/config.go:222-228`

~~Go only validates `agent.command`. Python validates additional fields.~~

**Fixed:** Added validation for:
- `maximumIterations` must be positive integer
- `outputTruncateChars` must be positive integer
- `guardrails[].command` must not be empty
- `guardrails[].failAction` must be APPEND/PREPEND/REPLACE

Note: `streamAgentOutput` is a boolean type in Go and JSON unmarshaling handles type validation.

## Not Issues (Intentional Differences)

- **Prompt file re-read**: Go re-reads prompt file each iteration; this is correct (allows dynamic updates)
- **Extra features in Go**: verbose mode, version flag, signal handling (SIGINT/SIGTERM), distinct exit codes (0/1/2/130)
- **Fail-fast on missing commit message**: Go returns error if agent doesn't provide commit message; this is correct (fail-fast is preferred over silent skip)
