package main

import (
	"testing"
)

func TestRunCmd_Validate_BothPromptSources(t *testing.T) {
	cmd := RunCmd{
		Prompt:     "prompt",
		PromptFile: "file.txt",
	}
	err := cmd.Validate()
	if err == nil {
		t.Error("expected error when both prompt sources are set")
	}
	expected := "cannot specify both --prompt and --prompt-file"
	if err.Error() != expected {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunCmd_Validate_NoPromptSource(t *testing.T) {
	cmd := RunCmd{}

	err := cmd.Validate()
	if err == nil {
		t.Error("expected error when no prompt source is set")
	}
	expected := "must specify prompt (--prompt or --prompt-file)"
	if err.Error() != expected {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunCmd_Validate_OnlyPrompt(t *testing.T) {
	cmd := RunCmd{
		Prompt: "some prompt",
	}

	err := cmd.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCmd_Validate_OnlyPromptFile(t *testing.T) {
	cmd := RunCmd{
		PromptFile: "some/file.txt",
	}

	err := cmd.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCmd_Validate_EmptyPromptWithPromptFile(t *testing.T) {
	// Empty string prompt should be treated as "not provided"
	cmd := RunCmd{
		Prompt:     "",
		PromptFile: "some/file.txt",
	}

	err := cmd.Validate()
	if err != nil {
		t.Errorf("unexpected error when prompt is empty but promptFile is set: %v", err)
	}
}

func TestRunCmd_Validate_PromptWithEmptyPromptFile(t *testing.T) {
	// Empty string promptFile should be treated as "not provided"
	cmd := RunCmd{
		Prompt:     "some prompt",
		PromptFile: "",
	}

	err := cmd.Validate()
	if err != nil {
		t.Errorf("unexpected error when prompt is set but promptFile is empty: %v", err)
	}
}

func TestRunCmd_Validate_WhitespaceOnlyPrompt(t *testing.T) {
	// Whitespace-only prompt is considered "provided" due to len check
	cmd := RunCmd{
		Prompt: "   ",
	}

	err := cmd.Validate()
	if err != nil {
		t.Errorf("whitespace-only prompt should pass validation: %v", err)
	}
}

func TestRunCmd_Struct_DefaultValues(t *testing.T) {
	cmd := RunCmd{}

	// Verify default values
	if cmd.Verbose {
		t.Error("expected verbose to be false by default")
	}
	if cmd.MaximumIterations != 0 {
		t.Errorf("expected max iterations to be 0 by default, got %d", cmd.MaximumIterations)
	}
	if cmd.CompletionResponse != "" {
		t.Errorf("expected completion response to be empty by default, got %q", cmd.CompletionResponse)
	}
}

func TestRunCmd_GetPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{"with prompt", "myprompt", "myprompt"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := RunCmd{
				Prompt: tt.prompt,
			}
			got := cmd.GetPrompt()
			if got != tt.expected {
				t.Errorf("GetPrompt() = %q, want %q", got, tt.expected)
			}
		})
	}
}
