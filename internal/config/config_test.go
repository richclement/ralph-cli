package config

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte(localContent), 0o644); err != nil {
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

func TestValidate_EmptyCompletionResponse(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.CompletionResponse = ""

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for empty completionResponse")
	}
	if !strings.Contains(err.Error(), "completionResponse") {
		t.Errorf("error should mention completionResponse, got: %v", err)
	}
}

func TestValidate_ZeroMaximumIterations(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.MaximumIterations = 0

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for zero maximumIterations")
	}
	if !strings.Contains(err.Error(), "maximumIterations") {
		t.Errorf("error should mention maximumIterations, got: %v", err)
	}
}

func TestValidate_NegativeOutputTruncateChars(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.OutputTruncateChars = -1

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for negative outputTruncateChars")
	}
	if !strings.Contains(err.Error(), "outputTruncateChars") {
		t.Errorf("error should mention outputTruncateChars, got: %v", err)
	}
}

func TestValidate_SCMTasksWithoutCommand(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.SCM = &SCMConfig{
		Tasks: []string{"commit", "push"},
		// Command is empty
	}

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for SCM tasks without command")
	}
	if !strings.Contains(err.Error(), "scm.command") {
		t.Errorf("error should mention scm.command, got: %v", err)
	}
}

func TestValidate_SCMWithValidConfig(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.SCM = &SCMConfig{
		Command: "git",
		Tasks:   []string{"commit", "push"},
	}

	err := s.Validate()
	if err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidate_SCMEmptyTasks(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.SCM = &SCMConfig{
		Tasks: []string{},
		// Command empty is OK when tasks are empty
	}

	err := s.Validate()
	if err != nil {
		t.Errorf("Validate() should not error for empty SCM tasks: %v", err)
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

func TestDeepMerge_InvalidTypeErrors(t *testing.T) {
	tests := []struct {
		name      string
		localJSON string
		wantErr   string
	}{
		{
			name:      "maximumIterations wrong type",
			localJSON: `{"maximumIterations": "not-an-int"}`,
			wantErr:   "maximumIterations:",
		},
		{
			name:      "completionResponse wrong type",
			localJSON: `{"completionResponse": 123}`,
			wantErr:   "completionResponse:",
		},
		{
			name:      "outputTruncateChars wrong type",
			localJSON: `{"outputTruncateChars": "not-an-int"}`,
			wantErr:   "outputTruncateChars:",
		},
		{
			name:      "streamAgentOutput wrong type",
			localJSON: `{"streamAgentOutput": "not-a-bool"}`,
			wantErr:   "streamAgentOutput:",
		},
		{
			name:      "agent wrong type",
			localJSON: `{"agent": "not-an-object"}`,
			wantErr:   "agent:",
		},
		{
			name:      "agent.command wrong type",
			localJSON: `{"agent": {"command": 123}}`,
			wantErr:   "agent.command:",
		},
		{
			name:      "agent.flags wrong type",
			localJSON: `{"agent": {"flags": "not-an-array"}}`,
			wantErr:   "agent.flags:",
		},
		{
			name:      "guardrails wrong type",
			localJSON: `{"guardrails": "not-an-array"}`,
			wantErr:   "guardrails:",
		},
		{
			name:      "scm wrong type",
			localJSON: `{"scm": "not-an-object"}`,
			wantErr:   "scm:",
		},
		{
			name:      "scm.command wrong type",
			localJSON: `{"scm": {"command": 123}}`,
			wantErr:   "scm.command:",
		},
		{
			name:      "scm.tasks wrong type",
			localJSON: `{"scm": {"tasks": "not-an-array"}}`,
			wantErr:   "scm.tasks:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewDefaults()
			err := deepMerge(&s, []byte(tt.localJSON))
			if err == nil {
				t.Errorf("deepMerge() should return error for %s", tt.name)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("deepMerge() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadWithLocal_InvalidTypeError(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "settings.json")
	localPath := filepath.Join(dir, "settings.local.json")

	baseContent := `{
		"maximumIterations": 10,
		"agent": {"command": "claude"}
	}`
	// Invalid: maximumIterations should be int, not string
	localContent := `{"maximumIterations": "not-a-number"}`

	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte(localContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWithLocal(basePath)
	if err == nil {
		t.Error("LoadWithLocal() should return error for invalid local settings type")
	}
	if !strings.Contains(err.Error(), "maximumIterations") {
		t.Errorf("error should mention maximumIterations, got: %v", err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	content := `{invalid json}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}
}

func TestLoad_ReAppliesDefaults(t *testing.T) {
	// Test that Load re-applies defaults for zero/empty values in JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// JSON with zero/empty values that should get default values
	content := `{
		"maximumIterations": 0,
		"completionResponse": "",
		"outputTruncateChars": 0,
		"agent": {"command": "claude"}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Zero values should be replaced with defaults
	if s.MaximumIterations != DefaultMaximumIterations {
		t.Errorf("MaximumIterations = %d, want default %d", s.MaximumIterations, DefaultMaximumIterations)
	}
	if s.CompletionResponse != DefaultCompletionResponse {
		t.Errorf("CompletionResponse = %q, want default %q", s.CompletionResponse, DefaultCompletionResponse)
	}
	if s.OutputTruncateChars != DefaultOutputTruncateChars {
		t.Errorf("OutputTruncateChars = %d, want default %d", s.OutputTruncateChars, DefaultOutputTruncateChars)
	}
}

func TestLoadWithLocal_NoLocalFile(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "settings.json")
	// No local file created

	baseContent := `{
		"maximumIterations": 15,
		"agent": {"command": "claude"}
	}`

	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadWithLocal(basePath)
	if err != nil {
		t.Fatalf("LoadWithLocal returned error: %v", err)
	}

	// Should have values from base file
	if s.MaximumIterations != 15 {
		t.Errorf("MaximumIterations = %d, want 15", s.MaximumIterations)
	}
}

func TestLoadWithLocal_BaseFileError(t *testing.T) {
	// Create a path that will fail to read (e.g., directory instead of file)
	dir := t.TempDir()
	basePath := filepath.Join(dir, "settings.json")

	// Create settings.json as a directory to cause a read error
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWithLocal(basePath)
	if err == nil {
		t.Error("LoadWithLocal() should return error when base file is a directory")
	}
}

func TestLoadWithLocal_LocalFileReadError(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "settings.json")
	localPath := filepath.Join(dir, "settings.local.json")

	baseContent := `{"agent": {"command": "claude"}}`
	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create local file as a directory to cause a read error
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWithLocal(basePath)
	if err == nil {
		t.Error("LoadWithLocal() should return error when local file is a directory")
	}
}

func TestDeepMerge_InvalidTopLevelJSON(t *testing.T) {
	s := NewDefaults()
	err := deepMerge(&s, []byte(`not valid json`))
	if err == nil {
		t.Error("deepMerge() should return error for invalid JSON")
	}
}

func TestDeepMerge_SCMCreatesNewObject(t *testing.T) {
	s := NewDefaults()
	// SCM is nil by default

	localJSON := `{"scm": {"command": "git", "tasks": ["commit"]}}`
	if err := deepMerge(&s, []byte(localJSON)); err != nil {
		t.Fatal(err)
	}

	if s.SCM == nil {
		t.Error("SCM should be created")
	}
	if s.SCM.Command != "git" {
		t.Errorf("SCM.Command = %q, want git", s.SCM.Command)
	}
	if len(s.SCM.Tasks) != 1 || s.SCM.Tasks[0] != "commit" {
		t.Errorf("SCM.Tasks = %v, want [commit]", s.SCM.Tasks)
	}
}

func TestValidate_GuardrailEmptyCommand(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.Guardrails = []Guardrail{
		{Command: "", FailAction: "APPEND"},
	}

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for empty guardrail command")
	}
	if !strings.Contains(err.Error(), "guardrails[0].command") {
		t.Errorf("error should mention guardrails[0].command, got: %v", err)
	}
}

func TestValidate_GuardrailInvalidFailAction(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.Guardrails = []Guardrail{
		{Command: "make test", FailAction: "INVALID"},
	}

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for invalid failAction")
	}
	if !strings.Contains(err.Error(), "guardrails[0].failAction") {
		t.Errorf("error should mention guardrails[0].failAction, got: %v", err)
	}
}

func TestValidate_GuardrailValidFailActions(t *testing.T) {
	tests := []string{"APPEND", "PREPEND", "REPLACE", "append", "prepend", "replace"}
	for _, action := range tests {
		t.Run(action, func(t *testing.T) {
			s := NewDefaults()
			s.Agent.Command = "claude"
			s.Guardrails = []Guardrail{
				{Command: "make test", FailAction: action},
			}

			err := s.Validate()
			if err != nil {
				t.Errorf("Validate() returned error for valid failAction %q: %v", action, err)
			}
		})
	}
}

func TestValidate_NegativeMaximumIterations(t *testing.T) {
	s := NewDefaults()
	s.Agent.Command = "claude"
	s.MaximumIterations = -5

	err := s.Validate()
	if err == nil {
		t.Error("Validate() should return error for negative maximumIterations")
	}
}
