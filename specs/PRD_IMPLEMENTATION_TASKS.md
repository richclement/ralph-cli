# Ralph Loop Harness (Go) Implementation Tasks

This task list breaks the PRD into small, well-scoped steps that a junior
developer or another agent can implement independently. Each task includes
intent, scope, and clear completion checks.

## 0. Repo and Tooling Baseline

### Task 0.1: Initialize Go module
- **Intent**: Ensure the module path matches the PRD.
- **Work**:
  - Run `go mod init github.com/richclement/ralph-cli`.
- **Done when**:
  - `go.mod` exists with module path `github.com/richclement/ralph-cli`.

### Task 0.2: Create project folders
- **Intent**: Establish the directory layout for packages and entry point.
- **Work**:
  - Create `cmd/ralph` and `internal/{config,agent,guardrail,loop,scm}`.
  - Ensure `specs/` already exists (do not remove PRD).
- **Done when**:
  - `cmd/ralph` and all `internal/*` folders exist.

### Task 0.3: Add base `.gitignore`
- **Intent**: Keep generated files out of git.
- **Work**:
  - Add `.gitignore` with entries from PRD.
- **Done when**:
  - `.gitignore` contains patterns for `ralph`, `dist/`, `.ralph/*.log`,
    IDE files, `vendor/`, `.DS_Store`.

### Task 0.4: Add `.golangci.yml`
- **Intent**: Standardize lint configuration.
- **Work**:
  - Create `.golangci.yml` with linters: `gofmt`, `govet`, `errcheck`,
    `staticcheck`, `unused`, `gosimple`, `ineffassign`.
- **Done when**:
  - Linter config file is present and valid YAML.

### Task 0.5: Add Makefile
- **Intent**: Provide standard build/test/lint targets.
- **Work**:
  - Create targets: `build`, `test`, `lint`, `fmt`, `clean`, `install`, `all`.
- **Done when**:
  - `make build` builds `./ralph`.
  - `make test` runs `go test ./...`.

### Task 0.6: Add dependencies
- **Intent**: Bring in CLI parsing library.
- **Work**:
  - Add `github.com/alecthomas/kong` to `go.mod`.
- **Done when**:
  - `go.mod` lists kong; `go.sum` updated.

### Task 0.7: Create runtime directory
- **Intent**: Ensure `.ralph/` directory exists for guardrail logs.
- **Work**:
  - Create `.ralph/` directory at startup if it does not exist.
  - Use `os.MkdirAll` to handle nested creation.
- **Done when**:
  - Running ralph creates `.ralph/` if missing.
  - Guardrail logs can be written without "directory not found" errors.

---

## 1. CLI and Entry Point

### Task 1.1: Define CLI struct and validation
- **Intent**: Match the PRD CLI interface.
- **Work**:
  - Create `cmd/ralph/main.go` with `CLI` struct and kong annotations.
  - Add validation: exactly one of prompt or prompt-file is provided.
- **Done when**:
  - Running `ralph --help` shows all flags.
  - Supplying both or neither prompt sources exits with code 2.

### Task 1.2: Wire version flag
- **Intent**: Support build-time versioning.
- **Work**:
  - Add `var version = "dev"` and pass to kong via `kong.Vars`.
- **Done when**:
  - `ralph --version` prints `dev` by default.
  - `go build -ldflags "-X main.version=1.0.0"` changes version output.

### Task 1.3: CLI to config mapping
- **Intent**: Translate CLI options into runtime settings.
- **Work**:
  - Map CLI fields to the final `Settings` struct (override base).
  - Support `--stream-agent-output` with negatable form.
- **Done when**:
  - CLI values override settings file values when both are set.

### Task 1.4: Prompt file validation
- **Intent**: Validate prompt file exists and is readable when specified.
- **Work**:
  - If `--prompt-file` is provided, verify file exists at startup.
  - Return exit code 2 with descriptive error if file not found or unreadable.
- **Done when**:
  - Missing prompt file exits with code 2 and error message.
  - Unreadable prompt file (permissions) exits with code 2.

---

## 2. Configuration Loading and Merging

### Task 2.1: Define settings structs
- **Intent**: Provide typed configuration representation.
- **Work**:
  - Create `internal/config/config.go` with `Settings`, `AgentConfig`,
    `Guardrail`, and `SCMConfig` per PRD.
- **Done when**:
  - Structs match PRD JSON fields and tags exactly.

### Task 2.2: Load base settings file
- **Intent**: Read `./.ralph/settings.json` if present.
- **Work**:
  - Implement `LoadSettings(path string) (Settings, error)`.
  - Provide defaults if file not found.
- **Done when**:
  - Missing file does not error; defaults are used.

### Task 2.3: Merge local override file
- **Intent**: Merge `./.ralph/settings.local.json` if present.
- **Work**:
  - Implement deep merge rules (scalars override, arrays replace,
    objects merge recursively).
- **Done when**:
  - Example in PRD produces expected merged result.

### Task 2.4: Apply CLI overrides
- **Intent**: Highest-priority CLI values override merged settings.
- **Work**:
  - Apply overrides after local merge.
  - Only override when CLI flag is explicitly set.
- **Done when**:
  - CLI values take precedence without losing other settings.

### Task 2.5: Defaults and validation
- **Intent**: Ensure required defaults are set and required fields present.
- **Work**:
  - Set defaults for maximumIterations, completionResponse,
    outputTruncateChars, streamAgentOutput.
  - Validate `agent.command` is configured; exit code 2 with error if missing.
  - No default for agent command—it must be explicitly configured.
- **Done when**:
  - Settings contain defaults when unset in files or CLI.
  - Missing `agent.command` exits with code 2 and descriptive error message.

---

## 3. Agent Execution

### Task 3.1: Agent runner interface
- **Intent**: Encapsulate command invocation and output capture.
- **Work**:
  - Create `internal/agent/agent.go` with a function like
    `Run(ctx, prompt, settings) (output string, err error)`.
- **Done when**:
  - The runner returns full output and respects context cancellation.

### Task 3.2: Command inference for non-REPL mode
- **Intent**: Add default flags based on agent command name.
- **Work**:
  - If agent command is `claude`, add `-p`.
  - If `codex`, add `e`.
  - If `amp`, add `-x`.
- **Done when**:
  - Inferred flag appears before user-provided flags in execution.

### Task 3.3: Streaming output support
- **Intent**: Stream agent output to console while capturing.
- **Work**:
  - If `streamAgentOutput` is true, tee stdout/stderr to console.
  - Always capture combined output in memory for completion detection.
- **Done when**:
  - Output appears live and full text is still returned.

---

## 4. Guardrails

### Task 4.1: Guardrail execution function
- **Intent**: Run shell commands across OSes.
- **Work**:
  - Create `internal/guardrail/guardrail.go`.
  - Use `sh -c` on Unix, `cmd /c` on Windows.
- **Done when**:
  - A guardrail command runs correctly on the current OS.

### Task 4.2: Guardrail output capture and logging
- **Intent**: Persist logs and truncate prompt feedback.
- **Work**:
  - Write full output to `./.ralph/guardrail_<iter>_<slug>.log`.
  - Truncate output sent to agent to `outputTruncateChars`.
- **Done when**:
  - Log file contains full output and truncated output respects limit.

### Task 4.3: Fail action handling
- **Intent**: Implement APPEND, PREPEND, REPLACE behavior.
- **Work**:
  - Provide helper to apply fail action to prompt text.
- **Done when**:
  - Each action matches PRD behavior with two newline separator.

### Task 4.4: Guardrail slug generation
- **Intent**: Generate filesystem-safe slug from guardrail command.
- **Work**:
  - Derive slug from command for log filename `guardrail_<iter>_<slug>.log`.
  - Replace non-alphanumeric characters with underscores.
  - Truncate slug to reasonable length (e.g., 50 chars) to avoid path issues.
  - Handle duplicate slugs by appending index if needed.
- **Done when**:
  - Commands like `./mvnw clean install -T 2C` produce slug `mvnw_clean_install_T_2C`.
  - Special characters and spaces are sanitized.

### Task 4.5: Output truncation behavior
- **Intent**: Define how truncation works for guardrail output.
- **Work**:
  - Truncate from end of output (keep first N chars).
  - Append `... [truncated]` indicator when truncation occurs.
- **Done when**:
  - Long output is truncated to `outputTruncateChars` plus indicator.
  - Indicator only appears when truncation actually happened.

---

## 5. Completion Detection

### Task 5.1: Implement regex extraction
- **Intent**: Detect completion response tags.
- **Work**:
  - Use `(?i)<response>(.*?)</response>` with first match.
  - Compare case-insensitively to `completionResponse`.
- **Done when**:
  - Detection succeeds only when guardrails pass for the iteration.

---

## 6. Loop Orchestration

### Task 6.1: Core loop control
- **Intent**: Drive iterations and exit codes.
- **Work**:
  - Create `internal/loop/loop.go`.
  - Enforce max iterations, exit code 1 on no completion.
- **Done when**:
  - Loop stops on completion or max iterations.

### Task 6.2: Prompt construction
- **Intent**: Build per-iteration prompt with guardrail feedback.
- **Work**:
  - Read prompt file every iteration if used.
  - Apply any previous guardrail failures per fail action.
- **Done when**:
  - Prompt updates reflect latest file and failure feedback.

### Task 6.3: Guardrail gating
- **Intent**: Ensure completion is only checked when guardrails pass.
- **Work**:
  - Run guardrails after agent output.
  - Skip completion check on guardrail failure.
- **Done when**:
  - Completion is ignored on any guardrail failure.

---

## 7. SCM Tasks

### Task 7.1: SCM task runner
- **Intent**: Run SCM tasks when guardrails pass.
- **Work**:
  - Create `internal/scm/scm.go`.
  - Execute `scm.command` with each task in order.
- **Done when**:
  - Tasks run only after guardrails pass.

### Task 7.2: Commit message flow
- **Intent**: Obtain message from agent and use for commit.
- **Work**:
  - Invoke agent using same mechanism as main loop (Task 3.1).
  - Use a fixed prompt requesting a short imperative commit message.
  - Parse agent output to extract commit message (first line or `<response>` tag).
  - For `commit` task, run `<cmd> commit -am "<message>"`.
  - If agent fails to provide valid message, use a fallback or abort with error.
- **Done when**:
  - Commit task uses agent-provided message.
  - Agent invocation for commit message uses same runner as main loop.

---

## 8. Signal Handling and Context

### Task 8.1: Context propagation
- **Intent**: Support graceful shutdown.
- **Work**:
  - Create `context.Context` in main and pass through loop, agent,
    and guardrail runners.
- **Done when**:
  - Cancellation stops further iterations cleanly.

### Task 8.2: Signal handling
- **Intent**: Exit cleanly on SIGINT/SIGTERM.
- **Work**:
  - Add signal handler per PRD; print shutdown message; exit 130.
- **Done when**:
  - Ctrl+C prints message and exits with code 130.

---

## 9. Verbose Logging

### Task 9.1: Add verbose logging helper
- **Intent**: Standardize debug output.
- **Work**:
  - Provide helper that writes to stderr with `[ralph]` prefix.
  - Log: settings load, merged config, agent command, prompt preview,
    guardrail start/end, completion checks, timing.
- **Done when**:
  - Logs appear only when `--verbose` is true.

---

## 10. Tests

### Task 10.1: Config load/merge tests
- **Intent**: Validate settings precedence rules.
- **Work**:
  - Add `internal/config/config_test.go` with base/local/CLI cases.
- **Done when**:
  - Tests cover deep merge behavior for scalars, arrays, objects.

### Task 10.2: CLI validation tests
- **Intent**: Ensure prompt selection rules.
- **Work**:
  - Add tests for mutual exclusion and missing prompt sources.
- **Done when**:
  - Invalid combinations return exit code 2.

### Task 10.3: Completion detection tests
- **Intent**: Ensure response matching logic.
- **Work**:
  - Test regex extraction and case-insensitive matching.
- **Done when**:
  - First `<response>` tag is used and matched correctly.

### Task 10.4: Guardrail fail action tests
- **Intent**: Verify prompt modification rules.
- **Work**:
  - Tests for APPEND, PREPEND, REPLACE with separators.
- **Done when**:
  - Prompt output matches PRD behavior.

### Task 10.5: Output truncation tests
- **Intent**: Ensure guardrail output limit.
- **Work**:
  - Test truncation to `outputTruncateChars`.
  - Verify `... [truncated]` indicator is appended.
- **Done when**:
  - Truncated output length is `outputTruncateChars` plus indicator.
  - Indicator only present when truncation occurred.

### Task 10.6: Integration tests
- **Intent**: Verify end-to-end loop behavior.
- **Work**:
  - Create integration test with mock agent (echo script or similar).
  - Test full loop: agent → guardrail → completion detection.
  - Test max iterations exit code 1.
  - Test successful completion exit code 0.
  - Test guardrail failure feedback into next iteration.
- **Done when**:
  - Integration test exercises full loop with mocked commands.
  - Tests verify correct exit codes and prompt construction.

### Task 10.7: Prompt file validation tests
- **Intent**: Ensure prompt file errors are handled.
- **Work**:
  - Test missing prompt file returns exit code 2.
  - Test unreadable prompt file returns exit code 2.
- **Done when**:
  - Invalid prompt file scenarios exit with code 2.

### Task 10.8: Agent command validation tests
- **Intent**: Ensure missing agent command is caught.
- **Work**:
  - Test that missing `agent.command` in settings exits with code 2.
  - Test descriptive error message is printed.
- **Done when**:
  - Missing agent command produces exit code 2 and error message.

---

## 11. README Updates

### Task 11.1: Document usage and config
- **Intent**: Update documentation per PRD.
- **Work**:
  - Add description, prerequisites, install steps, usage examples,
    configuration reference, and architecture overview.
- **Done when**:
  - README contains all required sections listed in PRD.

