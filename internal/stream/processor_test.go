package stream

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProcessor_Integration(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, ShowText: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)
	if proc == nil {
		t.Fatal("NewProcessor returned nil")
	}

	// Write sample Claude output
	messages := []string{
		`{"type":"system","subtype":"init","session_id":"test"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Let me help."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/test.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"package main"}]}}`,
		`{"type":"result","subtype":"success","result":"done","total_cost_usd":0.01}`,
	}

	for _, msg := range messages {
		_, err := proc.Write([]byte(msg + "\n"))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
	}

	// Close and wait
	if err := proc.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "Read") {
		t.Errorf("output missing tool name: %q", output)
	}
	if !strings.Contains(output, "/test.go") {
		t.Errorf("output missing file path: %q", output)
	}
	if !strings.Contains(output, "Complete") {
		t.Errorf("output missing completion: %q", output)
	}

	// Check stats
	events, errors := proc.Stats()
	if events < 3 {
		t.Errorf("expected at least 3 events, got %d", events)
	}
	if errors != 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}
}

func TestProcessor_MalformedJSON(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	// Write mix of valid and invalid JSON
	_, _ = proc.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Valid"}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{invalid json}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	_ = proc.Close()

	// Should still process valid messages
	output := buf.String()
	if !strings.Contains(output, "Complete") {
		t.Errorf("failed to recover from malformed JSON: %q", output)
	}

	_, errors := proc.Stats()
	if errors == 0 {
		t.Error("expected error count > 0 for malformed JSON")
	}
}

func TestProcessor_LastActivity(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	before := time.Now()
	time.Sleep(10 * time.Millisecond)

	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	time.Sleep(10 * time.Millisecond)
	_ = proc.Close()

	lastActivity := proc.LastActivity()
	if lastActivity.Before(before) {
		t.Errorf("LastActivity %v should be after %v", lastActivity, before)
	}
}

func TestProcessor_UnknownAgent(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "unknown", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("unknown-agent", formatter, nil, nil)
	if proc != nil {
		t.Error("expected nil processor for unknown agent")
	}
}

func TestProcessor_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	// Write a message in chunks (simulating streaming)
	msg := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`
	for i := 0; i < len(msg); i += 10 {
		end := i + 10
		if end > len(msg) {
			end = len(msg)
		}
		_, _ = proc.Write([]byte(msg[i:end]))
	}
	_, _ = proc.Write([]byte("\n"))

	// Write complete message
	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	_ = proc.Close()

	events, _ := proc.Stats()
	if events < 2 {
		t.Errorf("expected at least 2 events, got %d", events)
	}
}

func TestProcessor_EmptyWrite(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	// Write empty data
	n, err := proc.Write([]byte{})
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}

	_ = proc.Close()
}

func TestProcessor_DoubleClose(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	// Close twice should not panic
	_ = proc.Close()
	_ = proc.Close()
}

func TestProcessor_ToolStartEnd(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, Verbose: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	// Tool use followed by result
	_, _ = proc.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls"}}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"file1\nfile2"}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	_ = proc.Close()

	output := buf.String()
	if !strings.Contains(output, "Bash") {
		t.Errorf("output missing tool name: %q", output)
	}
	if !strings.Contains(output, "ls") {
		t.Errorf("output missing command: %q", output)
	}
}

func TestProcessor_ToolError(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, nil)

	// Tool use with error result
	_, _ = proc.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Edit","input":{"file_path":"/etc/passwd"}}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"","is_error":true,"error":"Permission denied"}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	_ = proc.Close()

	output := buf.String()
	// New format uses "Error" header with ❌ icon
	if !strings.Contains(output, "Error") {
		t.Errorf("output missing Error: %q", output)
	}
	if !strings.Contains(output, "❌") {
		t.Errorf("output missing error icon: %q", output)
	}
	if !strings.Contains(output, "Permission denied") {
		t.Errorf("output missing error message: %q", output)
	}
}

func TestProcessor_RawLog(t *testing.T) {
	var buf bytes.Buffer
	var rawLog bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("claude", formatter, nil, &rawLog)

	_, _ = proc.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

	_ = proc.Close()

	logOutput := rawLog.String()
	if !strings.Contains(logOutput, `"type":"assistant"`) {
		t.Errorf("raw log missing assistant line: %q", logOutput)
	}
	if !strings.Contains(logOutput, `"type":"result"`) {
		t.Errorf("raw log missing result line: %q", logOutput)
	}
}

func TestProcessor_CodexIntegration(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "codex", UseColor: false, ShowText: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("codex", formatter, nil, nil)
	if proc == nil {
		t.Fatal("NewProcessor returned nil for codex")
	}

	// Write sample Codex output
	messages := []string{
		`{"type":"thread.started","thread_id":"thread_abc123"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.started","item":{"id":"cmd_1","type":"command_execution","command":"ls -la"}}`,
		`{"type":"item.completed","item":{"id":"cmd_1","type":"command_execution","command":"ls -la","aggregated_output":"file1.txt\nfile2.txt","exit_code":0}}`,
		`{"type":"item.started","item":{"id":"msg_1","type":"agent_message"}}`,
		`{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"Listed the files successfully."}}`,
		`{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50,"cached_input_tokens":20}}`,
	}

	for _, msg := range messages {
		_, err := proc.Write([]byte(msg + "\n"))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "Bash") {
		t.Errorf("output missing tool name Bash: %q", output)
	}
	if !strings.Contains(output, "Complete") {
		t.Errorf("output missing completion: %q", output)
	}

	// Check stats
	events, errors := proc.Stats()
	if events < 3 {
		t.Errorf("expected at least 3 events, got %d", events)
	}
	if errors != 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}
}

func TestProcessor_CodexCommandError(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "codex", UseColor: false, UseEmoji: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("codex", formatter, nil, nil)

	// Write Codex output with command error
	exitCode := 1
	messages := []string{
		`{"type":"turn.started"}`,
		`{"type":"item.started","item":{"id":"cmd_1","type":"command_execution","command":"cat /nonexistent"}}`,
	}
	for _, msg := range messages {
		_, _ = proc.Write([]byte(msg + "\n"))
	}

	// Item completed with error - need to construct JSON manually for exit_code
	errorItem := `{"type":"item.completed","item":{"id":"cmd_1","type":"command_execution","command":"cat /nonexistent","aggregated_output":"No such file","exit_code":` + string(rune('0'+exitCode)) + `}}`
	_, _ = proc.Write([]byte(errorItem + "\n"))
	_, _ = proc.Write([]byte(`{"type":"turn.completed","usage":{"input_tokens":50,"output_tokens":25}}` + "\n"))

	_ = proc.Close()

	output := buf.String()
	// Should show error indication
	if !strings.Contains(output, "❌") && !strings.Contains(output, "Error") {
		t.Errorf("output missing error indicator: %q", output)
	}
}

func TestProcessor_AmpIntegration(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "amp", UseColor: false, ShowText: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("amp", formatter, nil, nil)
	if proc == nil {
		t.Fatal("NewProcessor returned nil for amp")
	}

	// Write sample Amp output
	messages := []string{
		`{"type":"system","subtype":"init","session_id":"sess_123","tools":["Read","Write","Bash"]}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"I'll help you with that."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/test.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"package main"}]}}`,
		`{"type":"result","subtype":"success","result":"Task completed successfully. DONE","duration_ms":1500,"num_turns":2}`,
	}

	for _, msg := range messages {
		_, err := proc.Write([]byte(msg + "\n"))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "Read") {
		t.Errorf("output missing tool name Read: %q", output)
	}
	if !strings.Contains(output, "/test.go") {
		t.Errorf("output missing file path: %q", output)
	}
	if !strings.Contains(output, "Complete") {
		t.Errorf("output missing completion: %q", output)
	}

	// Check stats
	events, errors := proc.Stats()
	if events < 3 {
		t.Errorf("expected at least 3 events, got %d", events)
	}
	if errors != 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}
}

func TestProcessor_AmpToolError(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "amp", UseColor: false, UseEmoji: true}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("amp", formatter, nil, nil)

	// Write Amp output with tool error
	_, _ = proc.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Write","input":{"file_path":"/etc/passwd"}}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"Permission denied","is_error":true}]}}` + "\n"))
	_, _ = proc.Write([]byte(`{"type":"result","subtype":"success","result":"Could not complete task"}` + "\n"))

	_ = proc.Close()

	output := buf.String()
	// Should show error indication
	if !strings.Contains(output, "❌") && !strings.Contains(output, "Error") {
		t.Errorf("output missing error indicator: %q", output)
	}
	if !strings.Contains(output, "Permission denied") {
		t.Errorf("output missing error message: %q", output)
	}
}

func TestProcessor_AmpErrorResult(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "amp", UseColor: false}
	formatter := NewFormatter(&buf, config)

	proc := NewProcessor("amp", formatter, nil, nil)

	// Write Amp output with error result
	_, _ = proc.Write([]byte(`{"type":"result","subtype":"error_during_execution","error":"Something went wrong","is_error":true}` + "\n"))

	_ = proc.Close()

	output := buf.String()
	// Should still show completion (even if it's an error completion)
	if !strings.Contains(output, "Complete") {
		t.Errorf("output missing completion for error result: %q", output)
	}
}
