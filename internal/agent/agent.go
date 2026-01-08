package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/creack/pty"

	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/stream"
)

const (
	// RalphDir is the directory where ralph stores its files.
	RalphDir = ".ralph"
)

// RunOptions configures agent execution behavior.
type RunOptions struct {
	TextMode bool // Use text output format (no JSON streaming)
}

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
	cmd, cleanup := r.buildCmd(ctx, prompt, iteration, RunOptions{})
	defer cleanup()

	var outputBuf bytes.Buffer

	// Create stream processor if agent supports structured output
	proc, rawLogFile := r.createStreamProcessor()
	if proc != nil {
		if rawLogFile != nil {
			defer func() { _ = rawLogFile.Close() }()
		}
		defer func() { _ = proc.Close() }()
	}

	// PTY mode: not compatible with structured output parsing (raw bytes)
	// Only use PTY when we don't have a stream processor
	if r.Settings.StreamAgentOutput && runtime.GOOS != "windows" && proc == nil {
		ptmx, err := pty.Start(cmd)
		if err == nil {
			copyDone := make(chan error, 1)
			go func() {
				copyDone <- streamPTY(ptmx, &outputBuf, r.Stdout)
			}()

			err = cmd.Wait()
			_ = ptmx.Close()
			copyErr := <-copyDone
			if err == nil && copyErr != nil {
				err = copyErr
			}
			return outputBuf.String(), err
		}
		if r.Verbose {
			_, _ = fmt.Fprintf(r.Stderr, "[ralph] PTY start failed, falling back to pipe streaming: %v\n", err)
		}
		// Recreate command for fallback (PTY consumed the original)
		cmd, _ = r.buildCmd(ctx, prompt, iteration, RunOptions{})
	}

	r.configureOutput(cmd, &outputBuf, proc)

	err := cmd.Run()

	// Log stats if verbose
	if proc != nil && r.Verbose {
		events, errors := proc.Stats()
		_, _ = fmt.Fprintf(r.Stderr, "[ralph] stream stats: %d events, %d errors\n", events, errors)
	}

	return outputBuf.String(), err
}

// createStreamProcessor creates a stream processor if streaming is enabled.
// Returns the processor and raw log file (both may be nil).
func (r *Runner) createStreamProcessor() (*stream.Processor, *os.File) {
	if !r.Settings.StreamAgentOutput {
		return nil, nil
	}

	agentName := strings.ToLower(strings.TrimSuffix(filepath.Base(r.Settings.Agent.Command), ".exe"))
	var rawLog io.Writer
	var rawLogFile *os.File

	if agentName == "claude" {
		if err := os.MkdirAll(RalphDir, 0o755); err != nil {
			if r.Verbose {
				_, _ = fmt.Fprintf(r.Stderr, "[ralph] failed to create %s: %v\n", RalphDir, err)
			}
		} else {
			logPath := filepath.Join(RalphDir, "stream-json.log")
			file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				if r.Verbose {
					_, _ = fmt.Fprintf(r.Stderr, "[ralph] failed to open stream log %s: %v\n", logPath, err)
				}
			} else {
				rawLogFile = file
				rawLog = file
			}
		}
	}

	config := stream.DefaultFormatterConfig(filepath.Base(r.Settings.Agent.Command))
	formatter := stream.NewFormatter(r.Stdout, config)

	var debugLog *log.Logger
	if r.Verbose {
		debugLog = log.New(r.Stderr, "[stream-debug] ", log.LstdFlags)
	}

	return stream.NewProcessor(r.Settings.Agent.Command, formatter, debugLog, rawLog), rawLogFile
}

// configureOutput sets up stdout/stderr for the command based on streaming settings.
func (r *Runner) configureOutput(cmd *exec.Cmd, outputBuf *bytes.Buffer, proc *stream.Processor) {
	if r.Settings.StreamAgentOutput {
		if proc != nil {
			// Structured output: parse JSON, format events
			cmd.Stdout = io.MultiWriter(outputBuf, proc)
			// Still stream stderr to user for prompts/errors
			cmd.Stderr = io.MultiWriter(outputBuf, r.Stderr)
		} else {
			// No parser available: raw streaming fallback
			cmd.Stdout = io.MultiWriter(outputBuf, r.Stdout)
			cmd.Stderr = io.MultiWriter(outputBuf, r.Stderr)
		}
	} else {
		// Capture output only
		cmd.Stdout = outputBuf
		cmd.Stderr = outputBuf
	}
}

type flushWriter interface {
	Flush() error
}

type flushWriterNoErr interface {
	Flush()
}

func streamPTY(src io.Reader, writers ...io.Writer) error {
	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if writeErr := writeAndFlush(writers, buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF || isPTYEOF(err) {
				return nil
			}
			return err
		}
	}
}

func writeAndFlush(writers []io.Writer, p []byte) error {
	for _, w := range writers {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n != len(p) {
			return io.ErrShortWrite
		}
		if fw, ok := w.(flushWriter); ok {
			_ = fw.Flush()
		} else if fw, ok := w.(flushWriterNoErr); ok {
			fw.Flush()
		}
	}
	return nil
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

// buildCmd creates an exec.Cmd with proper shell wrapping and logging.
// Returns the command and a cleanup function for any temp files.
func (r *Runner) buildCmd(ctx context.Context, prompt string, iteration int, opts RunOptions) (*exec.Cmd, func()) {
	args, promptFile := r.buildArgs(prompt, iteration, opts)

	cleanup := func() {
		if promptFile != "" {
			_ = os.Remove(promptFile)
		}
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
	cmd.Stdin = nil

	return cmd, cleanup
}

// buildArgs constructs the argument list for the agent command.
// Returns the args and the path to any prompt file created (empty if none).
func (r *Runner) buildArgs(prompt string, iteration int, opts RunOptions) ([]string, string) {
	var args []string
	var promptFile string

	// Infer non-REPL flag based on command name
	cmdName := filepath.Base(r.Settings.Agent.Command)
	cmdName = strings.TrimSuffix(cmdName, ".exe") // handle Windows
	cmdLower := strings.ToLower(cmdName)

	// For amp, we need special argument ordering:
	// amp expects: [flags...] -x <prompt>
	// The -x flag must immediately precede the prompt
	if cmdLower == "amp" {
		return r.buildAmpArgs(prompt, opts)
	}

	// Standard argument ordering for other agents
	switch cmdLower {
	case "claude":
		args = append(args, "-p")
	case "codex":
		args = append(args, "e")
	}

	// Add output format flags
	if opts.TextMode {
		// Text mode: simple text output (for commit messages, etc.)
		if cmdLower == "claude" {
			args = append(args, "--output-format", "text")
		}
	} else if r.Settings.StreamAgentOutput {
		// Streaming mode: structured JSON output
		if flags := stream.OutputFlags(r.Settings.Agent.Command); flags != nil {
			args = append(args, flags...)
		}
	}

	// Add user-configured flags
	args = append(args, r.Settings.Agent.Flags...)

	// For Codex with "e" subcommand, write prompt to file (matching Python behavior)
	if cmdLower == "codex" && len(args) > 0 && args[0] == "e" {
		promptFile = filepath.Join(RalphDir, fmt.Sprintf("prompt_%03d.txt", iteration))
		if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err == nil {
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

// buildAmpArgs constructs arguments for the amp CLI.
// Amp requires: [flags...] -x <prompt>
// The -x flag must immediately precede the prompt argument.
func (r *Runner) buildAmpArgs(prompt string, opts RunOptions) ([]string, string) {
	var args []string

	// Add output format flags first
	if opts.TextMode {
		// For text mode, omit --stream-json but keep autonomy flag
		args = append(args, "--dangerously-allow-all")
	} else if r.Settings.StreamAgentOutput {
		// Streaming mode: structured JSON output
		if flags := stream.OutputFlags(r.Settings.Agent.Command); flags != nil {
			args = append(args, flags...)
		}
	}

	// Add user-configured flags
	args = append(args, r.Settings.Agent.Flags...)

	// Add -x and prompt last (amp requires prompt immediately after -x)
	args = append(args, "-x", prompt)

	return args, ""
}

// RunTextMode executes the agent with text output format (no JSON streaming).
// Used for simple requests like commit messages where we just need the text response.
func (r *Runner) RunTextMode(ctx context.Context, prompt string, iteration int) (string, error) {
	cmd, cleanup := r.buildCmd(ctx, prompt, iteration, RunOptions{TextMode: true})
	defer cleanup()

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	return outputBuf.String(), err
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
