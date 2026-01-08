package stream

import (
	"testing"
)

func TestParserFor(t *testing.T) {
	tests := []struct {
		command  string
		wantName string
		wantNil  bool
	}{
		{"claude", "claude", false},
		{"/usr/local/bin/claude", "claude", false},
		{"claude.exe", "claude", false},
		{"codex", "codex", false},
		{"amp", "amp", false},
		{"unknown-agent", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			p := ParserFor(tt.command)
			if tt.wantNil {
				if p != nil {
					t.Errorf("ParserFor(%q) = %v, want nil", tt.command, p)
				}
				return
			}
			if p == nil {
				t.Fatalf("ParserFor(%q) = nil, want parser", tt.command)
			}
			if got := p.Name(); got != tt.wantName {
				t.Errorf("parser.Name() = %q, want %q", got, tt.wantName)
			}
		})
	}
}

func TestOutputFlags(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"claude", []string{"--output-format", "stream-json", "--verbose"}},
		{"/path/to/claude", []string{"--output-format", "stream-json", "--verbose"}},
		{"amp", []string{"--stream-json", "--dangerously-allow-all"}},
		{"/usr/local/bin/amp", []string{"--stream-json", "--dangerously-allow-all"}},
		{"amp.exe", []string{"--stream-json", "--dangerously-allow-all"}},
		{"codex", []string{"--json", "--full-auto"}},
		{"/usr/local/bin/codex", []string{"--json", "--full-auto"}},
		{"codex.exe", []string{"--json", "--full-auto"}},
		{"unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := OutputFlags(tt.command)
			if len(got) != len(tt.want) {
				t.Errorf("OutputFlags(%q) = %v, want %v", tt.command, got, tt.want)
				return
			}
			for i, flag := range got {
				if flag != tt.want[i] {
					t.Errorf("OutputFlags(%q)[%d] = %q, want %q", tt.command, i, flag, tt.want[i])
				}
			}
		})
	}
}

func TestTextModeFlags(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"claude", []string{"--output-format", "text"}},
		{"/path/to/claude", []string{"--output-format", "text"}},
		{"codex", []string{"--full-auto"}},
		{"/usr/local/bin/codex", []string{"--full-auto"}},
		{"codex.exe", []string{"--full-auto"}},
		{"amp", []string{"--dangerously-allow-all"}},
		{"/usr/local/bin/amp", []string{"--dangerously-allow-all"}},
		{"amp.exe", []string{"--dangerously-allow-all"}},
		{"unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := TextModeFlags(tt.command)
			if len(got) != len(tt.want) {
				t.Errorf("TextModeFlags(%q) = %v, want %v", tt.command, got, tt.want)
				return
			}
			for i, flag := range got {
				if flag != tt.want[i] {
					t.Errorf("TextModeFlags(%q)[%d] = %q, want %q", tt.command, i, flag, tt.want[i])
				}
			}
		})
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventToolStart, "ToolStart"},
		{EventToolEnd, "ToolEnd"},
		{EventText, "Text"},
		{EventResult, "Result"},
		{EventProgress, "Progress"},
		{EventUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.want {
				t.Errorf("EventType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEventIsError(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		e := &Event{ToolError: "some error"}
		if !e.IsError() {
			t.Error("expected IsError() = true")
		}
	})

	t.Run("without error", func(t *testing.T) {
		e := &Event{}
		if e.IsError() {
			t.Error("expected IsError() = false")
		}
	})
}

func TestOutputCaptureFor(t *testing.T) {
	tests := []struct {
		command  string
		baseDir  string
		wantNil  bool
		wantFile string
	}{
		{"codex", ".ralph", false, ".ralph/codex_output.txt"},
		{"/usr/local/bin/codex", ".ralph", false, ".ralph/codex_output.txt"},
		{"codex.exe", "/tmp/test", false, "/tmp/test/codex_output.txt"},
		{"claude", ".ralph", true, ""},
		{"amp", ".ralph", true, ""},
		{"unknown", ".ralph", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := OutputCaptureFor(tt.command, tt.baseDir)
			if tt.wantNil {
				if got != nil {
					t.Errorf("OutputCaptureFor(%q, %q) = %v, want nil", tt.command, tt.baseDir, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("OutputCaptureFor(%q, %q) = nil, want OutputCapture", tt.command, tt.baseDir)
			}
			if got.File != tt.wantFile {
				t.Errorf("OutputCaptureFor(%q, %q).File = %q, want %q", tt.command, tt.baseDir, got.File, tt.wantFile)
			}
		})
	}
}
