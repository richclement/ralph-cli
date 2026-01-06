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

func TestRunner_RunTask_GenericTask(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		SCM: &config.SCMConfig{
			Command: "echo",
			Tasks:   []string{"status"},
		},
	}

	runner := NewRunner(settings, false)
	runner.Stdout = &stdout
	runner.Stderr = &stderr
	runner.AgentRunner.Stdout = &stdout
	runner.AgentRunner.Stderr = &stderr

	// Run a generic task (not commit or push)
	err := runner.runTask(context.Background(), "status", 1)
	if err != nil {
		t.Errorf("Expected no error for generic task, got %v", err)
	}
}

func TestRunner_RunPush(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		SCM: &config.SCMConfig{
			Command: "echo",
			Tasks:   []string{"push"},
		},
	}

	runner := NewRunner(settings, false)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	err := runner.runPush(context.Background())
	if err != nil {
		t.Errorf("Expected no error for runPush, got %v", err)
	}
}

func TestRunner_Run_WithTasks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		SCM: &config.SCMConfig{
			Command: "echo",
			Tasks:   []string{"status"}, // Just status, not commit (which requires agent interaction)
		},
	}

	runner := NewRunner(settings, true) // verbose mode
	runner.Stdout = &stdout
	runner.Stderr = &stderr
	runner.AgentRunner.Stdout = &stdout
	runner.AgentRunner.Stderr = &stderr

	err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verbose output should contain task info
	if !bytes.Contains(stderr.Bytes(), []byte("[ralph]")) {
		t.Error("Expected verbose output")
	}
}

func TestRunner_RunCommit_NoValidMessage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			// Use a command that outputs only whitespace
			Command: "echo",
		},
		SCM: &config.SCMConfig{
			Command: "git",
			Tasks:   []string{"commit"},
		},
	}

	runner := NewRunner(settings, false)
	runner.Stdout = &stdout
	runner.Stderr = &stderr
	runner.AgentRunner.Stdout = &stdout
	runner.AgentRunner.Stderr = &stderr

	// Echo with empty prompt will output just a newline
	// The extractCommitMessage on this should return something, not empty
	// since echo adds a newline which becomes whitespace
	// Let's use a shell command that actually outputs nothing
	settings.Agent.Command = "true" // outputs nothing

	err := runner.runCommit(context.Background(), 1)
	// Should fail because true outputs nothing, so no valid commit message
	if err == nil {
		t.Error("Expected error for empty commit message")
	}
}

func TestRunner_RunTask_CancelledContext(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "sleep",
		},
		SCM: &config.SCMConfig{
			Command: "sleep",
			Tasks:   []string{"10"},
		},
	}

	runner := NewRunner(settings, false)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runner.runTask(ctx, "status", 1)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestRunner_Run_MultipleTasks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		SCM: &config.SCMConfig{
			Command: "echo",
			Tasks:   []string{"status", "diff"}, // Multiple generic tasks
		},
	}

	runner := NewRunner(settings, true)
	runner.Stdout = &stdout
	runner.Stderr = &stderr
	runner.AgentRunner.Stdout = &stdout
	runner.AgentRunner.Stderr = &stderr

	err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Errorf("Expected no error for multiple tasks, got %v", err)
	}
}

func TestRunner_Run_TaskFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		SCM: &config.SCMConfig{
			Command: "false", // Always fails
			Tasks:   []string{"something"},
		},
	}

	runner := NewRunner(settings, false)
	runner.Stdout = &stdout
	runner.Stderr = &stderr
	runner.AgentRunner.Stdout = &stdout
	runner.AgentRunner.Stderr = &stderr

	err := runner.Run(context.Background(), 1)
	if err == nil {
		t.Error("Expected error when task fails")
	}
}
