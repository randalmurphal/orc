// Package gate provides gate evaluation for orc phase transitions.
package gate

import (
	"slices"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// Resolver resolves the effective gate type for a phase.
// Resolution order (highest to lowest precedence):
// 1. Task-specific override (task_gate_overrides table)
// 2. Weight-specific override (config.Gates.WeightOverrides)
// 3. Phase-specific override (config.Gates.PhaseOverrides OR phase_gates table)
// 4. Check if phase is enabled (EnabledPhases/DisabledPhases)
// 5. Default type (config.Gates.DefaultType)
type Resolver struct {
	cfg             *config.Config
	taskOverrides   map[string]*db.TaskGateOverride // keyed by phase_id
	phaseGates      map[string]*db.PhaseGate        // keyed by phase_id
}

// ResolverOption configures a Resolver.
type ResolverOption func(*Resolver)

// WithTaskOverrides sets task-specific gate overrides.
func WithTaskOverrides(overrides map[string]*db.TaskGateOverride) ResolverOption {
	return func(r *Resolver) {
		r.taskOverrides = overrides
	}
}

// WithPhaseGates sets phase gate configurations from the database.
func WithPhaseGates(gates map[string]*db.PhaseGate) ResolverOption {
	return func(r *Resolver) {
		r.phaseGates = gates
	}
}

// NewResolver creates a new gate resolver.
func NewResolver(cfg *config.Config, opts ...ResolverOption) *Resolver {
	r := &Resolver{
		cfg:           cfg,
		taskOverrides: make(map[string]*db.TaskGateOverride),
		phaseGates:    make(map[string]*db.PhaseGate),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ResolveResult contains the resolved gate configuration.
type ResolveResult struct {
	GateType GateType
	Enabled  bool
	Source   string // For debugging: "task_override", "weight_override", "phase_override", "phase_gate", "enabled_phases", "default"
}

// Resolve determines the effective gate type for a phase.
// taskWeight should be the string representation of the task weight (e.g., "small", "medium", "large").
func (r *Resolver) Resolve(phaseID, taskWeight string) ResolveResult {
	// 1. Check task-specific override (highest precedence)
	if override, ok := r.taskOverrides[phaseID]; ok {
		return ResolveResult{
			GateType: GateType(override.GateType),
			Enabled:  true,
			Source:   "task_override",
		}
	}

	// 2. Check weight-specific override
	if r.cfg != nil && r.cfg.Gates.WeightOverrides != nil {
		if weightOverrides, ok := r.cfg.Gates.WeightOverrides[taskWeight]; ok {
			if gateType, ok := weightOverrides[phaseID]; ok {
				return ResolveResult{
					GateType: GateType(gateType),
					Enabled:  true,
					Source:   "weight_override",
				}
			}
		}
	}

	// 3. Check phase-specific override from config
	if r.cfg != nil && r.cfg.Gates.PhaseOverrides != nil {
		if gateType, ok := r.cfg.Gates.PhaseOverrides[phaseID]; ok {
			return ResolveResult{
				GateType: GateType(gateType),
				Enabled:  true,
				Source:   "phase_override",
			}
		}
	}

	// 4. Check phase gates from database
	if gate, ok := r.phaseGates[phaseID]; ok && gate.Enabled {
		return ResolveResult{
			GateType: GateType(gate.GateType),
			Enabled:  true,
			Source:   "phase_gate",
		}
	}

	// 5. Check if phase is enabled via EnabledPhases/DisabledPhases
	if !r.isPhaseEnabled(phaseID) {
		return ResolveResult{
			GateType: GateSkip,
			Enabled:  false,
			Source:   "disabled",
		}
	}

	// 6. Return default
	defaultType := GateAuto
	if r.cfg != nil && r.cfg.Gates.DefaultType != "" {
		defaultType = GateType(r.cfg.Gates.DefaultType)
	}

	return ResolveResult{
		GateType: defaultType,
		Enabled:  true,
		Source:   "default",
	}
}

// isPhaseEnabled checks if a phase should have gates enabled based on config.
func (r *Resolver) isPhaseEnabled(phaseID string) bool {
	if r.cfg == nil {
		return true // Default to enabled when no config
	}

	// DisabledPhases takes precedence
	if len(r.cfg.Gates.DisabledPhases) > 0 {
		if slices.Contains(r.cfg.Gates.DisabledPhases, phaseID) {
			return false
		}
	}

	// If AllPhases is true, enable all (unless in DisabledPhases above)
	if r.cfg.Gates.AllPhases {
		return true
	}

	// If EnabledPhases is set, only those phases have gates
	if len(r.cfg.Gates.EnabledPhases) > 0 {
		return slices.Contains(r.cfg.Gates.EnabledPhases, phaseID)
	}

	// Default: gates are enabled for all phases
	return true
}

// IsPhaseGated returns true if the phase should have a gate evaluated.
// A phase is gated unless:
// - It's in DisabledPhases
// - AllPhases is false AND EnabledPhases is set AND phase is not in EnabledPhases
// - The resolved gate type is "skip"
func (r *Resolver) IsPhaseGated(phaseID, taskWeight string) bool {
	result := r.Resolve(phaseID, taskWeight)
	return result.Enabled && result.GateType != GateSkip
}
