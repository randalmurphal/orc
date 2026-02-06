package executor

import (
	"context"
	"fmt"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/variable"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// PhaseTypeExecutor executes a non-LLM phase type.
type PhaseTypeExecutor interface {
	ExecutePhase(ctx context.Context, params PhaseTypeParams) (PhaseResult, error)
	Name() string
}

// PhaseTypeParams contains all context needed by a PhaseTypeExecutor.
type PhaseTypeParams struct {
	PhaseTemplate   *db.PhaseTemplate
	Task            *orcv1.Task
	Vars            variable.VariableSet
	RCtx            *variable.ResolutionContext
	KnowledgeConfig *KnowledgePhaseConfig
}

// PhaseTypeRegistry maps type strings to PhaseTypeExecutor implementations.
type PhaseTypeRegistry struct {
	executors map[string]PhaseTypeExecutor
}

// NewPhaseTypeRegistry creates an empty registry.
func NewPhaseTypeRegistry() *PhaseTypeRegistry {
	return &PhaseTypeRegistry{
		executors: make(map[string]PhaseTypeExecutor),
	}
}

// NewDefaultPhaseTypeRegistry creates a registry pre-populated with the
// built-in "llm" and "knowledge" executors.
func NewDefaultPhaseTypeRegistry() *PhaseTypeRegistry {
	r := NewPhaseTypeRegistry()
	r.Register("llm", &llmPhaseTypeExecutor{})
	r.Register("knowledge", NewKnowledgePhaseExecutor(nil))
	r.Register("script", NewScriptPhaseExecutor())
	r.Register("api", NewAPIPhaseExecutor())
	return r
}

// Register adds a PhaseTypeExecutor for the given type string.
// Panics if executor is nil.
func (r *PhaseTypeRegistry) Register(typeName string, executor PhaseTypeExecutor) {
	if executor == nil {
		panic(fmt.Sprintf("cannot register nil executor for type %q", typeName))
	}
	r.executors[typeName] = executor
}

// Get returns the executor for the given type string.
// Empty string defaults to "llm". Returns an error for unknown types.
func (r *PhaseTypeRegistry) Get(typeName string) (PhaseTypeExecutor, error) {
	if typeName == "" {
		typeName = "llm"
	}
	exec, ok := r.executors[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown phase type %q: no executor registered", typeName)
	}
	return exec, nil
}

// llmPhaseTypeExecutor is a sentinel executor for the "llm" type.
// It is never actually called — the dispatch logic in executePhase() handles
// LLM types by falling through to executeWithClaude(). This executor exists
// so that Get("llm") succeeds and the registry validates the type.
type llmPhaseTypeExecutor struct{}

func (e *llmPhaseTypeExecutor) ExecutePhase(_ context.Context, params PhaseTypeParams) (PhaseResult, error) {
	return PhaseResult{
		PhaseID: params.PhaseTemplate.ID,
		Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
	}, nil
}

func (e *llmPhaseTypeExecutor) Name() string {
	return "llm"
}
