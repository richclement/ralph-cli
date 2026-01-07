package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/richclement/ralph-cli/internal/config"
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

// CLI defines the command-line interface for ralph.
type CLI struct {
	Prompt             string           `arg:"" optional:"" help:"Prompt string to send to agent."`
	PromptFlag         string           `name:"prompt" short:"p" help:"Prompt string to send to agent."`
	PromptFile         string           `name:"prompt-file" short:"f" help:"Path to file containing prompt text."`
	MaximumIterations  int              `name:"maximum-iterations" short:"m" help:"Maximum iterations before stopping."`
	CompletionResponse string           `name:"completion-response" short:"c" help:"Completion response text."`
	Settings           string           `name:"settings" default:".ralph/settings.json" help:"Path to settings file."`
	StreamAgentOutput  *bool            `name:"stream-agent-output" help:"Stream agent output to console." negatable:""`
	Verbose            bool             `name:"verbose" short:"V" help:"Enable verbose/debug output."`
	Version            kong.VersionFlag `name:"version" short:"v" help:"Print version and exit."`
}

// Validate checks that exactly one prompt source is provided.
func (c *CLI) Validate() error {
	hasPositional := c.Prompt != ""
	hasPromptFlag := c.PromptFlag != ""
	hasPromptFile := c.PromptFile != ""

	count := 0
	if hasPositional {
		count++
	}
	if hasPromptFlag {
		count++
	}
	if hasPromptFile {
		count++
	}

	if count > 1 {
		return fmt.Errorf("cannot specify multiple prompt sources (positional, --prompt, --prompt-file)")
	}
	if count == 0 {
		return fmt.Errorf("must specify prompt (positional arg, --prompt, or --prompt-file)")
	}
	return nil
}

// GetPrompt returns the effective prompt from either positional arg or --prompt flag.
func (c *CLI) GetPrompt() string {
	if c.PromptFlag != "" {
		return c.PromptFlag
	}
	return c.Prompt
}

func main() {
	var cli CLI
	parser, err := kong.New(&cli,
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
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	// Validate prompt options
	if err := cli.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	// Validate prompt file exists if specified
	if cli.PromptFile != "" {
		if _, err := os.Stat(cli.PromptFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: prompt file not found: %s\n", cli.PromptFile)
			os.Exit(2)
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot access prompt file: %v\n", err)
			os.Exit(2)
		}
	}

	// Ensure .ralph directory exists
	if err := os.MkdirAll(".ralph", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create .ralph directory: %v\n", err)
		os.Exit(2)
	}

	// Load configuration
	settings, err := config.LoadWithLocal(cli.Settings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load settings: %v\n", err)
		os.Exit(2)
	}

	// Build CLI overrides - only set if explicitly provided
	overrides := config.CLIOverrides{}
	if cli.MaximumIterations != 0 {
		overrides.MaximumIterations = &cli.MaximumIterations
	}
	if cli.CompletionResponse != "" {
		overrides.CompletionResponse = &cli.CompletionResponse
	}
	overrides.StreamAgentOutput = cli.StreamAgentOutput

	settings.ApplyCLIOverrides(overrides)

	// Validate settings
	if err := settings.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	// Verbose logging helper
	verboseLog := func(format string, args ...interface{}) {
		if cli.Verbose {
			fmt.Fprintf(os.Stderr, "[ralph] "+format+"\n", args...)
		}
	}

	verboseLog("Settings loaded from %s", cli.Settings)
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
		Prompt:     cli.GetPrompt(),
		PromptFile: cli.PromptFile,
		Settings:   &settings,
		Verbose:    cli.Verbose,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	})

	exitCode := loopRunner.Run(runCtx)

	// SCM tasks are now run inside the loop after each successful guardrail pass

	_ = ctx // Unused kong context
	os.Exit(exitCode)
}
