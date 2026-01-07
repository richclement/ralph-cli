package main

import (
	"testing"
)

func TestCLI_Validate_MultiplePromptSources(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		promptFlag string
		promptFile string
	}{
		{"positional and file", "prompt", "", "file.txt"},
		{"flag and file", "", "prompt", "file.txt"},
		{"positional and flag", "prompt", "flagprompt", ""},
		{"all three", "prompt", "flagprompt", "file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := CLI{
				Prompt:     tt.prompt,
				PromptFlag: tt.promptFlag,
				PromptFile: tt.promptFile,
			}
			err := cli.Validate()
			if err == nil {
				t.Error("expected error when multiple prompt sources are set")
			}
			expected := "cannot specify multiple prompt sources (positional, --prompt, --prompt-file)"
			if err.Error() != expected {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

func TestCLI_Validate_NoPromptSource(t *testing.T) {
	cli := CLI{}

	err := cli.Validate()
	if err == nil {
		t.Error("expected error when no prompt source is set")
	}
	expected := "must specify prompt (positional arg, --prompt, or --prompt-file)"
	if err.Error() != expected {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCLI_Validate_OnlyPrompt(t *testing.T) {
	cli := CLI{
		Prompt: "some prompt",
	}

	err := cli.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCLI_Validate_OnlyPromptFile(t *testing.T) {
	cli := CLI{
		PromptFile: "some/file.txt",
	}

	err := cli.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCLI_Validate_OnlyPromptFlag(t *testing.T) {
	cli := CLI{
		PromptFlag: "some prompt via flag",
	}

	err := cli.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCLI_Validate_EmptyPromptWithPromptFile(t *testing.T) {
	// Empty string prompt should be treated as "not provided"
	cli := CLI{
		Prompt:     "",
		PromptFile: "some/file.txt",
	}

	err := cli.Validate()
	if err != nil {
		t.Errorf("unexpected error when prompt is empty but promptFile is set: %v", err)
	}
}

func TestCLI_Validate_PromptWithEmptyPromptFile(t *testing.T) {
	// Empty string promptFile should be treated as "not provided"
	cli := CLI{
		Prompt:     "some prompt",
		PromptFile: "",
	}

	err := cli.Validate()
	if err != nil {
		t.Errorf("unexpected error when prompt is set but promptFile is empty: %v", err)
	}
}

func TestCLI_Validate_WhitespaceOnlyPrompt(t *testing.T) {
	// Whitespace-only prompt is considered "provided" due to len check
	cli := CLI{
		Prompt: "   ",
	}

	err := cli.Validate()
	if err != nil {
		t.Errorf("whitespace-only prompt should pass validation: %v", err)
	}
}

func TestCLI_Struct_DefaultValues(t *testing.T) {
	cli := CLI{}

	// Verify default values
	if cli.Settings != "" {
		t.Errorf("expected empty default settings path, got %q", cli.Settings)
	}
	if cli.Verbose {
		t.Error("expected verbose to be false by default")
	}
	if cli.MaximumIterations != 0 {
		t.Errorf("expected max iterations to be 0 by default, got %d", cli.MaximumIterations)
	}
	if cli.CompletionResponse != "" {
		t.Errorf("expected completion response to be empty by default, got %q", cli.CompletionResponse)
	}
}

func TestCLI_GetPrompt(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		promptFlag string
		expected   string
	}{
		{"positional only", "positional", "", "positional"},
		{"flag only", "", "flagprompt", "flagprompt"},
		{"flag takes precedence", "positional", "flagprompt", "flagprompt"},
		{"neither", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := CLI{
				Prompt:     tt.prompt,
				PromptFlag: tt.promptFlag,
			}
			got := cli.GetPrompt()
			if got != tt.expected {
				t.Errorf("GetPrompt() = %q, want %q", got, tt.expected)
			}
		})
	}
}
