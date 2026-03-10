package controlplane

// RecommendationCandidate is the provider-agnostic schema for a recommendation
// draft before it is persisted to the project recommendation store.
type RecommendationCandidate struct {
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	ProposedAction string `json:"proposed_action"`
	Evidence       string `json:"evidence"`
	DedupeKey      string `json:"dedupe_key"`
}

// AttentionSignal is the provider-agnostic schema for task states that need
// human or agent attention in prompt context.
type AttentionSignal struct {
	Kind    string `json:"kind"`
	TaskID  string `json:"task_id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Phase   string `json:"phase,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// PromotedDraft captures a draft artifact that may later be promoted into a
// task, decision, or another project record.
type PromotedDraft struct {
	TargetType string `json:"target_type"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Content    string `json:"content"`
}

// HandoffPack is the provider-agnostic schema for passing compact work context
// between phases, agents, or human reviewers.
type HandoffPack struct {
	TaskID        string          `json:"task_id,omitempty"`
	TaskTitle     string          `json:"task_title,omitempty"`
	CurrentPhase  string          `json:"current_phase,omitempty"`
	Summary       string          `json:"summary"`
	NextSteps     []string        `json:"next_steps,omitempty"`
	OpenQuestions []string        `json:"open_questions,omitempty"`
	Risks         []string        `json:"risks,omitempty"`
	Drafts        []PromotedDraft `json:"drafts,omitempty"`
}
