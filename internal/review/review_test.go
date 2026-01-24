package review

import (
	"bytes"
	"context"
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
