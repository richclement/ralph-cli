package main

import (
	"testing"
)

func TestCLI_Validate_BothPromptAndPromptFile(t *testing.T) {
	cli := CLI{
		Prompt:     "some prompt",
		PromptFile: "some/file.txt",
	}

	err := cli.Validate()
	if err == nil {
		t.Error("expected error when both Prompt and PromptFile are set")
	}
	if err.Error() != "cannot specify both prompt and --prompt-file" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCLI_Validate_NeitherPromptNorPromptFile(t *testing.T) {
	cli := CLI{}

	err := cli.Validate()
	if err == nil {
		t.Error("expected error when neither Prompt nor PromptFile is set")
	}
	if err.Error() != "must specify either prompt or --prompt-file" {
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
