package stream

import (
	"path/filepath"
	"strings"
)

// Agent name constants for supported CLI agents.
const (
	AgentClaude = "claude"
	AgentCodex  = "codex"
	AgentAmp    = "amp"
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
	case AgentClaude:
		return NewClaudeParser()
	case AgentCodex:
		return NewCodexParser()
	case AgentAmp:
		return NewAmpParser()
	default:
		return nil
	}
}

// OutputFlags returns agent-specific flags for structured output
// Returns nil if agent doesn't support structured output
func OutputFlags(agentCommand string) []string {
	switch NormalizeName(agentCommand) {
	case AgentClaude:
		return []string{"--output-format", "stream-json", "--verbose"}
	case AgentAmp:
		return []string{"--stream-json", "--dangerously-allow-all"}
	case AgentCodex:
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
	case AgentClaude:
		return []string{"--output-format", "text"}
	case AgentCodex:
		// Text mode: omit --json, keep --full-auto for autonomy
		return []string{"--full-auto"}
	case AgentAmp:
		// Text mode: omit --stream-json, keep autonomy flag
		return []string{"--dangerously-allow-all"}
	default:
		return nil
	}
}

// OutputCapture holds configuration for capturing agent output to a file.
// Some agents (like Codex) write their output to a file instead of stdout.
type OutputCapture struct {
	// File is the path where the agent will write output.
	File string
}

// OutputCaptureFor returns output capture configuration for an agent.
// Returns nil if the agent writes to stdout normally.
func OutputCaptureFor(agentCommand string, baseDir string) *OutputCapture {
	switch NormalizeName(agentCommand) {
	case AgentCodex:
		return &OutputCapture{
			File: filepath.Join(baseDir, "codex_output.txt"),
		}
	default:
		return nil
	}
}
