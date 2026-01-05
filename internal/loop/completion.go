package loop

import (
	"regexp"
	"strings"
)

var responseTagRegex = regexp.MustCompile(`(?i)<response>(.*?)</response>`)

// ExtractResponse extracts content from the first <response> tag.
// Returns the content and whether a tag was found.
func ExtractResponse(output string) (string, bool) {
	matches := responseTagRegex.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

// IsComplete checks if the agent output indicates completion.
// The match is case-insensitive.
func IsComplete(output, completionResponse string) bool {
	response, found := ExtractResponse(output)
	if !found {
		return false
	}
	return strings.EqualFold(response, completionResponse)
}
