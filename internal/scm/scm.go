package scm

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/response"
)

const commitMessagePrompt = "Provide a short imperative commit message for the changes. Output only the message, no explanation."

// Runner executes SCM tasks.
type Runner struct {
	Settings    *config.Settings
	AgentRunner *agent.Runner
	Stdout      io.Writer
	Stderr      io.Writer
	Verbose     bool
}

// NewRunner creates an SCM runner.
func NewRunner(settings *config.Settings, verbose bool) *Runner {
	agentRunner := agent.NewRunner(settings)

	return &Runner{
		Settings:    settings,
		AgentRunner: agentRunner,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Verbose:     verbose,
	}
}

// Run executes SCM tasks if configured.
// The iteration parameter is used for naming prompt files (for Codex).
func (r *Runner) Run(ctx context.Context, iteration int) error {
	if r.Settings.SCM == nil || len(r.Settings.SCM.Tasks) == 0 {
		return nil
	}

	for _, task := range r.Settings.SCM.Tasks {
		r.log("Running SCM task: %s", task)

		if err := r.runTask(ctx, task, iteration); err != nil {
			return fmt.Errorf("SCM task %q failed: %w", task, err)
		}
	}

	return nil
}

// runTask executes a single SCM task.
func (r *Runner) runTask(ctx context.Context, task string, iteration int) error {
	switch strings.ToLower(task) {
	case "commit":
		return r.runCommit(ctx, iteration)
	case "push":
		return r.runPush(ctx)
	default:
		// Generic task execution
		cmd := fmt.Sprintf("%s %s", r.Settings.SCM.Command, task)
		_, err := agent.RunShell(ctx, cmd, true, r.Stdout, r.Stderr)
		return err
	}
}

// runCommit gets a commit message from the agent and commits.
func (r *Runner) runCommit(ctx context.Context, iteration int) error {
	r.log("Getting commit message from agent")

	// Get commit message from agent
	output, err := r.AgentRunner.Run(ctx, commitMessagePrompt, iteration)
	if err != nil {
		return fmt.Errorf("failed to get commit message: %w", err)
	}

	message := extractCommitMessage(output)
	if message == "" {
		return fmt.Errorf("agent did not provide a valid commit message")
	}

	r.log("Commit message: %s", message)

	// Run commit
	cmd := fmt.Sprintf("%s commit -am %q", r.Settings.SCM.Command, message)
	_, err = agent.RunShell(ctx, cmd, true, r.Stdout, r.Stderr)
	return err
}

// runPush pushes to remote.
func (r *Runner) runPush(ctx context.Context) error {
	cmd := fmt.Sprintf("%s push", r.Settings.SCM.Command)
	_, err := agent.RunShell(ctx, cmd, true, r.Stdout, r.Stderr)
	return err
}

// extractCommitMessage extracts the commit message from agent output.
func extractCommitMessage(output string) string {
	// First check for <response> tag
	if resp, found := response.ExtractResponse(output); found {
		return strings.TrimSpace(resp)
	}

	// Otherwise use first non-empty line
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	return ""
}

// log writes verbose output.
func (r *Runner) log(format string, args ...interface{}) {
	if r.Verbose {
		_, _ = fmt.Fprintf(r.Stderr, "[ralph] "+format+"\n", args...)
	}
}
