package db

import (
	"encoding/json"
	"fmt"
)

// GateInputConfig defines what context the gate evaluator receives.
// Used for JSON serialization in the gate_input_config column.
type GateInputConfig struct {
	IncludePhaseOutput []string `json:"include_phase_output,omitempty"`
	IncludeTask        bool     `json:"include_task,omitempty"`
	ExtraVars          []string `json:"extra_vars,omitempty"`
}

// GateOutputConfig defines what happens with gate evaluation results.
// Used for JSON serialization in the gate_output_config column.
type GateOutputConfig struct {
	VariableName string `json:"variable_name,omitempty"`
	OnApproved   string `json:"on_approved,omitempty"`
	OnRejected   string `json:"on_rejected,omitempty"`
	RetryFrom    string `json:"retry_from,omitempty"`
	Script       string `json:"script,omitempty"`
}

// BeforePhaseTrigger defines a trigger that runs before a phase starts.
// Used for JSON serialization in the before_triggers column.
type BeforePhaseTrigger struct {
	AgentID      string           `json:"agent_id"`
	InputConfig  *GateInputConfig  `json:"input_config,omitempty"`
	OutputConfig *GateOutputConfig `json:"output_config,omitempty"`
	Mode         string           `json:"mode,omitempty"`
}

// WorkflowTrigger defines a workflow-level lifecycle trigger.
// Used for JSON serialization in the triggers column.
type WorkflowTrigger struct {
	Event        string           `json:"event"`
	AgentID      string           `json:"agent_id"`
	InputConfig  *GateInputConfig  `json:"input_config,omitempty"`
	OutputConfig *GateOutputConfig `json:"output_config,omitempty"`
	Mode         string           `json:"mode,omitempty"`
	Enabled      bool             `json:"enabled,omitempty"`
}

// ParseGateInputConfig parses a JSON string into GateInputConfig.
// Returns nil for empty string. Returns error for invalid JSON.
func ParseGateInputConfig(jsonStr string) (*GateInputConfig, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var cfg GateInputConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, fmt.Errorf("parse gate input config: %w", err)
	}
	return &cfg, nil
}

// MarshalGateInputConfig serializes GateInputConfig to JSON string.
// Returns empty string for nil.
func MarshalGateInputConfig(cfg *GateInputConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal gate input config: %w", err)
	}
	return string(data), nil
}

// ParseGateOutputConfig parses a JSON string into GateOutputConfig.
// Returns nil for empty string. Returns error for invalid JSON.
func ParseGateOutputConfig(jsonStr string) (*GateOutputConfig, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var cfg GateOutputConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, fmt.Errorf("parse gate output config: %w", err)
	}
	return &cfg, nil
}

// MarshalGateOutputConfig serializes GateOutputConfig to JSON string.
// Returns empty string for nil.
func MarshalGateOutputConfig(cfg *GateOutputConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal gate output config: %w", err)
	}
	return string(data), nil
}

// ParseBeforeTriggers parses a JSON string into a slice of BeforePhaseTrigger.
// Returns nil for empty string. Returns error for invalid JSON.
func ParseBeforeTriggers(jsonStr string) ([]BeforePhaseTrigger, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var triggers []BeforePhaseTrigger
	if err := json.Unmarshal([]byte(jsonStr), &triggers); err != nil {
		return nil, fmt.Errorf("parse before triggers: %w", err)
	}
	return triggers, nil
}

// MarshalBeforeTriggers serializes a slice of BeforePhaseTrigger to JSON string.
// Returns empty string for nil/empty slice.
func MarshalBeforeTriggers(triggers []BeforePhaseTrigger) (string, error) {
	if len(triggers) == 0 {
		return "", nil
	}
	data, err := json.Marshal(triggers)
	if err != nil {
		return "", fmt.Errorf("marshal before triggers: %w", err)
	}
	return string(data), nil
}

// ParseWorkflowTriggers parses a JSON string into a slice of WorkflowTrigger.
// Returns nil for empty string. Returns error for invalid JSON.
func ParseWorkflowTriggers(jsonStr string) ([]WorkflowTrigger, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var triggers []WorkflowTrigger
	if err := json.Unmarshal([]byte(jsonStr), &triggers); err != nil {
		return nil, fmt.Errorf("parse workflow triggers: %w", err)
	}
	return triggers, nil
}

// MarshalWorkflowTriggers serializes a slice of WorkflowTrigger to JSON string.
// Returns empty string for nil/empty slice.
func MarshalWorkflowTriggers(triggers []WorkflowTrigger) (string, error) {
	if len(triggers) == 0 {
		return "", nil
	}
	data, err := json.Marshal(triggers)
	if err != nil {
		return "", fmt.Errorf("marshal workflow triggers: %w", err)
	}
	return string(data), nil
}
