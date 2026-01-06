package guardrail

import (
	"testing"
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
