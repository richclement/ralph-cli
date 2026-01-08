package guardrail

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	ExitCode  int
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
// Use RunWithSlugTracker to avoid log file collisions when running multiple guardrails.
func (r *Runner) Run(ctx context.Context, g config.Guardrail, iteration int) Result {
	return r.RunWithSlugTracker(ctx, g, iteration, nil)
}

// RunWithSlugTracker executes a single guardrail with slug collision tracking.
// The slugCounts map tracks how many times each slug has been used.
func (r *Runner) RunWithSlugTracker(ctx context.Context, g config.Guardrail, iteration int, slugCounts map[string]int) Result {
	result := Result{
		Guardrail: g,
	}

	r.print("Guardrail start: %s", g.Command)

	// Run the command
	output, err := agent.RunShell(ctx, g.Command, false, r.Stdout, r.Stderr)
	result.Output = output
	result.Success = err == nil

	// Extract exit code from error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1 // Default to 1 for non-exit errors
		}
	}

	// Write full output to log file with de-duplication
	slug := GenerateSlug(g.Command)
	logFile := r.generateLogFilename(iteration, slug, slugCounts)
	if writeErr := os.WriteFile(logFile, []byte(output), 0o644); writeErr != nil {
		_, _ = fmt.Fprintf(r.Stderr, "[ralph] warning: failed to write guardrail log: %v\n", writeErr)
	}
	result.LogFile = logFile

	if result.Success {
		r.print("Guardrail end: %s (exit 0)", g.Command)
	} else {
		r.print("Guardrail end: %s (exit %d, action=%s)", g.Command, result.ExitCode, g.FailAction)
	}

	return result
}

// generateLogFilename creates a unique log filename, handling duplicate slugs.
func (r *Runner) generateLogFilename(iteration int, slug string, slugCounts map[string]int) string {
	if slugCounts == nil {
		// No tracking, use simple filename
		return filepath.Join(r.OutputDir, fmt.Sprintf("guardrail_%03d_%s.log", iteration, slug))
	}

	count := slugCounts[slug]
	slugCounts[slug] = count + 1

	if count == 0 {
		// First occurrence, no suffix needed
		return filepath.Join(r.OutputDir, fmt.Sprintf("guardrail_%03d_%s.log", iteration, slug))
	}
	// Duplicate slug, add suffix
	return filepath.Join(r.OutputDir, fmt.Sprintf("guardrail_%03d_%s_%d.log", iteration, slug, count))
}

// print writes status output that should always be visible.
func (r *Runner) print(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(r.Stderr, format+"\n", args...)
}

// RunAll executes all guardrails and returns results.
// Handles duplicate slug collisions by adding index suffixes to log filenames.
func (r *Runner) RunAll(ctx context.Context, guardrails []config.Guardrail, iteration int) []Result {
	results := make([]Result, 0, len(guardrails))
	slugCounts := make(map[string]int)
	for _, g := range guardrails {
		result := r.RunWithSlugTracker(ctx, g, iteration, slugCounts)
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
// Deprecated: Use GetFailedResults and FormatFailureMessage instead for proper failAction handling.
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

// GetFailedResults returns only the failed results from a list of guardrail results.
func GetFailedResults(results []Result) []Result {
	var failed []Result
	for _, r := range results {
		if !r.Success {
			failed = append(failed, r)
		}
	}
	return failed
}

// FormatFailureMessage formats a single guardrail failure for inclusion in the prompt.
// Includes exit code, log file path, optional hint, and truncated output.
func FormatFailureMessage(result Result, truncateLimit int) string {
	truncated := TruncateOutput(result.Output, truncateLimit)

	// Build the message with optional hint
	var hintLine string
	if result.Guardrail.Hint != "" {
		hintLine = fmt.Sprintf("Hint: %s\n", result.Guardrail.Hint)
	}

	return fmt.Sprintf(`Guardrail "%s" failed with exit code %d.
%sOutput file: %s
Output (truncated):
%s`, result.Guardrail.Command, result.ExitCode, hintLine, result.LogFile, truncated)
}
