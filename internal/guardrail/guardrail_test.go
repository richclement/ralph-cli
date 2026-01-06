package guardrail

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/richclement/ralph-cli/internal/config"
)

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "simple command",
			command: "make test",
			want:    "make_test",
		},
		{
			name:    "mvnw command",
			command: "./mvnw clean install -T 2C",
			want:    "mvnw_clean_install_T_2C",
		},
		{
			name:    "command with special chars",
			command: "npm run test:unit",
			want:    "npm_run_test_unit",
		},
		{
			name:    "long command truncation",
			command: "this_is_a_very_long_command_that_should_be_truncated_at_fifty_characters_total",
			want:    "this_is_a_very_long_command_that_should_be_truncat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSlug(tt.command)
			if got != tt.want {
				t.Errorf("GenerateSlug() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		limit  int
		want   string
	}{
		{
			name:   "no truncation needed",
			output: "short output",
			limit:  100,
			want:   "short output",
		},
		{
			name:   "truncation needed",
			output: "this is a longer output that needs to be truncated",
			limit:  20,
			want:   "this is a longer out... [truncated]",
		},
		{
			name:   "zero limit no truncation",
			output: "any output",
			limit:  0,
			want:   "any output",
		},
		{
			name:   "negative limit no truncation",
			output: "any output",
			limit:  -1,
			want:   "any output",
		},
		{
			name:   "exact length",
			output: "exact",
			limit:  5,
			want:   "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateOutput(tt.output, tt.limit)
			if got != tt.want {
				t.Errorf("TruncateOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyFailAction(t *testing.T) {
	basePrompt := "base prompt"
	failedOutput := "error output"

	tests := []struct {
		name   string
		action string
		want   string
	}{
		{
			name:   "append",
			action: "APPEND",
			want:   "base prompt\n\nerror output",
		},
		{
			name:   "prepend",
			action: "PREPEND",
			want:   "error output\n\nbase prompt",
		},
		{
			name:   "replace",
			action: "REPLACE",
			want:   "error output",
		},
		{
			name:   "lowercase append",
			action: "append",
			want:   "base prompt\n\nerror output",
		},
		{
			name:   "default is append",
			action: "unknown",
			want:   "base prompt\n\nerror output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyFailAction(basePrompt, failedOutput, tt.action)
			if got != tt.want {
				t.Errorf("ApplyFailAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateLogFilename(t *testing.T) {
	r := &Runner{OutputDir: ".ralph"}

	tests := []struct {
		name       string
		iteration  int
		slug       string
		slugCounts map[string]int
		want       string
	}{
		{
			name:       "nil slugCounts",
			iteration:  1,
			slug:       "make_test",
			slugCounts: nil,
			want:       ".ralph/guardrail_001_make_test.log",
		},
		{
			name:       "first occurrence",
			iteration:  1,
			slug:       "make_test",
			slugCounts: map[string]int{},
			want:       ".ralph/guardrail_001_make_test.log",
		},
		{
			name:       "second occurrence",
			iteration:  1,
			slug:       "make_test",
			slugCounts: map[string]int{"make_test": 1},
			want:       ".ralph/guardrail_001_make_test_1.log",
		},
		{
			name:       "third occurrence",
			iteration:  1,
			slug:       "make_test",
			slugCounts: map[string]int{"make_test": 2},
			want:       ".ralph/guardrail_001_make_test_2.log",
		},
		{
			name:       "different slug unaffected",
			iteration:  1,
			slug:       "npm_test",
			slugCounts: map[string]int{"make_test": 3},
			want:       ".ralph/guardrail_001_npm_test.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.generateLogFilename(tt.iteration, tt.slug, tt.slugCounts)
			if got != tt.want {
				t.Errorf("generateLogFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateLogFilename_Increments(t *testing.T) {
	r := &Runner{OutputDir: ".ralph"}
	slugCounts := make(map[string]int)

	// First call for make_test
	got1 := r.generateLogFilename(1, "make_test", slugCounts)
	if got1 != ".ralph/guardrail_001_make_test.log" {
		t.Errorf("First call: got %q, want %q", got1, ".ralph/guardrail_001_make_test.log")
	}

	// Second call for make_test
	got2 := r.generateLogFilename(1, "make_test", slugCounts)
	if got2 != ".ralph/guardrail_001_make_test_1.log" {
		t.Errorf("Second call: got %q, want %q", got2, ".ralph/guardrail_001_make_test_1.log")
	}

	// Third call for make_test
	got3 := r.generateLogFilename(1, "make_test", slugCounts)
	if got3 != ".ralph/guardrail_001_make_test_2.log" {
		t.Errorf("Third call: got %q, want %q", got3, ".ralph/guardrail_001_make_test_2.log")
	}

	// Call for different slug
	got4 := r.generateLogFilename(1, "npm_test", slugCounts)
	if got4 != ".ralph/guardrail_001_npm_test.log" {
		t.Errorf("Different slug: got %q, want %q", got4, ".ralph/guardrail_001_npm_test.log")
	}
}

func TestNewRunner(t *testing.T) {
	runner := NewRunner(1000, true)

	if runner.OutputDir != ".ralph" {
		t.Errorf("Expected OutputDir to be '.ralph', got %q", runner.OutputDir)
	}
	if runner.OutputTruncateChars != 1000 {
		t.Errorf("Expected OutputTruncateChars to be 1000, got %d", runner.OutputTruncateChars)
	}
	if runner.Verbose != true {
		t.Error("Expected Verbose to be true")
	}
	if runner.Stdout != os.Stdout {
		t.Error("Expected Stdout to default to os.Stdout")
	}
	if runner.Stderr != os.Stderr {
		t.Error("Expected Stderr to default to os.Stderr")
	}
}

func TestNewRunner_VerboseFalse(t *testing.T) {
	runner := NewRunner(500, false)

	if runner.Verbose != false {
		t.Error("Expected Verbose to be false")
	}
	if runner.OutputTruncateChars != 500 {
		t.Errorf("Expected OutputTruncateChars to be 500, got %d", runner.OutputTruncateChars)
	}
}

func TestAllPassed_AllSuccess(t *testing.T) {
	results := []Result{
		{Success: true},
		{Success: true},
		{Success: true},
	}

	if !AllPassed(results) {
		t.Error("Expected AllPassed to return true when all results are successful")
	}
}

func TestAllPassed_OneFailed(t *testing.T) {
	results := []Result{
		{Success: true},
		{Success: false},
		{Success: true},
	}

	if AllPassed(results) {
		t.Error("Expected AllPassed to return false when one result failed")
	}
}

func TestAllPassed_AllFailed(t *testing.T) {
	results := []Result{
		{Success: false},
		{Success: false},
	}

	if AllPassed(results) {
		t.Error("Expected AllPassed to return false when all results failed")
	}
}

func TestAllPassed_EmptyResults(t *testing.T) {
	results := []Result{}

	if !AllPassed(results) {
		t.Error("Expected AllPassed to return true for empty results")
	}
}

func TestGetFailedResults(t *testing.T) {
	results := []Result{
		{Guardrail: config.Guardrail{Command: "cmd1"}, Success: true},
		{Guardrail: config.Guardrail{Command: "cmd2"}, Success: false},
		{Guardrail: config.Guardrail{Command: "cmd3"}, Success: true},
		{Guardrail: config.Guardrail{Command: "cmd4"}, Success: false},
	}

	failed := GetFailedResults(results)

	if len(failed) != 2 {
		t.Errorf("Expected 2 failed results, got %d", len(failed))
	}
	if failed[0].Guardrail.Command != "cmd2" {
		t.Errorf("Expected first failed to be cmd2, got %s", failed[0].Guardrail.Command)
	}
	if failed[1].Guardrail.Command != "cmd4" {
		t.Errorf("Expected second failed to be cmd4, got %s", failed[1].Guardrail.Command)
	}
}

func TestGetFailedResults_NoFailures(t *testing.T) {
	results := []Result{
		{Success: true},
		{Success: true},
	}

	failed := GetFailedResults(results)

	if len(failed) != 0 {
		t.Errorf("Expected 0 failed results, got %d", len(failed))
	}
}

func TestFormatFailureMessage(t *testing.T) {
	result := Result{
		Guardrail: config.Guardrail{Command: "make test"},
		Output:    "test output here",
		ExitCode:  1,
		LogFile:   ".ralph/guardrail_001_make_test.log",
	}

	msg := FormatFailureMessage(result, 1000)

	if !strings.Contains(msg, "make test") {
		t.Error("Expected message to contain command")
	}
	if !strings.Contains(msg, "exit code 1") {
		t.Error("Expected message to contain exit code")
	}
	if !strings.Contains(msg, ".ralph/guardrail_001_make_test.log") {
		t.Error("Expected message to contain log file path")
	}
	if !strings.Contains(msg, "test output here") {
		t.Error("Expected message to contain output")
	}
}

func TestFormatFailureMessage_WithTruncation(t *testing.T) {
	result := Result{
		Guardrail: config.Guardrail{Command: "make test"},
		Output:    "this is a very long output that should be truncated",
		ExitCode:  2,
		LogFile:   ".ralph/guardrail_001_make_test.log",
	}

	msg := FormatFailureMessage(result, 10)

	if !strings.Contains(msg, "... [truncated]") {
		t.Error("Expected message to contain truncation indicator")
	}
}

func TestGetFailedOutputForPrompt(t *testing.T) {
	results := []Result{
		{
			Guardrail: config.Guardrail{Command: "cmd1"},
			Output:    "error 1",
			Success:   false,
		},
		{
			Guardrail: config.Guardrail{Command: "cmd2"},
			Output:    "error 2",
			Success:   false,
		},
		{
			Guardrail: config.Guardrail{Command: "cmd3"},
			Output:    "ok",
			Success:   true,
		},
	}

	output := GetFailedOutputForPrompt(results, 1000)

	if !strings.Contains(output, "cmd1") {
		t.Error("Expected output to contain cmd1")
	}
	if !strings.Contains(output, "error 1") {
		t.Error("Expected output to contain error 1")
	}
	if !strings.Contains(output, "cmd2") {
		t.Error("Expected output to contain cmd2")
	}
	if !strings.Contains(output, "error 2") {
		t.Error("Expected output to contain error 2")
	}
	// Should not include successful command
	if strings.Contains(output, "cmd3") {
		t.Error("Expected output to NOT contain cmd3 (successful)")
	}
}

func TestRunner_Run_Success(t *testing.T) {
	// Create temp directory for logs
	tmpDir := t.TempDir()

	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           tmpDir,
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             false,
	}

	g := config.Guardrail{
		Command:    "echo success",
		FailAction: "APPEND",
	}

	result := runner.Run(context.Background(), g, 1)

	if !result.Success {
		t.Error("Expected success for 'echo success'")
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Output, "success") {
		t.Errorf("Expected output to contain 'success', got %q", result.Output)
	}
}

func TestRunner_Run_Failure(t *testing.T) {
	tmpDir := t.TempDir()

	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           tmpDir,
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             false,
	}

	g := config.Guardrail{
		Command:    "exit 1",
		FailAction: "APPEND",
	}

	result := runner.Run(context.Background(), g, 1)

	if result.Success {
		t.Error("Expected failure for 'exit 1'")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
}

func TestRunner_RunAll(t *testing.T) {
	tmpDir := t.TempDir()

	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           tmpDir,
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             false,
	}

	guardrails := []config.Guardrail{
		{Command: "echo test1", FailAction: "APPEND"},
		{Command: "echo test2", FailAction: "APPEND"},
	}

	results := runner.RunAll(context.Background(), guardrails, 1)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if !results[0].Success || !results[1].Success {
		t.Error("Expected both guardrails to succeed")
	}
}

func TestRunner_Print(t *testing.T) {
	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           ".ralph",
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             false,
	}

	runner.print("test message %d", 42)

	if !strings.Contains(stderr.String(), "test message 42") {
		t.Errorf("Expected stderr to contain 'test message 42', got %q", stderr.String())
	}
}

func TestRunner_RunWithSlugTracker_ExitCodeExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           tmpDir,
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             false,
	}

	// Test with specific exit code
	g := config.Guardrail{
		Command:    "exit 42",
		FailAction: "APPEND",
	}

	slugCounts := make(map[string]int)
	result := runner.RunWithSlugTracker(context.Background(), g, 1, slugCounts)

	if result.Success {
		t.Error("Expected failure for 'exit 42'")
	}
	if result.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", result.ExitCode)
	}
}

func TestRunner_RunAll_DuplicateSlugs(t *testing.T) {
	tmpDir := t.TempDir()

	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           tmpDir,
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             false,
	}

	// Two guardrails with the same slug
	guardrails := []config.Guardrail{
		{Command: "echo test", FailAction: "APPEND"},
		{Command: "echo test", FailAction: "APPEND"}, // Same command, same slug
	}

	results := runner.RunAll(context.Background(), guardrails, 1)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Log files should be different
	if results[0].LogFile == results[1].LogFile {
		t.Errorf("Expected different log files, but both are %q", results[0].LogFile)
	}
}

func TestGetFailedOutputForPrompt_NoFailures(t *testing.T) {
	results := []Result{
		{Success: true},
		{Success: true},
	}

	output := GetFailedOutputForPrompt(results, 1000)

	if output != "" {
		t.Errorf("Expected empty output for all passing, got %q", output)
	}
}

func TestGetFailedOutputForPrompt_WithTruncation(t *testing.T) {
	longOutput := "this is a very long error message that should get truncated"
	results := []Result{
		{
			Guardrail: config.Guardrail{Command: "cmd"},
			Output:    longOutput,
			Success:   false,
		},
	}

	output := GetFailedOutputForPrompt(results, 10)

	if !strings.Contains(output, "[truncated]") {
		t.Error("Expected output to be truncated")
	}
}

func TestGenerateSlug_LeadingTrailingUnderscores(t *testing.T) {
	// Test that leading/trailing special chars result in trimmed underscore
	tests := []struct {
		command string
		want    string
	}{
		{"./script.sh", "script_sh"},
		{"___test___", "test"},
		{"@#$test@#$", "test"},
	}

	for _, tt := range tests {
		got := GenerateSlug(tt.command)
		if got != tt.want {
			t.Errorf("GenerateSlug(%q) = %q, want %q", tt.command, got, tt.want)
		}
	}
}

func TestRunner_Run_WithVerbose(t *testing.T) {
	tmpDir := t.TempDir()

	var stderr bytes.Buffer
	runner := &Runner{
		OutputDir:           tmpDir,
		OutputTruncateChars: 1000,
		Stdout:              &bytes.Buffer{},
		Stderr:              &stderr,
		Verbose:             true,
	}

	g := config.Guardrail{
		Command:    "echo verbose",
		FailAction: "APPEND",
	}

	result := runner.Run(context.Background(), g, 1)

	if !result.Success {
		t.Error("Expected success")
	}
	// Verbose mode should print start/end messages
	if !strings.Contains(stderr.String(), "Guardrail start") {
		t.Error("Expected 'Guardrail start' in verbose output")
	}
}
