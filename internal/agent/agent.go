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

// Agent Behavioral Differences
// ============================
// Agent-specific configuration is distributed across this file and stream/parser.go.
// This comment summarizes the differences for each supported agent.
//
// | Behavior           | claude                              | amp                                   | codex                          |
// |--------------------|-------------------------------------|---------------------------------------|--------------------------------|
// | Non-REPL flag      | -p                                  | -x (must be last, before prompt)      | e (subcommand)                 |
// | Streaming flags    | --output-format stream-json         | --stream-json --dangerously-allow-all | --json --full-auto             |
// | Text mode flags    | --output-format text                | --dangerously-allow-all               | --full-auto                    |
// | Prompt handling    | inline argument                     | argument after -x flag                | written to file (.ralph/)      |
// | Output handling    | stdout                              | stdout                                | -o flag to file, then read     |
// | Arg ordering       | [flag] [mode-flags] [user-flags] p  | [mode-flags] [user-flags] -x p        | e [mode-flags] [user-flags] pf |
//
// Key locations:
//   - Non-REPL flags: buildArgs() switch statement
//   - Streaming/text flags: stream.OutputFlags(), stream.TextModeFlags()
//   - Prompt file handling: buildArgs() codex-specific block
//   - Output file handling: RunTextMode() with stream.OutputCaptureFor()
//   - Amp arg ordering: buildAmpArgs() separate function
//
// When adding a new agent, update:
//   1. stream.NormalizeName constants (AgentX)
//   2. buildArgs() for non-REPL flag
//   3. stream.OutputFlags() for streaming mode
//   4. stream.TextModeFlags() for text mode
//   5. stream.ParserFor() if agent has structured output
//   6. This comment

const (
	// RalphDir is the directory where ralph stores its files.
	RalphDir = ".ralph"
)

// RunOptions configures agent execution behavior.
type RunOptions struct {
	TextMode   bool   // Use text output format (no JSON streaming)
	OutputFile string // Output file for text mode (used by Codex -o flag)
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

	var rawLog io.Writer
	var rawLogFile *os.File

	// Enable raw JSON logging for agents with structured output
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
	cmdLower := stream.NormalizeName(r.Settings.Agent.Command)

	// For amp, we need special argument ordering:
	// amp expects: [flags...] -x <prompt>
	// The -x flag must immediately precede the prompt
	if cmdLower == stream.AgentAmp {
		return r.buildAmpArgs(prompt, opts)
	}

	// Standard argument ordering for other agents
	switch cmdLower {
	case stream.AgentClaude:
		args = append(args, "-p")
	case stream.AgentCodex:
		args = append(args, "e")
	}

	// Add output format flags
	if opts.TextMode {
		// Text mode: simple text output (for commit messages, etc.)
		if flags := stream.TextModeFlags(r.Settings.Agent.Command); flags != nil {
			args = append(args, flags...)
		}
		// Codex-specific: add -o flag to write output to file
		if cmdLower == stream.AgentCodex && opts.OutputFile != "" {
			args = append(args, "-o", opts.OutputFile)
		}
	} else if r.Settings.StreamAgentOutput {
		// Streaming mode: structured JSON output
		if flags := stream.OutputFlags(r.Settings.Agent.Command); flags != nil {
			args = append(args, flags...)
		}
	}

	// Add user-configured flags
	args = append(args, r.Settings.Agent.Flags...)

	// For Codex with "e" subcommand in streaming mode, write prompt to file.
	// In text mode (commit messages), pass prompt directly to avoid -o flag issues.
	if cmdLower == stream.AgentCodex && len(args) > 0 && args[0] == "e" && !opts.TextMode {
		promptFile = filepath.Join(RalphDir, fmt.Sprintf("prompt_%03d.txt", iteration))
		if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err == nil {
			args = append(args, promptFile)
		} else {
			// Fall back to passing prompt as argument if file write fails
			args = append(args, prompt)
			promptFile = ""
		}
	} else {
		// Add the prompt as argument for other agents (and Codex text mode)
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
		// Text mode: use agent-specific text flags
		if flags := stream.TextModeFlags(r.Settings.Agent.Command); flags != nil {
			args = append(args, flags...)
		}
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
	opts := RunOptions{TextMode: true}

	// Check if agent needs output capture (writes to file instead of stdout)
	capture := stream.OutputCaptureFor(r.Settings.Agent.Command, RalphDir)
	if capture != nil {
		// Ensure .ralph directory exists
		if err := os.MkdirAll(RalphDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", RalphDir, err)
		}
		opts.OutputFile = capture.File
	}

	cmd, cleanup := r.buildCmd(ctx, prompt, iteration, opts)
	defer cleanup()

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()

	// If using output capture, read from file and clean up
	if capture != nil && opts.OutputFile != "" {
		defer func() { _ = os.Remove(opts.OutputFile) }()

		content, readErr := os.ReadFile(opts.OutputFile)
		if readErr != nil {
			// If we can't read the output file, fall back to stdout
			if err != nil {
				return outputBuf.String(), err
			}
			return outputBuf.String(), fmt.Errorf("failed to read agent output file: %w", readErr)
		}
		return string(content), err
	}

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
