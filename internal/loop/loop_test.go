package loop

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/guardrail"
)

func TestExitCodeConstants(t *testing.T) {
	// Verify exit codes match PRD
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d, want 0", ExitSuccess)
	}
	if ExitMaxIterations != 1 {
		t.Errorf("ExitMaxIterations = %d, want 1", ExitMaxIterations)
	}
	if ExitConfigError != 2 {
		t.Errorf("ExitConfigError = %d, want 2", ExitConfigError)
	}
	if ExitSignalInterrupt != 130 {
		t.Errorf("ExitSignalInterrupt = %d, want 130", ExitSignalInterrupt)
	}
}

func TestNewRunner_DefaultsStdoutStderr(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "test",
		Settings: settings,
	})

	if runner.opts.Stdout != os.Stdout {
		t.Error("Expected Stdout to default to os.Stdout")
	}
	if runner.opts.Stderr != os.Stderr {
		t.Error("Expected Stderr to default to os.Stderr")
	}
}

func TestNewRunner_CustomStdoutStderr(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	var stdout, stderr bytes.Buffer

	runner := NewRunner(Options{
		Prompt:   "test",
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	if runner.opts.Stdout != &stdout {
		t.Error("Expected custom Stdout to be preserved")
	}
	if runner.opts.Stderr != &stderr {
		t.Error("Expected custom Stderr to be preserved")
	}
}

func TestBuildPrompt_DirectPrompt(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "direct prompt text",
		Settings: settings,
	})

	prompt, err := runner.buildPrompt(nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "direct prompt text" {
		t.Errorf("got %q, want %q", prompt, "direct prompt text")
	}
}

func TestBuildPrompt_IterationSummaryDisabled(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   3,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	prompt, err := runner.buildPrompt(nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "base prompt" {
		t.Errorf("got %q, want %q", prompt, "base prompt")
	}
}

func TestBuildPrompt_IterationSummaryEnabled(t *testing.T) {
	settings := &config.Settings{
		Agent:                         config.AgentConfig{Command: "echo"},
		MaximumIterations:             10,
		CompletionResponse:            "DONE",
		OutputTruncateChars:           1000,
		IncludeIterationCountInPrompt: true,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	prompt, err := runner.buildPrompt(nil, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Iteration 3 of 10, 7 remaining.\n\nbase prompt"
	if prompt != expected {
		t.Errorf("got %q, want %q", prompt, expected)
	}
}

func TestBuildPrompt_PromptFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("file prompt content"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		PromptFile: promptFile,
		Settings:   settings,
	})

	prompt, err := runner.buildPrompt(nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "file prompt content" {
		t.Errorf("got %q, want %q", prompt, "file prompt content")
	}
}

func TestBuildPrompt_PromptFileNotFound(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		PromptFile: "/nonexistent/path/to/prompt.txt",
		Settings:   settings,
	})

	_, err := runner.buildPrompt(nil, 1)
	if err == nil {
		t.Error("expected error for nonexistent prompt file")
	}
}

func TestBuildPrompt_WithFailedResults(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	failedResults := []guardrail.Result{
		{
			Guardrail: config.Guardrail{
				Command:    "make test",
				FailAction: "APPEND",
			},
			ExitCode: 1,
			Output:   "test failure output",
		},
	}

	prompt, err := runner.buildPrompt(failedResults, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain base prompt
	if len(prompt) <= len("base prompt") {
		t.Error("expected prompt to be augmented with failure info")
	}

	// With APPEND action, base prompt should come first
	if prompt[:len("base prompt")] != "base prompt" {
		t.Errorf("expected prompt to start with base prompt, got %q", prompt[:20])
	}
}

func TestBuildPrompt_WithPrependAction(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	failedResults := []guardrail.Result{
		{
			Guardrail: config.Guardrail{
				Command:    "make test",
				FailAction: "PREPEND",
			},
			ExitCode: 1,
			Output:   "test failure output",
		},
	}

	prompt, err := runner.buildPrompt(failedResults, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With PREPEND action, base prompt should NOT come first
	if len(prompt) >= len("base prompt") && prompt[:len("base prompt")] == "base prompt" {
		t.Error("expected failure info to be prepended to base prompt")
	}
}

func TestBuildPrompt_IterationSummaryWithPrependAction(t *testing.T) {
	settings := &config.Settings{
		Agent:                         config.AgentConfig{Command: "echo"},
		MaximumIterations:             5,
		CompletionResponse:            "DONE",
		OutputTruncateChars:           1000,
		IncludeIterationCountInPrompt: true,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	failedResults := []guardrail.Result{
		{
			Guardrail: config.Guardrail{
				Command:    "make test",
				FailAction: "PREPEND",
			},
			ExitCode: 1,
			Output:   "test failure output",
		},
	}

	prompt, err := runner.buildPrompt(failedResults, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	summary := "Iteration 2 of 5, 3 remaining."
	if !strings.HasPrefix(prompt, summary+"\n\n") {
		gotPrefix := prompt
		if len(prompt) > len(summary) {
			gotPrefix = prompt[:len(summary)]
		}
		t.Fatalf("expected prompt to start with iteration summary, got %q", gotPrefix)
	}

	failureIndex := strings.Index(prompt, "test failure output")
	baseIndex := strings.Index(prompt, "base prompt")
	if failureIndex == -1 || baseIndex == -1 {
		t.Fatalf("expected prompt to include failure output and base prompt")
	}
	if failureIndex < len(summary) || failureIndex > baseIndex {
		t.Errorf("expected failure output to appear after summary and before base prompt")
	}
}

func TestBuildPrompt_WithReplaceAction(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	failedResults := []guardrail.Result{
		{
			Guardrail: config.Guardrail{
				Command:    "make test",
				FailAction: "REPLACE",
			},
			ExitCode: 1,
			Output:   "test failure output",
		},
	}

	prompt, err := runner.buildPrompt(failedResults, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With REPLACE action, base prompt should be completely replaced
	if prompt == "base prompt" {
		t.Error("expected base prompt to be replaced")
	}
}

func TestRunner_Log_VerboseEnabled(t *testing.T) {
	var stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "test",
		Settings: settings,
		Verbose:  true,
		Stderr:   &stderr,
	})

	runner.log("test message %s", "arg")

	if stderr.Len() == 0 {
		t.Error("expected output when verbose enabled")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("[ralph]")) {
		t.Error("expected [ralph] prefix in verbose output")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("test message arg")) {
		t.Error("expected message content in verbose output")
	}
}

func TestRunner_Log_VerboseDisabled(t *testing.T) {
	var stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "test",
		Settings: settings,
		Verbose:  false,
		Stderr:   &stderr,
	})

	runner.log("test message")

	if stderr.Len() != 0 {
		t.Error("expected no output when verbose disabled")
	}
}

func TestRunner_Print(t *testing.T) {
	var stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "test",
		Settings: settings,
		Stderr:   &stderr,
	})

	runner.print("status message %d", 42)

	if !bytes.Contains(stderr.Bytes(), []byte("status message 42")) {
		t.Errorf("expected message content in output, got %q", stderr.String())
	}
}

func TestRunner_Run_CancelledContext(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   5,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "test",
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exitCode := runner.Run(ctx)

	if exitCode != ExitSignalInterrupt {
		t.Errorf("expected ExitSignalInterrupt (%d), got %d", ExitSignalInterrupt, exitCode)
	}
}

func TestRunner_Run_SuccessWithCompletionResponse(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   5,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
		StreamAgentOutput:   false,
	}

	runner := NewRunner(Options{
		// Echo will output JSON result format that IsComplete expects
		Prompt:   `{"type":"result","result":"DONE"}`,
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	exitCode := runner.Run(context.Background())

	if exitCode != ExitSuccess {
		t.Errorf("expected ExitSuccess (%d), got %d", ExitSuccess, exitCode)
	}
}

func TestRunner_Run_MaxIterationsReached(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   2, // Only 2 iterations
		CompletionResponse:  "NEVER_MATCH",
		OutputTruncateChars: 1000,
		StreamAgentOutput:   false,
	}

	runner := NewRunner(Options{
		Prompt:   "no completion here",
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	exitCode := runner.Run(context.Background())

	if exitCode != ExitMaxIterations {
		t.Errorf("expected ExitMaxIterations (%d), got %d", ExitMaxIterations, exitCode)
	}

	// Verify the stderr message indicates max iterations reached
	if !bytes.Contains(stderr.Bytes(), []byte("Maximum iterations reached")) {
		t.Error("expected 'Maximum iterations reached' message in stderr")
	}
}

func TestRunner_Run_WithGuardrails_AllPass(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   5,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
		StreamAgentOutput:   false,
		Guardrails: []config.Guardrail{
			{Command: "true", FailAction: "APPEND"},
			{Command: "true", FailAction: "APPEND"},
		},
	}

	// Create temp directory for guardrail logs
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph directory
	_ = os.MkdirAll(".ralph", 0o755)

	runner := NewRunner(Options{
		Prompt:   `{"type":"result","result":"DONE"}`,
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	exitCode := runner.Run(context.Background())

	if exitCode != ExitSuccess {
		t.Errorf("expected ExitSuccess (%d), got %d", ExitSuccess, exitCode)
	}

	// Verify all guardrails passed message
	if !bytes.Contains(stderr.Bytes(), []byte("All guardrails passed")) {
		t.Error("expected 'All guardrails passed' message in stderr")
	}
}

func TestRunner_Run_WithGuardrails_OneFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   2,
		CompletionResponse:  "NEVER_MATCH",
		OutputTruncateChars: 1000,
		StreamAgentOutput:   false,
		Guardrails: []config.Guardrail{
			{Command: "true", FailAction: "APPEND"},
			{Command: "false", FailAction: "APPEND"}, // This will fail
		},
	}

	// Create temp directory for guardrail logs
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph directory
	_ = os.MkdirAll(".ralph", 0o755)

	runner := NewRunner(Options{
		Prompt:   "test prompt",
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
		Verbose:  true,
	})

	exitCode := runner.Run(context.Background())

	// Should hit max iterations since guardrail keeps failing
	if exitCode != ExitMaxIterations {
		t.Errorf("expected ExitMaxIterations (%d), got %d", ExitMaxIterations, exitCode)
	}
}

func TestRunner_Run_WithSCM(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   5,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
		StreamAgentOutput:   false,
		SCM: &config.SCMConfig{
			Command: "echo",
			Tasks:   []string{}, // Empty tasks - won't actually run anything
		},
	}

	runner := NewRunner(Options{
		Prompt:   `{"type":"result","result":"DONE"}`,
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
		Verbose:  true,
	})

	exitCode := runner.Run(context.Background())

	if exitCode != ExitSuccess {
		t.Errorf("expected ExitSuccess (%d), got %d", ExitSuccess, exitCode)
	}
}

func TestRunner_Run_VerbosePromptPreview(t *testing.T) {
	var stdout, stderr bytes.Buffer
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
		StreamAgentOutput:   false,
	}

	// Create a long prompt to test truncation
	longPrompt := ""
	for i := 0; i < 300; i++ {
		longPrompt += "x"
	}

	runner := NewRunner(Options{
		Prompt:   longPrompt,
		Settings: settings,
		Stdout:   &stdout,
		Stderr:   &stderr,
		Verbose:  true,
	})

	_ = runner.Run(context.Background())

	// Verbose output should include truncated prompt preview with "..."
	stderrStr := stderr.String()
	if !bytes.Contains([]byte(stderrStr), []byte("Prompt:")) {
		t.Error("expected 'Prompt:' in verbose output")
	}
	if !bytes.Contains([]byte(stderrStr), []byte("...")) {
		t.Error("expected truncated prompt preview with '...'")
	}
}

func TestBuildPrompt_MultipleFailures(t *testing.T) {
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		MaximumIterations:   1,
		CompletionResponse:  "DONE",
		OutputTruncateChars: 1000,
	}

	runner := NewRunner(Options{
		Prompt:   "base prompt",
		Settings: settings,
	})

	// Test with multiple failed results with different actions
	failedResults := []guardrail.Result{
		{
			Guardrail: config.Guardrail{
				Command:    "test1",
				FailAction: "APPEND",
			},
			ExitCode: 1,
			Output:   "error 1",
		},
		{
			Guardrail: config.Guardrail{
				Command:    "test2",
				FailAction: "APPEND",
			},
			ExitCode: 2,
			Output:   "error 2",
		},
	}

	prompt, err := runner.buildPrompt(failedResults, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both errors should be in the prompt
	if !bytes.Contains([]byte(prompt), []byte("test1")) {
		t.Error("expected test1 in prompt")
	}
	if !bytes.Contains([]byte(prompt), []byte("test2")) {
		t.Error("expected test2 in prompt")
	}
}
