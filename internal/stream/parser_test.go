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
		{"unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := OutputFlags(tt.command)
			if len(got) != len(tt.want) {
				t.Errorf("OutputFlags(%q) = %v, want %v", tt.command, got, tt.want)
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
