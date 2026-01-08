package stream

import (
	"encoding/json"
	"fmt"
	"time"
)

// Codex streaming JSON message types
type codexRawMessage struct {
	Type     string      `json:"type"`                // thread.started, turn.started, item.started, item.completed, turn.completed
	ThreadID string      `json:"thread_id,omitempty"` // for thread.started
	Item     *codexItem  `json:"item,omitempty"`      // for item.started, item.completed
	Usage    *codexUsage `json:"usage,omitempty"`     // for turn.completed
}

type codexItem struct {
	ID               string `json:"id"`
	Type             string `json:"type"`                        // reasoning, command_execution, agent_message
	Text             string `json:"text,omitempty"`              // for reasoning, agent_message
	Command          string `json:"command,omitempty"`           // for command_execution
	AggregatedOutput string `json:"aggregated_output,omitempty"` // for command_execution
	ExitCode         *int   `json:"exit_code,omitempty"`         // for command_execution
	Status           string `json:"status,omitempty"`            // in_progress, completed
}

type codexUsage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

// CodexParser implements Parser for OpenAI Codex CLI
type CodexParser struct {
	activeCommands map[string]time.Time // itemID -> start time
}

func NewCodexParser() *CodexParser {
	return &CodexParser{
		activeCommands: make(map[string]time.Time),
	}
}

func (p *CodexParser) Name() string {
	return "codex"
}

func (p *CodexParser) Parse(data []byte) ([]*Event, error) {
	var raw codexRawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	now := time.Now()

	switch raw.Type {
	case "thread.started":
		return []*Event{{
			Type:      EventProgress,
			Timestamp: now,
			Text:      fmt.Sprintf("thread: %s", raw.ThreadID),
		}}, nil

	case "turn.started":
		// Skip - no meaningful info to display
		return nil, nil

	case "item.started":
		return p.parseItemStarted(raw.Item, now)

	case "item.completed":
		return p.parseItemCompleted(raw.Item, now)

	case "turn.completed":
		return p.parseTurnCompleted(raw.Usage, now)

	default:
		return []*Event{{
			Type:      EventUnknown,
			Timestamp: now,
			Text:      fmt.Sprintf("unknown codex message type: %s", raw.Type),
		}}, nil
	}
}

func (p *CodexParser) parseItemStarted(item *codexItem, now time.Time) ([]*Event, error) {
	if item == nil {
		return nil, nil
	}

	if item.Type == "command_execution" {
		p.activeCommands[item.ID] = now
		return []*Event{{
			Type:      EventToolStart,
			Timestamp: now,
			ToolName:  "Bash",
			ToolID:    item.ID,
			ToolInput: item.Command,
		}}, nil
	}

	// Other item types don't have a "started" event we care about
	return nil, nil
}

func (p *CodexParser) parseItemCompleted(item *codexItem, now time.Time) ([]*Event, error) {
	if item == nil {
		return nil, nil
	}

	switch item.Type {
	case "command_execution":
		event := &Event{
			Type:      EventToolEnd,
			Timestamp: now,
			ToolID:    item.ID,
		}

		// Remove from active commands
		delete(p.activeCommands, item.ID)

		// Check for non-zero exit code
		if item.ExitCode != nil && *item.ExitCode != 0 {
			event.ToolError = fmt.Sprintf("exit code %d: %s", *item.ExitCode, item.AggregatedOutput)
		} else {
			event.ToolOutput = item.AggregatedOutput
		}

		return []*Event{event}, nil

	case "reasoning":
		if item.Text != "" {
			return []*Event{{
				Type:      EventText,
				Timestamp: now,
				Text:      item.Text,
			}}, nil
		}
		return nil, nil

	case "agent_message":
		if item.Text != "" {
			return []*Event{{
				Type:      EventText,
				Timestamp: now,
				Text:      item.Text,
			}}, nil
		}
		return nil, nil

	default:
		return []*Event{{
			Type:      EventUnknown,
			Timestamp: now,
			Text:      fmt.Sprintf("unknown item type: %s", item.Type),
		}}, nil
	}
}

func (p *CodexParser) parseTurnCompleted(usage *codexUsage, now time.Time) ([]*Event, error) {
	event := &Event{
		Type:       EventResult,
		Timestamp:  now,
		IsComplete: true,
	}

	if usage != nil {
		event.Result = fmt.Sprintf("tokens: %d in, %d out", usage.InputTokens, usage.OutputTokens)
	}

	return []*Event{event}, nil
}
