package loop

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/guardrail"
	"github.com/richclement/ralph-cli/internal/response"
	"github.com/richclement/ralph-cli/internal/review"
	"github.com/richclement/ralph-cli/internal/scm"
)

// ExitCode constants per PRD.
const (
	ExitSuccess         = 0
	ExitMaxIterations   = 1
	ExitConfigError     = 2
	ExitSignalInterrupt = 130
)

// Options configures the loop.
type Options struct {
	Prompt     string
	PromptFile string
	Settings   *config.Settings
	Verbose    bool
	Stdout     io.Writer
	Stderr     io.Writer
}

// Runner orchestrates the main loop.
type Runner struct {
	opts            Options
	agentRunner     *agent.Runner
	guardrailRunner *guardrail.Runner
	reviewRunner    *review.Runner
	scmRunner       *scm.Runner
}

// NewRunner creates a loop runner.
func NewRunner(opts Options) *Runner {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	// Enable response debug logging in verbose mode
	if opts.Verbose {
		response.DebugLog = log.New(opts.Stderr, "[response-debug] ", 0)
	}

	agentRunner := agent.NewRunner(opts.Settings)
	agentRunner.Stdout = opts.Stdout
	agentRunner.Stderr = opts.Stderr
	agentRunner.Verbose = opts.Verbose

	guardrailRunner := guardrail.NewRunner(opts.Settings.OutputTruncateChars, opts.Verbose)
	guardrailRunner.Stdout = opts.Stdout
	guardrailRunner.Stderr = opts.Stderr

	scmRunner := scm.NewRunner(opts.Settings, opts.Verbose)
	scmRunner.Stdout = opts.Stdout
	scmRunner.Stderr = opts.Stderr

	reviewRunner := review.NewRunner(opts.Settings, agentRunner, guardrailRunner, opts.Verbose)
	reviewRunner.Stdout = opts.Stdout
	reviewRunner.Stderr = opts.Stderr

	return &Runner{
		opts:            opts,
		agentRunner:     agentRunner,
		guardrailRunner: guardrailRunner,
		reviewRunner:    reviewRunner,
		scmRunner:       scmRunner,
	}
}

// Run executes the main loop and returns the exit code.
func (r *Runner) Run(ctx context.Context) int {
	var failedResults []guardrail.Result
	var loopsSinceLastReview int

	// Check if reviews are enabled
	reviewsEnabled := r.opts.Settings.Reviews != nil && r.opts.Settings.Reviews.ReviewsEnabled()
	var reviewAfter int
	if reviewsEnabled {
		reviewAfter = r.opts.Settings.Reviews.ReviewAfter
	}

	for iteration := 1; iteration <= r.opts.Settings.MaximumIterations; iteration++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			r.log("Loop cancelled")
			return ExitSignalInterrupt
		default:
		}

		r.print("\n=== Ralph iteration %d/%d ===", iteration, r.opts.Settings.MaximumIterations)

		// Build the prompt
		prompt, err := r.buildPrompt(failedResults, iteration)
		if err != nil {
			_, _ = fmt.Fprintf(r.opts.Stderr, "error: failed to read prompt file: %v\n", err)
			return ExitConfigError
		}

		if r.opts.Verbose {
			preview := prompt
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			r.log("Prompt: %s", preview)
		}

		// Run agent
		r.log("Running agent: %s", r.opts.Settings.Agent.Command)
		output, err := r.agentRunner.Run(ctx, prompt, iteration)
		if err != nil {
			// Check if cancelled
			if ctx.Err() != nil {
				return ExitSignalInterrupt
			}
			// Agent failure is fatal - exit with error
			_, _ = fmt.Fprintf(r.opts.Stderr, "error: agent failed: %v\n", err)
			return ExitConfigError
		}

		// Run guardrails
		if len(r.opts.Settings.Guardrails) > 0 {
			r.log("Running %d guardrail(s)", len(r.opts.Settings.Guardrails))
			results := r.guardrailRunner.RunAll(ctx, r.opts.Settings.Guardrails, iteration)

			if !guardrail.AllPassed(results) {
				r.log("Guardrail(s) failed")
				// Store failed results for next iteration's prompt building
				failedResults = guardrail.GetFailedResults(results)
				for _, res := range failedResults {
					r.log("Fail action: %s for %s", res.Guardrail.FailAction, res.Guardrail.Command)
				}
				loopsSinceLastReview++
				continue
			}
			r.print("All guardrails passed")
		}

		// Clear failed results since guardrails passed
		failedResults = nil

		// Run review cycle if configured and threshold reached
		// Note: we only reach here if guardrails passed (or there are none)
		if reviewsEnabled && loopsSinceLastReview >= reviewAfter {
			r.print("\n--- Starting review cycle ---")
			stats, err := r.reviewRunner.Run(ctx, iteration)
			if err != nil {
				if ctx.Err() != nil {
					return ExitSignalInterrupt
				}
				r.log("Review cycle error: %v", err)
			} else {
				r.log("Review cycle complete: %d prompts, %d retries, %d guardrail fails",
					stats.PromptsRun, stats.TotalRetries, stats.GuardrailFails)
			}
			r.print("--- Review cycle complete ---\n")
			loopsSinceLastReview = 0
		}

		// Run SCM tasks after guardrails pass (and reviews if any)
		if r.opts.Settings.SCM != nil {
			r.log("Running SCM tasks")
			if err := r.scmRunner.Run(ctx, iteration); err != nil {
				r.log("SCM error: %v", err)
				// SCM errors don't stop the loop, just log them
			}
		}

		// Check for completion
		r.log("Checking completion: output_len=%d, completion_response=%q", len(output), r.opts.Settings.CompletionResponse)
		if response.IsComplete(output, r.opts.Settings.CompletionResponse) {
			r.print("Completion response matched")
			return ExitSuccess
		}
		r.log("No completion detected, continuing")

		loopsSinceLastReview++
	}

	r.print("Maximum iterations reached without completion response.")
	return ExitMaxIterations
}

// buildPrompt constructs the prompt for the current iteration.
func (r *Runner) buildPrompt(failedResults []guardrail.Result, iteration int) (string, error) {
	var basePrompt string

	if r.opts.PromptFile != "" {
		// Re-read file each iteration per PRD
		data, err := os.ReadFile(r.opts.PromptFile)
		if err != nil {
			return "", err
		}
		basePrompt = string(data)
	} else {
		basePrompt = r.opts.Prompt
	}

	prompt := basePrompt
	if len(failedResults) > 0 {
		// Apply each failed guardrail's action sequentially (matching Python behavior)
		for _, result := range failedResults {
			failureMessage := guardrail.FormatFailureMessage(result, r.opts.Settings.OutputTruncateChars)
			prompt = guardrail.ApplyFailAction(prompt, failureMessage, result.Guardrail.FailAction)
		}
	}

	if r.opts.Settings.IncludeIterationCountInPrompt {
		remaining := r.opts.Settings.MaximumIterations - iteration
		summary := fmt.Sprintf("Iteration %d of %d, %d remaining.", iteration, r.opts.Settings.MaximumIterations, remaining)
		prompt = summary + "\n\n" + prompt
	}
	return prompt, nil
}

// log writes verbose/debug output.
func (r *Runner) log(format string, args ...any) {
	if r.opts.Verbose {
		_, _ = fmt.Fprintf(r.opts.Stderr, "[ralph] "+format+"\n", args...)
	}
}

// print writes status output that should always be visible.
func (r *Runner) print(format string, args ...any) {
	_, _ = fmt.Fprintf(r.opts.Stderr, format+"\n", args...)
}
