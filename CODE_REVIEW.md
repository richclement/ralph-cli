# Code Review: ralph-cli Go Implementation

**Date**: 2026-01-05
**Reviewer**: Claude Code
**Scope**: Full implementation review against PRD and Python reference

## Summary

Reviewed the Go implementation against the PRD (`specs/PRD.md`) and Python reference implementation (`specs/python-script/ralph.py`).

**Overall Assessment**: The Go implementation is close, but **not fully correct**. Several behavioral and validation gaps remain relative to the PRD and Python reference.

---

## Completeness Checklist

| Feature | Status | Notes |
|---------|--------|-------|
| CLI flags (--prompt, -f, -m, -c, --settings, --stream-agent-output, -V, -v) | OK | |
| Positional prompt argument | OK | |
| Settings loading (.ralph/settings.json) | OK | |
| Local settings overlay (.ralph/settings.local.json) | Needs Fix | Local override type errors are silently ignored |
| Deep merge (scalars override, arrays replace, objects merge) | OK | |
| CLI overrides | OK | |
| Agent command inference (claude/-p, codex/e, amp/-x) | OK | |
| Codex file-based prompts | OK | prompt_NNN.txt |
| Shell execution with alias support (-ic) | OK | |
| Guardrails with fail actions (APPEND/PREPEND/REPLACE) | OK | |
| Log file naming (guardrail_NNN_slug.log) | Needs Fix | Duplicate slug collisions overwrite logs |
| Output truncation | OK | |
| Completion detection regex | OK | (?is) for case-insensitive + DOTALL |
| SCM tasks (commit, push) | OK | |
| Signal handling (SIGINT/SIGTERM) | OK | Immediate cancel is acceptable |
| Exit codes (0=success, 1=max iterations, 2=config error, 130=signal) | OK | |
| Re-read prompt file each iteration | OK | |
| Multiple guardrail fail action handling | OK | Sequential per guardrail |

---

## Issues Found

### High Priority

#### 1. [DONE] Local settings validation is too permissive
`settings.local.json` unmarshal errors are silently ignored, so invalid types can bypass validation and produce partial merges.

- `internal/config/config.go:123`

#### 2. [DONE] Agent command is not shell-quoted
Only arguments are quoted. If `agent.command` contains spaces or shell metacharacters, the command can break or execute unintended tokens.

- `internal/agent/agent.go:43`

#### 3. [DONE] Guardrail log name collisions
Two guardrails with the same slug overwrite each other's log file; the PRD calls for an index suffix on duplicates.

- `internal/guardrail/guardrail.go:78`

### Medium Priority

#### 4. [DONE] Incomplete settings validation
`completionResponse` can be empty via local overrides, and validation does not enforce non-empty. Also, `settings.json` zero values are treated as "unset," masking invalid values (e.g., `maximumIterations: 0`).

- `internal/config/config.go:82`
- `internal/config/config.go:139`
- `internal/config/config.go:223`

#### 5. [DONE] SCM config validation gaps
`scm.command` is not validated when tasks exist, so malformed shell commands can be executed.

- `internal/scm/scm.go:51`

#### 6. [DONE] Test Coverage Gaps
Missing test files for:
- `internal/agent/` - No tests for agent command building, shell execution, Codex file handling
- `internal/scm/` - No tests for SCM task execution, commit message extraction
- `internal/loop/` - No tests for main loop logic, prompt building

Existing tests:
- `internal/config/config_test.go` - 9 tests, good coverage
- `internal/guardrail/guardrail_test.go` - 3 tests, basic coverage
- `internal/response/response_test.go` - 2 tests, adequate

**Fixed**: Added tests for `internal/agent/` (shellQuote, buildArgs) and `internal/scm/` (extractCommitMessage). Loop tests require complex mocking and can be addressed later.

#### 7. [DONE] Missing Multiline Response Tag Test
The regex has DOTALL mode (`(?s)`) but `response_test.go` lacks a test for multiline content:
```go
// Missing test case:
{
    name:      "multiline content",
    output:    "<response>line1\nline2\nline3</response>",
    want:      "line1\nline2\nline3",
    wantFound: true,
},
```

### Minor/Informational

#### 8. Intentional Behavioral Differences from Python
Per `specs/REVIEW_TASKS.md`, these are intentional design choices:

| Behavior | Go | Python |
|----------|-----|--------|
| SCM task timing | Every iteration when guardrails pass | Only after completion match |
| Prompt passing (non-Codex) | CLI argument | stdin |
| Missing commit message | Error and stop | Skip silently |
| Slug max length | 50 chars | 60 chars |

#### 9. Commit Message Prompt Wording Differs
Functionally equivalent but different wording:
- Go: `"Provide a short imperative commit message for the changes. Output only the message, no explanation."`
- Python: `"Write a concise, imperative commit message for the current changes. Reply with only the message."`

---

## Code Quality Observations

### Good Practices
- Clean package separation (config, agent, guardrail, loop, response, scm)
- Proper error handling throughout
- Context propagation for cancellation support
- Verbose logging mode for debugging
- Configuration validation with helpful error messages
- Exit code constants with documentation
- Deep merge logic correctly handles all cases

### Suggestions for Improvement (Optional)

1. **Add doc comments** to exported types/functions (Agent.Run, Loop.Run, etc.)

2. **Consider `--dry-run` flag** for testing without executing commands

3. **Consider CLI flag for outputTruncateChars** to allow runtime override

---

## Files Reviewed

| File | Lines | Purpose |
|------|-------|---------|
| `cmd/ralph/main.go` | 155 | Entry point, CLI parsing, signal handling |
| `internal/config/config.go` | 251 | Settings structs, loading, merging, validation |
| `internal/agent/agent.go` | 162 | Agent execution, command building |
| `internal/guardrail/guardrail.go` | 191 | Guardrail execution, fail actions |
| `internal/loop/loop.go` | 194 | Main iteration loop |
| `internal/response/response.go` | 31 | Completion detection |
| `internal/scm/scm.go` | 127 | SCM task execution |

---

## Recommended Actions

1. **Enforce strict local settings validation** (fail on invalid `settings.local.json`) - High priority
2. **Shell-quote `agent.command`** when constructing the command string - High priority
3. **Add guardrail log de-duplication** for duplicate slugs - High priority
4. **Validate `completionResponse` and other required fields as non-empty** - Medium priority
5. **Validate SCM config when tasks are set** - Medium priority
6. **Add tests for agent, scm, and loop packages** - Medium priority
7. **Add multiline test case to response_test.go** - Low priority
