package stream

import (
	"path/filepath"
	"strings"
)

// NormalizeName extracts and normalizes the agent name from a command path.
// Handles full paths and Windows .exe suffix.
func NormalizeName(agentCommand string) string {
	name := strings.ToLower(filepath.Base(agentCommand))
	return strings.TrimSuffix(name, ".exe")
}

// Parser interface - each agent implements this
type Parser interface {
	// Parse decodes raw bytes and returns parsed events
	// Returns nil event for non-displayable messages
	// May return multiple events for a single message
	Parse(data []byte) ([]*Event, error)

	// Name returns the agent name for display
	Name() string
}

// ParserFor returns the appropriate parser for an agent command
// Returns nil if agent doesn't support structured output parsing
func ParserFor(agentCommand string) Parser {
	switch NormalizeName(agentCommand) {
	case "claude":
		return NewClaudeParser()
	case "codex":
		return NewCodexParser()
	case "amp":
		return NewAmpParser()
	default:
		return nil
	}
}

// OutputFlags returns agent-specific flags for structured output
// Returns nil if agent doesn't support structured output
func OutputFlags(agentCommand string) []string {
	switch NormalizeName(agentCommand) {
	case "claude":
		return []string{"--output-format", "stream-json", "--verbose"}
	case "amp":
		return []string{"--stream-json", "--dangerously-allow-all"}
	case "codex":
		return []string{"--json", "--full-auto"}
	default:
		return nil
	}
}

// TextModeFlags returns agent-specific flags for text output (no JSON streaming).
// Used for simple requests like commit messages where we just need text response.
// Returns nil if agent doesn't require special text mode flags.
func TextModeFlags(agentCommand string) []string {
	switch NormalizeName(agentCommand) {
	case "claude":
		return []string{"--output-format", "text"}
	case "codex":
		// Text mode: omit --json, keep --full-auto for autonomy
		return []string{"--full-auto"}
	default:
		return nil
	}
}
