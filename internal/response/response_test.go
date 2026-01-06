package response

import (
	"testing"
)

func TestExtractResponse(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		want      string
		wantFound bool
	}{
		{
			name:      "simple tag",
			output:    "some output <response>DONE</response> more text",
			want:      "DONE",
			wantFound: true,
		},
		{
			name:      "tag with whitespace",
			output:    "<response>  DONE  </response>",
			want:      "DONE",
			wantFound: true,
		},
		{
			name:      "uppercase tag",
			output:    "<RESPONSE>done</RESPONSE>",
			want:      "done",
			wantFound: true,
		},
		{
			name:      "mixed case tag",
			output:    "<Response>Done</Response>",
			want:      "Done",
			wantFound: true,
		},
		{
			name:      "no tag",
			output:    "just some regular output",
			want:      "",
			wantFound: false,
		},
		{
			name:      "first of multiple tags",
			output:    "<response>FIRST</response> text <response>SECOND</response>",
			want:      "FIRST",
			wantFound: true,
		},
		{
			name:      "empty tag",
			output:    "<response></response>",
			want:      "",
			wantFound: true,
		},
		{
			name:      "multiline content",
			output:    "<response>line1\nline2\nline3</response>",
			want:      "line1\nline2\nline3",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := ExtractResponse(tt.output)
			if found != tt.wantFound {
				t.Errorf("ExtractResponse() found = %v, want %v", found, tt.wantFound)
			}
			if got != tt.want {
				t.Errorf("ExtractResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsComplete(t *testing.T) {
	tests := []struct {
		name               string
		output             string
		completionResponse string
		want               bool
	}{
		{
			name:               "exact match",
			output:             "<response>DONE</response>",
			completionResponse: "DONE",
			want:               true,
		},
		{
			name:               "case insensitive match",
			output:             "<response>done</response>",
			completionResponse: "DONE",
			want:               true,
		},
		{
			name:               "different response",
			output:             "<response>NOT_DONE</response>",
			completionResponse: "DONE",
			want:               false,
		},
		{
			name:               "no tag",
			output:             "DONE",
			completionResponse: "DONE",
			want:               false,
		},
		{
			name:               "custom completion response",
			output:             "<response>FINISHED</response>",
			completionResponse: "FINISHED",
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsComplete(tt.output, tt.completionResponse)
			if got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}
