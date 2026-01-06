package stream

import (
	"encoding/json"
	"fmt"
	"time"
)

// Claude stream-json raw message types
type claudeRawMessage struct {
	Type      string         `json:"type"`
	Subtype   string         `json:"subtype,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Message   *claudeMessage `json:"message,omitempty"`
	Result    string         `json:"result,omitempty"`
	Cost      float64        `json:"total_cost_usd,omitempty"`
}

type claudeMessage struct {
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Type      string                 `json:"type"`
	ID        string                 `json:"id,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Text      string                 `json:"text,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	IsError   bool                   `json:"is_error,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// ClaudeParser implements Parser for Claude Code stream-json
type ClaudeParser struct {
	cumulativeCost float64
	activeTools    map[string]time.Time // toolID -> start time
}

func NewClaudeParser() *ClaudeParser {
	return &ClaudeParser{
		activeTools: make(map[string]time.Time),
	}
}

func (p *ClaudeParser) Name() string {
	return "claude"
}

func (p *ClaudeParser) Parse(data []byte) ([]*Event, error) {
	var raw claudeRawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	now := time.Now()

	switch raw.Type {
	case "assistant":
		return p.parseAssistant(&raw, now)
	case "user":
		return p.parseToolResult(&raw, now)
	case "result":
		costDelta := raw.Cost - p.cumulativeCost
		p.cumulativeCost = raw.Cost
		return []*Event{{
			Type:       EventResult,
			Timestamp:  now,
			Result:     raw.Result,
			Cost:       raw.Cost,
			CostDelta:  costDelta,
			IsComplete: true,
		}}, nil
	case "system":
		// Could extract session info, tools list for verbose mode
		return []*Event{{
			Type:      EventProgress,
			Timestamp: now,
			Text:      fmt.Sprintf("session: %s", raw.SessionID),
		}}, nil
	default:
		// Log unknown types for debugging, don't fail
		return []*Event{{
			Type:      EventUnknown,
			Timestamp: now,
			Text:      fmt.Sprintf("unknown message type: %s", raw.Type),
		}}, nil
	}
}

func (p *ClaudeParser) parseAssistant(raw *claudeRawMessage, now time.Time) ([]*Event, error) {
	if raw.Message == nil {
		return nil, nil
	}

	var events []*Event
	for _, c := range raw.Message.Content {
		switch c.Type {
		case "tool_use":
			p.activeTools[c.ID] = now
			events = append(events, &Event{
				Type:      EventToolStart,
				Timestamp: now,
				ToolName:  c.Name,
				ToolID:    c.ID,
				ToolInput: extractToolInput(c.Name, c.Input),
			})
		case "text":
			if c.Text != "" {
				events = append(events, &Event{
					Type:      EventText,
					Timestamp: now,
					Text:      c.Text,
				})
			}
		}
	}
	return events, nil
}

func (p *ClaudeParser) parseToolResult(raw *claudeRawMessage, now time.Time) ([]*Event, error) {
	if raw.Message == nil {
		return nil, nil
	}

	var events []*Event
	for _, c := range raw.Message.Content {
		if c.Type != "tool_result" {
			continue
		}

		event := &Event{
			Type:      EventToolEnd,
			Timestamp: now,
			ToolID:    c.ToolUseID,
		}

		// Calculate duration if we tracked the start
		if startTime, ok := p.activeTools[c.ToolUseID]; ok {
			delete(p.activeTools, c.ToolUseID)
			_ = now.Sub(startTime) // Duration available for future use
		}

		if c.IsError {
			event.ToolError = c.Error
			if event.ToolError == "" {
				event.ToolError = c.Content // Sometimes error is in content
			}
		} else {
			event.ToolOutput = truncate(c.Content, 100)
		}

		events = append(events, event)
	}
	return events, nil
}

// extractToolInput summarizes tool input for display
func extractToolInput(name string, input map[string]interface{}) string {
	switch name {
	case "Read":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Write", "Edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			return truncate(cmd, 60)
		}
		if desc, ok := input["description"].(string); ok {
			return truncate(desc, 60)
		}
	case "Glob", "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			return truncate(pattern, 40)
		}
	case "Task":
		if desc, ok := input["description"].(string); ok {
			return truncate(desc, 40)
		}
	case "WebFetch", "WebSearch":
		if url, ok := input["url"].(string); ok {
			return truncate(url, 50)
		}
		if query, ok := input["query"].(string); ok {
			return truncate(query, 50)
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
