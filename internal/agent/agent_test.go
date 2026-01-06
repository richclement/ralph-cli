package agent

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/richclement/ralph-cli/internal/config"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "claude",
			want:  "claude",
		},
		{
			name:  "string with spaces",
			input: "my command",
			want:  "'my command'",
		},
		{
			name:  "string with single quote",
			input: "it's",
			want:  "'it'\"'\"'s'",
		},
		{
			name:  "string with special chars",
			input: "test$var",
			want:  "'test$var'",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "path with spaces",
			input: "/path/to/my agent",
			want:  "'/path/to/my agent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildArgs_Claude(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
			Flags:   []string{"--model", "opus"},
		},
	}
	r := NewRunner(settings)

	args, promptFile := r.buildArgs("test prompt", 1)

	// Should have -p flag for claude
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected first arg to be -p, got %v", args)
	}

	// Should include user flags
	if len(args) < 3 || args[1] != "--model" || args[2] != "opus" {
		t.Errorf("Expected user flags, got %v", args)
	}

	// Should include prompt
	if len(args) < 4 || args[3] != "test prompt" {
		t.Errorf("Expected prompt as last arg, got %v", args)
	}

	// No prompt file for claude
	if promptFile != "" {
		t.Errorf("Expected no prompt file for claude, got %q", promptFile)
	}
}

func TestBuildArgs_Amp(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "amp",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1)

	// Should have -x flag for amp
	if len(args) < 1 || args[0] != "-x" {
		t.Errorf("Expected first arg to be -x, got %v", args)
	}

	// Should include prompt
	if len(args) < 2 || args[1] != "test prompt" {
		t.Errorf("Expected prompt as last arg, got %v", args)
	}
}

func TestBuildArgs_UnknownAgent(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "custom-agent",
			Flags:   []string{"--custom-flag"},
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1)

	// Should NOT have any inferred flag
	if len(args) < 1 || args[0] == "-p" || args[0] == "-x" || args[0] == "e" {
		t.Errorf("Should not have inferred flag for unknown agent, got %v", args)
	}

	// Should include user flags
	if args[0] != "--custom-flag" {
		t.Errorf("Expected user flag first, got %v", args)
	}

	// Should include prompt
	if args[1] != "test prompt" {
		t.Errorf("Expected prompt, got %v", args)
	}
}

func TestBuildArgs_PathWithExtension(t *testing.T) {
	// Test that Windows .exe extension is handled
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude.exe",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test", 1)

	// Should still detect claude and add -p
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected -p flag for claude.exe, got %v", args)
	}
}

func TestBuildArgs_FullPath(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "/usr/local/bin/claude",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test", 1)

	// Should extract basename and detect claude
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected -p flag for /usr/local/bin/claude, got %v", args)
	}
}

func TestNewRunner(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
			Flags:   []string{"--model", "opus"},
		},
		StreamAgentOutput: true,
	}

	runner := NewRunner(settings)

	if runner.Settings != settings {
		t.Error("Expected settings to be assigned")
	}
	if runner.Stdout != os.Stdout {
		t.Error("Expected Stdout to default to os.Stdout")
	}
	if runner.Stderr != os.Stderr {
		t.Error("Expected Stderr to default to os.Stderr")
	}
	if runner.Verbose != false {
		t.Error("Expected Verbose to default to false")
	}
}

func TestRunShell_Success(t *testing.T) {
	var stdout, stderr bytes.Buffer

	output, err := RunShell(context.Background(), "echo hello", false, &stdout, &stderr)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if output != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", output)
	}
}

func TestRunShell_WithStream(t *testing.T) {
	var stdout, stderr bytes.Buffer

	output, err := RunShell(context.Background(), "echo hello", true, &stdout, &stderr)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if output != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", output)
	}
	// When streaming, output should also go to stdout
	if stdout.String() != "hello\n" {
		t.Errorf("Expected stdout to contain 'hello\\n', got %q", stdout.String())
	}
}

func TestRunShell_CommandFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer

	_, err := RunShell(context.Background(), "exit 1", false, &stdout, &stderr)

	if err == nil {
		t.Error("Expected error for failed command")
	}
}

func TestRunShell_CancelledContext(t *testing.T) {
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := RunShell(ctx, "sleep 10", false, &stdout, &stderr)

	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestBuildArgs_Codex(t *testing.T) {
	// Create .ralph directory for prompt file
	if err := os.MkdirAll(RalphDir, 0755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
		},
	}
	r := NewRunner(settings)

	args, promptFile := r.buildArgs("test prompt", 1)

	// Should have "e" subcommand for codex
	if len(args) < 1 || args[0] != "e" {
		t.Errorf("Expected first arg to be 'e', got %v", args)
	}

	// Should have prompt file
	if promptFile == "" {
		t.Error("Expected prompt file for codex")
	}

	// Prompt file should be in args
	if len(args) < 2 || args[1] != promptFile {
		t.Errorf("Expected prompt file as second arg, got %v", args)
	}
}
