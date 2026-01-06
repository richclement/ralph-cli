package stream

import (
	"path/filepath"
	"strings"
)

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
	name := strings.ToLower(filepath.Base(agentCommand))
	name = strings.TrimSuffix(name, ".exe")

	switch name {
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
	name := strings.ToLower(filepath.Base(agentCommand))
	name = strings.TrimSuffix(name, ".exe")

	switch name {
	case "claude":
		return []string{"--output-format", "stream-json", "--verbose"}
	// case "codex": return []string{"--output", "json"} // when known
	// case "amp": return []string{"--format", "json"} // when known
	default:
		return nil
	}
}
