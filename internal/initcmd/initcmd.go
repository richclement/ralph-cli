package initcmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/richclement/ralph-cli/internal/config"
)

const (
	settingsPath      = ".ralph/settings.json"
	settingsLocalPath = ".ralph/settings.local.json"
	settingsDir       = ".ralph"
)

// exitCode is used for testing to capture exit codes instead of calling os.Exit
var exitFunc = os.Exit

// isTerminalFunc allows mocking TTY detection in tests
var isTerminalFunc = func(fd int) bool {
	return term.IsTerminal(fd)
}

// Run executes the interactive init command.
// It returns an error if initialization fails.
func Run() error {
	// TTY detection
	if !isTerminalFunc(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "error: init requires an interactive terminal")
		exitFunc(2)
		return nil
	}

	// Signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nAborted.")
		exitFunc(130)
	}()

	reader := bufio.NewReader(os.Stdin)

	// Check for existing settings file
	if _, err := os.Stat(settingsPath); err == nil {
		if err := handleExistingSettings(reader); err != nil {
			return err
		}
	}

	// Collect inputs
	agentCommand, err := promptAgentCommand(reader)
	if err != nil {
		return err
	}

	agentFlags, err := promptAgentFlags(reader)
	if err != nil {
		return err
	}

	maxIterations, err := promptMaxIterations(reader)
	if err != nil {
		return err
	}

	completionResponse, err := promptCompletionResponse(reader)
	if err != nil {
		return err
	}

	includeIterationCount, err := promptIncludeIterationCount(reader)
	if err != nil {
		return err
	}

	guardrails, err := promptGuardrails(reader)
	if err != nil {
		return err
	}

	reviews, err := promptReviews(reader)
	if err != nil {
		return err
	}

	scm, err := promptSCM(reader)
	if err != nil {
		return err
	}

	// Build settings object
	settings := config.NewDefaults()
	settings.Agent.Command = agentCommand
	settings.Agent.Flags = agentFlags
	settings.MaximumIterations = maxIterations
	settings.CompletionResponse = completionResponse
	settings.IncludeIterationCountInPrompt = includeIterationCount
	settings.Guardrails = guardrails
	if reviews != nil {
		settings.Reviews = reviews
	}
	if scm != nil {
		settings.SCM = scm
	}

	// Validate before writing
	if err := settings.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid settings: %v\n", err)
		exitFunc(2)
		return nil
	}

	// Write settings file
	if err := writeSettings(settings); err != nil {
		return err
	}

	fmt.Println("\nSettings written to .ralph/settings.json")
	return nil
}

func handleExistingSettings(reader *bufio.Reader) error {
	// Check if local overlay exists
	localExists := false
	if _, err := os.Stat(settingsLocalPath); err == nil {
		localExists = true
	}

	// Try to load and display existing settings
	settings, loadErr := config.LoadWithLocal(settingsPath)
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "error: existing settings file is malformed: %v\n", loadErr)
	} else {
		// Pretty print settings
		data, _ := json.MarshalIndent(settings, "", "  ")
		fmt.Println(string(data))

		// Show which files were loaded
		if localExists {
			fmt.Println("Loaded from .ralph/settings.json (with local overlay from settings.local.json)")
		} else {
			fmt.Println("Loaded from .ralph/settings.json")
		}
	}

	// Prompt for overwrite
	response, err := prompt(reader, "Overwrite? (y/N): ")
	if err != nil {
		return err
	}

	if strings.ToLower(response) != "y" {
		exitFunc(0)
		return nil
	}

	return nil
}

func prompt(reader *bufio.Reader, message string) (string, error) {
	fmt.Print(message)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Println("\nAborted.")
			exitFunc(130)
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptWithDefault(reader *bufio.Reader, message, defaultValue string) (string, error) {
	fmt.Printf("%s [%s]: ", message, defaultValue)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Println("\nAborted.")
			exitFunc(130)
			return "", nil
		}
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func promptAgentCommand(reader *bufio.Reader) (string, error) {
	for {
		input, err := prompt(reader, "Agent command (e.g., claude, codex, amp, or other LLM CLI): ")
		if err != nil {
			return "", err
		}
		if input == "" {
			fmt.Fprintln(os.Stderr, "Agent command is required")
			continue
		}
		return input, nil
	}
}

func promptAgentFlags(reader *bufio.Reader) ([]string, error) {
	input, err := prompt(reader, "Agent flags (comma-separated, optional): ")
	if err != nil {
		return nil, err
	}

	flags := []string{}
	if input != "" {
		parts := strings.Split(input, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				flags = append(flags, trimmed)
			}
		}
	}
	return flags, nil
}

func promptMaxIterations(reader *bufio.Reader) (int, error) {
	for {
		input, err := promptWithDefault(reader, "Maximum iterations", "10")
		if err != nil {
			return 0, err
		}
		parsed, parseErr := strconv.Atoi(input)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, "Please enter a valid number")
			continue
		}
		if parsed <= 0 {
			fmt.Fprintln(os.Stderr, "Must be greater than 0")
			continue
		}
		return parsed, nil
	}
}

func promptCompletionResponse(reader *bufio.Reader) (string, error) {
	return promptWithDefault(reader, "Completion response", "DONE")
}

func promptIncludeIterationCount(reader *bufio.Reader) (bool, error) {
	defaultValue := strconv.FormatBool(config.NewDefaults().IncludeIterationCountInPrompt)
	for {
		input, err := promptWithDefault(reader, "Include iteration count in prompt", defaultValue)
		if err != nil {
			return false, err
		}
		normalized := strings.ToLower(strings.TrimSpace(input))
		switch normalized {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		parsed, parseErr := strconv.ParseBool(normalized)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, "Please enter true/false or y/n")
			continue
		}
		return parsed, nil
	}
}

func promptGuardrails(reader *bufio.Reader) ([]config.Guardrail, error) {
	var guardrails []config.Guardrail
	for {
		cmd, err := prompt(reader, "Add guardrail command (leave blank to finish): ")
		if err != nil {
			return nil, err
		}
		if cmd == "" {
			break
		}

		// Prompt for fail action
		var failAction string
		for {
			action, err := prompt(reader, "  Fail action (APPEND|PREPEND|REPLACE): ")
			if err != nil {
				return nil, err
			}
			action = strings.ToUpper(action)
			if action != "APPEND" && action != "PREPEND" && action != "REPLACE" {
				fmt.Fprintln(os.Stderr, "  Invalid action. Must be APPEND, PREPEND, or REPLACE")
				continue
			}
			failAction = action
			break
		}

		// Prompt for optional hint
		hint, err := prompt(reader, "  Hint (optional, guidance for agent on failure): ")
		if err != nil {
			return nil, err
		}

		guardrails = append(guardrails, config.Guardrail{
			Command:    cmd,
			FailAction: failAction,
			Hint:       hint,
		})
	}
	return guardrails, nil
}

func promptSCM(reader *bufio.Reader) (*config.SCMConfig, error) {
	configureSCM, err := prompt(reader, "Configure SCM? (y/N): ")
	if err != nil {
		return nil, err
	}

	if strings.ToLower(configureSCM) != "y" {
		return nil, nil
	}

	scmCommand, err := prompt(reader, "  SCM command (e.g., git): ")
	if err != nil {
		return nil, err
	}

	tasksInput, err := prompt(reader, "  SCM tasks (comma-separated, e.g., commit,push): ")
	if err != nil {
		return nil, err
	}

	var tasks []string
	if tasksInput != "" {
		parts := strings.Split(tasksInput, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				tasks = append(tasks, trimmed)
			}
		}
	}

	return &config.SCMConfig{
		Command: scmCommand,
		Tasks:   tasks,
	}, nil
}

func promptReviews(reader *bufio.Reader) (*config.ReviewsConfig, error) {
	configureReviews, err := prompt(reader, "Configure review cycles? (y/N): ")
	if err != nil {
		return nil, err
	}

	if strings.ToLower(configureReviews) != "y" {
		return nil, nil
	}

	reviewAfter, err := promptReviewAfter(reader)
	if err != nil {
		return nil, err
	}

	retryLimit, err := promptGuardrailRetryLimit(reader)
	if err != nil {
		return nil, err
	}

	prompts, err := promptReviewPrompts(reader)
	if err != nil {
		return nil, err
	}

	return &config.ReviewsConfig{
		ReviewAfter:         reviewAfter,
		GuardrailRetryLimit: retryLimit,
		Prompts:             prompts,
	}, nil
}

func promptReviewAfter(reader *bufio.Reader) (int, error) {
	for {
		input, err := promptWithDefault(reader, "  Review after iterations", "5")
		if err != nil {
			return 0, err
		}
		parsed, parseErr := strconv.Atoi(input)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, "  Please enter a valid number")
			continue
		}
		if parsed < 0 {
			fmt.Fprintln(os.Stderr, "  Must be 0 or greater")
			continue
		}
		return parsed, nil
	}
}

func promptGuardrailRetryLimit(reader *bufio.Reader) (int, error) {
	for {
		input, err := promptWithDefault(reader, "  Guardrail retry limit", "3")
		if err != nil {
			return 0, err
		}
		parsed, parseErr := strconv.Atoi(input)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, "  Please enter a valid number")
			continue
		}
		if parsed < 0 {
			fmt.Fprintln(os.Stderr, "  Must be 0 or greater")
			continue
		}
		return parsed, nil
	}
}

func promptReviewPrompts(reader *bufio.Reader) ([]config.ReviewPrompt, error) {
	// Show default prompts with multi-line formatting for readability
	fmt.Println("  Default review prompts:")
	for _, p := range config.DefaultReviewPrompts() {
		fmt.Printf("    %s:\n      %s\n", p.Name, p.Prompt)
	}

	fmt.Print("  Use default review prompts? [Y/n]: ")
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Println("\nAborted.")
			exitFunc(130)
			return nil, nil
		}
		return nil, err
	}
	useDefaults := strings.TrimSpace(line)

	if useDefaults == "" || strings.ToLower(useDefaults) == "y" {
		return config.DefaultReviewPrompts(), nil
	}

	// Custom prompt loop
	var prompts []config.ReviewPrompt
	for {
		nameInput, err := prompt(reader, "  Add review prompt (leave blank to finish):\n    Name: ")
		if err != nil {
			return nil, err
		}
		if nameInput == "" {
			break
		}

		promptInput, err := prompt(reader, "    Prompt: ")
		if err != nil {
			return nil, err
		}
		if promptInput == "" {
			break
		}

		prompts = append(prompts, config.ReviewPrompt{
			Name:   nameInput,
			Prompt: promptInput,
		})
	}

	return prompts, nil
}

func writeSettings(settings config.Settings) error {
	// Create directory
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create directory: %v\n", err)
		exitFunc(2)
		return nil
	}

	// Marshal settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to serialize settings: %v\n", err)
		exitFunc(2)
		return nil
	}

	// Add trailing newline
	data = append(data, '\n')

	// Write file
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write settings: %v\n", err)
		exitFunc(2)
		return nil
	}

	return nil
}
