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
			name:               "json result ends with completion",
			output:             `{"type":"result","result":"Task completed successfully. DONE"}`,
			completionResponse: "DONE",
			want:               true,
		},
		{
			name:               "json result ends with completion case insensitive",
			output:             `{"type":"result","result":"All tasks finished. done"}`,
			completionResponse: "DONE",
			want:               true,
		},
		{
			name:               "json result does not end with completion",
			output:             `{"type":"result","result":"I'm not DONE yet, more work needed"}`,
			completionResponse: "DONE",
			want:               false,
		},
		{
			name:               "json result multi-word completion",
			output:             `{"type":"result","result":"Review complete. Task completed successfully"}`,
			completionResponse: "Task completed successfully",
			want:               true,
		},
		{
			name:               "json result exact match",
			output:             `{"type":"result","result":"DONE"}`,
			completionResponse: "DONE",
			want:               true,
		},
		{
			name:               "no json result",
			output:             "DONE",
			completionResponse: "DONE",
			want:               false,
		},
		{
			name:               "completion response is literal string",
			output:             `{"type":"result","result":"Finished. <response>DONE</response>"}`,
			completionResponse: "<response>DONE</response>",
			want:               true,
		},
		{
			name:               "tags in result not special",
			output:             `{"type":"result","result":"<response>DONE</response>"}`,
			completionResponse: "DONE",
			want:               false,
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
			name:   "assistant text response tag",
			output: `{"type":"assistant","message":{"content":[{"type":"text","text":"done\\n<response>DONE</response>"}]}}`,
			want:   "DONE",
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

func TestIsComplete_LongResult(t *testing.T) {
	// Long markdown response ending with completion string
	longResult := `## Summary\n\nThis is a very long code review with lots of content...\n\n`
	longResult += `### Issues Found\n\n1. Issue one\n2. Issue two\n\n`
	longResult += `### Recommendation\n\nApprove with minor changes.\n\nDONE`

	output := `{"type":"result","result":"` + longResult + `"}`

	if !IsComplete(output, "DONE") {
		t.Error("IsComplete should detect DONE at end of long JSON result")
	}
}

func TestExtractFromJSON_Codex(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
		found  bool
	}{
		{
			name: "codex turn completed with agent_message",
			output: `{"type":"thread.started","thread_id":"abc123"}
{"type":"turn.started"}
{"type":"item.started","item":{"id":"msg_1","type":"agent_message"}}
{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"Task completed. DONE"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`,
			want:  "Task completed. DONE",
			found: true,
		},
		{
			name: "codex turn completed without agent_message",
			output: `{"type":"turn.started"}
{"type":"item.started","item":{"id":"cmd_1","type":"command_execution","command":"ls"}}
{"type":"item.completed","item":{"id":"cmd_1","type":"command_execution","exit_code":0}}
{"type":"turn.completed","usage":{"input_tokens":50,"output_tokens":25}}`,
			want:  "",
			found: true,
		},
		{
			name: "codex multiple agent_messages takes last",
			output: `{"type":"turn.started"}
{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"First message"}}
{"type":"item.completed","item":{"id":"msg_2","type":"agent_message","text":"Final response. DONE"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":75}}`,
			want:  "Final response. DONE",
			found: true,
		},
		{
			name: "codex no turn.completed - not complete",
			output: `{"type":"turn.started"}
{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"Still working..."}}`,
			want:  "",
			found: false,
		},
		{
			name: "codex reasoning item ignored",
			output: `{"type":"turn.started"}
{"type":"item.completed","item":{"id":"rsn_1","type":"reasoning","text":"Thinking about this..."}}
{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"Done thinking. DONE"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`,
			want:  "Done thinking. DONE",
			found: true,
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

func TestIsComplete_Codex(t *testing.T) {
	// Full Codex output with completion response
	output := `{"type":"thread.started","thread_id":"abc123"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"I've completed the task. DONE"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`

	if !IsComplete(output, "DONE") {
		t.Error("IsComplete should detect DONE in Codex agent_message")
	}

	if !IsComplete(output, "done") {
		t.Error("IsComplete should be case-insensitive for Codex")
	}

	if IsComplete(output, "FINISHED") {
		t.Error("IsComplete should return false for non-matching completion")
	}
}

func TestExtractFromJSON_Amp(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
		found  bool
	}{
		{
			name:   "amp success result",
			output: `{"type":"system","subtype":"init","session_id":"test"}` + "\n" + `{"type":"result","subtype":"success","result":"Task completed. DONE","duration_ms":1000}`,
			want:   "Task completed. DONE",
			found:  true,
		},
		{
			name:   "amp error result",
			output: `{"type":"result","subtype":"error_during_execution","error":"Command failed","is_error":true}`,
			want:   "",
			found:  true,
		},
		{
			name:   "amp with assistant messages",
			output: `{"type":"assistant","message":{"content":[{"type":"text","text":"Working on it..."}]}}` + "\n" + `{"type":"result","subtype":"success","result":"All done. DONE"}`,
			want:   "All done. DONE",
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

func TestIsComplete_Amp(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"test"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Let me help."}]}}
{"type":"result","subtype":"success","result":"Task finished. DONE","duration_ms":500}`

	if !IsComplete(output, "DONE") {
		t.Error("IsComplete should detect DONE in Amp result")
	}

	if !IsComplete(output, "done") {
		t.Error("IsComplete should be case-insensitive for Amp")
	}
}
