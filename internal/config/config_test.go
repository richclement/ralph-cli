package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	s := NewDefaults()
	if s.MaximumIterations != DefaultMaximumIterations {
		t.Errorf("MaximumIterations = %d, want %d", s.MaximumIterations, DefaultMaximumIterations)
	}
	if s.CompletionResponse != DefaultCompletionResponse {
		t.Errorf("CompletionResponse = %q, want %q", s.CompletionResponse, DefaultCompletionResponse)
	}
	if s.OutputTruncateChars != DefaultOutputTruncateChars {
		t.Errorf("OutputTruncateChars = %d, want %d", s.OutputTruncateChars, DefaultOutputTruncateChars)
	}
	if s.StreamAgentOutput != DefaultStreamAgentOutput {
		t.Errorf("StreamAgentOutput = %v, want %v", s.StreamAgentOutput, DefaultStreamAgentOutput)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	s, err := Load("/nonexistent/path/settings.json")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	// Should return defaults
	if s.MaximumIterations != DefaultMaximumIterations {
		t.Errorf("MaximumIterations = %d, want %d", s.MaximumIterations, DefaultMaximumIterations)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	content := `{
		"maximumIterations": 5,
		"completionResponse": "FINISHED",
		"outputTruncateChars": 1000,
		"streamAgentOutput": false,
		"agent": {
			"command": "claude",
			"flags": ["--model", "opus"]
		},
		"guardrails": [
			{"command": "make test", "failAction": "APPEND"}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if s.MaximumIterations != 5 {
		t.Errorf("MaximumIterations = %d, want 5", s.MaximumIterations)
	}
	if s.CompletionResponse != "FINISHED" {
		t.Errorf("CompletionResponse = %q, want FINISHED", s.CompletionResponse)
	}
	if s.OutputTruncateChars != 1000 {
		t.Errorf("OutputTruncateChars = %d, want 1000", s.OutputTruncateChars)
	}
	if s.StreamAgentOutput != false {
		t.Errorf("StreamAgentOutput = %v, want false", s.StreamAgentOutput)
	}
	if s.Agent.Command != "claude" {
		t.Errorf("Agent.Command = %q, want claude", s.Agent.Command)
	}
	if len(s.Agent.Flags) != 2 || s.Agent.Flags[0] != "--model" {
		t.Errorf("Agent.Flags = %v, want [--model opus]", s.Agent.Flags)
	}
	if len(s.Guardrails) != 1 || s.Guardrails[0].Command != "make test" {
		t.Errorf("Guardrails = %v, unexpected", s.Guardrails)
	}
}

func TestDeepMerge_ScalarOverride(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.Agent.Flags = []string{"--model", "opus"}

	localJSON := `{"maximumIterations": 20, "completionResponse": "COMPLETE"}`
	if err := deepMerge(&s, []byte(localJSON)); err != nil {
		t.Fatal(err)
	}

	if s.MaximumIterations != 20 {
		t.Errorf("MaximumIterations = %d, want 20", s.MaximumIterations)
	}
	if s.CompletionResponse != "COMPLETE" {
		t.Errorf("CompletionResponse = %q, want COMPLETE", s.CompletionResponse)
	}
	// Should preserve existing agent
	if s.Agent.Command != "claude" {
		t.Errorf("Agent.Command = %q, should be preserved", s.Agent.Command)
	}
}

func TestDeepMerge_ArrayReplace(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.Agent.Flags = []string{"--model", "opus"}

	localJSON := `{"agent": {"flags": ["--verbose"]}}`
	if err := deepMerge(&s, []byte(localJSON)); err != nil {
		t.Fatal(err)
	}

	// Flags should be replaced, not appended
	if len(s.Agent.Flags) != 1 || s.Agent.Flags[0] != "--verbose" {
		t.Errorf("Agent.Flags = %v, want [--verbose]", s.Agent.Flags)
	}
	// Command should be preserved
	if s.Agent.Command != "claude" {
		t.Errorf("Agent.Command = %q, should be preserved", s.Agent.Command)
	}
}

func TestDeepMerge_ObjectMerge(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.Agent.Flags = []string{"--model", "opus"}

	// PRD example: local only changes flags, command preserved
	localJSON := `{"agent": {"flags": ["--verbose"]}}`
	if err := deepMerge(&s, []byte(localJSON)); err != nil {
		t.Fatal(err)
	}

	if s.Agent.Command != "claude" {
		t.Errorf("Agent.Command = %q, want claude (should be preserved)", s.Agent.Command)
	}
	if len(s.Agent.Flags) != 1 || s.Agent.Flags[0] != "--verbose" {
		t.Errorf("Agent.Flags = %v, want [--verbose]", s.Agent.Flags)
	}
}

func TestLoadWithLocal(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "settings.json")
	localPath := filepath.Join(dir, "settings.local.json")

	baseContent := `{
		"maximumIterations": 10,
		"agent": {"command": "claude", "flags": ["--model opus"]}
	}`
	localContent := `{
		"agent": {"flags": ["--verbose"]}
	}`

	if err := os.WriteFile(basePath, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadWithLocal(basePath)
	if err != nil {
		t.Fatalf("LoadWithLocal returned error: %v", err)
	}

	// Per PRD example: command preserved, flags replaced
	if s.Agent.Command != "claude" {
		t.Errorf("Agent.Command = %q, want claude", s.Agent.Command)
	}
	if len(s.Agent.Flags) != 1 || s.Agent.Flags[0] != "--verbose" {
		t.Errorf("Agent.Flags = %v, want [--verbose]", s.Agent.Flags)
	}
}

func TestApplyCLIOverrides(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"

	maxIter := 25
	compResp := "FINISHED"
	stream := false

	overrides := CLIOverrides{
		MaximumIterations:  &maxIter,
		CompletionResponse: &compResp,
		StreamAgentOutput:  &stream,
	}

	s.ApplyCLIOverrides(overrides)

	if s.MaximumIterations != 25 {
		t.Errorf("MaximumIterations = %d, want 25", s.MaximumIterations)
	}
	if s.CompletionResponse != "FINISHED" {
		t.Errorf("CompletionResponse = %q, want FINISHED", s.CompletionResponse)
	}
	if s.StreamAgentOutput != false {
		t.Errorf("StreamAgentOutput = %v, want false", s.StreamAgentOutput)
	}
}

func TestApplyCLIOverrides_Partial(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"

	// Only override one field
	maxIter := 25
	overrides := CLIOverrides{
		MaximumIterations: &maxIter,
	}

	s.ApplyCLIOverrides(overrides)

	if s.MaximumIterations != 25 {
		t.Errorf("MaximumIterations = %d, want 25", s.MaximumIterations)
	}
	// Others should remain default
	if s.CompletionResponse != DefaultCompletionResponse {
		t.Errorf("CompletionResponse = %q, want default", s.CompletionResponse)
	}
	if s.StreamAgentOutput != DefaultStreamAgentOutput {
		t.Errorf("StreamAgentOutput = %v, want default", s.StreamAgentOutput)
	}
}

func TestValidate_MissingAgentCommand(t *testing.T) {
	s := NewDefaults()
	// Agent.Command is empty

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for missing agent.command")
	}
}

func TestValidate_ValidSettings(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"

	err := s.Validate()
	if err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}
