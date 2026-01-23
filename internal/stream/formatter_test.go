package stream

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatter_ToolStart(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolID:    "t1",
		ToolInput: "/path/to/file.go",
	})

	got := buf.String()
	if !strings.Contains(got, "⏺") {
		t.Errorf("output missing tool start icon: %q", got)
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
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:     EventToolStart,
		ToolName: "Read",
		ToolID:   "t1",
	})

	got := buf.String()
	if !strings.Contains(got, "Read") {
		t.Errorf("output missing tool name: %q", got)
	}
	// Should not have parentheses when no input
	if strings.Contains(got, "(") {
		t.Errorf("output should not have parentheses for empty input: %q", got)
	}
}

func TestFormatter_ToolStartNoEmoji(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolID:    "t1",
		ToolInput: "/path/to/file.go",
	})

	got := buf.String()
	if strings.Contains(got, "⏺") {
		t.Errorf("output should not have emoji when UseEmoji=false: %q", got)
	}
	if !strings.Contains(got, "Read") {
		t.Errorf("output missing tool name: %q", got)
	}
}

func TestFormatter_Error(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolEnd,
		ToolID:    "t1",
		ToolError: "Permission denied",
	})

	got := buf.String()
	if !strings.Contains(got, "❌") {
		t.Errorf("output missing error icon: %q", got)
	}
	if !strings.Contains(got, "Error") {
		t.Errorf("output missing Error: %q", got)
	}
	if !strings.Contains(got, "Permission denied") {
		t.Errorf("output missing error message: %q", got)
	}
}

func TestFormatter_ToolEndNoError(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true, Verbose: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolID:     "t1",
		ToolOutput: "",
	})

	// Non-verbose mode with empty output should not show anything
	if buf.Len() > 0 {
		t.Errorf("expected no output for empty successful tool end in non-verbose mode, got: %q", buf.String())
	}
}

func TestFormatter_ToolEndWithOutput(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true, Verbose: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolID:     "t1",
		ToolOutput: "file contents here",
	})

	got := buf.String()
	if !strings.Contains(got, "✅") {
		t.Errorf("output missing success icon: %q", got)
	}
	if !strings.Contains(got, "Result") {
		t.Errorf("output missing Result: %q", got)
	}
	if !strings.Contains(got, "file contents here") {
		t.Errorf("output missing tool output: %q", got)
	}
}

func TestFormatter_ToolCorrelation(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	// Tool start
	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolID:    "tool_123",
		ToolInput: "/path/to/file.go",
	})

	// Tool end with same ID
	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolID:     "tool_123",
		ToolOutput: "package main",
	})

	got := buf.String()
	// Should show both start and result
	if !strings.Contains(got, "⏺") {
		t.Errorf("output missing tool start: %q", got)
	}
	if !strings.Contains(got, "✅") {
		t.Errorf("output missing tool result: %q", got)
	}
}

func TestFormatter_Result(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventResult,
		Cost: 0.0234,
	})

	got := buf.String()
	if !strings.Contains(got, "Complete") {
		t.Errorf("output missing Complete: %q", got)
	}
	if !strings.Contains(got, "$0.02") {
		t.Errorf("output missing cost: %q", got)
	}
	if !strings.Contains(got, "tools:") {
		t.Errorf("output missing tools count: %q", got)
	}
}

func TestFormatter_ResultWithStats(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	// Simulate some tool activity
	f.FormatEvent(&Event{Type: EventToolStart, ToolName: "Read", ToolID: "t1"})
	f.FormatEvent(&Event{Type: EventToolEnd, ToolID: "t1", ToolOutput: "ok"})
	f.FormatEvent(&Event{Type: EventToolStart, ToolName: "Edit", ToolID: "t2"})
	f.FormatEvent(&Event{Type: EventToolEnd, ToolID: "t2", ToolError: "failed"})

	buf.Reset()
	f.FormatEvent(&Event{
		Type: EventResult,
		Cost: 0.05,
	})

	got := buf.String()
	if !strings.Contains(got, "tools: 2") {
		t.Errorf("output should show tools: 2, got: %q", got)
	}
	if !strings.Contains(got, "errors: 1") {
		t.Errorf("output should show errors: 1, got: %q", got)
	}
}

func TestFormatter_Text(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowText: true, UseColor: false, UseEmoji: true}
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
	config := FormatterConfig{AgentName: "claude", ShowText: false, UseColor: false, UseEmoji: true}
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
	config := FormatterConfig{AgentName: "claude", ShowText: true, UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventText,
		Text: "First line\nSecond line\nThird line",
	})

	got := buf.String()
	if !strings.Contains(got, "Second line") {
		t.Errorf("output missing second line: %q", got)
	}
}

func TestFormatter_Todo(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventTodo,
		TodoItems: []TodoItem{
			{Content: "Read the file", Status: "completed"},
			{Content: "Edit the code", Status: "in_progress"},
			{Content: "Run tests", Status: "pending"},
		},
	})

	got := buf.String()
	if !strings.Contains(got, "Todo List") {
		t.Errorf("output missing Todo List header: %q", got)
	}
	if !strings.Contains(got, "Read the file") {
		t.Errorf("output missing completed task: %q", got)
	}
	if !strings.Contains(got, "Edit the code") {
		t.Errorf("output missing in_progress task: %q", got)
	}
	if !strings.Contains(got, "ACTIVE") {
		t.Errorf("output missing ACTIVE marker: %q", got)
	}
	if !strings.Contains(got, "Progress:") {
		t.Errorf("output missing progress summary: %q", got)
	}
}

func TestFormatter_TodoNoEmoji(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventTodo,
		TodoItems: []TodoItem{
			{Content: "Task 1", Status: "completed"},
			{Content: "Task 2", Status: "pending"},
		},
	})

	got := buf.String()
	if !strings.Contains(got, "[x]") {
		t.Errorf("output missing [x] for completed: %q", got)
	}
	if !strings.Contains(got, "[ ]") {
		t.Errorf("output missing [ ] for pending: %q", got)
	}
}

func TestFormatter_Progress(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", ShowProgress: true, UseColor: false, UseEmoji: true}
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
	config := FormatterConfig{AgentName: "claude", ShowProgress: false, UseColor: false, UseEmoji: true}
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
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(nil)

	if buf.Len() > 0 {
		t.Errorf("expected no output for nil event, got: %q", buf.String())
	}
}

func TestFormatter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
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
					ToolID:    "t1",
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

func TestFormatter_TruncatedOutput(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{
		AgentName:      "claude",
		UseColor:       false,
		UseEmoji:       true,
		MaxOutputLines: 2,
		MaxOutputChars: 50,
	}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolID:     "t1",
		ToolOutput: "line1\nline2\nline3\nline4\nline5",
	})

	got := buf.String()
	if !strings.Contains(got, "line1") {
		t.Errorf("output missing first line: %q", got)
	}
	if !strings.Contains(got, "more lines") {
		t.Errorf("output missing truncation indicator: %q", got)
	}
}

func TestFormatter_Timestamp(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{
		AgentName:     "claude",
		UseColor:      false,
		UseEmoji:      true,
		ShowTimestamp: true,
	}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolID:    "t1",
		ToolInput: "/path/to/file.go",
	})

	got := buf.String()
	// Should contain a timestamp like [HH:MM:SS]
	if !strings.Contains(got, "[") || !strings.Contains(got, ":") {
		t.Errorf("output missing timestamp: %q", got)
	}
}

func TestFormatter_ToolEndNoEmoji(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: false}
	f := NewFormatter(&buf, config)

	// Send tool start first for correlation
	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolID:    "t1",
		ToolInput: "/path/to/file.go",
	})
	buf.Reset()

	// Successful result
	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolID:     "t1",
		ToolOutput: "package main",
	})

	got := buf.String()
	if strings.Contains(got, "✅") {
		t.Errorf("output should not have emoji when UseEmoji=false: %q", got)
	}
	if !strings.Contains(got, "[OK]") {
		t.Errorf("output missing [OK] fallback: %q", got)
	}
	if !strings.Contains(got, "Result") {
		t.Errorf("output missing Result: %q", got)
	}
}

func TestFormatter_ToolEndErrorNoEmoji(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: false}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:      EventToolEnd,
		ToolID:    "t1",
		ToolError: "Permission denied",
	})

	got := buf.String()
	if strings.Contains(got, "❌") {
		t.Errorf("output should not have emoji when UseEmoji=false: %q", got)
	}
	if !strings.Contains(got, "[ERR]") {
		t.Errorf("output missing [ERR] fallback: %q", got)
	}
	if !strings.Contains(got, "Error") {
		t.Errorf("output missing Error: %q", got)
	}
}

func TestFormatter_ToolCorrelationShowsToolName(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	// Tool start
	f.FormatEvent(&Event{
		Type:      EventToolStart,
		ToolName:  "Read",
		ToolID:    "tool_123",
		ToolInput: "/path/to/file.go",
	})
	buf.Reset()

	// Tool end with same ID
	f.FormatEvent(&Event{
		Type:       EventToolEnd,
		ToolID:     "tool_123",
		ToolOutput: "package main",
	})

	got := buf.String()
	// Should show correlated tool name in result
	if !strings.Contains(got, "← Read") {
		t.Errorf("output missing correlated tool name: %q", got)
	}
}

func TestFormatter_TodoShowsAllCounters(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventTodo,
		TodoItems: []TodoItem{
			{Content: "Task 1", Status: "completed"},
			{Content: "Task 2", Status: "in_progress"},
			{Content: "Task 3", Status: "pending"},
			{Content: "Task 4", Status: "pending"},
		},
	})

	got := buf.String()
	// Should show all counters: 1 active, 2 pending
	if !strings.Contains(got, "1 active") {
		t.Errorf("output missing active count: %q", got)
	}
	if !strings.Contains(got, "2 pending") {
		t.Errorf("output missing pending count: %q", got)
	}
}

func TestCountLinesChars(t *testing.T) {
	tests := []struct {
		input     string
		wantLines int
		wantChars int
	}{
		{"", 0, 0},
		{"hello", 1, 5},
		{"line1\nline2", 2, 11},
		{"a\nb\nc", 3, 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lines, chars := countLinesChars(tt.input)
			if lines != tt.wantLines {
				t.Errorf("countLinesChars(%q) lines = %d, want %d", tt.input, lines, tt.wantLines)
			}
			if chars != tt.wantChars {
				t.Errorf("countLinesChars(%q) chars = %d, want %d", tt.input, chars, tt.wantChars)
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
	if !config.UseEmoji {
		t.Error("UseEmoji should default to true")
	}
	if config.MaxOutputLines != 3 {
		t.Errorf("MaxOutputLines = %d, want 3", config.MaxOutputLines)
	}
	if config.MaxOutputChars != 120 {
		t.Errorf("MaxOutputChars = %d, want 120", config.MaxOutputChars)
	}
}

func TestFormatter_ResultWithTokens(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:            EventResult,
		Cost:            5.76,
		InputTokens:     1234567,
		OutputTokens:    45000,
		CacheReadTokens: 850000,
	})

	got := buf.String()
	if !strings.Contains(got, "1.2M in") {
		t.Errorf("output missing formatted input tokens: %q", got)
	}
	if !strings.Contains(got, "850K cached") {
		t.Errorf("output missing cached tokens: %q", got)
	}
	if !strings.Contains(got, "45K out") {
		t.Errorf("output missing output tokens: %q", got)
	}
}

func TestFormatter_ResultNoCache(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type:         EventResult,
		Cost:         0.05,
		InputTokens:  25000,
		OutputTokens: 673,
	})

	got := buf.String()
	if !strings.Contains(got, "25K in") {
		t.Errorf("output missing input tokens: %q", got)
	}
	if strings.Contains(got, "cached") {
		t.Errorf("output should not have cached when cache is 0: %q", got)
	}
	if !strings.Contains(got, "673 out") {
		t.Errorf("output missing output tokens: %q", got)
	}
}

func TestFormatter_ResultNoTokens(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{AgentName: "claude", UseColor: false, UseEmoji: true}
	f := NewFormatter(&buf, config)

	f.FormatEvent(&Event{
		Type: EventResult,
		Cost: 0.05,
	})

	got := buf.String()
	if strings.Contains(got, "tokens:") {
		t.Errorf("output should not have tokens when both are 0: %q", got)
	}
}

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1K"},
		{1500, "2K"},
		{25000, "25K"},
		{999999, "1000K"},
		{1000000, "1.0M"},
		{1234567, "1.2M"},
		{2500000, "2.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatTokenCount(tt.input)
			if got != tt.want {
				t.Errorf("formatTokenCount(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{61 * time.Second, "1m1s"},
		{90 * time.Second, "1m30s"},
		{120 * time.Second, "2m"},
		{367 * time.Second, "6m7s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateInput(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"first\nsecond", 20, "first"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateInput(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateInput(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}
