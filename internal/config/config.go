package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Default values per PRD.
const (
	DefaultMaximumIterations   = 10
	DefaultCompletionResponse  = "DONE"
	DefaultOutputTruncateChars = 5000
	DefaultStreamAgentOutput   = true
)

// Settings represents the runtime configuration.
type Settings struct {
	MaximumIterations             int            `json:"maximumIterations"`
	CompletionResponse            string         `json:"completionResponse"`
	OutputTruncateChars           int            `json:"outputTruncateChars"`
	StreamAgentOutput             bool           `json:"streamAgentOutput"`
	IncludeIterationCountInPrompt bool           `json:"includeIterationCountInPrompt"`
	Agent                         AgentConfig    `json:"agent"`
	Guardrails                    []Guardrail    `json:"guardrails"`
	SCM                           *SCMConfig     `json:"scm,omitempty"`
	Reviews                       *ReviewsConfig `json:"reviews,omitempty"`
}

// AgentConfig defines the agent command and flags.
type AgentConfig struct {
	Command string   `json:"command"`
	Flags   []string `json:"flags"`
}

// Guardrail defines a guardrail command and its fail action.
type Guardrail struct {
	Command    string `json:"command"`
	FailAction string `json:"failAction"` // APPEND, PREPEND, REPLACE
	Hint       string `json:"hint,omitempty"`
}

// SCMConfig defines SCM command and tasks.
type SCMConfig struct {
	Command string   `json:"command"`
	Tasks   []string `json:"tasks"`
}

// ReviewsConfig defines automated review cycles configuration.
type ReviewsConfig struct {
	ReviewAfter         int            `json:"reviewAfter"`
	GuardrailRetryLimit int            `json:"guardrailRetryLimit"`
	Prompts             []ReviewPrompt `json:"prompts"`
	// PromptsOmitted tracks whether prompts was omitted (nil) vs explicitly empty.
	// When true, default prompts should be used.
	PromptsOmitted bool `json:"-"`
}

// ReviewPrompt defines a single review prompt in the cycle.
type ReviewPrompt struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// CLIOverrides captures CLI flags that can override settings.
type CLIOverrides struct {
	MaximumIterations  *int
	CompletionResponse *string
	StreamAgentOutput  *bool
}

// NewDefaults returns a Settings struct with default values.
func NewDefaults() Settings {
	return Settings{
		MaximumIterations:             DefaultMaximumIterations,
		CompletionResponse:            DefaultCompletionResponse,
		OutputTruncateChars:           DefaultOutputTruncateChars,
		StreamAgentOutput:             DefaultStreamAgentOutput,
		IncludeIterationCountInPrompt: false,
	}
}

// Load reads the settings from the given path.
// Returns defaults if file not found.
func Load(path string) (Settings, error) {
	settings := NewDefaults()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}

	// Re-apply defaults for zero values if not set in JSON
	if settings.MaximumIterations == 0 {
		settings.MaximumIterations = DefaultMaximumIterations
	}
	if settings.CompletionResponse == "" {
		settings.CompletionResponse = DefaultCompletionResponse
	}
	if settings.OutputTruncateChars == 0 {
		settings.OutputTruncateChars = DefaultOutputTruncateChars
	}

	// Detect if reviews.prompts was omitted (nil slice means omitted, empty slice means explicit [])
	if settings.Reviews != nil && settings.Reviews.Prompts == nil {
		settings.Reviews.PromptsOmitted = true
	}

	return settings, nil
}

// LoadWithLocal loads base settings and merges local overrides.
func LoadWithLocal(basePath string) (Settings, error) {
	settings, err := Load(basePath)
	if err != nil {
		return settings, err
	}

	// Determine local path
	dir := filepath.Dir(basePath)
	localPath := filepath.Join(dir, "settings.local.json")

	localData, err := os.ReadFile(localPath)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}

	// Deep merge local into base
	if err := deepMerge(&settings, localData); err != nil {
		return settings, err
	}

	return settings, nil
}

// deepMerge merges JSON data into existing settings.
// Scalars override, arrays replace, objects merge recursively.
// Returns an error if any field has an invalid type.
func deepMerge(settings *Settings, data []byte) error {
	// Parse into a map for selective merging
	var local map[string]json.RawMessage
	if err := json.Unmarshal(data, &local); err != nil {
		return err
	}

	// Handle scalar overrides
	if v, ok := local["maximumIterations"]; ok {
		var val int
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("maximumIterations: %w", err)
		}
		settings.MaximumIterations = val
	}
	if v, ok := local["completionResponse"]; ok {
		var val string
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("completionResponse: %w", err)
		}
		settings.CompletionResponse = val
	}
	if v, ok := local["outputTruncateChars"]; ok {
		var val int
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("outputTruncateChars: %w", err)
		}
		settings.OutputTruncateChars = val
	}
	if v, ok := local["streamAgentOutput"]; ok {
		var val bool
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("streamAgentOutput: %w", err)
		}
		settings.StreamAgentOutput = val
	}
	if v, ok := local["includeIterationCountInPrompt"]; ok {
		var val bool
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("includeIterationCountInPrompt: %w", err)
		}
		settings.IncludeIterationCountInPrompt = val
	}

	// Handle agent object (recursive merge)
	if v, ok := local["agent"]; ok {
		var agentLocal map[string]json.RawMessage
		if err := json.Unmarshal(v, &agentLocal); err != nil {
			return fmt.Errorf("agent: %w", err)
		}
		if cmd, ok := agentLocal["command"]; ok {
			var val string
			if err := json.Unmarshal(cmd, &val); err != nil {
				return fmt.Errorf("agent.command: %w", err)
			}
			settings.Agent.Command = val
		}
		if flags, ok := agentLocal["flags"]; ok {
			var val []string
			if err := json.Unmarshal(flags, &val); err != nil {
				return fmt.Errorf("agent.flags: %w", err)
			}
			settings.Agent.Flags = val // arrays replace
		}
	}

	// Handle guardrails array (replace)
	if v, ok := local["guardrails"]; ok {
		var val []Guardrail
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("guardrails: %w", err)
		}
		settings.Guardrails = val
	}

	// Handle SCM object (recursive merge)
	if v, ok := local["scm"]; ok {
		var scmLocal map[string]json.RawMessage
		if err := json.Unmarshal(v, &scmLocal); err != nil {
			return fmt.Errorf("scm: %w", err)
		}
		if settings.SCM == nil {
			settings.SCM = &SCMConfig{}
		}
		if cmd, ok := scmLocal["command"]; ok {
			var val string
			if err := json.Unmarshal(cmd, &val); err != nil {
				return fmt.Errorf("scm.command: %w", err)
			}
			settings.SCM.Command = val
		}
		if tasks, ok := scmLocal["tasks"]; ok {
			var val []string
			if err := json.Unmarshal(tasks, &val); err != nil {
				return fmt.Errorf("scm.tasks: %w", err)
			}
			settings.SCM.Tasks = val // arrays replace
		}
	}

	// Handle reviews object (recursive merge)
	if v, ok := local["reviews"]; ok {
		var reviewsLocal map[string]json.RawMessage
		if err := json.Unmarshal(v, &reviewsLocal); err != nil {
			return fmt.Errorf("reviews: %w", err)
		}
		if settings.Reviews == nil {
			settings.Reviews = &ReviewsConfig{PromptsOmitted: true}
		}
		if ra, ok := reviewsLocal["reviewAfter"]; ok {
			var val int
			if err := json.Unmarshal(ra, &val); err != nil {
				return fmt.Errorf("reviews.reviewAfter: %w", err)
			}
			settings.Reviews.ReviewAfter = val
		}
		if grl, ok := reviewsLocal["guardrailRetryLimit"]; ok {
			var val int
			if err := json.Unmarshal(grl, &val); err != nil {
				return fmt.Errorf("reviews.guardrailRetryLimit: %w", err)
			}
			settings.Reviews.GuardrailRetryLimit = val
		}
		if prompts, ok := reviewsLocal["prompts"]; ok {
			var val []ReviewPrompt
			if err := json.Unmarshal(prompts, &val); err != nil {
				return fmt.Errorf("reviews.prompts: %w", err)
			}
			settings.Reviews.Prompts = val // arrays replace
			settings.Reviews.PromptsOmitted = false
		}
	}

	return nil
}

// ApplyCLIOverrides applies CLI flag overrides to settings.
func (s *Settings) ApplyCLIOverrides(overrides CLIOverrides) {
	if overrides.MaximumIterations != nil {
		s.MaximumIterations = *overrides.MaximumIterations
	}
	if overrides.CompletionResponse != nil {
		s.CompletionResponse = *overrides.CompletionResponse
	}
	if overrides.StreamAgentOutput != nil {
		s.StreamAgentOutput = *overrides.StreamAgentOutput
	}
}

// Validate checks that required settings are present and valid.
func (s *Settings) Validate() error {
	if s.Agent.Command == "" {
		return fmt.Errorf("agent.command must be configured in settings file")
	}

	if s.MaximumIterations <= 0 {
		return fmt.Errorf("maximumIterations must be a positive integer, got %d", s.MaximumIterations)
	}

	if s.CompletionResponse == "" {
		return fmt.Errorf("completionResponse must not be empty")
	}

	if s.OutputTruncateChars <= 0 {
		return fmt.Errorf("outputTruncateChars must be a positive integer, got %d", s.OutputTruncateChars)
	}

	// Validate guardrails
	validActions := map[string]bool{"APPEND": true, "PREPEND": true, "REPLACE": true}
	for i, g := range s.Guardrails {
		if g.Command == "" {
			return fmt.Errorf("guardrails[%d].command must not be empty", i)
		}
		action := strings.ToUpper(g.FailAction)
		if !validActions[action] {
			return fmt.Errorf("guardrails[%d].failAction must be APPEND, PREPEND, or REPLACE, got %q", i, g.FailAction)
		}
	}

	// Validate SCM config: command required when tasks exist
	if s.SCM != nil && len(s.SCM.Tasks) > 0 && s.SCM.Command == "" {
		return fmt.Errorf("scm.command must be configured when scm.tasks is non-empty")
	}

	// Validate reviews config
	if s.Reviews != nil {
		if s.Reviews.ReviewAfter < 0 {
			return fmt.Errorf("reviews.reviewAfter must be non-negative, got %d", s.Reviews.ReviewAfter)
		}
		if s.Reviews.GuardrailRetryLimit < 0 {
			return fmt.Errorf("reviews.guardrailRetryLimit must be non-negative, got %d", s.Reviews.GuardrailRetryLimit)
		}
		for i, p := range s.Reviews.Prompts {
			if p.Name == "" {
				return fmt.Errorf("reviews.prompts[%d].name must not be empty", i)
			}
			if p.Prompt == "" {
				return fmt.Errorf("reviews.prompts[%d].prompt must not be empty", i)
			}
		}
	}

	return nil
}

// DefaultReviewPrompts returns the default review prompts per the PRD.
func DefaultReviewPrompts() []ReviewPrompt {
	return []ReviewPrompt{
		{
			Name:   "detailed",
			Prompt: "Review your implementation for correctness, edge cases, and error handling.",
		},
		{
			Name:   "architecture",
			Prompt: "Step back and review the overall design. Are we solving the right problem the right way?",
		},
		{
			Name:   "security",
			Prompt: "Review for security vulnerabilities: injection, auth, data exposure, and input validation.",
		},
		{
			Name:   "codeHealth",
			Prompt: "Review for code health: naming, structure, duplication, and simplicity.",
		},
	}
}

// ReviewsEnabled returns true if review cycles are configured and enabled.
func (r *ReviewsConfig) ReviewsEnabled() bool {
	if r == nil || r.ReviewAfter == 0 {
		return false
	}
	// Enabled if prompts were omitted (use defaults) or prompts array is non-empty
	return r.PromptsOmitted || len(r.Prompts) > 0
}

// GetPrompts returns the effective review prompts (custom or defaults).
func (r *ReviewsConfig) GetPrompts() []ReviewPrompt {
	if r.PromptsOmitted || len(r.Prompts) == 0 {
		return DefaultReviewPrompts()
	}
	return r.Prompts
}
