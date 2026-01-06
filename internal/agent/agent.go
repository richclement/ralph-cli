package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/creack/pty"
	"github.com/richclement/ralph-cli/internal/config"
)

const (
	// RalphDir is the directory where ralph stores its files.
	RalphDir = ".ralph"
)

// Runner executes agent commands.
type Runner struct {
	Settings *config.Settings
	Stdout   io.Writer // For streaming output, defaults to os.Stdout
	Stderr   io.Writer // For streaming output, defaults to os.Stderr
	Verbose  bool      // Enable verbose logging
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
// The iteration parameter is used for naming prompt files (for Codex).
func (r *Runner) Run(ctx context.Context, prompt string, iteration int) (string, error) {
	args, promptFile := r.buildArgs(prompt, iteration)
	if promptFile != "" {
		defer func() {
			_ = os.Remove(promptFile)
		}()
	}

	// Build the full command string for shell execution with proper quoting
	cmdParts := []string{shellQuote(r.Settings.Agent.Command)}
	for _, arg := range args {
		cmdParts = append(cmdParts, shellQuote(arg))
	}
	cmdStr := strings.Join(cmdParts, " ")

	// Always log the full command being executed
	_, _ = fmt.Fprintf(r.Stderr, "[ralph] Agent command: %s\n", cmdStr)

	// Use user's shell with -ic to support aliases and functions
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-ic", cmdStr)

	var outputBuf bytes.Buffer

	if r.Settings.StreamAgentOutput && runtime.GOOS != "windows" {
		ptmx, err := pty.Start(cmd)
		if err == nil {
			defer func() {
				_ = ptmx.Close()
			}()

			mw := io.MultiWriter(&outputBuf, r.Stdout)
			copyDone := make(chan struct{})
			var copyErr error
			go func() {
				_, copyErr = io.Copy(mw, ptmx)
				close(copyDone)
			}()

			err = cmd.Wait()
			<-copyDone
			if err == nil && copyErr != nil {
				err = copyErr
			}
			return outputBuf.String(), err
		}
		if r.Verbose {
			_, _ = fmt.Fprintf(r.Stderr, "[ralph] PTY start failed, falling back to pipe streaming: %v\n", err)
		}
		cmd = exec.CommandContext(ctx, shell, "-ic", cmdStr)
	}

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

// shellQuote quotes a string for safe use in shell commands.
func shellQuote(s string) string {
	// If string contains no special characters, return as-is
	if !strings.ContainsAny(s, " \t\n'\"\\$`!*?[]{}()#<>&|;") {
		return s
	}
	// Use single quotes, escaping any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// buildArgs constructs the argument list for the agent command.
// Returns the args and the path to any prompt file created (empty if none).
func (r *Runner) buildArgs(prompt string, iteration int) ([]string, string) {
	var args []string
	var promptFile string

	// Infer non-REPL flag based on command name
	cmdName := filepath.Base(r.Settings.Agent.Command)
	cmdName = strings.TrimSuffix(cmdName, ".exe") // handle Windows

	switch strings.ToLower(cmdName) {
	case "claude":
		args = append(args, "-p")
	case "codex":
		args = append(args, "e")
	case "amp":
		args = append(args, "-x")
	}

	// Add user-configured flags
	args = append(args, r.Settings.Agent.Flags...)

	// For Codex with "e" subcommand, write prompt to file (matching Python behavior)
	if strings.ToLower(cmdName) == "codex" && len(args) > 0 && args[0] == "e" {
		promptFile = filepath.Join(RalphDir, fmt.Sprintf("prompt_%03d.txt", iteration))
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err == nil {
			args = append(args, promptFile)
		} else {
			// Fall back to passing prompt as argument if file write fails
			args = append(args, prompt)
			promptFile = ""
		}
	} else {
		// Add the prompt as argument for other agents
		args = append(args, prompt)
	}

	return args, promptFile
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
