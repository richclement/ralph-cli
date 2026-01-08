package stream

import (
	"encoding/json"
	"fmt"
	"time"
)

// Amp stream-json raw message types
type ampRawMessage struct {
	Type    string `json:"type"`    // system, assistant, user, result
	Subtype string `json:"subtype"` // init, success, error_during_execution
}

type ampSystemInit struct {
	ampRawMessage
	SessionID string   `json:"session_id"`
	Tools     []string `json:"tools"`
}

type ampAssistant struct {
	ampRawMessage
	Message *ampMessage `json:"message"`
}

type ampUser struct {
	ampRawMessage
	Message *ampMessage `json:"message"`
}

type ampMessage struct {
	Content []ampContent `json:"content"`
	Usage   *ampUsage    `json:"usage,omitempty"`
}

type ampContent struct {
	Type      string `json:"type"`         // text, tool_use, tool_result
	Text      string `json:"text"`         // for text type
	ID        string `json:"id"`           // for tool_use
	Name      string `json:"name"`         // for tool_use
	ToolUseID string `json:"tool_use_id"`  // for tool_result
	Content   string `json:"content"`      // for tool_result
	IsError   bool   `json:"is_error"`     // for tool_result
	Input     any    `json:"input"`        // for tool_use (varies)
}

type ampResult struct {
	ampRawMessage
	Result     string    `json:"result"`
	Error      string    `json:"error"`
	DurationMs int       `json:"duration_ms"`
	NumTurns   int       `json:"num_turns"`
	Usage      *ampUsage `json:"usage"`
	IsError    bool      `json:"is_error"`
}

type ampUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AmpParser implements Parser for Sourcegraph Amp
type AmpParser struct {
	activeTools map[string]time.Time // toolID -> start time
}

func NewAmpParser() *AmpParser {
	return &AmpParser{
		activeTools: make(map[string]time.Time),
	}
}

func (p *AmpParser) Name() string {
	return "amp"
}

func (p *AmpParser) Parse(data []byte) ([]*Event, error) {
	// First decode the base type to determine message kind
	var raw ampRawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	now := time.Now()

	switch raw.Type {
	case "system":
		return p.parseSystem(data, now)
	case "assistant":
		return p.parseAssistant(data, now)
	case "user":
		return p.parseUser(data, now)
	case "result":
		return p.parseResult(data, now)
	default:
		return []*Event{{
			Type:      EventUnknown,
			Timestamp: now,
			Text:      fmt.Sprintf("unknown amp message type: %s", raw.Type),
		}}, nil
	}
}

func (p *AmpParser) parseSystem(data []byte, now time.Time) ([]*Event, error) {
	var msg ampSystemInit
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse system: %w", err)
	}

	if msg.Subtype == "init" {
		return []*Event{{
			Type:      EventProgress,
			Timestamp: now,
			Text:      fmt.Sprintf("session: %s", msg.SessionID),
		}}, nil
	}

	return nil, nil
}

func (p *AmpParser) parseAssistant(data []byte, now time.Time) ([]*Event, error) {
	var msg ampAssistant
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse assistant: %w", err)
	}

	if msg.Message == nil {
		return nil, nil
	}

	var events []*Event
	for _, c := range msg.Message.Content {
		switch c.Type {
		case "text":
			if c.Text != "" {
				events = append(events, &Event{
					Type:      EventText,
					Timestamp: now,
					Text:      c.Text,
				})
			}
		case "tool_use":
			p.activeTools[c.ID] = now
			events = append(events, &Event{
				Type:      EventToolStart,
				Timestamp: now,
				ToolName:  c.Name,
				ToolID:    c.ID,
				ToolInput: extractAmpToolInput(c.Name, c.Input),
			})
		}
	}
	return events, nil
}

func (p *AmpParser) parseUser(data []byte, now time.Time) ([]*Event, error) {
	var msg ampUser
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	if msg.Message == nil {
		return nil, nil
	}

	var events []*Event
	for _, c := range msg.Message.Content {
		if c.Type != "tool_result" {
			continue
		}

		event := &Event{
			Type:      EventToolEnd,
			Timestamp: now,
			ToolID:    c.ToolUseID,
		}

		// Remove from active tools (no-op if not tracked)
		delete(p.activeTools, c.ToolUseID)

		if c.IsError {
			event.ToolError = c.Content
		} else {
			event.ToolOutput = c.Content
		}

		events = append(events, event)
	}
	return events, nil
}

func (p *AmpParser) parseResult(data []byte, now time.Time) ([]*Event, error) {
	var msg ampResult
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	event := &Event{
		Type:       EventResult,
		Timestamp:  now,
		IsComplete: true,
	}

	switch msg.Subtype {
	case "success":
		event.Result = msg.Result
	case "error_during_execution":
		event.Result = msg.Error
		event.ToolError = msg.Error
	default:
		// Handle other error subtypes similarly
		if msg.IsError {
			event.Result = msg.Error
			event.ToolError = msg.Error
		} else {
			event.Result = msg.Result
		}
	}

	return []*Event{event}, nil
}

// extractAmpToolInput summarizes tool input for display
func extractAmpToolInput(name string, input any) string {
	inputMap, ok := input.(map[string]any)
	if !ok {
		return ""
	}

	switch name {
	case "Read", "read_file":
		if path, ok := inputMap["file_path"].(string); ok {
			return path
		}
		if path, ok := inputMap["path"].(string); ok {
			return path
		}
	case "Write", "Edit", "write_file", "edit_file":
		if path, ok := inputMap["file_path"].(string); ok {
			return path
		}
		if path, ok := inputMap["path"].(string); ok {
			return path
		}
	case "Bash", "bash", "run_command":
		if cmd, ok := inputMap["command"].(string); ok {
			return cmd
		}
	case "Glob", "Grep", "glob", "grep", "search":
		if pattern, ok := inputMap["pattern"].(string); ok {
			return pattern
		}
		if query, ok := inputMap["query"].(string); ok {
			return query
		}
	}
	return ""
}
