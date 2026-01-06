package stream

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatter_ToolStart(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolInput: "/path/to/file.go",
	})

	got := buf.String()
	if !strings.Contains(got, "[claude]") {
		t.Errorf("output missing prefix: %q", got)
	}
	if !strings.Contains(got, "Read") {
		t.Errorf("output missing tool name: %q", got)
	}
	if !strings.Contains(got, "/path/to/file.go") {
		t.Errorf("output missing tool input: %q", got)
	}
}

func TestFormatter_ToolStartNoInput(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:     EventToolStart,
		ToolName: "Read",
	})

	got := buf.String()
	if !strings.Contains(got, "Read") {
		t.Errorf("output missing tool name: %q", got)
	}
	// Should not have ": " when no input
	if strings.Contains(got, ": ") {
		t.Errorf("output should not have colon for empty input: %q", got)
	}
}

func TestFormatter_Error(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolEnd,
		ToolError: "Permission denied",
	})

	got := buf.String()
	if !strings.Contains(got, "ERROR") {
		t.Errorf("output missing ERROR: %q", got)
	}
	if !strings.Contains(got, "Permission denied") {
		t.Errorf("output missing error message: %q", got)
	}
}

func TestFormatter_ToolEndNoError(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, Verbose: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolOutput: "some output",
	})

	// Non-verbose mode should not show output for successful tool
	if buf.Len() > 0 {
		t.Errorf("expected no output for non-error tool end in non-verbose mode, got: %q", buf.String())
	}
}

func TestFormatter_ToolEndVerbose(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, Verbose: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolOutput: "file contents here",
	})

	got := buf.String()
	if !strings.Contains(got, "->") {
		t.Errorf("verbose output missing arrow: %q", got)
	}
	if !strings.Contains(got, "file contents here") {
		t.Errorf("verbose output missing tool output: %q", got)
	}
}

func TestFormatter_Result(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventResult,
		Cost: 0.0234,
	})

	got := buf.String()
	if !strings.Contains(got, "Complete") {
		t.Errorf("output missing Complete: %q", got)
	}
	if !strings.Contains(got, "0.0234") {
		t.Errorf("output missing cost: %q", got)
	}
}

func TestFormatter_Text(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowText: true, UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventText,
		Text: "I'll help you with that.",
	})

	got := buf.String()
	if !strings.Contains(got, "I'll help you with that.") {
		t.Errorf("output missing text: %q", got)
	}
}

func TestFormatter_TextDisabled(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowText: false, UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventText,
		Text: "Some assistant text",
	})

	if buf.Len() > 0 {
		t.Errorf("expected no output when ShowText=false, got: %q", buf.String())
	}
}

func TestFormatter_TextMultiline(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowText: true, UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventText,
		Text: "First line\nSecond line\nThird line",
	})

	got := buf.String()
	// Should only show first line
	if strings.Contains(got, "Second") {
		t.Errorf("output should only contain first line: %q", got)
	}
}

func TestFormatter_Progress(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowProgress: true, UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventProgress,
		Text: "session: abc123",
	})

	got := buf.String()
	if !strings.Contains(got, "session: abc123") {
		t.Errorf("output missing progress text: %q", got)
	}
}

func TestFormatter_ProgressDisabled(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowProgress: false, UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventProgress,
		Text: "session: abc123",
	})

	if buf.Len() > 0 {
		t.Errorf("expected no output when ShowProgress=false, got: %q", buf.String())
	}
}

func TestFormatter_NilEvent(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(nil)

	if buf.Len() > 0 {
		t.Errorf("expected no output for nil event, got: %q", buf.String())
	}
}

func TestFormatter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false}
	f := NewFormatter(&buf, config)

	// Simulate concurrent writes
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				f.FormatEvent(&Event{
					Type:      EventToolStart,
					ToolName:  "Read",
					ToolInput: "/path",
				})
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify no panic - output may be interleaved but shouldn't crash
	lines := strings.Split(buf.String(), "\n")
	if len(lines) < 1000 {
		t.Errorf("expected ~1000 lines, got %d", len(lines))
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"first\nsecond", 20, "first"},
		{"this is a very long string that should be truncated", 20, "this is a very lo..."},
		{"multiline\nand long text that exceeds the maximum", 10, "multiline"}, // 9 chars, under limit
		{"", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := firstLine(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("firstLine(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestDefaultFormatterConfig(t *testing.T) {
	config := DefaultFormatterConfig("test-agent")

	if config.AgentName != "test-agent" {
		t.Errorf("AgentName = %q, want %q", config.AgentName, "test-agent")
	}
	if !config.ShowText {
		t.Error("ShowText should default to true")
	}
	if config.ShowProgress {
		t.Error("ShowProgress should default to false")
	}
	if config.Verbose {
		t.Error("Verbose should default to false")
	}
}
