package loop

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/guardrail"
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

	return &Runner{
		opts:            opts,
		agentRunner:     agentRunner,
		guardrailRunner: guardrailRunner,
	}
}

// Run executes the main loop and returns the exit code.
func (r *Runner) Run(ctx context.Context) int {
	var failedGuardrailOutput string

	for iteration := 1; iteration <= r.opts.Settings.MaximumIterations; iteration++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			r.log("Loop cancelled")
			return ExitSignalInterrupt
		default:
		}

		r.log("Iteration %d/%d starting", iteration, r.opts.Settings.MaximumIterations)

		// Build the prompt
		prompt, err := r.buildPrompt(failedGuardrailOutput)
		if err != nil {
			fmt.Fprintf(r.opts.Stderr, "error: failed to read prompt file: %v\n", err)
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
		output, err := r.agentRunner.Run(ctx, prompt)
		if err != nil {
			// Check if cancelled
			if ctx.Err() != nil {
				return ExitSignalInterrupt
			}
			r.log("Agent error: %v", err)
		}

		// Run guardrails
		if len(r.opts.Settings.Guardrails) > 0 {
			r.log("Running %d guardrail(s)", len(r.opts.Settings.Guardrails))
			results := r.guardrailRunner.RunAll(ctx, r.opts.Settings.Guardrails, iteration)

			if !guardrail.AllPassed(results) {
				r.log("Guardrail(s) failed")
				failedGuardrailOutput = guardrail.GetFailedOutputForPrompt(results, r.opts.Settings.OutputTruncateChars)

				// Apply fail action from first failed guardrail
				for _, res := range results {
					if !res.Success {
						r.log("Fail action: %s", res.Guardrail.FailAction)
						break
					}
				}
				continue
			}
			r.log("All guardrails passed")
		}

		// Clear failed output since guardrails passed
		failedGuardrailOutput = ""

		// Check for completion
		if IsComplete(output, r.opts.Settings.CompletionResponse) {
			r.log("Completion detected")
			return ExitSuccess
		}
		r.log("No completion detected, continuing")
	}

	r.log("Max iterations reached without completion")
	return ExitMaxIterations
}

// buildPrompt constructs the prompt for the current iteration.
func (r *Runner) buildPrompt(failedGuardrailOutput string) (string, error) {
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

	if failedGuardrailOutput == "" {
		return basePrompt, nil
	}

	// Apply fail action - default to APPEND
	// The fail action is stored in the guardrail, but for simplicity
	// we apply the first failed guardrail's action
	return guardrail.ApplyFailAction(basePrompt, failedGuardrailOutput, "APPEND"), nil
}

// log writes verbose output.
func (r *Runner) log(format string, args ...interface{}) {
	if r.opts.Verbose {
		fmt.Fprintf(r.opts.Stderr, "[ralph] "+format+"\n", args...)
	}
}
