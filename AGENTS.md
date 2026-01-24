# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ralph is a deterministic outer loop that repeatedly runs a CLI LLM agent (claude, codex, amp) until it returns a completion response. Enforces guardrails (build/lint/test) between iterations and optionally runs SCM commands on success.

## Build & Development Commands

```bash
make build      # Build binary to ./ralph
make test       # Run all tests
make lint       # Run golangci-lint
make fmt        # Format code
make all        # fmt + lint + test + build
make install    # Install to $GOPATH/bin
```

Run a single test:
```bash
go test -v ./internal/config -run TestLoadWithLocal
```

## Architecture

Review @docs/ARCHITECTURE.md for detailed write up.

Entry point: `cmd/ralph/main.go` - CLI parsing (kong), signal handling, orchestrates loop + SCM

Internal packages:
- `config/` - Settings structs, JSON loading, deep merge of base/local configs, CLI override application
- `loop/` - Main iteration loop, completion detection via JSON result extraction
- `agent/` - Agent command execution with auto-inferred non-REPL flags (-p for claude, e for codex, -x for amp)
- `guardrail/` - Guardrail execution, fail actions (APPEND/PREPEND/REPLACE), output truncation, log file generation
- `scm/` - SCM task runner; commit task prompts agent for commit message
- `stream/` - Streaming JSON parsers (Claude, Codex, Amp), event model, formatter with tool correlation and token stats

## Configuration

Settings loaded from `.ralph/settings.json` with optional `.ralph/settings.local.json` overlay. CLI flags take highest priority.

Deep merge: scalars override, arrays replace, objects merge recursively.

Required: `agent.command` must be set in settings.

## Completion Detection

Completion is detected from the JSON result in stream-json mode. Agent output matches if result ends with `completionResponse` (case-insensitive).

Default `completionResponse` is `DONE`. Completion only checked after all guardrails pass.

## Exit Codes

- 0: Success (completion matched)
- 1: Max iterations reached
- 2: Config/validation error
- 130: Signal interrupt
