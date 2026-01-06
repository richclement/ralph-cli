package response

import (
	"encoding/json"
	"regexp"
	"strings"
)

// responseTagRegex matches <response>...</response> tags.
// (?i) = case insensitive, (?s) = DOTALL (. matches newlines)
var responseTagRegex = regexp.MustCompile(`(?is)<response>(.*?)</response>`)

// resultMessage matches Claude's result JSON structure
type resultMessage struct {
	Type   string `json:"type"`
	Result string `json:"result"`
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
	// Scan backwards for efficiency - result is always last
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Quick check before parsing
		if !strings.Contains(line, `"type":"result"`) {
			continue
		}

		var msg resultMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Type == "result" {
			return msg.Result, true
		}
	}
	return "", false
}

// IsComplete checks if agent output indicates completion.
// For stream-json mode: checks JSON result first.
// For text mode: falls back to <response> regex.
// The match is case-insensitive.
func IsComplete(output, completionResponse string) bool {
	// Try JSON extraction first (stream-json mode)
	if result, found := ExtractFromJSON(output); found {
		return strings.EqualFold(strings.TrimSpace(result), strings.TrimSpace(completionResponse))
	}

	// Fall back to <response> regex if no JSON result found
	if resp, found := ExtractResponse(output); found {
		return strings.EqualFold(strings.TrimSpace(resp), strings.TrimSpace(completionResponse))
	}

	return false
}
