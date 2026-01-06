package scm

import (
	"testing"
)

func TestExtractCommitMessage(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "response tag",
			output: "Some output\n<response>Fix the bug in parser</response>\nMore output",
			want:   "Fix the bug in parser",
		},
		{
			name:   "response tag with whitespace",
			output: "<response>  Add feature  </response>",
			want:   "Add feature",
		},
		{
			name:   "first non-empty line",
			output: "\n\nUpdate dependencies\nMore details here",
			want:   "Update dependencies",
		},
		{
			name:   "single line",
			output: "Simple commit message",
			want:   "Simple commit message",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
		{
			name:   "only whitespace",
			output: "  \n  \n  ",
			want:   "",
		},
		{
			name:   "response tag preferred over first line",
			output: "Agent thinking...\nAnalysis:\n<response>Refactor module</response>",
			want:   "Refactor module",
		},
		{
			name:   "multiline response tag content",
			output: "<response>Add feature\nwith details</response>",
			want:   "Add feature\nwith details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommitMessage(tt.output)
			if got != tt.want {
				t.Errorf("extractCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
