# Changelog

## Unreleased

## 0.1.0 - 2025-01-06

### Added

- Initial release with core agent loop functionality
- Guardrail system for request validation
- SCM integration for git operations
- JSON streaming response parser

## 0.2.0 - 2025-01-07

### Added

- Added `--prompt/-p` flag to CLI and removed positional prompt message
- `ralph init` command to enable guided setup of `./ralph/settings.json`

### Fixed

- Error in handling agent response when generating a commit message for a change set

## 0.3.0 - 2025-01-07

### Added

- Added optional setting `includeIterationCountInPrompt` to inject current iteration, max iterations, and remaining iterations into agent's prompt
- Added support for `includeIterationCountInPrompt` to `ralph init`

## 0.4.0 - 2025-01-09

### Added

- Added support for codex
- Added support for Amp
- Added guardrail `hint` field to allow custom hints to be included in the agent prompt when a guardrail fails
- Improved completion response handling, `<response></response>` tags are no longer needed

## 0.5.0 - 2025-01-23

### Added

- Improved output of streaming json content from AI Agent to improve dev's visibility into what the agent is doing
- Added token counts (input, cached, output) to Completion line at the end of each Agent loop
- Updated docs and README to fix out of date information

### Fixed

- Error where error code would be output when the Ralph loop completed successfully
