package stream

import (
	"testing"
)

func TestClaudeParser_ToolUse(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/path/to/file.go"}}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventToolStart {
		t.Errorf("Type = %v, want EventToolStart", e.Type)
	}
	if e.ToolName != "Read" {
		t.Errorf("ToolName = %q, want %q", e.ToolName, "Read")
	}
	if e.ToolInput != "/path/to/file.go" {
		t.Errorf("ToolInput = %q, want %q", e.ToolInput, "/path/to/file.go")
	}
}

func TestClaudeParser_ToolResult(t *testing.T) {
	p := NewClaudeParser()

	// First, register a tool start
	startInput := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Bash","input":{"command":"ls"}}]}}`
	_, _ = p.Parse([]byte(startInput))

	// Then parse the result
	resultInput := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"file1.go\nfile2.go"}]}}`

	events, err := p.Parse([]byte(resultInput))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventToolEnd {
		t.Errorf("Type = %v, want EventToolEnd", e.Type)
	}
	if e.ToolID != "tool_1" {
		t.Errorf("ToolID = %q, want %q", e.ToolID, "tool_1")
	}
}

func TestClaudeParser_ToolError(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"","is_error":true,"error":"Permission denied"}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if !e.IsError() {
		t.Error("expected IsError() = true")
	}
	if e.ToolError != "Permission denied" {
		t.Errorf("ToolError = %q, want %q", e.ToolError, "Permission denied")
	}
}

func TestClaudeParser_Text(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"I'll help you with that."}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventText {
		t.Errorf("Type = %v, want EventText", e.Type)
	}
	if e.Text != "I'll help you with that." {
		t.Errorf("Text = %q, want %q", e.Text, "I'll help you with that.")
	}
}

func TestClaudeParser_Result(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"result","subtype":"success","result":"done","total_cost_usd":0.0234}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventResult {
		t.Errorf("Type = %v, want EventResult", e.Type)
	}
	if e.Result != "done" {
		t.Errorf("Result = %q, want %q", e.Result, "done")
	}
	if e.Cost != 0.0234 {
		t.Errorf("Cost = %v, want %v", e.Cost, 0.0234)
	}
	if !e.IsComplete {
		t.Error("expected IsComplete = true")
	}
}

func TestClaudeParser_UnknownType(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"future_type","data":"something"}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should return unknown event, not fail
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].Type != EventUnknown {
		t.Errorf("Type = %v, want EventUnknown", events[0].Type)
	}
}

func TestClaudeParser_MalformedJSON(t *testing.T) {
	p := NewClaudeParser()

	input := `{not valid json`

	_, err := p.Parse([]byte(input))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestClaudeParser_SystemMessage(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"system","subtype":"init","session_id":"abc123","tools":["Read","Write"]}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventProgress {
		t.Errorf("Type = %v, want EventProgress", e.Type)
	}
	if e.Text != "session: abc123" {
		t.Errorf("Text = %q, want %q", e.Text, "session: abc123")
	}
}

func TestClaudeParser_CostDelta(t *testing.T) {
	p := NewClaudeParser()

	// First result
	input1 := `{"type":"result","result":"partial","total_cost_usd":0.01}`
	events1, _ := p.Parse([]byte(input1))
	if !floatEquals(events1[0].CostDelta, 0.01) {
		t.Errorf("first CostDelta = %v, want %v", events1[0].CostDelta, 0.01)
	}

	// Second result
	input2 := `{"type":"result","result":"done","total_cost_usd":0.03}`
	events2, _ := p.Parse([]byte(input2))
	// 0.03 - 0.01 = 0.02
	if !floatEquals(events2[0].CostDelta, 0.02) {
		t.Errorf("second CostDelta = %v, want %v", events2[0].CostDelta, 0.02)
	}
}

// floatEquals compares floats with tolerance for floating point precision
func floatEquals(a, b float64) bool {
	const epsilon = 1e-9
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func TestClaudeParser_ToolErrorInContent(t *testing.T) {
	p := NewClaudeParser()

	// Error message in content field, not error field
	input := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"File not found","is_error":true}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	e := events[0]
	if e.ToolError != "File not found" {
		t.Errorf("ToolError = %q, want %q", e.ToolError, "File not found")
	}
}

func TestExtractToolInput(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  string
	}{
		{"Read", map[string]interface{}{"file_path": "/path/to/file"}, "/path/to/file"},
		{"Edit", map[string]interface{}{"file_path": "/path/to/file"}, "/path/to/file"},
		{"Write", map[string]interface{}{"file_path": "/path/to/file"}, "/path/to/file"},
		{"Bash", map[string]interface{}{"command": "go test ./..."}, "go test ./..."},
		{"Bash", map[string]interface{}{"description": "Run tests"}, "Run tests"},
		{"Grep", map[string]interface{}{"pattern": "TODO"}, "TODO"},
		{"Glob", map[string]interface{}{"pattern": "*.go"}, "*.go"},
		{"Task", map[string]interface{}{"description": "Find files"}, "Find files"},
		{"WebFetch", map[string]interface{}{"url": "https://example.com"}, "https://example.com"},
		{"WebSearch", map[string]interface{}{"query": "golang tutorials"}, "golang tutorials"},
		{"Unknown", map[string]interface{}{"foo": "bar"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolInput(tt.name, tt.input)
			if got != tt.want {
				t.Errorf("extractToolInput(%q, %v) = %q, want %q", tt.name, tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestClaudeParser_MultipleContent(t *testing.T) {
	p := NewClaudeParser()

	// Message with both text and tool_use
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me read that file."},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/test.go"}}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	if events[0].Type != EventText {
		t.Errorf("first event Type = %v, want EventText", events[0].Type)
	}
	if events[1].Type != EventToolStart {
		t.Errorf("second event Type = %v, want EventToolStart", events[1].Type)
	}
}

func TestClaudeParser_NilMessage(t *testing.T) {
	p := NewClaudeParser()

	// Assistant message with no message field
	input := `{"type":"assistant"}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if events != nil {
		t.Errorf("expected nil events for nil message, got %v", events)
	}
}

func TestClaudeParser_TodoWrite(t *testing.T) {
	p := NewClaudeParser()

	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"TodoWrite","input":{"todos":[{"id":"1","content":"Read file","status":"completed"},{"id":"2","content":"Edit code","status":"in_progress"},{"id":"3","content":"Run tests","status":"pending","priority":"high"}]}}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventTodo {
		t.Errorf("Type = %v, want EventTodo", e.Type)
	}

	if len(e.TodoItems) != 3 {
		t.Fatalf("got %d todo items, want 3", len(e.TodoItems))
	}

	// Check first item
	if e.TodoItems[0].Content != "Read file" {
		t.Errorf("TodoItems[0].Content = %q, want %q", e.TodoItems[0].Content, "Read file")
	}
	if e.TodoItems[0].Status != "completed" {
		t.Errorf("TodoItems[0].Status = %q, want %q", e.TodoItems[0].Status, "completed")
	}

	// Check second item
	if e.TodoItems[1].Status != "in_progress" {
		t.Errorf("TodoItems[1].Status = %q, want %q", e.TodoItems[1].Status, "in_progress")
	}

	// Check third item with priority
	if e.TodoItems[2].Priority != "high" {
		t.Errorf("TodoItems[2].Priority = %q, want %q", e.TodoItems[2].Priority, "high")
	}
}

func TestClaudeParser_TodoWriteSubject(t *testing.T) {
	p := NewClaudeParser()

	// TodoWrite with "subject" field instead of "content"
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"TodoWrite","input":{"todos":[{"id":"1","subject":"Task with subject","status":"pending"}]}}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if e.Type != EventTodo {
		t.Errorf("Type = %v, want EventTodo", e.Type)
	}

	if e.TodoItems[0].Content != "Task with subject" {
		t.Errorf("TodoItems[0].Content = %q, want %q", e.TodoItems[0].Content, "Task with subject")
	}
}

func TestClaudeParser_TodoWriteEmpty(t *testing.T) {
	p := NewClaudeParser()

	// TodoWrite with empty todos
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"TodoWrite","input":{"todos":[]}}]}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Empty todos should produce no events
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty todos, got %d", len(events))
	}
}

func TestGetString(t *testing.T) {
	m := map[string]any{
		"str":    "value",
		"num":    42,
		"empty":  "",
		"nested": map[string]any{"key": "nested"},
	}

	tests := []struct {
		key  string
		want string
	}{
		{"str", "value"},
		{"num", ""},
		{"empty", ""},
		{"missing", ""},
		{"nested", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getString(m, tt.key)
			if got != tt.want {
				t.Errorf("getString(m, %q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
