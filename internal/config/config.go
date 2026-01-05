package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	MaximumIterations   int         `json:"maximumIterations"`
	CompletionResponse  string      `json:"completionResponse"`
	OutputTruncateChars int         `json:"outputTruncateChars"`
	StreamAgentOutput   bool        `json:"streamAgentOutput"`
	Agent               AgentConfig `json:"agent"`
	Guardrails          []Guardrail `json:"guardrails"`
	SCM                 *SCMConfig  `json:"scm,omitempty"`
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
}

// SCMConfig defines SCM command and tasks.
type SCMConfig struct {
	Command string   `json:"command"`
	Tasks   []string `json:"tasks"`
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
		MaximumIterations:   DefaultMaximumIterations,
		CompletionResponse:  DefaultCompletionResponse,
		OutputTruncateChars: DefaultOutputTruncateChars,
		StreamAgentOutput:   DefaultStreamAgentOutput,
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
func deepMerge(settings *Settings, data []byte) error {
	// Parse into a map for selective merging
	var local map[string]json.RawMessage
	if err := json.Unmarshal(data, &local); err != nil {
		return err
	}

	// Handle scalar overrides
	if v, ok := local["maximumIterations"]; ok {
		var val int
		if err := json.Unmarshal(v, &val); err == nil {
			settings.MaximumIterations = val
		}
	}
	if v, ok := local["completionResponse"]; ok {
		var val string
		if err := json.Unmarshal(v, &val); err == nil {
			settings.CompletionResponse = val
		}
	}
	if v, ok := local["outputTruncateChars"]; ok {
		var val int
		if err := json.Unmarshal(v, &val); err == nil {
			settings.OutputTruncateChars = val
		}
	}
	if v, ok := local["streamAgentOutput"]; ok {
		var val bool
		if err := json.Unmarshal(v, &val); err == nil {
			settings.StreamAgentOutput = val
		}
	}

	// Handle agent object (recursive merge)
	if v, ok := local["agent"]; ok {
		var agentLocal map[string]json.RawMessage
		if err := json.Unmarshal(v, &agentLocal); err == nil {
			if cmd, ok := agentLocal["command"]; ok {
				var val string
				if err := json.Unmarshal(cmd, &val); err == nil {
					settings.Agent.Command = val
				}
			}
			if flags, ok := agentLocal["flags"]; ok {
				var val []string
				if err := json.Unmarshal(flags, &val); err == nil {
					settings.Agent.Flags = val // arrays replace
				}
			}
		}
	}

	// Handle guardrails array (replace)
	if v, ok := local["guardrails"]; ok {
		var val []Guardrail
		if err := json.Unmarshal(v, &val); err == nil {
			settings.Guardrails = val
		}
	}

	// Handle SCM object (recursive merge)
	if v, ok := local["scm"]; ok {
		var scmLocal map[string]json.RawMessage
		if err := json.Unmarshal(v, &scmLocal); err == nil {
			if settings.SCM == nil {
				settings.SCM = &SCMConfig{}
			}
			if cmd, ok := scmLocal["command"]; ok {
				var val string
				if err := json.Unmarshal(cmd, &val); err == nil {
					settings.SCM.Command = val
				}
			}
			if tasks, ok := scmLocal["tasks"]; ok {
				var val []string
				if err := json.Unmarshal(tasks, &val); err == nil {
					settings.SCM.Tasks = val // arrays replace
				}
			}
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

// Validate checks that required settings are present.
func (s *Settings) Validate() error {
	if s.Agent.Command == "" {
		return errors.New("agent.command must be configured in settings file")
	}
	return nil
}
