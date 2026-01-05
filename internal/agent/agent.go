package agent

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/richclement/ralph-cli/internal/config"
)

// Runner executes agent commands.
type Runner struct {
	Settings *config.Settings
	Stdout   io.Writer // For streaming output, defaults to os.Stdout
	Stderr   io.Writer // For streaming output, defaults to os.Stderr
}

// NewRunner creates a new agent runner.
func NewRunner(settings *config.Settings) *Runner {
	return &Runner{
		Settings: settings,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}

// Run executes the agent with the given prompt and returns the combined output.
func (r *Runner) Run(ctx context.Context, prompt string) (string, error) {
	args := r.buildArgs(prompt)
	cmd := exec.CommandContext(ctx, r.Settings.Agent.Command, args...)

	var outputBuf bytes.Buffer

	if r.Settings.StreamAgentOutput {
		// Tee output to both buffer and console
		cmd.Stdout = io.MultiWriter(&outputBuf, r.Stdout)
		cmd.Stderr = io.MultiWriter(&outputBuf, r.Stderr)
	} else {
		// Capture output only
		cmd.Stdout = &outputBuf
		cmd.Stderr = &outputBuf
	}

	// Set stdin to /dev/null to prevent hanging
	cmd.Stdin = nil

	err := cmd.Run()
	return outputBuf.String(), err
}

// buildArgs constructs the argument list for the agent command.
func (r *Runner) buildArgs(prompt string) []string {
	var args []string

	// Infer non-REPL flag based on command name
	cmdName := filepath.Base(r.Settings.Agent.Command)
	cmdName = strings.TrimSuffix(cmdName, ".exe") // handle Windows

	switch cmdName {
	case "claude":
		args = append(args, "-p")
	case "codex":
		args = append(args, "e")
	case "amp":
		args = append(args, "-x")
	}

	// Add user-configured flags
	args = append(args, r.Settings.Agent.Flags...)

	// Add the prompt
	args = append(args, prompt)

	return args
}

// RunShell executes a shell command and returns combined output.
// Used for guardrails and SCM commands.
func RunShell(ctx context.Context, command string, stream bool, stdout, stderr io.Writer) (string, error) {
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/c"
	} else {
		shell = "sh"
		flag = "-c"
	}

	cmd := exec.CommandContext(ctx, shell, flag, command)

	var outputBuf bytes.Buffer

	if stream {
		cmd.Stdout = io.MultiWriter(&outputBuf, stdout)
		cmd.Stderr = io.MultiWriter(&outputBuf, stderr)
	} else {
		cmd.Stdout = &outputBuf
		cmd.Stderr = &outputBuf
	}

	cmd.Stdin = nil

	err := cmd.Run()
	return outputBuf.String(), err
}
