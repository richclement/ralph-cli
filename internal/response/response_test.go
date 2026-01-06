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

func TestExtractFromJSON(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
		found  bool
	}{
		{
			name:   "valid result",
			output: `{"type":"system"}` + "\n" + `{"type":"result","result":"done","total_cost_usd":0.01}`,
			want:   "done",
			found:  true,
		},
		{
			name:   "result in middle",
			output: "line1\n{\"type\":\"result\",\"result\":\"complete\"}\nline3",
			want:   "complete",
			found:  true,
		},
		{
			name:   "no result",
			output: `{"type":"assistant"}` + "\n" + `{"type":"user"}`,
			want:   "",
			found:  false,
		},
		{
			name:   "malformed json",
			output: `{not json}` + "\n" + `{"type":"result"`,
			want:   "",
			found:  false,
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
			found:  false,
		},
		{
			name:   "result with extra fields",
			output: `{"type":"result","subtype":"success","result":"DONE","total_cost_usd":0.0234}`,
			want:   "DONE",
			found:  true,
		},
		{
			name:   "multiple results takes last",
			output: `{"type":"result","result":"first"}` + "\n" + `{"type":"result","result":"last"}`,
			want:   "last",
			found:  true,
		},
		{
			name:   "whitespace around lines",
			output: "  {\"type\":\"result\",\"result\":\"done\"}  \n",
			want:   "done",
			found:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := ExtractFromJSON(tt.output)
			if found != tt.found {
				t.Errorf("ExtractFromJSON() found = %v, want %v", found, tt.found)
			}
			if got != tt.want {
				t.Errorf("ExtractFromJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsComplete_JSON(t *testing.T) {
	output := `{"type":"assistant"}` + "\n" + `{"type":"result","result":"done"}`

	if !IsComplete(output, "done") {
		t.Error("IsComplete should return true for matching result")
	}

	if !IsComplete(output, "DONE") {
		t.Error("IsComplete should be case-insensitive")
	}

	if IsComplete(output, "other") {
		t.Error("IsComplete should return false for non-matching result")
	}
}

func TestIsComplete_Fallback(t *testing.T) {
	// No JSON result, should fall back to <response> regex
	output := "Some output\n<response>completed</response>"

	if !IsComplete(output, "completed") {
		t.Error("IsComplete should fall back to <response> extraction")
	}
}

func TestIsComplete_JSONPriority(t *testing.T) {
	// Both JSON and <response> present - JSON should take priority
	output := `{"type":"result","result":"json_result"}` + "\n<response>xml_result</response>"

	if !IsComplete(output, "json_result") {
		t.Error("IsComplete should use JSON result when present")
	}

	if IsComplete(output, "xml_result") {
		t.Error("IsComplete should prefer JSON over <response> tag")
	}
}
