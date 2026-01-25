// Package api provides the Connect RPC and REST API server for orc.
// This file contains conversion functions between domain types and proto types.
// These are temporary until domain types are consolidated with proto types (Stream 4).
package api

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// Task Conversions
// ============================================================================

// TaskToProto converts a domain task.Task to proto orcv1.Task.
func TaskToProto(t *task.Task) *orcv1.Task {
	if t == nil {
		return nil
	}

	pb := &orcv1.Task{
		Id:                t.ID,
		Title:             t.Title,
		Branch:            t.Branch,
		Weight:            taskWeightToProto(t.Weight),
		Status:            taskStatusToProto(t.Status),
		Queue:             taskQueueToProto(t.Queue),
		Priority:          taskPriorityToProto(t.Priority),
		Category:          taskCategoryToProto(t.Category),
		BlockedBy:         t.BlockedBy,
		RelatedTo:         t.RelatedTo,
		IsAutomation:      t.IsAutomation,
		RequiresUiTesting: t.RequiresUITesting,
		CreatedAt:         timestamppb.New(t.CreatedAt),
		UpdatedAt:         timestamppb.New(t.UpdatedAt),
		Execution:         executionStateToProto(&t.Execution),
	}

	// Optional fields
	if t.Description != "" {
		pb.Description = &t.Description
	}
	if t.CurrentPhase != "" {
		pb.CurrentPhase = &t.CurrentPhase
	}
	if t.InitiativeID != "" {
		pb.InitiativeId = &t.InitiativeID
	}
	if t.WorkflowID != "" {
		pb.WorkflowId = &t.WorkflowID
	}
	if t.TargetBranch != "" {
		pb.TargetBranch = &t.TargetBranch
	}
	if t.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*t.StartedAt)
	}
	if t.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*t.CompletedAt)
	}

	// Nested types
	if t.TestingRequirements != nil {
		pb.TestingRequirements = testingRequirementsToProto(t.TestingRequirements)
	}
	if t.Quality != nil {
		pb.Quality = qualityMetricsToProto(t.Quality)
	}
	if t.PR != nil {
		pb.Pr = prInfoToProto(t.PR)
	}

	// Computed fields
	pb.Blocks = t.Blocks
	pb.ReferencedBy = t.ReferencedBy
	pb.IsBlocked = t.IsBlocked
	pb.UnmetBlockers = t.UnmetBlockers
	pb.DependencyStatus = dependencyStatusToProto(t.DependencyStatus)

	return pb
}

// ProtoToTask converts a proto orcv1.Task to domain task.Task.
func ProtoToTask(pb *orcv1.Task) *task.Task {
	if pb == nil {
		return nil
	}

	t := &task.Task{
		ID:                pb.Id,
		Title:             pb.Title,
		Branch:            pb.Branch,
		Weight:            protoToTaskWeight(pb.Weight),
		Status:            protoToTaskStatus(pb.Status),
		Queue:             protoToTaskQueue(pb.Queue),
		Priority:          protoToTaskPriority(pb.Priority),
		Category:          protoToTaskCategory(pb.Category),
		BlockedBy:         pb.BlockedBy,
		RelatedTo:         pb.RelatedTo,
		IsAutomation:      pb.IsAutomation,
		RequiresUITesting: pb.RequiresUiTesting,
		CreatedAt:         pb.CreatedAt.AsTime(),
		UpdatedAt:         pb.UpdatedAt.AsTime(),
		Metadata:          make(map[string]string),
	}

	// Optional fields
	if pb.Description != nil {
		t.Description = *pb.Description
	}
	if pb.CurrentPhase != nil {
		t.CurrentPhase = *pb.CurrentPhase
	}
	if pb.InitiativeId != nil {
		t.InitiativeID = *pb.InitiativeId
	}
	if pb.WorkflowId != nil {
		t.WorkflowID = *pb.WorkflowId
	}
	if pb.TargetBranch != nil {
		t.TargetBranch = *pb.TargetBranch
	}
	if pb.StartedAt != nil {
		startedAt := pb.StartedAt.AsTime()
		t.StartedAt = &startedAt
	}
	if pb.CompletedAt != nil {
		completedAt := pb.CompletedAt.AsTime()
		t.CompletedAt = &completedAt
	}

	// Nested types
	if pb.TestingRequirements != nil {
		t.TestingRequirements = protoToTestingRequirements(pb.TestingRequirements)
	}
	if pb.Quality != nil {
		t.Quality = protoToQualityMetrics(pb.Quality)
	}
	if pb.Pr != nil {
		t.PR = protoToPRInfo(pb.Pr)
	}
	if pb.Execution != nil {
		t.Execution = *protoToExecutionState(pb.Execution)
	} else {
		t.Execution = task.InitExecutionState()
	}

	return t
}

// ============================================================================
// Task Enum Conversions
// ============================================================================

func taskWeightToProto(w task.Weight) orcv1.TaskWeight {
	switch w {
	case task.WeightTrivial:
		return orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL
	case task.WeightSmall:
		return orcv1.TaskWeight_TASK_WEIGHT_SMALL
	case task.WeightMedium:
		return orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	case task.WeightLarge:
		return orcv1.TaskWeight_TASK_WEIGHT_LARGE
	default:
		return orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED
	}
}

func protoToTaskWeight(w orcv1.TaskWeight) task.Weight {
	switch w {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		return task.WeightTrivial
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		return task.WeightSmall
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		return task.WeightMedium
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return task.WeightLarge
	default:
		return task.WeightMedium
	}
}

func taskStatusToProto(s task.Status) orcv1.TaskStatus {
	switch s {
	case task.StatusCreated:
		return orcv1.TaskStatus_TASK_STATUS_CREATED
	case task.StatusClassifying:
		return orcv1.TaskStatus_TASK_STATUS_CLASSIFYING
	case task.StatusPlanned:
		return orcv1.TaskStatus_TASK_STATUS_PLANNED
	case task.StatusRunning:
		return orcv1.TaskStatus_TASK_STATUS_RUNNING
	case task.StatusPaused:
		return orcv1.TaskStatus_TASK_STATUS_PAUSED
	case task.StatusBlocked:
		return orcv1.TaskStatus_TASK_STATUS_BLOCKED
	case task.StatusFinalizing:
		return orcv1.TaskStatus_TASK_STATUS_FINALIZING
	case task.StatusCompleted:
		return orcv1.TaskStatus_TASK_STATUS_COMPLETED
	case task.StatusFailed:
		return orcv1.TaskStatus_TASK_STATUS_FAILED
	case task.StatusResolved:
		return orcv1.TaskStatus_TASK_STATUS_RESOLVED
	default:
		return orcv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func protoToTaskStatus(s orcv1.TaskStatus) task.Status {
	switch s {
	case orcv1.TaskStatus_TASK_STATUS_CREATED:
		return task.StatusCreated
	case orcv1.TaskStatus_TASK_STATUS_CLASSIFYING:
		return task.StatusClassifying
	case orcv1.TaskStatus_TASK_STATUS_PLANNED:
		return task.StatusPlanned
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return task.StatusRunning
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return task.StatusPaused
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return task.StatusBlocked
	case orcv1.TaskStatus_TASK_STATUS_FINALIZING:
		return task.StatusFinalizing
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return task.StatusCompleted
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return task.StatusFailed
	case orcv1.TaskStatus_TASK_STATUS_RESOLVED:
		return task.StatusResolved
	default:
		return task.StatusCreated
	}
}

func taskQueueToProto(q task.Queue) orcv1.TaskQueue {
	switch q {
	case task.QueueActive:
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	case task.QueueBacklog:
		return orcv1.TaskQueue_TASK_QUEUE_BACKLOG
	default:
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	}
}

func protoToTaskQueue(q orcv1.TaskQueue) task.Queue {
	switch q {
	case orcv1.TaskQueue_TASK_QUEUE_ACTIVE:
		return task.QueueActive
	case orcv1.TaskQueue_TASK_QUEUE_BACKLOG:
		return task.QueueBacklog
	default:
		return task.QueueActive
	}
}

func taskPriorityToProto(p task.Priority) orcv1.TaskPriority {
	switch p {
	case task.PriorityCritical:
		return orcv1.TaskPriority_TASK_PRIORITY_CRITICAL
	case task.PriorityHigh:
		return orcv1.TaskPriority_TASK_PRIORITY_HIGH
	case task.PriorityNormal:
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	case task.PriorityLow:
		return orcv1.TaskPriority_TASK_PRIORITY_LOW
	default:
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	}
}

func protoToTaskPriority(p orcv1.TaskPriority) task.Priority {
	switch p {
	case orcv1.TaskPriority_TASK_PRIORITY_CRITICAL:
		return task.PriorityCritical
	case orcv1.TaskPriority_TASK_PRIORITY_HIGH:
		return task.PriorityHigh
	case orcv1.TaskPriority_TASK_PRIORITY_NORMAL:
		return task.PriorityNormal
	case orcv1.TaskPriority_TASK_PRIORITY_LOW:
		return task.PriorityLow
	default:
		return task.PriorityNormal
	}
}

func taskCategoryToProto(c task.Category) orcv1.TaskCategory {
	switch c {
	case task.CategoryFeature:
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	case task.CategoryBug:
		return orcv1.TaskCategory_TASK_CATEGORY_BUG
	case task.CategoryRefactor:
		return orcv1.TaskCategory_TASK_CATEGORY_REFACTOR
	case task.CategoryChore:
		return orcv1.TaskCategory_TASK_CATEGORY_CHORE
	case task.CategoryDocs:
		return orcv1.TaskCategory_TASK_CATEGORY_DOCS
	case task.CategoryTest:
		return orcv1.TaskCategory_TASK_CATEGORY_TEST
	default:
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	}
}

func protoToTaskCategory(c orcv1.TaskCategory) task.Category {
	switch c {
	case orcv1.TaskCategory_TASK_CATEGORY_FEATURE:
		return task.CategoryFeature
	case orcv1.TaskCategory_TASK_CATEGORY_BUG:
		return task.CategoryBug
	case orcv1.TaskCategory_TASK_CATEGORY_REFACTOR:
		return task.CategoryRefactor
	case orcv1.TaskCategory_TASK_CATEGORY_CHORE:
		return task.CategoryChore
	case orcv1.TaskCategory_TASK_CATEGORY_DOCS:
		return task.CategoryDocs
	case orcv1.TaskCategory_TASK_CATEGORY_TEST:
		return task.CategoryTest
	default:
		return task.CategoryFeature
	}
}

func dependencyStatusToProto(ds task.DependencyStatus) orcv1.DependencyStatus {
	switch ds {
	case task.DependencyStatusBlocked:
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_BLOCKED
	case task.DependencyStatusReady:
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_READY
	case task.DependencyStatusNone:
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_NONE
	default:
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_UNSPECIFIED
	}
}

func prStatusToProto(s task.PRStatus) orcv1.PRStatus {
	switch s {
	case task.PRStatusNone:
		return orcv1.PRStatus_PR_STATUS_NONE
	case task.PRStatusDraft:
		return orcv1.PRStatus_PR_STATUS_DRAFT
	case task.PRStatusPendingReview:
		return orcv1.PRStatus_PR_STATUS_PENDING_REVIEW
	case task.PRStatusChangesRequested:
		return orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED
	case task.PRStatusApproved:
		return orcv1.PRStatus_PR_STATUS_APPROVED
	case task.PRStatusMerged:
		return orcv1.PRStatus_PR_STATUS_MERGED
	case task.PRStatusClosed:
		return orcv1.PRStatus_PR_STATUS_CLOSED
	default:
		return orcv1.PRStatus_PR_STATUS_UNSPECIFIED
	}
}

func protoToPRStatus(s orcv1.PRStatus) task.PRStatus {
	switch s {
	case orcv1.PRStatus_PR_STATUS_NONE:
		return task.PRStatusNone
	case orcv1.PRStatus_PR_STATUS_DRAFT:
		return task.PRStatusDraft
	case orcv1.PRStatus_PR_STATUS_PENDING_REVIEW:
		return task.PRStatusPendingReview
	case orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED:
		return task.PRStatusChangesRequested
	case orcv1.PRStatus_PR_STATUS_APPROVED:
		return task.PRStatusApproved
	case orcv1.PRStatus_PR_STATUS_MERGED:
		return task.PRStatusMerged
	case orcv1.PRStatus_PR_STATUS_CLOSED:
		return task.PRStatusClosed
	default:
		return task.PRStatusNone
	}
}

func phaseStatusToProto(s task.PhaseStatus) orcv1.PhaseStatus {
	switch s {
	case task.PhaseStatusPending:
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	case task.PhaseStatusRunning:
		return orcv1.PhaseStatus_PHASE_STATUS_RUNNING
	case task.PhaseStatusCompleted:
		return orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	case task.PhaseStatusFailed:
		return orcv1.PhaseStatus_PHASE_STATUS_FAILED
	case task.PhaseStatusPaused:
		return orcv1.PhaseStatus_PHASE_STATUS_PAUSED
	case task.PhaseStatusInterrupted:
		return orcv1.PhaseStatus_PHASE_STATUS_INTERRUPTED
	case task.PhaseStatusSkipped:
		return orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
	case task.PhaseStatusBlocked:
		return orcv1.PhaseStatus_PHASE_STATUS_BLOCKED
	default:
		return orcv1.PhaseStatus_PHASE_STATUS_UNSPECIFIED
	}
}

func protoToPhaseStatus(s orcv1.PhaseStatus) task.PhaseStatus {
	switch s {
	case orcv1.PhaseStatus_PHASE_STATUS_PENDING:
		return task.PhaseStatusPending
	case orcv1.PhaseStatus_PHASE_STATUS_RUNNING:
		return task.PhaseStatusRunning
	case orcv1.PhaseStatus_PHASE_STATUS_COMPLETED:
		return task.PhaseStatusCompleted
	case orcv1.PhaseStatus_PHASE_STATUS_FAILED:
		return task.PhaseStatusFailed
	case orcv1.PhaseStatus_PHASE_STATUS_PAUSED:
		return task.PhaseStatusPaused
	case orcv1.PhaseStatus_PHASE_STATUS_INTERRUPTED:
		return task.PhaseStatusInterrupted
	case orcv1.PhaseStatus_PHASE_STATUS_SKIPPED:
		return task.PhaseStatusSkipped
	case orcv1.PhaseStatus_PHASE_STATUS_BLOCKED:
		return task.PhaseStatusBlocked
	default:
		return task.PhaseStatusPending
	}
}

// ============================================================================
// Task Nested Type Conversions
// ============================================================================

func testingRequirementsToProto(tr *task.TestingRequirements) *orcv1.TestingRequirements {
	if tr == nil {
		return nil
	}
	return &orcv1.TestingRequirements{
		Unit:   tr.Unit,
		E2E:    tr.E2E,
		Visual: tr.Visual,
	}
}

func protoToTestingRequirements(pb *orcv1.TestingRequirements) *task.TestingRequirements {
	if pb == nil {
		return nil
	}
	return &task.TestingRequirements{
		Unit:   pb.Unit,
		E2E:    pb.E2E,
		Visual: pb.Visual,
	}
}

func qualityMetricsToProto(qm *task.QualityMetrics) *orcv1.QualityMetrics {
	if qm == nil {
		return nil
	}
	pb := &orcv1.QualityMetrics{
		ReviewRejections:   int32(qm.ReviewRejections),
		ManualIntervention: qm.ManualIntervention,
		TotalRetries:       int32(qm.TotalRetries),
	}
	if qm.PhaseRetries != nil {
		pb.PhaseRetries = make(map[string]int32, len(qm.PhaseRetries))
		for k, v := range qm.PhaseRetries {
			pb.PhaseRetries[k] = int32(v)
		}
	}
	if qm.ManualInterventionReason != "" {
		pb.ManualInterventionReason = &qm.ManualInterventionReason
	}
	return pb
}

func protoToQualityMetrics(pb *orcv1.QualityMetrics) *task.QualityMetrics {
	if pb == nil {
		return nil
	}
	qm := &task.QualityMetrics{
		ReviewRejections:   int(pb.ReviewRejections),
		ManualIntervention: pb.ManualIntervention,
		TotalRetries:       int(pb.TotalRetries),
	}
	if pb.PhaseRetries != nil {
		qm.PhaseRetries = make(map[string]int, len(pb.PhaseRetries))
		for k, v := range pb.PhaseRetries {
			qm.PhaseRetries[k] = int(v)
		}
	}
	if pb.ManualInterventionReason != nil {
		qm.ManualInterventionReason = *pb.ManualInterventionReason
	}
	return qm
}

func prInfoToProto(pr *task.PRInfo) *orcv1.PRInfo {
	if pr == nil {
		return nil
	}
	pb := &orcv1.PRInfo{
		Status:        prStatusToProto(pr.Status),
		Mergeable:     pr.Mergeable,
		ReviewCount:   int32(pr.ReviewCount),
		ApprovalCount: int32(pr.ApprovalCount),
		Merged:        pr.Merged,
	}
	if pr.URL != "" {
		pb.Url = &pr.URL
	}
	if pr.Number > 0 {
		n := int32(pr.Number)
		pb.Number = &n
	}
	if pr.ChecksStatus != "" {
		pb.ChecksStatus = &pr.ChecksStatus
	}
	if pr.LastCheckedAt != nil {
		pb.LastCheckedAt = timestamppb.New(*pr.LastCheckedAt)
	}
	if pr.MergedAt != nil {
		pb.MergedAt = timestamppb.New(*pr.MergedAt)
	}
	if pr.MergeCommitSHA != "" {
		pb.MergeCommitSha = &pr.MergeCommitSHA
	}
	if pr.TargetBranch != "" {
		pb.TargetBranch = &pr.TargetBranch
	}
	return pb
}

func protoToPRInfo(pb *orcv1.PRInfo) *task.PRInfo {
	if pb == nil {
		return nil
	}
	pr := &task.PRInfo{
		Status:        protoToPRStatus(pb.Status),
		Mergeable:     pb.Mergeable,
		ReviewCount:   int(pb.ReviewCount),
		ApprovalCount: int(pb.ApprovalCount),
		Merged:        pb.Merged,
	}
	if pb.Url != nil {
		pr.URL = *pb.Url
	}
	if pb.Number != nil {
		pr.Number = int(*pb.Number)
	}
	if pb.ChecksStatus != nil {
		pr.ChecksStatus = *pb.ChecksStatus
	}
	if pb.LastCheckedAt != nil {
		t := pb.LastCheckedAt.AsTime()
		pr.LastCheckedAt = &t
	}
	if pb.MergedAt != nil {
		t := pb.MergedAt.AsTime()
		pr.MergedAt = &t
	}
	if pb.MergeCommitSha != nil {
		pr.MergeCommitSHA = *pb.MergeCommitSha
	}
	if pb.TargetBranch != nil {
		pr.TargetBranch = *pb.TargetBranch
	}
	return pr
}

// ============================================================================
// Execution State Conversions
// ============================================================================

func executionStateToProto(e *task.ExecutionState) *orcv1.ExecutionState {
	if e == nil {
		return nil
	}
	pb := &orcv1.ExecutionState{
		CurrentIteration: int32(e.CurrentIteration),
		Tokens:           tokenUsageToProto(&e.Tokens),
		Cost:             costTrackingToProto(&e.Cost),
	}
	if e.Phases != nil {
		pb.Phases = make(map[string]*orcv1.PhaseState, len(e.Phases))
		for k, v := range e.Phases {
			pb.Phases[k] = phaseStateToProto(v)
		}
	}
	if len(e.Gates) > 0 {
		pb.Gates = make([]*orcv1.GateDecision, len(e.Gates))
		for i, g := range e.Gates {
			pb.Gates[i] = gateDecisionToProto(&g)
		}
	}
	if e.Session != nil {
		pb.Session = sessionInfoToProto(e.Session)
	}
	if e.Error != "" {
		pb.Error = &e.Error
	}
	if e.RetryContext != nil {
		pb.RetryContext = retryContextToProto(e.RetryContext)
	}
	if e.JSONLPath != "" {
		pb.JsonlPath = &e.JSONLPath
	}
	return pb
}

func protoToExecutionState(pb *orcv1.ExecutionState) *task.ExecutionState {
	if pb == nil {
		return nil
	}
	e := &task.ExecutionState{
		CurrentIteration: int(pb.CurrentIteration),
		Tokens:           *protoToTokenUsage(pb.Tokens),
		Cost:             *protoToCostTracking(pb.Cost),
	}
	if pb.Phases != nil {
		e.Phases = make(map[string]*task.PhaseState, len(pb.Phases))
		for k, v := range pb.Phases {
			e.Phases[k] = protoToPhaseState(v)
		}
	}
	if len(pb.Gates) > 0 {
		e.Gates = make([]task.GateDecision, len(pb.Gates))
		for i, g := range pb.Gates {
			e.Gates[i] = *protoToGateDecision(g)
		}
	}
	if pb.Session != nil {
		e.Session = protoToSessionInfo(pb.Session)
	}
	if pb.Error != nil {
		e.Error = *pb.Error
	}
	if pb.RetryContext != nil {
		e.RetryContext = protoToRetryContext(pb.RetryContext)
	}
	if pb.JsonlPath != nil {
		e.JSONLPath = *pb.JsonlPath
	}
	return e
}

func phaseStateToProto(ps *task.PhaseState) *orcv1.PhaseState {
	if ps == nil {
		return nil
	}
	pb := &orcv1.PhaseState{
		Status:     phaseStatusToProto(ps.Status),
		StartedAt:  timestamppb.New(ps.StartedAt),
		Iterations: int32(ps.Iterations),
		Artifacts:  ps.Artifacts,
		Tokens:     tokenUsageToProto(&ps.Tokens),
	}
	if ps.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*ps.CompletedAt)
	}
	if ps.InterruptedAt != nil {
		pb.InterruptedAt = timestamppb.New(*ps.InterruptedAt)
	}
	if ps.CommitSHA != "" {
		pb.CommitSha = &ps.CommitSHA
	}
	if ps.Error != "" {
		pb.Error = &ps.Error
	}
	if len(ps.ValidationHistory) > 0 {
		pb.ValidationHistory = make([]*orcv1.ValidationEntry, len(ps.ValidationHistory))
		for i, v := range ps.ValidationHistory {
			pb.ValidationHistory[i] = validationEntryToProto(&v)
		}
	}
	if ps.SessionID != "" {
		pb.SessionId = &ps.SessionID
	}
	return pb
}

func protoToPhaseState(pb *orcv1.PhaseState) *task.PhaseState {
	if pb == nil {
		return nil
	}
	ps := &task.PhaseState{
		Status:     protoToPhaseStatus(pb.Status),
		StartedAt:  pb.StartedAt.AsTime(),
		Iterations: int(pb.Iterations),
		Artifacts:  pb.Artifacts,
		Tokens:     *protoToTokenUsage(pb.Tokens),
	}
	if pb.CompletedAt != nil {
		t := pb.CompletedAt.AsTime()
		ps.CompletedAt = &t
	}
	if pb.InterruptedAt != nil {
		t := pb.InterruptedAt.AsTime()
		ps.InterruptedAt = &t
	}
	if pb.CommitSha != nil {
		ps.CommitSHA = *pb.CommitSha
	}
	if pb.Error != nil {
		ps.Error = *pb.Error
	}
	if len(pb.ValidationHistory) > 0 {
		ps.ValidationHistory = make([]task.ValidationEntry, len(pb.ValidationHistory))
		for i, v := range pb.ValidationHistory {
			ps.ValidationHistory[i] = *protoToValidationEntry(v)
		}
	}
	if pb.SessionId != nil {
		ps.SessionID = *pb.SessionId
	}
	return ps
}

func tokenUsageToProto(tu *task.TokenUsage) *orcv1.TokenUsage {
	if tu == nil {
		return nil
	}
	return &orcv1.TokenUsage{
		InputTokens:              int32(tu.InputTokens),
		OutputTokens:             int32(tu.OutputTokens),
		CacheCreationInputTokens: int32(tu.CacheCreationInputTokens),
		CacheReadInputTokens:     int32(tu.CacheReadInputTokens),
		TotalTokens:              int32(tu.TotalTokens),
	}
}

func protoToTokenUsage(pb *orcv1.TokenUsage) *task.TokenUsage {
	if pb == nil {
		return &task.TokenUsage{}
	}
	return &task.TokenUsage{
		InputTokens:              int(pb.InputTokens),
		OutputTokens:             int(pb.OutputTokens),
		CacheCreationInputTokens: int(pb.CacheCreationInputTokens),
		CacheReadInputTokens:     int(pb.CacheReadInputTokens),
		TotalTokens:              int(pb.TotalTokens),
	}
}

func costTrackingToProto(ct *task.CostTracking) *orcv1.CostTracking {
	if ct == nil {
		return nil
	}
	pb := &orcv1.CostTracking{
		TotalCostUsd:  ct.TotalCostUSD,
		LastUpdatedAt: timestamppb.New(ct.LastUpdatedAt),
	}
	if ct.PhaseCosts != nil {
		pb.PhaseCosts = make(map[string]float64, len(ct.PhaseCosts))
		for k, v := range ct.PhaseCosts {
			pb.PhaseCosts[k] = v
		}
	}
	return pb
}

func protoToCostTracking(pb *orcv1.CostTracking) *task.CostTracking {
	if pb == nil {
		return &task.CostTracking{}
	}
	ct := &task.CostTracking{
		TotalCostUSD: pb.TotalCostUsd,
	}
	if pb.LastUpdatedAt != nil {
		ct.LastUpdatedAt = pb.LastUpdatedAt.AsTime()
	}
	if pb.PhaseCosts != nil {
		ct.PhaseCosts = make(map[string]float64, len(pb.PhaseCosts))
		for k, v := range pb.PhaseCosts {
			ct.PhaseCosts[k] = v
		}
	}
	return ct
}

func gateDecisionToProto(gd *task.GateDecision) *orcv1.GateDecision {
	if gd == nil {
		return nil
	}
	pb := &orcv1.GateDecision{
		Phase:     gd.Phase,
		GateType:  gd.GateType,
		Approved:  gd.Approved,
		Timestamp: timestamppb.New(gd.Timestamp),
	}
	if gd.Reason != "" {
		pb.Reason = &gd.Reason
	}
	return pb
}

func protoToGateDecision(pb *orcv1.GateDecision) *task.GateDecision {
	if pb == nil {
		return nil
	}
	gd := &task.GateDecision{
		Phase:     pb.Phase,
		GateType:  pb.GateType,
		Approved:  pb.Approved,
		Timestamp: pb.Timestamp.AsTime(),
	}
	if pb.Reason != nil {
		gd.Reason = *pb.Reason
	}
	return gd
}

func sessionInfoToProto(si *task.SessionInfo) *orcv1.SessionInfo {
	if si == nil {
		return nil
	}
	return &orcv1.SessionInfo{
		Id:           si.ID,
		Model:        si.Model,
		Status:       si.Status,
		CreatedAt:    timestamppb.New(si.CreatedAt),
		LastActivity: timestamppb.New(si.LastActivity),
		TurnCount:    int32(si.TurnCount),
	}
}

func protoToSessionInfo(pb *orcv1.SessionInfo) *task.SessionInfo {
	if pb == nil {
		return nil
	}
	return &task.SessionInfo{
		ID:           pb.Id,
		Model:        pb.Model,
		Status:       pb.Status,
		CreatedAt:    pb.CreatedAt.AsTime(),
		LastActivity: pb.LastActivity.AsTime(),
		TurnCount:    int(pb.TurnCount),
	}
}

func retryContextToProto(rc *task.RetryContext) *orcv1.RetryContext {
	if rc == nil {
		return nil
	}
	pb := &orcv1.RetryContext{
		FromPhase: rc.FromPhase,
		ToPhase:   rc.ToPhase,
		Reason:    rc.Reason,
		Attempt:   int32(rc.Attempt),
		Timestamp: timestamppb.New(rc.Timestamp),
	}
	if rc.FailureOutput != "" {
		pb.FailureOutput = &rc.FailureOutput
	}
	if rc.ContextFile != "" {
		pb.ContextFile = &rc.ContextFile
	}
	return pb
}

func protoToRetryContext(pb *orcv1.RetryContext) *task.RetryContext {
	if pb == nil {
		return nil
	}
	rc := &task.RetryContext{
		FromPhase: pb.FromPhase,
		ToPhase:   pb.ToPhase,
		Reason:    pb.Reason,
		Attempt:   int(pb.Attempt),
		Timestamp: pb.Timestamp.AsTime(),
	}
	if pb.FailureOutput != nil {
		rc.FailureOutput = *pb.FailureOutput
	}
	if pb.ContextFile != nil {
		rc.ContextFile = *pb.ContextFile
	}
	return rc
}

func validationEntryToProto(ve *task.ValidationEntry) *orcv1.ValidationEntry {
	if ve == nil {
		return nil
	}
	pb := &orcv1.ValidationEntry{
		Iteration: int32(ve.Iteration),
		Type:      ve.Type,
		Decision:  ve.Decision,
		Timestamp: timestamppb.New(ve.Timestamp),
	}
	if ve.Reason != "" {
		pb.Reason = &ve.Reason
	}
	return pb
}

func protoToValidationEntry(pb *orcv1.ValidationEntry) *task.ValidationEntry {
	if pb == nil {
		return nil
	}
	ve := &task.ValidationEntry{
		Iteration: int(pb.Iteration),
		Type:      pb.Type,
		Decision:  pb.Decision,
		Timestamp: pb.Timestamp.AsTime(),
	}
	if pb.Reason != nil {
		ve.Reason = *pb.Reason
	}
	return ve
}

// ============================================================================
// Initiative Conversions
// ============================================================================

// InitiativeToProto converts a domain initiative.Initiative to proto orcv1.Initiative.
func InitiativeToProto(i *initiative.Initiative) *orcv1.Initiative {
	if i == nil {
		return nil
	}
	pb := &orcv1.Initiative{
		Version:   int32(i.Version),
		Id:        i.ID,
		Title:     i.Title,
		Status:    initiativeStatusToProto(i.Status),
		Owner:     identityToProto(&i.Owner),
		BlockedBy: i.BlockedBy,
		Blocks:    i.Blocks,
		CreatedAt: timestamppb.New(i.CreatedAt),
		UpdatedAt: timestamppb.New(i.UpdatedAt),
	}
	if i.Vision != "" {
		pb.Vision = &i.Vision
	}
	if len(i.Decisions) > 0 {
		pb.Decisions = make([]*orcv1.InitiativeDecision, len(i.Decisions))
		for idx, d := range i.Decisions {
			pb.Decisions[idx] = initiativeDecisionToProto(&d)
		}
	}
	if len(i.ContextFiles) > 0 {
		pb.ContextFiles = i.ContextFiles
	}
	if len(i.Tasks) > 0 {
		pb.Tasks = make([]*orcv1.TaskRef, len(i.Tasks))
		for idx, t := range i.Tasks {
			pb.Tasks[idx] = taskRefToProto(&t)
		}
	}
	if i.BranchBase != "" {
		pb.BranchBase = &i.BranchBase
	}
	if i.BranchPrefix != "" {
		pb.BranchPrefix = &i.BranchPrefix
	}
	if i.MergeStatus != "" {
		pb.MergeStatus = mergeStatusToProto(i.MergeStatus)
	}
	if i.MergeCommit != "" {
		pb.MergeCommit = &i.MergeCommit
	}
	return pb
}

// ProtoToInitiative converts a proto orcv1.Initiative to domain initiative.Initiative.
func ProtoToInitiative(pb *orcv1.Initiative) *initiative.Initiative {
	if pb == nil {
		return nil
	}
	i := &initiative.Initiative{
		Version:   int(pb.Version),
		ID:        pb.Id,
		Title:     pb.Title,
		Status:    protoToInitiativeStatus(pb.Status),
		BlockedBy: pb.BlockedBy,
		Blocks:    pb.Blocks,
		CreatedAt: pb.CreatedAt.AsTime(),
		UpdatedAt: pb.UpdatedAt.AsTime(),
	}
	if pb.Owner != nil {
		i.Owner = *protoToIdentity(pb.Owner)
	}
	if pb.Vision != nil {
		i.Vision = *pb.Vision
	}
	if len(pb.Decisions) > 0 {
		i.Decisions = make([]initiative.Decision, len(pb.Decisions))
		for idx, d := range pb.Decisions {
			i.Decisions[idx] = *protoToInitiativeDecision(d)
		}
	}
	if len(pb.ContextFiles) > 0 {
		i.ContextFiles = pb.ContextFiles
	}
	if len(pb.Tasks) > 0 {
		i.Tasks = make([]initiative.TaskRef, len(pb.Tasks))
		for idx, t := range pb.Tasks {
			i.Tasks[idx] = *protoToTaskRef(t)
		}
	}
	if pb.BranchBase != nil {
		i.BranchBase = *pb.BranchBase
	}
	if pb.BranchPrefix != nil {
		i.BranchPrefix = *pb.BranchPrefix
	}
	i.MergeStatus = protoToMergeStatus(pb.MergeStatus)
	if pb.MergeCommit != nil {
		i.MergeCommit = *pb.MergeCommit
	}
	return i
}

func initiativeStatusToProto(s initiative.Status) orcv1.InitiativeStatus {
	switch s {
	case initiative.StatusDraft:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT
	case initiative.StatusActive:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	case initiative.StatusCompleted:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED
	case initiative.StatusArchived:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED
	default:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED
	}
}

func protoToInitiativeStatus(s orcv1.InitiativeStatus) initiative.Status {
	switch s {
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT:
		return initiative.StatusDraft
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE:
		return initiative.StatusActive
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED:
		return initiative.StatusCompleted
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED:
		return initiative.StatusArchived
	default:
		return initiative.StatusDraft
	}
}

func mergeStatusToProto(s string) orcv1.MergeStatus {
	switch s {
	case initiative.MergeStatusNone:
		return orcv1.MergeStatus_MERGE_STATUS_NONE
	case initiative.MergeStatusPending:
		return orcv1.MergeStatus_MERGE_STATUS_PENDING
	case initiative.MergeStatusInProgress:
		return orcv1.MergeStatus_MERGE_STATUS_IN_PROGRESS
	case initiative.MergeStatusMerged:
		return orcv1.MergeStatus_MERGE_STATUS_MERGED
	case initiative.MergeStatusFailed:
		return orcv1.MergeStatus_MERGE_STATUS_FAILED
	default:
		return orcv1.MergeStatus_MERGE_STATUS_UNSPECIFIED
	}
}

func protoToMergeStatus(s orcv1.MergeStatus) string {
	switch s {
	case orcv1.MergeStatus_MERGE_STATUS_NONE:
		return initiative.MergeStatusNone
	case orcv1.MergeStatus_MERGE_STATUS_PENDING:
		return initiative.MergeStatusPending
	case orcv1.MergeStatus_MERGE_STATUS_IN_PROGRESS:
		return initiative.MergeStatusInProgress
	case orcv1.MergeStatus_MERGE_STATUS_MERGED:
		return initiative.MergeStatusMerged
	case orcv1.MergeStatus_MERGE_STATUS_FAILED:
		return initiative.MergeStatusFailed
	default:
		return ""
	}
}

func identityToProto(id *initiative.Identity) *orcv1.Identity {
	if id == nil {
		return nil
	}
	pb := &orcv1.Identity{
		Initials: id.Initials,
	}
	if id.DisplayName != "" {
		pb.DisplayName = &id.DisplayName
	}
	if id.Email != "" {
		pb.Email = &id.Email
	}
	return pb
}

func protoToIdentity(pb *orcv1.Identity) *initiative.Identity {
	if pb == nil {
		return nil
	}
	id := &initiative.Identity{
		Initials: pb.Initials,
	}
	if pb.DisplayName != nil {
		id.DisplayName = *pb.DisplayName
	}
	if pb.Email != nil {
		id.Email = *pb.Email
	}
	return id
}

func initiativeDecisionToProto(d *initiative.Decision) *orcv1.InitiativeDecision {
	if d == nil {
		return nil
	}
	pb := &orcv1.InitiativeDecision{
		Id:       d.ID,
		Date:     timestamppb.New(d.Date),
		By:       d.By,
		Decision: d.Decision,
	}
	if d.Rationale != "" {
		pb.Rationale = &d.Rationale
	}
	return pb
}

func protoToInitiativeDecision(pb *orcv1.InitiativeDecision) *initiative.Decision {
	if pb == nil {
		return nil
	}
	d := &initiative.Decision{
		ID:       pb.Id,
		Date:     pb.Date.AsTime(),
		By:       pb.By,
		Decision: pb.Decision,
	}
	if pb.Rationale != nil {
		d.Rationale = *pb.Rationale
	}
	return d
}

func taskRefToProto(tr *initiative.TaskRef) *orcv1.TaskRef {
	if tr == nil {
		return nil
	}
	return &orcv1.TaskRef{
		Id:        tr.ID,
		Title:     tr.Title,
		Status:    taskStatusStringToProto(tr.Status),
		DependsOn: tr.DependsOn,
	}
}

func protoToTaskRef(pb *orcv1.TaskRef) *initiative.TaskRef {
	if pb == nil {
		return nil
	}
	return &initiative.TaskRef{
		ID:        pb.Id,
		Title:     pb.Title,
		Status:    protoToTaskStatusString(pb.Status),
		DependsOn: pb.DependsOn,
	}
}

// taskStatusStringToProto converts a string status (used in initiative.TaskRef) to proto enum.
func taskStatusStringToProto(s string) orcv1.TaskStatus {
	return taskStatusToProto(task.Status(s))
}

// protoToTaskStatusString converts proto enum to string status (used in initiative.TaskRef).
func protoToTaskStatusString(s orcv1.TaskStatus) string {
	return string(protoToTaskStatus(s))
}

// ============================================================================
// Helper Functions
// ============================================================================

// TimeToTimestamp converts a time.Time to protobuf timestamp.
func TimeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// TimestampToTime converts a protobuf timestamp to time.Time.
func TimestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// OptionalTimeToTimestamp converts an optional time.Time pointer to protobuf timestamp.
func OptionalTimeToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// TimestampToOptionalTime converts a protobuf timestamp to optional time.Time pointer.
func TimestampToOptionalTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}
