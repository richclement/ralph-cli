package stream

import (
	"testing"
)

func TestAmpParser_Name(t *testing.T) {
	p := NewAmpParser()
	if got := p.Name(); got != "amp" {
		t.Errorf("Name() = %q, want %q", got, "amp")
	}
}

func TestAmpParser_ParseSystemInit(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "system", "subtype": "init", "session_id": "sess-123", "tools": ["read", "write"]}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventProgress {
		t.Errorf("Type = %v, want EventProgress", e.Type)
	}
	if e.Text != "session: sess-123" {
		t.Errorf("Text = %q, want %q", e.Text, "session: sess-123")
	}
}

func TestAmpParser_ParseAssistantText(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "assistant", "message": {"content": [{"type": "text", "text": "Hello world"}]}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventText {
		t.Errorf("Type = %v, want EventText", e.Type)
	}
	if e.Text != "Hello world" {
		t.Errorf("Text = %q, want %q", e.Text, "Hello world")
	}
}

func TestAmpParser_ParseAssistantToolUse(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "assistant", "message": {"content": [{"type": "tool_use", "id": "tool-1", "name": "read_file", "input": {"path": "/tmp/test.txt"}}]}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventToolStart {
		t.Errorf("Type = %v, want EventToolStart", e.Type)
	}
	if e.ToolName != "read_file" {
		t.Errorf("ToolName = %q, want %q", e.ToolName, "read_file")
	}
	if e.ToolID != "tool-1" {
		t.Errorf("ToolID = %q, want %q", e.ToolID, "tool-1")
	}
	if e.ToolInput != "/tmp/test.txt" {
		t.Errorf("ToolInput = %q, want %q", e.ToolInput, "/tmp/test.txt")
	}
}

func TestAmpParser_ParseUserToolResult(t *testing.T) {
	p := NewAmpParser()

	// First, simulate a tool start to track it
	toolStart := []byte(`{"type": "assistant", "message": {"content": [{"type": "tool_use", "id": "tool-1", "name": "read_file", "input": {}}]}}`)
	_, _ = p.Parse(toolStart)

	// Now parse the tool result
	data := []byte(`{"type": "user", "message": {"content": [{"type": "tool_result", "tool_use_id": "tool-1", "content": "file contents here"}]}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventToolEnd {
		t.Errorf("Type = %v, want EventToolEnd", e.Type)
	}
	if e.ToolID != "tool-1" {
		t.Errorf("ToolID = %q, want %q", e.ToolID, "tool-1")
	}
	if e.ToolOutput != "file contents here" {
		t.Errorf("ToolOutput = %q, want %q", e.ToolOutput, "file contents here")
	}
}

func TestAmpParser_ParseUserToolResultError(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "user", "message": {"content": [{"type": "tool_result", "tool_use_id": "tool-2", "content": "permission denied", "is_error": true}]}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventToolEnd {
		t.Errorf("Type = %v, want EventToolEnd", e.Type)
	}
	if e.ToolError != "permission denied" {
		t.Errorf("ToolError = %q, want %q", e.ToolError, "permission denied")
	}
}

func TestAmpParser_ParseResultSuccess(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "result", "subtype": "success", "result": "Task completed successfully", "duration_ms": 1234, "num_turns": 3, "usage": {"input_tokens": 100, "output_tokens": 50}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventResult {
		t.Errorf("Type = %v, want EventResult", e.Type)
	}
	if !e.IsComplete {
		t.Error("IsComplete = false, want true")
	}
	if e.Result != "Task completed successfully" {
		t.Errorf("Result = %q, want %q", e.Result, "Task completed successfully")
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", e.InputTokens)
	}
	if e.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", e.OutputTokens)
	}
}

func TestAmpParser_ParseResultWithCache(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "result", "subtype": "success", "result": "done", "usage": {"input_tokens": 1000, "output_tokens": 500, "cache_creation_input_tokens": 100, "cache_read_input_tokens": 400}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	e := events[0]
	if e.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", e.InputTokens)
	}
	if e.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", e.OutputTokens)
	}
	if e.CacheReadTokens != 400 {
		t.Errorf("CacheReadTokens = %d, want 400", e.CacheReadTokens)
	}
	if e.CacheWriteTokens != 100 {
		t.Errorf("CacheWriteTokens = %d, want 100", e.CacheWriteTokens)
	}
}

func TestAmpParser_ParseResultError(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "result", "subtype": "error_during_execution", "error": "Something went wrong", "is_error": true}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventResult {
		t.Errorf("Type = %v, want EventResult", e.Type)
	}
	if !e.IsComplete {
		t.Error("IsComplete = false, want true")
	}
	if e.Result != "Something went wrong" {
		t.Errorf("Result = %q, want %q", e.Result, "Something went wrong")
	}
	if e.ToolError != "Something went wrong" {
		t.Errorf("ToolError = %q, want %q", e.ToolError, "Something went wrong")
	}
}

func TestAmpParser_ParseUnknownType(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "unknown_type"}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Parse() returned %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventUnknown {
		t.Errorf("Type = %v, want EventUnknown", e.Type)
	}
}

func TestAmpParser_ParseInvalidJSON(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`not valid json`)

	_, err := p.Parse(data)
	if err == nil {
		t.Error("Parse() expected error for invalid JSON")
	}
}

func TestAmpParser_ParseNilMessage(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "assistant"}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Parse() returned %d events, want 0", len(events))
	}
}

func TestAmpParser_MultipleContent(t *testing.T) {
	p := NewAmpParser()
	data := []byte(`{"type": "assistant", "message": {"content": [{"type": "text", "text": "First"}, {"type": "text", "text": "Second"}]}}`)

	events, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Parse() returned %d events, want 2", len(events))
	}

	if events[0].Text != "First" {
		t.Errorf("events[0].Text = %q, want %q", events[0].Text, "First")
	}
	if events[1].Text != "Second" {
		t.Errorf("events[1].Text = %q, want %q", events[1].Text, "Second")
	}
}

func TestExtractAmpToolInput(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input any
		want  string
	}{
		{
			name:  "read_file with path",
			tool:  "read_file",
			input: map[string]any{"path": "/tmp/test.txt"},
			want:  "/tmp/test.txt",
		},
		{
			name:  "read_file with file_path",
			tool:  "read_file",
			input: map[string]any{"file_path": "/tmp/test.txt"},
			want:  "/tmp/test.txt",
		},
		{
			name:  "Read with file_path",
			tool:  "Read",
			input: map[string]any{"file_path": "/tmp/test.txt"},
			want:  "/tmp/test.txt",
		},
		{
			name:  "bash with command",
			tool:  "bash",
			input: map[string]any{"command": "ls -la"},
			want:  "ls -la",
		},
		{
			name:  "grep with pattern",
			tool:  "grep",
			input: map[string]any{"pattern": "TODO"},
			want:  "TODO",
		},
		{
			name:  "search with query",
			tool:  "search",
			input: map[string]any{"query": "find me"},
			want:  "find me",
		},
		{
			name:  "nil input",
			tool:  "read_file",
			input: nil,
			want:  "",
		},
		{
			name:  "non-map input",
			tool:  "read_file",
			input: "string input",
			want:  "",
		},
		{
			name:  "unknown tool",
			tool:  "unknown_tool",
			input: map[string]any{"something": "value"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAmpToolInput(tt.tool, tt.input)
			if got != tt.want {
				t.Errorf("extractAmpToolInput(%q, %v) = %q, want %q", tt.tool, tt.input, got, tt.want)
			}
		})
	}
}
