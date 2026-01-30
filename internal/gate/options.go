package gate

import (
	"log/slog"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/db"
)

// LLMClientCreator creates claude.Client instances for AI gate evaluation.
// This interface breaks the import cycle between gate and executor packages.
type LLMClientCreator interface {
	NewSchemaClient(model string) claude.Client
}

// AgentLookup retrieves agents by ID. Implemented by db.ProjectDB.
type AgentLookup interface {
	GetAgent(id string) (*db.Agent, error)
}

// CostRecorder records cost entries. Implemented by db.GlobalDB.
type CostRecorder interface {
	RecordCost(entry db.CostEntry)
}

// Option configures an Evaluator.
type Option func(*Evaluator)

// WithAgentLookup sets the agent lookup for AI gate evaluation.
func WithAgentLookup(lookup AgentLookup) Option {
	return func(e *Evaluator) {
		e.agentLookup = lookup
	}
}

// WithClientCreator sets the LLM client creator for AI gate evaluation.
func WithClientCreator(creator LLMClientCreator) Option {
	return func(e *Evaluator) {
		e.clientCreator = creator
	}
}

// WithCostRecorder sets the cost recorder for AI gate evaluation.
func WithCostRecorder(recorder CostRecorder) Option {
	return func(e *Evaluator) {
		e.costRecorder = recorder
	}
}

// WithLogger sets the logger for the evaluator.
func WithLogger(logger *slog.Logger) Option {
	return func(e *Evaluator) {
		e.logger = logger
	}
}
