package scm

import (
	"bytes"
	"context"
	"testing"

	"github.com/richclement/ralph-cli/internal/config"
)

func TestNewRunner(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
		SCM: &config.SCMConfig{
			Command: "git",
			Tasks:   []string{"commit"},
		},
	}

	runner := NewRunner(settings, true)

	if runner.Settings != settings {
		t.Error("Expected settings to be assigned")
	}
	if runner.AgentRunner == nil {
		t.Error("Expected AgentRunner to be initialized")
	}
	if runner.Verbose != true {
		t.Error("Expected Verbose to be true")
	}
}

func TestNewRunner_VerboseFalse(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
	}

	runner := NewRunner(settings, false)

	if runner.Verbose != false {
		t.Error("Expected Verbose to be false")
	}
}

func TestRunner_Log_VerboseEnabled(t *testing.T) {
	var stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
	}

	runner := NewRunner(settings, true)
	runner.Stderr = &stderr

	runner.log("test message %s", "arg")

	if stderr.Len() == 0 {
		t.Error("Expected output when verbose enabled")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("[ralph]")) {
		t.Error("Expected [ralph] prefix in verbose output")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("test message arg")) {
		t.Error("Expected message content in verbose output")
	}
}

func TestRunner_Log_VerboseDisabled(t *testing.T) {
	var stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
	}

	runner := NewRunner(settings, false)
	runner.Stderr = &stderr

	runner.log("test message")

	if stderr.Len() != 0 {
		t.Error("Expected no output when verbose disabled")
	}
}

func TestExtractCommitMessage(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "response tag",
			output: "Some output\n<response>Fix the bug in parser</response>\nMore output",
			want:   "Fix the bug in parser",
		},
		{
			name:   "response tag with whitespace",
			output: "<response>  Add feature  </response>",
			want:   "Add feature",
		},
		{
			name:   "first non-empty line",
			output: "\n\nUpdate dependencies\nMore details here",
			want:   "Update dependencies",
		},
		{
			name:   "single line",
			output: "Simple commit message",
			want:   "Simple commit message",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
		{
			name:   "only whitespace",
			output: "  \n  \n  ",
			want:   "",
		},
		{
			name:   "response tag preferred over first line",
			output: "Agent thinking...\nAnalysis:\n<response>Refactor module</response>",
			want:   "Refactor module",
		},
		{
			name:   "multiline response tag content",
			output: "<response>Add feature\nwith details</response>",
			want:   "Add feature\nwith details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommitMessage(tt.output)
			if got != tt.want {
				t.Errorf("extractCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunner_Run_NoSCMConfig(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
		SCM: nil, // No SCM config
	}

	runner := NewRunner(settings, false)
	err := runner.Run(context.Background(), 1)

	if err != nil {
		t.Errorf("Expected nil error when SCM is nil, got %v", err)
	}
}

func TestRunner_Run_EmptyTasks(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
		SCM: &config.SCMConfig{
			Command: "git",
			Tasks:   []string{}, // Empty tasks
		},
	}

	runner := NewRunner(settings, false)
	err := runner.Run(context.Background(), 1)

	if err != nil {
		t.Errorf("Expected nil error when tasks are empty, got %v", err)
	}
}
