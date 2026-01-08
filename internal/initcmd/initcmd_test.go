package initcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richclement/ralph-cli/internal/config"
)

// testExitCode captures the exit code when exitFunc is called
var testExitCode int

// captureExit sets up exitFunc to capture exit codes for testing
func captureExit() {
	testExitCode = -1
	exitFunc = func(code int) {
		testExitCode = code
		panic("exit called") // Use panic to stop execution
	}
}

// restoreExit restores the original exitFunc
func restoreExit() {
	exitFunc = os.Exit
}

// withTempDir runs a function in a temporary directory
func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()
	fn(dir)
}

// withStdin temporarily replaces os.Stdin with a pipe containing the given input
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write input in a goroutine to avoid blocking
	go func() {
		defer w.Close()
		_, _ = w.WriteString(input)
	}()

	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	fn()
}

// mockTTY temporarily sets isTerminalFunc to return true
func mockTTY(isTTY bool) func() {
	old := isTerminalFunc
	isTerminalFunc = func(fd int) bool {
		return isTTY
	}
	return func() {
		isTerminalFunc = old
	}
}

func TestRun_HappyPath(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Input: agent command, flags, max iterations (default), completion (default),
		// iteration count (default), one guardrail, exit guardrail loop, decline SCM
		input := strings.Join([]string{
			"claude",       // agent command
			"--model,opus", // agent flags (comma-separated)
			"",             // max iterations (use default 10)
			"",             // completion response (use default DONE)
			"",             // include iteration count (default false)
			"make lint",    // guardrail command
			"APPEND",       // fail action
			"",             // exit guardrail loop
			"N",            // don't configure SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// Verify settings file was created
		data, err := os.ReadFile(filepath.Join(dir, ".ralph", "settings.json"))
		if err != nil {
			t.Fatalf("failed to read settings file: %v", err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatalf("failed to parse settings: %v", err)
		}

		// Verify values
		if settings.Agent.Command != "claude" {
			t.Errorf("agent.command = %q, want %q", settings.Agent.Command, "claude")
		}
		if len(settings.Agent.Flags) != 2 || settings.Agent.Flags[0] != "--model" || settings.Agent.Flags[1] != "opus" {
			t.Errorf("agent.flags = %v, want [--model opus]", settings.Agent.Flags)
		}
		if settings.MaximumIterations != 10 {
			t.Errorf("maximumIterations = %d, want %d", settings.MaximumIterations, 10)
		}
		if settings.CompletionResponse != "DONE" {
			t.Errorf("completionResponse = %q, want %q", settings.CompletionResponse, "DONE")
		}
		if settings.OutputTruncateChars != 5000 {
			t.Errorf("outputTruncateChars = %d, want %d", settings.OutputTruncateChars, 5000)
		}
		if !settings.StreamAgentOutput {
			t.Error("streamAgentOutput should be true by default")
		}
		if settings.IncludeIterationCountInPrompt {
			t.Error("includeIterationCountInPrompt should be false by default")
		}
		if len(settings.Guardrails) != 1 {
			t.Errorf("guardrails length = %d, want 1", len(settings.Guardrails))
		} else {
			if settings.Guardrails[0].Command != "make lint" {
				t.Errorf("guardrails[0].command = %q, want %q", settings.Guardrails[0].Command, "make lint")
			}
			if settings.Guardrails[0].FailAction != "APPEND" {
				t.Errorf("guardrails[0].failAction = %q, want %q", settings.Guardrails[0].FailAction, "APPEND")
			}
		}
		if settings.SCM != nil {
			t.Error("SCM should be nil when not configured")
		}
	})
}

func TestRun_WithSCM(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude", // agent command
			"",       // no flags
			"",       // default max iterations
			"",       // default completion
			"",       // default include iteration count
			"",       // no guardrails
			"y",      // configure SCM
			"git",    // SCM command
			"commit", // SCM tasks
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(filepath.Join(dir, ".ralph", "settings.json"))
		if err != nil {
			t.Fatalf("failed to read settings file: %v", err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatalf("failed to parse settings: %v", err)
		}

		if settings.SCM == nil {
			t.Fatal("SCM should not be nil")
		}
		if settings.SCM.Command != "git" {
			t.Errorf("scm.command = %q, want %q", settings.SCM.Command, "git")
		}
		if len(settings.SCM.Tasks) != 1 || settings.SCM.Tasks[0] != "commit" {
			t.Errorf("scm.tasks = %v, want [commit]", settings.SCM.Tasks)
		}
	})
}

func TestRun_ExistingSettings_DeclineOverwrite(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Create existing settings
		if err := os.MkdirAll(".ralph", 0o755); err != nil {
			t.Fatal(err)
		}
		existingSettings := config.Settings{
			Agent:               config.AgentConfig{Command: "existing-agent"},
			MaximumIterations:   20,
			CompletionResponse:  "OLD",
			OutputTruncateChars: 5000,
			StreamAgentOutput:   true,
		}
		data, _ := json.MarshalIndent(existingSettings, "", "  ")
		if err := os.WriteFile(".ralph/settings.json", data, 0o644); err != nil {
			t.Fatal(err)
		}

		// Decline overwrite
		input := "N\n"

		var exitCalled bool
		withStdin(t, input, func() {
			defer func() {
				if r := recover(); r != nil {
					if r == "exit called" {
						exitCalled = true
					} else {
						panic(r)
					}
				}
			}()
			_ = Run()
		})

		if !exitCalled {
			t.Error("expected exit to be called")
		}
		if testExitCode != 0 {
			t.Errorf("exit code = %d, want 0", testExitCode)
		}

		// Verify original file unchanged
		currentData, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(currentData, data) {
			t.Error("settings file was modified when it shouldn't have been")
		}
	})
}

func TestRun_ExistingSettings_AcceptOverwrite(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Create existing settings
		if err := os.MkdirAll(".ralph", 0o755); err != nil {
			t.Fatal(err)
		}
		existingSettings := config.Settings{
			Agent:               config.AgentConfig{Command: "existing-agent"},
			MaximumIterations:   20,
			CompletionResponse:  "OLD",
			OutputTruncateChars: 5000,
			StreamAgentOutput:   true,
		}
		data, _ := json.MarshalIndent(existingSettings, "", "  ")
		if err := os.WriteFile(".ralph/settings.json", data, 0o644); err != nil {
			t.Fatal(err)
		}

		// Accept overwrite and provide new settings
		input := strings.Join([]string{
			"y",         // overwrite
			"new-agent", // new agent command
			"",          // no flags
			"",          // default iterations
			"",          // default completion
			"",          // default include iteration count
			"",          // no guardrails
			"N",         // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// Verify new settings
		newData, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(newData, &settings); err != nil {
			t.Fatal(err)
		}

		if settings.Agent.Command != "new-agent" {
			t.Errorf("agent.command = %q, want %q", settings.Agent.Command, "new-agent")
		}
	})
}

func TestRun_ExistingSettings_WithLocalOverlay(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Create existing settings with local overlay
		if err := os.MkdirAll(".ralph", 0o755); err != nil {
			t.Fatal(err)
		}

		baseSettings := config.Settings{
			Agent:               config.AgentConfig{Command: "base-agent"},
			MaximumIterations:   10,
			CompletionResponse:  "DONE",
			OutputTruncateChars: 5000,
			StreamAgentOutput:   true,
		}
		baseData, _ := json.MarshalIndent(baseSettings, "", "  ")
		if err := os.WriteFile(".ralph/settings.json", baseData, 0o644); err != nil {
			t.Fatal(err)
		}

		// Local overlay changes maxIterations
		localSettings := map[string]interface{}{
			"maximumIterations": 50,
		}
		localData, _ := json.MarshalIndent(localSettings, "", "  ")
		if err := os.WriteFile(".ralph/settings.local.json", localData, 0o644); err != nil {
			t.Fatal(err)
		}

		// Capture stdout to verify overlay message
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Decline overwrite
		input := "N\n"
		var exitCalled bool
		withStdin(t, input, func() {
			defer func() {
				if rec := recover(); rec != nil {
					if rec == "exit called" {
						exitCalled = true
					} else {
						panic(rec)
					}
				}
			}()
			_ = Run()
		})

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stdout = oldStdout
		output := buf.String()

		if !exitCalled {
			t.Error("expected exit to be called")
		}

		// Verify overlay message
		if !strings.Contains(output, "with local overlay") {
			t.Errorf("expected 'with local overlay' in output, got: %s", output)
		}

		// Verify merged config shows the overridden value
		if !strings.Contains(output, `"maximumIterations": 50`) {
			t.Errorf("expected maximumIterations: 50 in output (merged), got: %s", output)
		}
	})
}

func TestRun_MalformedExistingSettings(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Create malformed settings file
		if err := os.MkdirAll(".ralph", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(".ralph/settings.json", []byte("{invalid json}"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Accept overwrite and provide new settings
		input := strings.Join([]string{
			"y",         // overwrite
			"new-agent", // agent command
			"",          // no flags
			"",          // default iterations
			"",          // default completion
			"",          // default include iteration count
			"",          // no guardrails
			"N",         // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stderr = oldStderr
		stderrOutput := buf.String()

		// Verify error message about malformed settings
		if !strings.Contains(stderrOutput, "malformed") {
			t.Errorf("expected 'malformed' in stderr, got: %s", stderrOutput)
		}

		// Verify new settings were written successfully
		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatalf("new settings should be valid JSON: %v", err)
		}
		if settings.Agent.Command != "new-agent" {
			t.Errorf("agent.command = %q, want %q", settings.Agent.Command, "new-agent")
		}
	})
}

func TestRun_EmptyAgentCommand_Reprompt(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Capture stderr to verify error message
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// First empty, then valid
		input := strings.Join([]string{
			"",       // empty - should reprompt
			"claude", // valid agent command
			"",       // no flags
			"",       // default iterations
			"",       // default completion
			"",       // default include iteration count
			"",       // no guardrails
			"N",      // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stderr = oldStderr
		stderrOutput := buf.String()

		// Verify error message
		if !strings.Contains(stderrOutput, "Agent command is required") {
			t.Errorf("expected 'Agent command is required' in stderr, got: %s", stderrOutput)
		}

		// Verify settings were created with the valid command
		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}
		if settings.Agent.Command != "claude" {
			t.Errorf("agent.command = %q, want %q", settings.Agent.Command, "claude")
		}
	})
}

func TestRun_InvalidFailAction_Reprompt(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		input := strings.Join([]string{
			"claude",    // agent command
			"",          // no flags
			"",          // default iterations
			"",          // default completion
			"",          // default include iteration count
			"make lint", // guardrail command
			"INVALID",   // invalid action - should reprompt
			"APPEND",    // valid action
			"",          // exit guardrail loop
			"N",         // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stderr = oldStderr
		stderrOutput := buf.String()

		// Verify error message
		if !strings.Contains(stderrOutput, "Invalid action") {
			t.Errorf("expected 'Invalid action' in stderr, got: %s", stderrOutput)
		}

		// Verify settings
		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}
		if len(settings.Guardrails) != 1 {
			t.Fatalf("expected 1 guardrail, got %d", len(settings.Guardrails))
		}
		if settings.Guardrails[0].FailAction != "APPEND" {
			t.Errorf("failAction = %q, want %q", settings.Guardrails[0].FailAction, "APPEND")
		}
	})
}

func TestRun_FailActionCaseNormalization(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude",    // agent command
			"",          // no flags
			"",          // default iterations
			"",          // default completion
			"",          // default include iteration count
			"make lint", // guardrail command
			"append",    // lowercase - should be normalized
			"",          // exit guardrail loop
			"N",         // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if settings.Guardrails[0].FailAction != "APPEND" {
			t.Errorf("failAction = %q, want %q (uppercase)", settings.Guardrails[0].FailAction, "APPEND")
		}
	})
}

func TestRun_GuardrailLoopExit(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude",    // agent command
			"",          // no flags
			"",          // default iterations
			"",          // default completion
			"",          // default include iteration count
			"make lint", // first guardrail
			"APPEND",
			"make test", // second guardrail
			"PREPEND",
			"",  // exit loop (empty command)
			"N", // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if len(settings.Guardrails) != 2 {
			t.Errorf("expected 2 guardrails, got %d", len(settings.Guardrails))
		}
	})
}

func TestRun_SCMDeclined(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude", // agent command
			"",       // no flags
			"",       // default iterations
			"",       // default completion
			"",       // default include iteration count
			"",       // no guardrails
			"N",      // decline SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if settings.SCM != nil {
			t.Error("SCM should be nil when declined")
		}
	})
}

func TestRun_NonTTY_Error(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(false)() // Not a TTY

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		var exitCalled bool
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					if rec == "exit called" {
						exitCalled = true
					} else {
						panic(rec)
					}
				}
			}()
			_ = Run()
		}()

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stderr = oldStderr
		stderrOutput := buf.String()

		if !exitCalled {
			t.Error("expected exit to be called")
		}
		if testExitCode != 2 {
			t.Errorf("exit code = %d, want 2", testExitCode)
		}
		if !strings.Contains(stderrOutput, "interactive terminal") {
			t.Errorf("expected 'interactive terminal' in stderr, got: %s", stderrOutput)
		}
	})
}

func TestRun_StdinEOF(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Capture stdout to see "Aborted." message
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Only partial input - will hit EOF
		input := "claude\n" // Only agent command, then EOF

		var exitCalled bool
		withStdin(t, input, func() {
			defer func() {
				if rec := recover(); rec != nil {
					if rec == "exit called" {
						exitCalled = true
					} else {
						panic(rec)
					}
				}
			}()
			_ = Run()
		})

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stdout = oldStdout
		output := buf.String()

		if !exitCalled {
			t.Error("expected exit to be called on EOF")
		}
		if testExitCode != 130 {
			t.Errorf("exit code = %d, want 130", testExitCode)
		}
		if !strings.Contains(output, "Aborted") {
			t.Errorf("expected 'Aborted' in output, got: %s", output)
		}
	})
}

func TestRun_WhitespaceTrimming(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Input with extra whitespace in comma-separated values
		input := strings.Join([]string{
			"claude",                  // agent command
			"  --model  ,  opus  ,  ", // flags with whitespace
			"",                        // default iterations
			"",                        // default completion
			"",                        // default include iteration count
			"",                        // no guardrails
			"y",                       // configure SCM
			"git",                     // SCM command
			"  commit  ,  push  ,  ",  // tasks with whitespace
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		// Verify flags are trimmed
		expectedFlags := []string{"--model", "opus"}
		if len(settings.Agent.Flags) != len(expectedFlags) {
			t.Errorf("flags = %v, want %v", settings.Agent.Flags, expectedFlags)
		} else {
			for i, f := range settings.Agent.Flags {
				if f != expectedFlags[i] {
					t.Errorf("flags[%d] = %q, want %q", i, f, expectedFlags[i])
				}
			}
		}

		// Verify SCM tasks are trimmed
		expectedTasks := []string{"commit", "push"}
		if settings.SCM == nil {
			t.Fatal("SCM should not be nil")
		}
		if len(settings.SCM.Tasks) != len(expectedTasks) {
			t.Errorf("scm.tasks = %v, want %v", settings.SCM.Tasks, expectedTasks)
		} else {
			for i, task := range settings.SCM.Tasks {
				if task != expectedTasks[i] {
					t.Errorf("scm.tasks[%d] = %q, want %q", i, task, expectedTasks[i])
				}
			}
		}
	})
}

func TestRun_InvalidMaxIterations_Reprompt(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		input := strings.Join([]string{
			"claude", // agent command
			"",       // no flags
			"abc",    // invalid - not a number
			"0",      // invalid - must be > 0
			"15",     // valid
			"",       // default completion
			"",       // default include iteration count
			"",       // no guardrails
			"N",      // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stderr = oldStderr
		stderrOutput := buf.String()

		// Verify error messages
		if !strings.Contains(stderrOutput, "valid number") {
			t.Errorf("expected 'valid number' in stderr, got: %s", stderrOutput)
		}
		if !strings.Contains(stderrOutput, "greater than 0") {
			t.Errorf("expected 'greater than 0' in stderr, got: %s", stderrOutput)
		}

		// Verify final value
		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if settings.MaximumIterations != 15 {
			t.Errorf("maximumIterations = %d, want 15", settings.MaximumIterations)
		}
	})
}

func TestRun_ZeroGuardrails(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude", // agent command
			"",       // no flags
			"",       // default iterations
			"",       // default completion
			"",       // default include iteration count
			"",       // immediately exit guardrails (no guardrails added)
			"N",      // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if len(settings.Guardrails) != 0 {
			t.Errorf("expected 0 guardrails, got %d", len(settings.Guardrails))
		}
	})
}

func TestRun_CustomCompletionResponse(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude",   // agent command
			"",         // no flags
			"",         // default iterations
			"FINISHED", // custom completion response
			"",         // default include iteration count
			"",         // no guardrails
			"N",        // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if settings.CompletionResponse != "FINISHED" {
			t.Errorf("completionResponse = %q, want %q", settings.CompletionResponse, "FINISHED")
		}
	})
}

func TestRun_IncludeIterationCountPrompt(t *testing.T) {
	withTempDir(t, func(dir string) {
		captureExit()
		defer restoreExit()
		defer mockTTY(true)()

		input := strings.Join([]string{
			"claude", // agent command
			"",       // no flags
			"",       // default iterations
			"",       // default completion
			"y",      // include iteration count
			"",       // no guardrails
			"N",      // no SCM
		}, "\n") + "\n"

		withStdin(t, input, func() {
			err := Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		data, err := os.ReadFile(".ralph/settings.json")
		if err != nil {
			t.Fatal(err)
		}

		var settings config.Settings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}

		if !settings.IncludeIterationCountInPrompt {
			t.Errorf("includeIterationCountInPrompt = %v, want true", settings.IncludeIterationCountInPrompt)
		}
	})
}
