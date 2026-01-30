// Package storage provides proto type conversion for database operations.
package storage

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// ============================================================================
// Proto Task <-> DB Task Conversion
// ============================================================================

// protoTaskToDBTask converts an orcv1.Task to db.Task for storage.
// Note: Executor fields (ExecutorPID, ExecutorHostname, LastHeartbeat) are now
// included in the proto and will be converted. However, SaveTask implementations
// should preserve existing executor values from the database when the proto values
// are zero/nil to avoid accidentally clearing live executor state.
func protoTaskToDBTask(t *orcv1.Task) *db.Task {
	if t == nil {
		return nil
	}

	// Convert metadata map to JSON
	var metadataJSON string
	if len(t.Metadata) > 0 {
		if data, err := json.Marshal(t.Metadata); err == nil {
			metadataJSON = string(data)
		}
	}

	// Convert quality metrics to JSON
	var qualityJSON string
	if t.Quality != nil {
		if data, err := json.Marshal(protoQualityToMap(t.Quality)); err == nil {
			qualityJSON = string(data)
		}
	}

	// Convert retry context to JSON
	var retryContextJSON string
	if t.Execution != nil && t.Execution.RetryContext != nil {
		if data, err := json.Marshal(protoRetryContextToMap(t.Execution.RetryContext)); err == nil {
			retryContextJSON = string(data)
		}
	}

	// Convert timestamps
	var startedAt, completedAt, lastHeartbeat *time.Time
	if t.StartedAt != nil {
		ts := t.StartedAt.AsTime()
		startedAt = &ts
	}
	if t.CompletedAt != nil {
		ts := t.CompletedAt.AsTime()
		completedAt = &ts
	}
	if t.LastHeartbeat != nil {
		ts := t.LastHeartbeat.AsTime()
		lastHeartbeat = &ts
	}

	createdAt := time.Now()
	if t.CreatedAt != nil {
		createdAt = t.CreatedAt.AsTime()
	}
	updatedAt := time.Now()
	if t.UpdatedAt != nil {
		updatedAt = t.UpdatedAt.AsTime()
	}

	// Get total cost from execution state
	var totalCostUSD float64
	if t.Execution != nil && t.Execution.Cost != nil {
		totalCostUSD = t.Execution.Cost.TotalCostUsd
	}

	// Convert PR labels and reviewers slices to JSON for db storage
	var prLabelsJSON, prReviewersJSON string
	if len(t.PrLabels) > 0 {
		if data, err := json.Marshal(t.PrLabels); err == nil {
			prLabelsJSON = string(data)
		}
	}
	if len(t.PrReviewers) > 0 {
		if data, err := json.Marshal(t.PrReviewers); err == nil {
			prReviewersJSON = string(data)
		}
	}

	return &db.Task{
		ID:               t.Id,
		Title:            t.Title,
		Description:      ptrToString(t.Description),
		Weight:           task.WeightFromProto(t.Weight),
		WorkflowID:       ptrToString(t.WorkflowId),
		Status:           task.StatusFromProto(t.Status),
		CurrentPhase:     ptrToString(t.CurrentPhase),
		Branch:           t.Branch,
		TargetBranch:     ptrToString(t.TargetBranch),
		Queue:            task.QueueFromProto(t.Queue),
		Priority:         task.PriorityFromProto(t.Priority),
		Category:         task.CategoryFromProto(t.Category),
		InitiativeID:     ptrToString(t.InitiativeId),
		CreatedAt:        createdAt,
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
		UpdatedAt:        updatedAt,
		Metadata:         metadataJSON,
		Quality:          qualityJSON,
		IsAutomation:     t.IsAutomation,
		TotalCostUSD:     totalCostUSD,
		RetryContext:     retryContextJSON,
		ExecutorPID:      int(t.ExecutorPid),
		ExecutorHostname: ptrToString(t.ExecutorHostname),
		LastHeartbeat:    lastHeartbeat,
		// Branch control fields
		BranchName:     t.BranchName,
		PrDraft:        t.PrDraft,
		PrLabels:       prLabelsJSON,
		PrReviewers:    prReviewersJSON,
		PrLabelsSet:    t.PrLabelsSet,
		PrReviewersSet: t.PrReviewersSet,
	}
}

// dbTaskToProtoTask converts a db.Task to orcv1.Task.
// Note: Executor fields from db.Task are not transferred to proto
// as they are internal implementation details.
func dbTaskToProtoTask(dbTask *db.Task) *orcv1.Task {
	if dbTask == nil {
		return nil
	}

	// Parse metadata JSON to map
	var metadata map[string]string
	if dbTask.Metadata != "" {
		_ = json.Unmarshal([]byte(dbTask.Metadata), &metadata)
	}

	// Parse quality JSON
	var quality *orcv1.QualityMetrics
	if dbTask.Quality != "" {
		var qm map[string]any
		if err := json.Unmarshal([]byte(dbTask.Quality), &qm); err == nil {
			quality = mapToProtoQuality(qm)
		}
	}

	// Convert timestamps
	var startedAt, completedAt *timestamppb.Timestamp
	if dbTask.StartedAt != nil {
		startedAt = timestamppb.New(*dbTask.StartedAt)
	}
	if dbTask.CompletedAt != nil {
		completedAt = timestamppb.New(*dbTask.CompletedAt)
	}

	// Initialize execution state
	execution := &orcv1.ExecutionState{
		Phases: make(map[string]*orcv1.PhaseState),
		Gates:  []*orcv1.GateDecision{},
		Tokens: &orcv1.TokenUsage{},
		Cost: &orcv1.CostTracking{
			TotalCostUsd: dbTask.TotalCostUSD,
		},
	}

	// Parse retry context
	if dbTask.RetryContext != "" {
		var rc map[string]any
		if err := json.Unmarshal([]byte(dbTask.RetryContext), &rc); err == nil {
			execution.RetryContext = mapToProtoRetryContext(rc)
		}
	}

	// Build executor tracking fields
	var lastHeartbeat *timestamppb.Timestamp
	if dbTask.LastHeartbeat != nil {
		lastHeartbeat = timestamppb.New(*dbTask.LastHeartbeat)
	}

	// Parse PR labels and reviewers from JSON
	var prLabels, prReviewers []string
	if dbTask.PrLabels != "" {
		_ = json.Unmarshal([]byte(dbTask.PrLabels), &prLabels)
	}
	if dbTask.PrReviewers != "" {
		_ = json.Unmarshal([]byte(dbTask.PrReviewers), &prReviewers)
	}

	return &orcv1.Task{
		Id:               dbTask.ID,
		Title:            dbTask.Title,
		Description:      stringToPtr(dbTask.Description),
		Weight:           task.WeightToProto(dbTask.Weight),
		WorkflowId:       stringToPtr(dbTask.WorkflowID),
		Status:           task.StatusToProto(dbTask.Status),
		CurrentPhase:     stringToPtr(dbTask.CurrentPhase),
		Branch:           dbTask.Branch,
		TargetBranch:     stringToPtr(dbTask.TargetBranch),
		Queue:            task.QueueToProto(dbTask.Queue),
		Priority:         task.PriorityToProto(dbTask.Priority),
		Category:         task.CategoryToProto(dbTask.Category),
		InitiativeId:     stringToPtr(dbTask.InitiativeID),
		CreatedAt:        timestamppb.New(dbTask.CreatedAt),
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
		UpdatedAt:        timestamppb.New(dbTask.UpdatedAt),
		Metadata:         metadata,
		Quality:          quality,
		IsAutomation:     dbTask.IsAutomation,
		Execution:        execution,
		ExecutorPid:      int32(dbTask.ExecutorPID),
		ExecutorHostname: stringToPtr(dbTask.ExecutorHostname),
		LastHeartbeat:    lastHeartbeat,
		// Branch control fields
		BranchName:     dbTask.BranchName,
		PrDraft:        dbTask.PrDraft,
		PrLabels:       prLabels,
		PrReviewers:    prReviewers,
		PrLabelsSet:    dbTask.PrLabelsSet,
		PrReviewersSet: dbTask.PrReviewersSet,
	}
}

// ============================================================================
// Proto Phase <-> DB Phase Conversion
// ============================================================================

// protoPhaseToDBPhase converts an orcv1.PhaseState to db.Phase.
func protoPhaseToDBPhase(taskID, phaseID string, ps *orcv1.PhaseState) *db.Phase {
	if ps == nil {
		return nil
	}

	var startedAt *time.Time
	if ps.StartedAt != nil {
		ts := ps.StartedAt.AsTime()
		if !ts.IsZero() {
			startedAt = &ts
		}
	}

	var completedAt *time.Time
	if ps.CompletedAt != nil {
		ts := ps.CompletedAt.AsTime()
		if !ts.IsZero() {
			completedAt = &ts
		}
	}

	var inputTokens, outputTokens int
	if ps.Tokens != nil {
		inputTokens = int(ps.Tokens.InputTokens)
		outputTokens = int(ps.Tokens.OutputTokens)
	}

	return &db.Phase{
		TaskID:       taskID,
		PhaseID:      phaseID,
		Status:       task.PhaseStatusFromProto(ps.Status),
		Iterations:   int(ps.Iterations),
		StartedAt:    startedAt,
		CompletedAt:  completedAt,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      0, // Not stored at phase level
		ErrorMessage: ptrToString(ps.Error),
		CommitSHA:    ptrToString(ps.CommitSha),
		SessionID:    ptrToString(ps.SessionId),
	}
}

// dbPhaseToProtoPhase converts a db.Phase to orcv1.PhaseState.
func dbPhaseToProtoPhase(dbPhase *db.Phase) *orcv1.PhaseState {
	if dbPhase == nil {
		return nil
	}

	var startedAt, completedAt *timestamppb.Timestamp
	if dbPhase.StartedAt != nil {
		startedAt = timestamppb.New(*dbPhase.StartedAt)
	}
	if dbPhase.CompletedAt != nil {
		completedAt = timestamppb.New(*dbPhase.CompletedAt)
	}

	return &orcv1.PhaseState{
		Status:      task.PhaseStatusToProto(dbPhase.Status),
		Iterations:  int32(dbPhase.Iterations),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Error:       stringToPtr(dbPhase.ErrorMessage),
		CommitSha:   stringToPtr(dbPhase.CommitSHA),
		SessionId:   stringToPtr(dbPhase.SessionID),
		Tokens: &orcv1.TokenUsage{
			InputTokens:  int32(dbPhase.InputTokens),
			OutputTokens: int32(dbPhase.OutputTokens),
		},
	}
}

// ============================================================================
// Proto Gate Decision <-> DB Gate Decision Conversion
// ============================================================================

// protoGateToDBGate converts an orcv1.GateDecision to db.GateDecision.
func protoGateToDBGate(taskID string, g *orcv1.GateDecision) *db.GateDecision {
	if g == nil {
		return nil
	}

	var decidedAt time.Time
	if g.Timestamp != nil {
		decidedAt = g.Timestamp.AsTime()
	} else {
		decidedAt = time.Now()
	}

	return &db.GateDecision{
		TaskID:    taskID,
		Phase:     g.Phase,
		GateType:  g.GateType,
		Approved:  g.Approved,
		Reason:    ptrToString(g.Reason),
		DecidedAt: decidedAt,
		// DecidedBy not in proto - set to empty or "system"
	}
}

// dbGateToProtoGate converts a db.GateDecision to orcv1.GateDecision.
func dbGateToProtoGate(dbGate *db.GateDecision) *orcv1.GateDecision {
	if dbGate == nil {
		return nil
	}

	return &orcv1.GateDecision{
		Phase:     dbGate.Phase,
		GateType:  dbGate.GateType,
		Approved:  dbGate.Approved,
		Reason:    stringToPtr(dbGate.Reason),
		Timestamp: timestamppb.New(dbGate.DecidedAt),
	}
}

// dbGatesToProtoGates converts a slice of db.GateDecision to orcv1.GateDecision.
func dbGatesToProtoGates(dbGates []db.GateDecision) []*orcv1.GateDecision {
	if len(dbGates) == 0 {
		return nil
	}

	gates := make([]*orcv1.GateDecision, len(dbGates))
	for i, g := range dbGates {
		gates[i] = dbGateToProtoGate(&g)
	}
	return gates
}

// ============================================================================
// Helper Conversion Functions
// ============================================================================

// ptrToString returns the value of a string pointer, or empty string if nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// stringToPtr returns a pointer to the string, or nil if empty.
func stringToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// protoQualityToMap converts orcv1.QualityMetrics to a map for JSON serialization.
func protoQualityToMap(q *orcv1.QualityMetrics) map[string]any {
	if q == nil {
		return nil
	}
	m := make(map[string]any)
	if len(q.PhaseRetries) > 0 {
		m["phase_retries"] = q.PhaseRetries
	}
	if q.ReviewRejections > 0 {
		m["review_rejections"] = q.ReviewRejections
	}
	if q.ManualIntervention {
		m["manual_intervention"] = q.ManualIntervention
	}
	if q.ManualInterventionReason != nil {
		m["manual_intervention_reason"] = *q.ManualInterventionReason
	}
	if q.TotalRetries > 0 {
		m["total_retries"] = q.TotalRetries
	}
	return m
}

// mapToProtoQuality converts a map to orcv1.QualityMetrics.
func mapToProtoQuality(m map[string]any) *orcv1.QualityMetrics {
	if m == nil {
		return nil
	}
	q := &orcv1.QualityMetrics{}
	// PhaseRetries is a map[string]int32 in proto
	if v, ok := m["phase_retries"].(map[string]any); ok {
		q.PhaseRetries = make(map[string]int32)
		for k, val := range v {
			if f, ok := val.(float64); ok {
				q.PhaseRetries[k] = int32(f)
			}
		}
	}
	if v, ok := m["review_rejections"].(float64); ok {
		q.ReviewRejections = int32(v)
	}
	if v, ok := m["manual_intervention"].(bool); ok {
		q.ManualIntervention = v
	}
	if v, ok := m["manual_intervention_reason"].(string); ok {
		q.ManualInterventionReason = &v
	}
	if v, ok := m["total_retries"].(float64); ok {
		q.TotalRetries = int32(v)
	}
	return q
}

// protoRetryContextToMap converts orcv1.RetryContext to a map for JSON serialization.
func protoRetryContextToMap(rc *orcv1.RetryContext) map[string]any {
	if rc == nil {
		return nil
	}
	m := make(map[string]any)
	m["from_phase"] = rc.FromPhase
	m["to_phase"] = rc.ToPhase
	m["reason"] = rc.Reason
	m["attempt"] = rc.Attempt
	if rc.FailureOutput != nil {
		m["failure_output"] = *rc.FailureOutput
	}
	if rc.ContextFile != nil {
		m["context_file"] = *rc.ContextFile
	}
	if rc.Timestamp != nil {
		m["timestamp"] = rc.Timestamp.AsTime().Format(time.RFC3339)
	}
	return m
}

// mapToProtoRetryContext converts a map to orcv1.RetryContext.
func mapToProtoRetryContext(m map[string]any) *orcv1.RetryContext {
	if m == nil {
		return nil
	}
	rc := &orcv1.RetryContext{}
	if v, ok := m["from_phase"].(string); ok {
		rc.FromPhase = v
	}
	if v, ok := m["to_phase"].(string); ok {
		rc.ToPhase = v
	}
	if v, ok := m["reason"].(string); ok {
		rc.Reason = v
	}
	if v, ok := m["attempt"].(float64); ok {
		rc.Attempt = int32(v)
	}
	if v, ok := m["failure_output"].(string); ok {
		rc.FailureOutput = &v
	}
	if v, ok := m["context_file"].(string); ok {
		rc.ContextFile = &v
	}
	if v, ok := m["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			rc.Timestamp = timestamppb.New(t)
		}
	}
	return rc
}

// ============================================================================
// Proto Initiative <-> DB Initiative Conversion
// ============================================================================

// protoInitiativeToDBInitiative converts an orcv1.Initiative to db.Initiative.
func protoInitiativeToDBInitiative(i *orcv1.Initiative) *db.Initiative {
	if i == nil {
		return nil
	}

	var ownerInitials, ownerDisplayName, ownerEmail string
	if i.Owner != nil {
		ownerInitials = i.Owner.Initials
		ownerDisplayName = ptrToString(i.Owner.DisplayName)
		ownerEmail = ptrToString(i.Owner.Email)
	}

	createdAt := time.Now()
	if i.CreatedAt != nil {
		createdAt = i.CreatedAt.AsTime()
	}
	updatedAt := time.Now()
	if i.UpdatedAt != nil {
		updatedAt = i.UpdatedAt.AsTime()
	}

	return &db.Initiative{
		ID:               i.Id,
		Title:            i.Title,
		Status:           initiativeStatusFromProto(i.Status),
		OwnerInitials:    ownerInitials,
		OwnerDisplayName: ownerDisplayName,
		OwnerEmail:       ownerEmail,
		Vision:           ptrToString(i.Vision),
		BranchBase:       ptrToString(i.BranchBase),
		BranchPrefix:     ptrToString(i.BranchPrefix),
		MergeStatus:      mergeStatusFromProto(i.MergeStatus),
		MergeCommit:      ptrToString(i.MergeCommit),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}
}

// dbInitiativeToProtoInitiative converts a db.Initiative to orcv1.Initiative.
func dbInitiativeToProtoInitiative(dbInit *db.Initiative) *orcv1.Initiative {
	if dbInit == nil {
		return nil
	}

	var owner *orcv1.Identity
	if dbInit.OwnerInitials != "" {
		owner = &orcv1.Identity{
			Initials:    dbInit.OwnerInitials,
			DisplayName: stringToPtr(dbInit.OwnerDisplayName),
			Email:       stringToPtr(dbInit.OwnerEmail),
		}
	}

	return &orcv1.Initiative{
		Id:           dbInit.ID,
		Title:        dbInit.Title,
		Status:       initiativeStatusToProto(dbInit.Status),
		Owner:        owner,
		Vision:       stringToPtr(dbInit.Vision),
		BranchBase:   stringToPtr(dbInit.BranchBase),
		BranchPrefix: stringToPtr(dbInit.BranchPrefix),
		MergeStatus:  mergeStatusToProto(dbInit.MergeStatus),
		MergeCommit:  stringToPtr(dbInit.MergeCommit),
		CreatedAt:    timestamppb.New(dbInit.CreatedAt),
		UpdatedAt:    timestamppb.New(dbInit.UpdatedAt),
		// Decisions, Tasks, BlockedBy, Blocks populated separately
	}
}

// dbDecisionToProtoDecision converts a db.InitiativeDecision to orcv1.InitiativeDecision.
func dbDecisionToProtoDecision(d *db.InitiativeDecision) *orcv1.InitiativeDecision {
	if d == nil {
		return nil
	}
	return &orcv1.InitiativeDecision{
		Id:        d.ID,
		Date:      timestamppb.New(d.DecidedAt),
		By:        d.DecidedBy,
		Decision:  d.Decision,
		Rationale: stringToPtr(d.Rationale),
	}
}

// protoDecisionToDBDecision converts an orcv1.InitiativeDecision to db.InitiativeDecision.
func protoDecisionToDBDecision(initiativeID string, d *orcv1.InitiativeDecision) *db.InitiativeDecision {
	if d == nil {
		return nil
	}
	decidedAt := time.Now()
	if d.Date != nil {
		decidedAt = d.Date.AsTime()
	}
	return &db.InitiativeDecision{
		ID:           d.Id,
		InitiativeID: initiativeID,
		DecidedAt:    decidedAt,
		DecidedBy:    d.By,
		Decision:     d.Decision,
		Rationale:    ptrToString(d.Rationale),
	}
}

// dbTaskRefToProtoTaskRef converts db task info to orcv1.TaskRef.
func dbTaskRefToProtoTaskRef(taskID, title, status string) *orcv1.TaskRef {
	return &orcv1.TaskRef{
		Id:     taskID,
		Title:  title,
		Status: task.StatusToProto(status),
	}
}

// ============================================================================
// Initiative Status Conversion
// ============================================================================

func initiativeStatusToProto(s string) orcv1.InitiativeStatus {
	switch s {
	case "draft":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT
	case "active":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	case "completed":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED
	case "archived":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED
	default:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED
	}
}

func initiativeStatusFromProto(s orcv1.InitiativeStatus) string {
	switch s {
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT:
		return "draft"
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE:
		return "active"
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED:
		return "completed"
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED:
		return "archived"
	default:
		return "draft"
	}
}

func mergeStatusToProto(s string) orcv1.MergeStatus {
	switch s {
	case "pending":
		return orcv1.MergeStatus_MERGE_STATUS_PENDING
	case "merged":
		return orcv1.MergeStatus_MERGE_STATUS_MERGED
	case "failed":
		return orcv1.MergeStatus_MERGE_STATUS_FAILED
	default:
		return orcv1.MergeStatus_MERGE_STATUS_UNSPECIFIED
	}
}

func mergeStatusFromProto(s orcv1.MergeStatus) string {
	switch s {
	case orcv1.MergeStatus_MERGE_STATUS_PENDING:
		return "pending"
	case orcv1.MergeStatus_MERGE_STATUS_MERGED:
		return "merged"
	case orcv1.MergeStatus_MERGE_STATUS_FAILED:
		return "failed"
	default:
		return ""
	}
}
