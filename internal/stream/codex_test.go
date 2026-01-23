package stream

import (
	"testing"
)

func TestCodexParser_ThreadStarted(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"thread.started","thread_id":"019b9f2e-3376-7870-ba4e-268994d47873"}`

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
	if e.Text != "thread: 019b9f2e-3376-7870-ba4e-268994d47873" {
		t.Errorf("Text = %q, want %q", e.Text, "thread: 019b9f2e-3376-7870-ba4e-268994d47873")
	}
}

func TestCodexParser_TurnStarted(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"turn.started"}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if events != nil {
		t.Errorf("expected nil events for turn.started, got %v", events)
	}
}

func TestCodexParser_ItemStartedCommand(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"/bin/zsh -lc 'cat .ralph/prompt_001.txt'","aggregated_output":"","exit_code":null,"status":"in_progress"}}`

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
	if e.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", e.ToolName, "Bash")
	}
	if e.ToolID != "item_1" {
		t.Errorf("ToolID = %q, want %q", e.ToolID, "item_1")
	}
	if e.ToolInput != "/bin/zsh -lc 'cat .ralph/prompt_001.txt'" {
		t.Errorf("ToolInput = %q, want %q", e.ToolInput, "/bin/zsh -lc 'cat .ralph/prompt_001.txt'")
	}
}

func TestCodexParser_ItemCompletedCommand(t *testing.T) {
	p := NewCodexParser()

	// First register the command start
	startInput := `{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"git status --short","status":"in_progress"}}`
	_, _ = p.Parse([]byte(startInput))

	// Then parse the completion
	input := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"git status --short","aggregated_output":" M internal/agent/agent.go\n M internal/agent/agent_test.go\n","exit_code":0,"status":"completed"}}`

	events, err := p.Parse([]byte(input))
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
	if e.ToolID != "item_1" {
		t.Errorf("ToolID = %q, want %q", e.ToolID, "item_1")
	}
	if e.ToolOutput != " M internal/agent/agent.go\n M internal/agent/agent_test.go\n" {
		t.Errorf("ToolOutput = %q, want %q", e.ToolOutput, " M internal/agent/agent.go\n M internal/agent/agent_test.go\n")
	}
	if e.ToolError != "" {
		t.Errorf("ToolError = %q, want empty", e.ToolError)
	}
}

func TestCodexParser_ItemCompletedCommandError(t *testing.T) {
	p := NewCodexParser()

	exitCode := 1
	input := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"cat nonexistent.txt","aggregated_output":"cat: nonexistent.txt: No such file or directory","exit_code":1,"status":"completed"}}`

	events, err := p.Parse([]byte(input))
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
	if !e.IsError() {
		t.Error("expected IsError() = true")
	}
	want := "exit code 1: cat: nonexistent.txt: No such file or directory"
	if e.ToolError != want {
		t.Errorf("ToolError = %q, want %q", e.ToolError, want)
	}
	_ = exitCode // silence unused warning
}

func TestCodexParser_ItemCompletedReasoning(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed","item":{"id":"item_2","type":"reasoning","text":"**Reading file contents**"}}`

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
	if e.Text != "**Reading file contents**" {
		t.Errorf("Text = %q, want %q", e.Text, "**Reading file contents**")
	}
}

func TestCodexParser_ItemCompletedAgentMessage(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed","item":{"id":"item_9","type":"agent_message","text":"No high-priority refactors found.\n\n**Findings**\n- None."}}`

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
	if e.Text != "No high-priority refactors found.\n\n**Findings**\n- None." {
		t.Errorf("Text = %q, want %q", e.Text, "No high-priority refactors found.\n\n**Findings**\n- None.")
	}
}

func TestCodexParser_TurnCompleted(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"turn.completed","usage":{"input_tokens":25030,"cached_input_tokens":21760,"output_tokens":673}}`

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
	if !e.IsComplete {
		t.Error("expected IsComplete = true")
	}
	if e.InputTokens != 25030 {
		t.Errorf("InputTokens = %d, want 25030", e.InputTokens)
	}
	if e.OutputTokens != 673 {
		t.Errorf("OutputTokens = %d, want 673", e.OutputTokens)
	}
	if e.CacheReadTokens != 21760 {
		t.Errorf("CacheReadTokens = %d, want 21760", e.CacheReadTokens)
	}
}

func TestCodexParser_TurnCompletedNoUsage(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"turn.completed"}`

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
	if !e.IsComplete {
		t.Error("expected IsComplete = true")
	}
}

func TestCodexParser_MalformedJSON(t *testing.T) {
	p := NewCodexParser()

	input := `{not valid json`

	_, err := p.Parse([]byte(input))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestCodexParser_UnknownType(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"future_type","data":"something"}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].Type != EventUnknown {
		t.Errorf("Type = %v, want EventUnknown", events[0].Type)
	}
}

func TestCodexParser_UnknownItemType(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed","item":{"id":"item_1","type":"new_item_type","text":"something"}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].Type != EventUnknown {
		t.Errorf("Type = %v, want EventUnknown", events[0].Type)
	}
}

func TestCodexParser_NilItem(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed"}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if events != nil {
		t.Errorf("expected nil events for nil item, got %v", events)
	}
}

func TestCodexParser_EmptyReasoningText(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed","item":{"id":"item_1","type":"reasoning","text":""}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if events != nil {
		t.Errorf("expected nil events for empty reasoning text, got %v", events)
	}
}

func TestCodexParser_EmptyAgentMessageText(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":""}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if events != nil {
		t.Errorf("expected nil events for empty agent_message text, got %v", events)
	}
}

func TestCodexParser_ItemStartedNonCommand(t *testing.T) {
	p := NewCodexParser()

	// item.started for non-command types should be skipped
	input := `{"type":"item.started","item":{"id":"item_1","type":"reasoning","text":"thinking..."}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if events != nil {
		t.Errorf("expected nil events for non-command item.started, got %v", events)
	}
}

func TestCodexParser_CommandWithZeroExitCode(t *testing.T) {
	p := NewCodexParser()

	input := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"echo hello","aggregated_output":"hello\n","exit_code":0,"status":"completed"}}`

	events, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	e := events[0]
	if e.IsError() {
		t.Error("expected IsError() = false for exit code 0")
	}
	if e.ToolOutput != "hello\n" {
		t.Errorf("ToolOutput = %q, want %q", e.ToolOutput, "hello\n")
	}
}
