package response

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
)

// DebugLog is an optional logger for debugging completion detection.
// Set this from the caller to enable debug output.
var DebugLog *log.Logger

func debugf(format string, args ...interface{}) {
	if DebugLog != nil {
		DebugLog.Printf(format, args...)
	}
}

// responseTagRegex matches <response>...</response> tags.
// (?i) = case insensitive, (?s) = DOTALL (. matches newlines)
var responseTagRegex = regexp.MustCompile(`(?is)<response>(.*?)</response>`)

// resultMessage matches Claude's result JSON structure
type resultMessage struct {
	Type   string `json:"type"`
	Result string `json:"result"`
}

type assistantMessage struct {
	Type    string `json:"type"`
	Message *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
	} `json:"message,omitempty"`
}

// ExtractResponse extracts content from the first <response> tag.
// Returns the content and whether a tag was found.
func ExtractResponse(output string) (string, bool) {
	matches := responseTagRegex.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

// ExtractFromJSON finds the last {"type":"result"} message and extracts the result field.
// This is used for stream-json mode where there are no <response> tags.
func ExtractFromJSON(output string) (string, bool) {
	debugf("ExtractFromJSON: scanning %d bytes of output", len(output))

	// Scan backwards for efficiency - result is always last
	lines := strings.Split(output, "\n")
	debugf("ExtractFromJSON: found %d lines", len(lines))

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Quick check before parsing
		if !strings.Contains(line, `"type":"result"`) {
			continue
		}

		debugf("ExtractFromJSON: found result line at index %d, length=%d", i, len(line))

		var msg resultMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			debugf("ExtractFromJSON: JSON unmarshal error: %v", err)
			continue
		}

		if msg.Type == "result" {
			debugf("ExtractFromJSON: extracted result field, length=%d", len(msg.Result))
			debugf("ExtractFromJSON: result preview (last 200 chars): %s", lastN(msg.Result, 200))
			return msg.Result, true
		}
	}

	// Fall back to assistant text content for <response> tags
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if !strings.Contains(line, `"type":"assistant"`) {
			continue
		}

		var msg assistantMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "assistant" || msg.Message == nil {
			continue
		}
		for _, content := range msg.Message.Content {
			if content.Type != "text" || content.Text == "" {
				continue
			}
			if resp, found := ExtractResponse(content.Text); found {
				return resp, true
			}
		}
	}
	return "", false
}

// IsComplete checks if agent output indicates completion.
// Requires stream-json mode output with a {"type":"result"} message.
// Matches if result ends with completionResponse (case-insensitive).
func IsComplete(output, completionResponse string) bool {
	debugf("IsComplete: checking for completionResponse=%q", completionResponse)

	// Extract result from JSON (stream-json mode)
	result, found := ExtractFromJSON(output)
	if !found {
		debugf("IsComplete: no JSON result found")
		return false
	}

	// Check if result ends with completion response (case-insensitive)
	result = strings.TrimSpace(result)
	expected := strings.TrimSpace(completionResponse)
	match := strings.HasSuffix(strings.ToLower(result), strings.ToLower(expected))
	debugf("IsComplete: ends-with check=%v", match)
	return match
}

// lastN returns the last n characters of s, or s if shorter.
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
