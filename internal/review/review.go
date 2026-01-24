package review

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/guardrail"
)

// Runner executes review cycles.
type Runner struct {
	agentRunner     *agent.Runner
	guardrailRunner *guardrail.Runner
	settings        *config.Settings
	Stdout          io.Writer
	Stderr          io.Writer
	Verbose         bool
}

// NewRunner creates a new review runner.
func NewRunner(settings *config.Settings, agentRunner *agent.Runner, guardrailRunner *guardrail.Runner, verbose bool) *Runner {
	return &Runner{
		agentRunner:     agentRunner,
		guardrailRunner: guardrailRunner,
		settings:        settings,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		Verbose:         verbose,
	}
}

// Stats tracks review cycle statistics.
type Stats struct {
	PromptsRun     int
	TotalRetries   int
	GuardrailFails int
}

// Run executes a full review cycle with all configured prompts.
// Returns stats about the review cycle and any error encountered.
func (r *Runner) Run(ctx context.Context, iteration int) (Stats, error) {
	stats := Stats{}

	if r.settings.Reviews == nil || !r.settings.Reviews.ReviewsEnabled() {
		return stats, nil
	}

	prompts := r.settings.Reviews.GetPrompts()
	retryLimit := r.settings.Reviews.GuardrailRetryLimit

	for _, reviewPrompt := range prompts {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		stats.PromptsRun++
		r.print("[Review: %s] Starting", reviewPrompt.Name)

		currentPrompt := reviewPrompt.Prompt
		retries := 0
		guardrailsPassed := false

		// Retry loop: run agent + guardrails until guardrails pass or retry limit reached
		for !guardrailsPassed {
			// Run agent with review prompt
			r.log("Running agent with review prompt: %s", reviewPrompt.Name)
			_, err := r.agentRunner.Run(ctx, currentPrompt, iteration)
			if err != nil {
				// Check if cancelled
				if ctx.Err() != nil {
					return stats, ctx.Err()
				}
				// Agent failure during review is non-fatal, log and continue
				r.log("Agent error during review %q: %v", reviewPrompt.Name, err)
			}

			// Run guardrails if configured
			if len(r.settings.Guardrails) == 0 {
				guardrailsPassed = true
				break
			}

			results := r.guardrailRunner.RunAll(ctx, r.settings.Guardrails, iteration)

			if guardrail.AllPassed(results) {
				guardrailsPassed = true
				r.print("[Review: %s] Guardrails passed", reviewPrompt.Name)
				break
			}

			// Guardrails failed
			stats.GuardrailFails++
			retries++
			stats.TotalRetries++

			if retries >= retryLimit {
				r.print("[Review: %s] Guardrail retry limit (%d) reached, moving to next review", reviewPrompt.Name, retryLimit)
				break
			}

			// Inject guardrail failures into the review prompt for next attempt
			failedResults := guardrail.GetFailedResults(results)
			currentPrompt = guardrail.InjectFailures(currentPrompt, failedResults, r.settings.OutputTruncateChars)

			r.log("[Review: %s] Guardrails failed, retry %d/%d", reviewPrompt.Name, retries, retryLimit)
		}

		r.print("[Review: %s] Complete", reviewPrompt.Name)
	}

	return stats, nil
}

// log writes verbose/debug output.
func (r *Runner) log(format string, args ...any) {
	if r.Verbose {
		_, _ = fmt.Fprintf(r.Stderr, "[ralph] "+format+"\n", args...)
	}
}

// print writes status output that should always be visible.
func (r *Runner) print(format string, args ...any) {
	_, _ = fmt.Fprintf(r.Stderr, format+"\n", args...)
}
