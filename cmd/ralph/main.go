package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/initcmd"
	"github.com/richclement/ralph-cli/internal/loop"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func versionString() string {
	v := version
	if commit != "" {
		v += " (" + commit + ")"
	}
	if date != "" {
		v += " " + date
	}
	return v
}

// CLI defines the command-line interface for ralph with subcommands.
type CLI struct {
	Init    InitCmd          `cmd:"" help:"Initialize settings file interactively."`
	Run     RunCmd           `cmd:"" default:"1" help:"Run the agent loop."`
	Version kong.VersionFlag `name:"version" short:"v" help:"Print version and exit."`
}

// InitCmd represents the init subcommand.
type InitCmd struct{}

// Run executes the init command.
func (c *InitCmd) Run() error {
	return initcmd.Run()
}

// RunCmd represents the run subcommand with all flags for running the agent loop.
type RunCmd struct {
	Prompt             string `name:"prompt" short:"p" help:"Prompt string to send to agent."`
	PromptFile         string `name:"prompt-file" short:"f" help:"Path to file containing prompt text."`
	MaximumIterations  int    `name:"maximum-iterations" short:"m" help:"Maximum iterations before stopping."`
	CompletionResponse string `name:"completion-response" short:"c" help:"Completion response text."`
	StreamAgentOutput  *bool  `name:"stream-agent-output" help:"Stream agent output to console." negatable:""`
	Verbose            bool   `name:"verbose" short:"V" help:"Enable verbose/debug output."`
}

// Validate checks that exactly one prompt source is provided.
func (c *RunCmd) Validate() error {
	hasPrompt := c.Prompt != ""
	hasPromptFile := c.PromptFile != ""

	if hasPrompt && hasPromptFile {
		return fmt.Errorf("cannot specify both --prompt and --prompt-file")
	}
	if !hasPrompt && !hasPromptFile {
		return fmt.Errorf("must specify prompt (--prompt or --prompt-file)")
	}
	return nil
}

// GetPrompt returns the prompt string.
func (c *RunCmd) GetPrompt() string {
	return c.Prompt
}

// runExitCode is a special error type that carries an exit code.
type runExitCode int

func (e runExitCode) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}

// Run executes the run command.
func (c *RunCmd) Run() error {
	// Validate prompt options
	if err := c.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return runExitCode(2)
	}

	// Validate prompt file exists if specified
	if c.PromptFile != "" {
		if _, err := os.Stat(c.PromptFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: prompt file not found: %s\n", c.PromptFile)
			return runExitCode(2)
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot access prompt file: %v\n", err)
			return runExitCode(2)
		}
	}

	// Hardcoded settings path (--settings flag removed)
	settingsPath := ".ralph/settings.json"

	// Ensure .ralph directory exists
	if err := os.MkdirAll(".ralph", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create .ralph directory: %v\n", err)
		return runExitCode(2)
	}

	// Load configuration
	settings, err := config.LoadWithLocal(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load settings: %v\n", err)
		return runExitCode(2)
	}

	// Build CLI overrides - only set if explicitly provided
	overrides := config.CLIOverrides{}
	if c.MaximumIterations != 0 {
		overrides.MaximumIterations = &c.MaximumIterations
	}
	if c.CompletionResponse != "" {
		overrides.CompletionResponse = &c.CompletionResponse
	}
	overrides.StreamAgentOutput = c.StreamAgentOutput

	settings.ApplyCLIOverrides(overrides)

	// Validate settings
	if err := settings.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return runExitCode(2)
	}

	// Verbose logging helper
	verboseLog := func(format string, args ...interface{}) {
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "[ralph] "+format+"\n", args...)
		}
	}

	verboseLog("Settings loaded from %s", settingsPath)
	verboseLog("Agent: %s %v", settings.Agent.Command, settings.Agent.Flags)
	verboseLog("Max iterations: %d", settings.MaximumIterations)
	verboseLog("Completion response: %s", settings.CompletionResponse)

	// Set up signal handling
	runCtx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "Received signal, shutting down...")
		cancel()
	}()

	// Run the main loop
	loopRunner := loop.NewRunner(loop.Options{
		Prompt:     c.GetPrompt(),
		PromptFile: c.PromptFile,
		Settings:   &settings,
		Verbose:    c.Verbose,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	})

	exitCode := loopRunner.Run(runCtx)

	// Only return an error for non-zero exit codes
	if exitCode == 0 {
		return nil
	}
	return runExitCode(exitCode)
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("ralph"),
		kong.Description("Deterministic outer loop that runs an AI agent until completion."),
		kong.Vars{"version": versionString()},
		kong.UsageOnError(),
		kong.Exit(func(code int) {
			// Map kong's exit codes to our exit code 2 for config errors
			if code != 0 {
				os.Exit(2)
			}
			os.Exit(0)
		}),
	)

	err := ctx.Run()
	if err != nil {
		// Check if it's a runExitCode (already printed error message)
		if exitCode, ok := err.(runExitCode); ok {
			os.Exit(int(exitCode))
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
