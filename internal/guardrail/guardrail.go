package guardrail

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
)

const (
	// FailAction types
	ActionAppend  = "APPEND"
	ActionPrepend = "PREPEND"
	ActionReplace = "REPLACE"

	truncateIndicator = "... [truncated]"
)

// Result holds the outcome of running a guardrail.
type Result struct {
	Guardrail config.Guardrail
	Output    string
	Success   bool
	LogFile   string
}

// Runner executes guardrails.
type Runner struct {
	OutputDir           string
	OutputTruncateChars int
	Stdout              io.Writer
	Stderr              io.Writer
	Verbose             bool
}

// NewRunner creates a new guardrail runner.
func NewRunner(outputTruncateChars int, verbose bool) *Runner {
	return &Runner{
		OutputDir:           ".ralph",
		OutputTruncateChars: outputTruncateChars,
		Stdout:              os.Stdout,
		Stderr:              os.Stderr,
		Verbose:             verbose,
	}
}

// Run executes a single guardrail and returns the result.
func (r *Runner) Run(ctx context.Context, g config.Guardrail, iteration int) Result {
	result := Result{
		Guardrail: g,
	}

	// Run the command
	output, err := agent.RunShell(ctx, g.Command, false, r.Stdout, r.Stderr)
	result.Output = output
	result.Success = err == nil

	// Write full output to log file
	slug := GenerateSlug(g.Command)
	logFile := filepath.Join(r.OutputDir, fmt.Sprintf("guardrail_%d_%s.log", iteration, slug))
	if writeErr := os.WriteFile(logFile, []byte(output), 0644); writeErr != nil {
		fmt.Fprintf(r.Stderr, "[ralph] warning: failed to write guardrail log: %v\n", writeErr)
	}
	result.LogFile = logFile

	return result
}

// RunAll executes all guardrails and returns results.
func (r *Runner) RunAll(ctx context.Context, guardrails []config.Guardrail, iteration int) []Result {
	results := make([]Result, 0, len(guardrails))
	for _, g := range guardrails {
		result := r.Run(ctx, g, iteration)
		results = append(results, result)
	}
	return results
}

// AllPassed returns true if all guardrail results were successful.
func AllPassed(results []Result) bool {
	for _, r := range results {
		if !r.Success {
			return false
		}
	}
	return true
}

// ApplyFailAction applies the fail action to construct the next prompt.
func ApplyFailAction(basePrompt, failedOutput, action string) string {
	switch strings.ToUpper(action) {
	case ActionPrepend:
		return failedOutput + "\n\n" + basePrompt
	case ActionReplace:
		return failedOutput
	case ActionAppend:
		fallthrough
	default:
		return basePrompt + "\n\n" + failedOutput
	}
}

// TruncateOutput truncates output to the specified limit and adds indicator.
func TruncateOutput(output string, limit int) string {
	if limit <= 0 || len(output) <= limit {
		return output
	}
	return output[:limit] + truncateIndicator
}

// GenerateSlug creates a filesystem-safe slug from a command string.
func GenerateSlug(command string) string {
	// Replace non-alphanumeric characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	slug := re.ReplaceAllString(command, "_")

	// Trim leading/trailing underscores
	slug = strings.Trim(slug, "_")

	// Truncate to 50 chars
	if len(slug) > 50 {
		slug = slug[:50]
	}

	return slug
}

// GetFailedOutputForPrompt returns truncated failed output formatted for the next prompt.
func GetFailedOutputForPrompt(results []Result, truncateLimit int) string {
	var parts []string
	for _, r := range results {
		if !r.Success {
			truncated := TruncateOutput(r.Output, truncateLimit)
			parts = append(parts, fmt.Sprintf("Guardrail failed: %s\nOutput:\n%s", r.Guardrail.Command, truncated))
		}
	}
	return strings.Join(parts, "\n\n")
}
