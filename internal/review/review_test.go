package review

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/richclement/ralph-cli/internal/agent"
	"github.com/richclement/ralph-cli/internal/config"
	"github.com/richclement/ralph-cli/internal/guardrail"
)

func TestRunner_ReviewsDisabled(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{Command: "echo"},
	}

	agentRunner := agent.NewRunner(settings)
	guardrailRunner := guardrail.NewRunner(5000, false)
	runner := NewRunner(settings, agentRunner, guardrailRunner, false)

	stats, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stats.PromptsRun != 0 {
		t.Errorf("PromptsRun = %d, want 0 when reviews disabled", stats.PromptsRun)
	}
}

func TestRunner_ReviewsDisabledZeroReviewAfter(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{Command: "echo"},
		Reviews: &config.ReviewsConfig{
			ReviewAfter:         0, // Disabled
			GuardrailRetryLimit: 3,
			PromptsOmitted:      true,
		},
	}

	agentRunner := agent.NewRunner(settings)
	guardrailRunner := guardrail.NewRunner(5000, false)
	runner := NewRunner(settings, agentRunner, guardrailRunner, false)

	stats, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stats.PromptsRun != 0 {
		t.Errorf("PromptsRun = %d, want 0 when reviewAfter=0", stats.PromptsRun)
	}
}

func TestRunner_ReviewsDisabledEmptyPrompts(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{Command: "echo"},
		Reviews: &config.ReviewsConfig{
			ReviewAfter:         5,
			GuardrailRetryLimit: 3,
			Prompts:             []config.ReviewPrompt{}, // Explicitly empty
			PromptsOmitted:      false,
		},
	}

	agentRunner := agent.NewRunner(settings)
	guardrailRunner := guardrail.NewRunner(5000, false)
	runner := NewRunner(settings, agentRunner, guardrailRunner, false)

	stats, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stats.PromptsRun != 0 {
		t.Errorf("PromptsRun = %d, want 0 when prompts explicitly empty", stats.PromptsRun)
	}
}

func TestRunner_CancelledContext(t *testing.T) {
	settings := &config.Settings{
		Agent:             config.AgentConfig{Command: "echo"},
		StreamAgentOutput: false,
		Reviews: &config.ReviewsConfig{
			ReviewAfter:         1,
			GuardrailRetryLimit: 3,
			Prompts: []config.ReviewPrompt{
				{Name: "test", Prompt: "Test review"},
			},
		},
	}

	agentRunner := agent.NewRunner(settings)
	guardrailRunner := guardrail.NewRunner(5000, false)
	runner := NewRunner(settings, agentRunner, guardrailRunner, false)

	// Suppress output
	runner.Stdout = &bytes.Buffer{}
	runner.Stderr = &bytes.Buffer{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := runner.Run(ctx, 1)
	if err != context.Canceled {
		t.Errorf("Run should return context.Canceled, got: %v", err)
	}
}

func TestStats_ZeroValues(t *testing.T) {
	stats := Stats{}
	if stats.PromptsRun != 0 {
		t.Errorf("PromptsRun should be 0 by default")
	}
	if stats.TotalRetries != 0 {
		t.Errorf("TotalRetries should be 0 by default")
	}
	if stats.GuardrailFails != 0 {
		t.Errorf("GuardrailFails should be 0 by default")
	}
}

func TestRunner_GuardrailRetryPath(t *testing.T) {
	// Create a temp directory for guardrail logs
	tmpDir := t.TempDir()

	// Use a script that fails on first two calls then succeeds
	// We use a counter file to track invocations
	counterFile := tmpDir + "/counter"
	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		OutputTruncateChars: 5000,
		Guardrails: []config.Guardrail{
			{
				// This script: reads counter, increments it, fails if < 3, passes otherwise
				Command:    fmt.Sprintf(`count=$(cat %s 2>/dev/null || echo 0); count=$((count + 1)); echo $count > %s; if [ $count -lt 3 ]; then echo "Fail attempt $count"; exit 1; else echo "Pass"; exit 0; fi`, counterFile, counterFile),
				FailAction: "APPEND",
			},
		},
		Reviews: &config.ReviewsConfig{
			ReviewAfter:         1,
			GuardrailRetryLimit: 5, // Allow up to 5 retries
			Prompts: []config.ReviewPrompt{
				{Name: "test", Prompt: "Test review"},
			},
		},
	}

	agentRunner := agent.NewRunner(settings)
	guardrailRunner := guardrail.NewRunner(5000, false)
	guardrailRunner.OutputDir = tmpDir
	runner := NewRunner(settings, agentRunner, guardrailRunner, false)

	// Suppress output
	runner.Stdout = &bytes.Buffer{}
	runner.Stderr = &bytes.Buffer{}

	stats, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Should have run 1 prompt
	if stats.PromptsRun != 1 {
		t.Errorf("PromptsRun = %d, want 1", stats.PromptsRun)
	}

	// Should have failed guardrails twice (calls 1 and 2 fail, call 3 passes)
	if stats.GuardrailFails != 2 {
		t.Errorf("GuardrailFails = %d, want 2", stats.GuardrailFails)
	}

	// Should have retried twice
	if stats.TotalRetries != 2 {
		t.Errorf("TotalRetries = %d, want 2", stats.TotalRetries)
	}
}

func TestRunner_GuardrailRetryLimitZero(t *testing.T) {
	// With guardrailRetryLimit=0, should run once and not retry even on failure
	tmpDir := t.TempDir()

	settings := &config.Settings{
		Agent:               config.AgentConfig{Command: "echo"},
		OutputTruncateChars: 5000,
		Guardrails: []config.Guardrail{
			{
				Command:    "exit 1", // Always fails
				FailAction: "APPEND",
			},
		},
		Reviews: &config.ReviewsConfig{
			ReviewAfter:         1,
			GuardrailRetryLimit: 0, // No retries - run once only
			Prompts: []config.ReviewPrompt{
				{Name: "test", Prompt: "Test review"},
			},
		},
	}

	agentRunner := agent.NewRunner(settings)
	guardrailRunner := guardrail.NewRunner(5000, false)
	guardrailRunner.OutputDir = tmpDir
	runner := NewRunner(settings, agentRunner, guardrailRunner, false)

	// Suppress output
	runner.Stdout = &bytes.Buffer{}
	runner.Stderr = &bytes.Buffer{}

	stats, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Should have run 1 prompt
	if stats.PromptsRun != 1 {
		t.Errorf("PromptsRun = %d, want 1", stats.PromptsRun)
	}

	// Should have 1 guardrail failure (the initial run)
	if stats.GuardrailFails != 1 {
		t.Errorf("GuardrailFails = %d, want 1", stats.GuardrailFails)
	}

	// Should have 1 retry counted (the failed attempt triggers retry counter)
	if stats.TotalRetries != 1 {
		t.Errorf("TotalRetries = %d, want 1", stats.TotalRetries)
	}
}
