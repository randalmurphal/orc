package db

import (
	"database/sql"
	"time"
)

func sqlNullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func sqlNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullFloat64ToPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func nullBoolToPtr(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}

// sqlNullString converts a string to sql.NullString, treating empty strings as NULL.
func sqlNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPhaseTemplate(row rowScanner) (*PhaseTemplate, error) {
	pt := &PhaseTemplate{}
	var createdAt, updatedAt string
	var thinkingEnabled sql.NullBool
	var description, agentID, subAgents, promptContent, promptPath, inputVars, outputSchema, artifactType, outputVarName sql.NullString
	var outputType, qualityChecks sql.NullString
	var retryFromPhase, retryPromptPath sql.NullString
	var gateInputConfig, gateOutputConfig, gateMode, gateAgentID sql.NullString
	var runtimeConfig sql.NullString
	var phaseType sql.NullString
	var provider sql.NullString

	err := row.Scan(
		&pt.ID, &pt.Name, &description, &agentID, &subAgents,
		&pt.PromptSource, &promptContent, &promptPath,
		&inputVars, &outputSchema, &pt.ProducesArtifact, &artifactType, &outputVarName,
		&outputType, &qualityChecks,
		&thinkingEnabled, &pt.GateType, &pt.Checkpoint,
		&retryFromPhase, &retryPromptPath, &pt.IsBuiltin, &createdAt, &updatedAt,
		&gateInputConfig, &gateOutputConfig, &gateMode, &gateAgentID,
		&runtimeConfig, &phaseType, &provider,
	)
	if err != nil {
		return nil, err
	}

	pt.Description = description.String
	pt.AgentID = agentID.String
	pt.SubAgents = subAgents.String
	pt.PromptContent = promptContent.String
	pt.PromptPath = promptPath.String
	pt.InputVariables = inputVars.String
	pt.OutputSchema = outputSchema.String
	pt.ArtifactType = artifactType.String
	pt.OutputVarName = outputVarName.String
	pt.OutputType = outputType.String
	pt.QualityChecks = qualityChecks.String
	pt.ThinkingEnabled = nullBoolToPtr(thinkingEnabled)
	pt.RetryFromPhase = retryFromPhase.String
	pt.RetryPromptPath = retryPromptPath.String
	pt.GateInputConfig = gateInputConfig.String
	pt.GateOutputConfig = gateOutputConfig.String
	pt.GateMode = gateMode.String
	pt.GateAgentID = gateAgentID.String
	pt.RuntimeConfig = runtimeConfig.String
	pt.Type = phaseType.String
	pt.Provider = provider.String
	pt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	pt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return pt, nil
}

func scanPhaseTemplateRow(rows *sql.Rows) (*PhaseTemplate, error) {
	return scanPhaseTemplate(rows)
}

func scanWorkflow(row rowScanner) (*Workflow, error) {
	w := &Workflow{}
	var createdAt, updatedAt string
	var description, defaultModel, completionAction, targetBranch, basedOn, triggers sql.NullString
	var defaultProvider sql.NullString

	err := row.Scan(
		&w.ID, &w.Name, &description, &defaultModel, &defaultProvider, &w.DefaultThinking,
		&completionAction, &targetBranch, &w.IsBuiltin, &basedOn, &triggers, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	w.Description = description.String
	w.DefaultModel = defaultModel.String
	w.DefaultProvider = defaultProvider.String
	w.CompletionAction = completionAction.String
	w.TargetBranch = targetBranch.String
	w.BasedOn = basedOn.String
	w.Triggers = triggers.String
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	w.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return w, nil
}

func scanWorkflowRow(rows *sql.Rows) (*Workflow, error) {
	return scanWorkflow(rows)
}

func scanWorkflowPhaseRow(rows *sql.Rows) (*WorkflowPhase, error) {
	wp := &WorkflowPhase{}
	var dependsOn, agentOverride, subAgentsOverride sql.NullString
	var modelOverride, gateTypeOverride, condition, qualityChecksOverride, loopConfig, runtimeConfigOverride sql.NullString
	var beforeTriggers sql.NullString
	var thinkingOverride sql.NullBool
	var posX, posY sql.NullFloat64
	var typeOverride sql.NullString
	var providerOverride sql.NullString

	err := rows.Scan(
		&wp.ID, &wp.WorkflowID, &wp.PhaseTemplateID, &wp.Sequence, &dependsOn,
		&agentOverride, &subAgentsOverride,
		&modelOverride, &providerOverride, &thinkingOverride, &gateTypeOverride, &condition,
		&qualityChecksOverride, &loopConfig, &runtimeConfigOverride, &beforeTriggers, &posX, &posY,
		&typeOverride,
	)
	if err != nil {
		return nil, err
	}

	wp.DependsOn = dependsOn.String
	wp.AgentOverride = agentOverride.String
	wp.SubAgentsOverride = subAgentsOverride.String
	wp.ModelOverride = modelOverride.String
	wp.ProviderOverride = providerOverride.String
	wp.ThinkingOverride = nullBoolToPtr(thinkingOverride)
	wp.GateTypeOverride = gateTypeOverride.String
	wp.Condition = condition.String
	wp.QualityChecksOverride = qualityChecksOverride.String
	wp.LoopConfig = loopConfig.String
	wp.RuntimeConfigOverride = runtimeConfigOverride.String
	wp.BeforeTriggers = beforeTriggers.String
	wp.PositionX = nullFloat64ToPtr(posX)
	wp.PositionY = nullFloat64ToPtr(posY)
	wp.TypeOverride = typeOverride.String
	return wp, nil
}

func scanWorkflowVariableRow(rows *sql.Rows) (*WorkflowVariable, error) {
	wv := &WorkflowVariable{}
	var description, defaultValue, scriptContent, extract sql.NullString

	err := rows.Scan(
		&wv.ID, &wv.WorkflowID, &wv.Name, &description, &wv.SourceType, &wv.SourceConfig,
		&wv.Required, &defaultValue, &wv.CacheTTLSeconds, &scriptContent, &extract,
	)
	if err != nil {
		return nil, err
	}

	wv.Description = description.String
	wv.DefaultValue = defaultValue.String
	wv.ScriptContent = scriptContent.String
	wv.Extract = extract.String
	return wv, nil
}

func scanWorkflowRun(row rowScanner) (*WorkflowRun, error) {
	wr := &WorkflowRun{}
	var createdAt, updatedAt string
	var startedAt, completedAt sql.NullString
	var taskID, instructions, currentPhase, variablesSnapshot, runError, startedBy sql.NullString

	err := row.Scan(
		&wr.ID, &wr.WorkflowID, &wr.ContextType, &wr.ContextData, &taskID,
		&wr.Prompt, &instructions, &wr.Status, &currentPhase, &startedAt, &completedAt,
		&variablesSnapshot, &wr.TotalCostUSD, &wr.TotalInputTokens, &wr.TotalOutputTokens,
		&runError, &createdAt, &updatedAt, &startedBy,
	)
	if err != nil {
		return nil, err
	}

	if taskID.Valid {
		wr.TaskID = &taskID.String
	}
	wr.Instructions = instructions.String
	wr.CurrentPhase = currentPhase.String
	wr.VariablesSnapshot = variablesSnapshot.String
	wr.Error = runError.String
	wr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	wr.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		wr.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		wr.CompletedAt = &t
	}
	if startedBy.Valid {
		wr.StartedBy = startedBy.String
	}
	return wr, nil
}

func scanWorkflowRunRow(rows *sql.Rows) (*WorkflowRun, error) {
	return scanWorkflowRun(rows)
}

func scanWorkflowRunPhaseRow(rows *sql.Rows) (*WorkflowRunPhase, error) {
	wrp := &WorkflowRunPhase{}
	var startedAt, completedAt sql.NullString
	var commitSHA, content, phaseError, sessionID sql.NullString

	err := rows.Scan(
		&wrp.ID, &wrp.WorkflowRunID, &wrp.PhaseTemplateID, &wrp.Status, &wrp.Iterations,
		&startedAt, &completedAt, &commitSHA, &wrp.InputTokens, &wrp.OutputTokens, &wrp.CostUSD,
		&content, &phaseError, &sessionID,
	)
	if err != nil {
		return nil, err
	}

	wrp.CommitSHA = commitSHA.String
	wrp.Content = content.String
	wrp.Error = phaseError.String
	wrp.SessionID = sessionID.String
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		wrp.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		wrp.CompletedAt = &t
	}
	return wrp, nil
}
