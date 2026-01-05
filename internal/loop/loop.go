package loop

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/guardrail"
	"github.com/richclement/ralph-cli/internal/response"
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

	agentRunner := agent.NewRunner(opts.Settings)
	agentRunner.Stdout = opts.Stdout
	agentRunner.Stderr = opts.Stderr

	guardrailRunner := guardrail.NewRunner(opts.Settings.OutputTruncateChars, opts.Verbose)
	guardrailRunner.Stdout = opts.Stdout
	guardrailRunner.Stderr = opts.Stderr

	scmRunner := scm.NewRunner(opts.Settings, opts.Verbose)
	scmRunner.Stdout = opts.Stdout
	scmRunner.Stderr = opts.Stderr

	return &Runner{
		opts:            opts,
		agentRunner:     agentRunner,
		guardrailRunner: guardrailRunner,
		scmRunner:       scmRunner,
	}
}

// Run executes the main loop and returns the exit code.
func (r *Runner) Run(ctx context.Context) int {
	var failedResults []guardrail.Result

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
		prompt, err := r.buildPrompt(failedResults)
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
				continue
			}
			r.print("All guardrails passed")
		}

		// Clear failed results since guardrails passed
		failedResults = nil

		// Run SCM tasks after guardrails pass (before checking completion)
		if r.opts.Settings.SCM != nil {
			r.log("Running SCM tasks")
			if err := r.scmRunner.Run(ctx, iteration); err != nil {
				r.log("SCM error: %v", err)
				// SCM errors don't stop the loop, just log them
			}
		}

		// Check for completion
		if response.IsComplete(output, r.opts.Settings.CompletionResponse) {
			r.print("Completion response matched")
			return ExitSuccess
		}
		r.log("No completion detected, continuing")
	}

	r.print("Maximum iterations reached without completion response.")
	return ExitMaxIterations
}

// buildPrompt constructs the prompt for the current iteration.
func (r *Runner) buildPrompt(failedResults []guardrail.Result) (string, error) {
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

	if len(failedResults) == 0 {
		return basePrompt, nil
	}

	// Apply each failed guardrail's action sequentially (matching Python behavior)
	prompt := basePrompt
	for _, result := range failedResults {
		failureMessage := guardrail.FormatFailureMessage(result, r.opts.Settings.OutputTruncateChars)
		prompt = guardrail.ApplyFailAction(prompt, failureMessage, result.Guardrail.FailAction)
	}
	return prompt, nil
}

// log writes verbose/debug output.
func (r *Runner) log(format string, args ...interface{}) {
	if r.opts.Verbose {
		_, _ = fmt.Fprintf(r.opts.Stderr, "[ralph] "+format+"\n", args...)
	}
}

// print writes status output that should always be visible.
func (r *Runner) print(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(r.opts.Stderr, format+"\n", args...)
}
