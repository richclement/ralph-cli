package agent

import (
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
