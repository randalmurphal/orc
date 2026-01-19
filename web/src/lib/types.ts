// Task types matching Go structs
export type TaskWeight = 'trivial' | 'small' | 'medium' | 'large' | 'greenfield';
export type TaskStatus = 'created' | 'classifying' | 'planned' | 'running' | 'paused' | 'blocked' | 'finalizing' | 'completed' | 'failed' | 'resolved';
export type PhaseStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
export type TaskQueue = 'active' | 'backlog';
export type TaskPriority = 'critical' | 'high' | 'normal' | 'low';
export type TaskCategory = 'feature' | 'bug' | 'refactor' | 'chore' | 'docs' | 'test';
export type DependencyStatus = 'blocked' | 'ready' | 'none';

export interface Task {
	id: string;
	title: string;
	description?: string;
	weight: TaskWeight;
	status: TaskStatus;
	current_phase?: string;
	branch: string;
	queue?: TaskQueue;
	priority?: TaskPriority;
	category?: TaskCategory;
	initiative_id?: string;
	target_branch?: string;
	blocked_by?: string[];
	blocks?: string[];
	related_to?: string[];
	referenced_by?: string[];
	is_blocked?: boolean;
	unmet_blockers?: string[];
	dependency_status?: DependencyStatus;
	created_at: string;
	updated_at: string;
	started_at?: string;
	completed_at?: string;
	metadata?: Record<string, string>;
}

// Priority sort order (lower = higher priority)
export const PRIORITY_ORDER: Record<TaskPriority, number> = {
	critical: 0,
	high: 1,
	normal: 2,
	low: 3
};

// Priority display labels and colors
export const PRIORITY_CONFIG: Record<TaskPriority, { label: string; color: string }> = {
	critical: { label: 'Critical', color: 'var(--status-error)' },
	high: { label: 'High', color: 'var(--status-warning)' },
	normal: { label: 'Normal', color: 'var(--text-muted)' },
	low: { label: 'Low', color: 'var(--text-muted)' }
};

// Category display labels and colors
// Icon names correspond to IconName type in @/components/ui/Icon
export type CategoryIconName = 'sparkles' | 'bug' | 'recycle' | 'tools' | 'file-text' | 'beaker';

export const CATEGORY_CONFIG: Record<TaskCategory, { label: string; color: string; icon: CategoryIconName }> = {
	feature: { label: 'Feature', color: 'var(--status-success)', icon: 'sparkles' },
	bug: { label: 'Bug', color: 'var(--status-error)', icon: 'bug' },
	refactor: { label: 'Refactor', color: 'var(--status-info)', icon: 'recycle' },
	chore: { label: 'Chore', color: 'var(--text-muted)', icon: 'tools' },
	docs: { label: 'Docs', color: 'var(--status-warning)', icon: 'file-text' },
	test: { label: 'Test', color: 'var(--cyan)', icon: 'beaker' }
};

export interface Phase {
	id: string;
	name: string;
	status: PhaseStatus;
	started_at?: string;
	completed_at?: string;
	iterations: number;
	commit_sha?: string;
	error?: string;
}

export interface Plan {
	version: number;
	weight: TaskWeight;
	description: string;
	phases: Phase[];
}

export interface TaskState {
	task_id: string;
	current_phase: string;
	current_iteration: number;
	status: string;
	started_at: string;
	updated_at: string;
	completed_at?: string;
	phases: Record<string, PhaseState>;
	gates: GateDecision[];
	tokens: TokenUsage;
	execution?: ExecutionInfo;
	error?: string;
	retries?: number;
}

export interface PhaseState {
	status: string;
	started_at?: string;
	completed_at?: string;
	iterations: number;
	commit_sha?: string;
	error?: string;
	tokens: TokenUsage;
}

// ExecutionInfo tracks the process executing a task
export interface ExecutionInfo {
	pid: number;
	hostname: string;
	started_at: string;
	last_heartbeat: string;
}

export interface GateDecision {
	phase: string;
	gate_type: string;
	approved: boolean;
	reason?: string;
	timestamp: string;
}

export interface TokenUsage {
	input_tokens: number;
	output_tokens: number;
	cache_creation_input_tokens?: number;
	cache_read_input_tokens?: number;
	total_tokens: number;
}

export interface TranscriptLine {
	timestamp: string;
	type: 'prompt' | 'response' | 'tool' | 'error';
	content: string;
}

// Re-export transcript types from api.ts
export type { Transcript, TranscriptFile, TodoItem, TodoSnapshot } from './api';

export interface Project {
	id: string;
	name: string;
	path: string;
	created_at: string;
}

// Diff types
export interface DiffStats {
	files_changed: number;
	additions: number;
	deletions: number;
}

export interface Line {
	type: 'context' | 'addition' | 'deletion';
	content: string;
	old_line?: number;
	new_line?: number;
}

export interface Hunk {
	old_start: number;
	old_lines: number;
	new_start: number;
	new_lines: number;
	lines: Line[];
}

export interface FileDiff {
	path: string;
	status: 'modified' | 'added' | 'deleted' | 'renamed';
	old_path?: string;
	additions: number;
	deletions: number;
	binary: boolean;
	syntax: string;
	hunks?: Hunk[];
	loadError?: string;
}

export interface DiffResult {
	base: string;
	head: string;
	stats: DiffStats;
	files: FileDiff[];
}

// Review comment types
export type CommentSeverity = 'suggestion' | 'issue' | 'blocker';
export type CommentStatus = 'open' | 'resolved' | 'wont_fix';

export interface ReviewComment {
	id: string;
	task_id: string;
	review_round: number;
	file_path?: string;
	line_number?: number;
	content: string;
	severity: CommentSeverity;
	status: CommentStatus;
	created_at: string;
	resolved_at?: string;
	resolved_by?: string;
}

export interface CreateCommentRequest {
	file_path?: string;
	line_number?: number;
	content: string;
	severity: CommentSeverity;
}

export interface UpdateCommentRequest {
	status?: 'resolved' | 'wont_fix';
	content?: string;
}

// GitHub PR types
export interface PR {
	number: number;
	title: string;
	body: string;
	state: 'open' | 'closed' | 'merged';
	url: string;
	html_url: string;
	head: string;
	base: string;
	mergeable: boolean;
	mergeable_state: string;
	draft: boolean;
	created_at: string;
	updated_at: string;
	merged_at?: string;
}

export interface PRComment {
	id: number;
	body: string;
	path: string;
	line: number;
	author: string;
	created_at: string;
	thread_id?: number;
}

export interface CheckRun {
	id: number;
	name: string;
	status: 'queued' | 'in_progress' | 'completed' | 'waiting' | 'pending' | 'requested';
	conclusion?: 'success' | 'failure' | 'neutral' | 'cancelled' | 'skipped' | 'timed_out' | 'action_required' | 'stale' | 'startup_failure';
	started_at?: string;
	completed_at?: string;
	html_url?: string;
}

export interface CheckSummary {
	passed: number;
	failed: number;
	pending: number;
	neutral: number;
	total: number;
}

// Attachment types
export interface Attachment {
	filename: string;
	size: number;
	content_type: string;
	created_at: string;
	is_image: boolean;
}

// Test Results types (Playwright)
export type TestResultStatus = 'passed' | 'failed' | 'skipped' | 'pending';

export interface TestResult {
	name: string;
	status: TestResultStatus;
	duration: number;
	error?: string;
	screenshots?: string[];
	trace?: string;
}

export interface TestSuite {
	name: string;
	tests: TestResult[];
}

export interface TestSummary {
	total: number;
	passed: number;
	failed: number;
	skipped: number;
}

export interface CoverageDetail {
	total: number;
	covered: number;
	percent: number;
}

export interface TestCoverage {
	percentage: number;
	lines?: CoverageDetail;
	branches?: CoverageDetail;
	functions?: CoverageDetail;
	statements?: CoverageDetail;
}

export interface TestReport {
	version: number;
	framework: string;
	started_at: string;
	completed_at: string;
	duration: number;
	summary: TestSummary;
	suites: TestSuite[];
	coverage?: TestCoverage;
}

export interface Screenshot {
	filename: string;
	page_name: string;
	test_name?: string;
	size: number;
	created_at: string;
}

export interface TestResultsInfo {
	has_results: boolean;
	report?: TestReport;
	screenshots: Screenshot[];
	has_traces: boolean;
	trace_files?: string[];
	has_html_report: boolean;
}

// Task comment types (general notes/discussion, distinct from review comments)
export type TaskCommentAuthorType = 'human' | 'agent' | 'system';

export interface TaskComment {
	id: string;
	task_id: string;
	author: string;
	author_type: TaskCommentAuthorType;
	content: string;
	phase?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateTaskCommentRequest {
	author?: string;
	author_type?: TaskCommentAuthorType;
	content: string;
	phase?: string;
}

export interface UpdateTaskCommentRequest {
	content?: string;
	phase?: string;
}

export interface TaskCommentStats {
	task_id: string;
	total_comments: number;
	human_count: number;
	agent_count: number;
	system_count: number;
}

// Initiative types
export type InitiativeStatus = 'draft' | 'active' | 'completed' | 'archived';

export interface InitiativeIdentity {
	initials: string;
	display_name?: string;
	email?: string;
}

export interface InitiativeDecision {
	id: string;
	date: string;
	by: string;
	decision: string;
	rationale?: string;
}

export interface InitiativeTaskRef {
	id: string;
	title: string;
	depends_on?: string[];
	status: string;
}

export interface Initiative {
	version: number;
	id: string;
	title: string;
	status: InitiativeStatus;
	owner?: InitiativeIdentity;
	vision?: string;
	branch_base?: string;
	branch_prefix?: string;
	decisions?: InitiativeDecision[];
	context_files?: string[];
	tasks?: InitiativeTaskRef[];
	blocked_by?: string[];
	blocks?: string[];
	created_at: string;
	updated_at: string;
}

// Activity states for task execution progress
export type ActivityState =
	| 'idle'
	| 'waiting_api'
	| 'streaming'
	| 'running_tool'
	| 'processing'
	| 'spec_analyzing'
	| 'spec_writing';

// Activity state display configuration
export const ACTIVITY_CONFIG: Record<ActivityState, { label: string; icon: string }> = {
	idle: { label: 'Idle', icon: '' },
	waiting_api: { label: 'Waiting for API', icon: '' },
	streaming: { label: 'Receiving response', icon: '' },
	running_tool: { label: 'Running tool', icon: '' },
	processing: { label: 'Processing', icon: '' },
	spec_analyzing: { label: 'Analyzing codebase', icon: '' },
	spec_writing: { label: 'Writing specification', icon: '' },
};

// Helper to check if activity is spec-phase specific
export function isSpecPhaseActivity(activity: ActivityState): boolean {
	return activity === 'spec_analyzing' || activity === 'spec_writing';
}

// WebSocket connection status
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

// WebSocket event types
export type WSEventType =
	| 'state'
	| 'transcript'
	| 'phase'
	| 'tokens'
	| 'error'
	| 'complete'
	| 'finalize'
	// Progress events
	| 'activity'
	| 'heartbeat'
	| 'warning'
	// File watcher events (triggered by external file changes)
	| 'task_created'
	| 'task_updated'
	| 'task_deleted'
	// Initiative events (triggered by initiative file changes)
	| 'initiative_created'
	| 'initiative_updated'
	| 'initiative_deleted';

// Activity update event data (from EventActivity events)
export interface ActivityUpdate {
	phase: string;
	activity: ActivityState;
}

// Special task ID for subscribing to all task events
export const GLOBAL_TASK_ID = '*';

export interface WSEvent {
	type: 'event';
	event: WSEventType;
	task_id: string;
	data: unknown;
	time: string;
}

export interface WSMessage {
	type: string;
	task_id?: string;
	action?: string;
	data?: unknown;
}

export interface WSError {
	type: 'error';
	error: string;
}

export type WSCallback = (event: WSEvent | WSError) => void;

// Toast notification types
export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
	id: string;
	type: ToastType;
	message: string;
	title?: string;
	duration?: number;
	dismissible?: boolean;
}

// Initiative progress tracking
export interface InitiativeProgress {
	id: string;
	completed: number;
	total: number;
}

// Status counts for dashboard
export interface StatusCounts {
	all: number;
	active: number;
	completed: number;
	failed: number;
	running: number;
	blocked: number;
}

// Branch types for branch registry
export type BranchType = 'initiative' | 'staging' | 'task';
export type BranchStatus = 'active' | 'merged' | 'stale' | 'orphaned';

export interface Branch {
	name: string;
	type: BranchType;
	owner_id?: string;
	created_at: string;
	last_activity: string;
	status: BranchStatus;
}

// Branch status display config
export const BRANCH_STATUS_CONFIG: Record<BranchStatus, { label: string; color: string }> = {
	active: { label: 'Active', color: 'var(--status-success)' },
	merged: { label: 'Merged', color: 'var(--status-info)' },
	stale: { label: 'Stale', color: 'var(--status-warning)' },
	orphaned: { label: 'Orphaned', color: 'var(--status-error)' },
};

// Branch type display config
export const BRANCH_TYPE_CONFIG: Record<BranchType, { label: string; icon: string }> = {
	initiative: { label: 'Initiative', icon: 'layers' },
	staging: { label: 'Staging', icon: 'git-branch' },
	task: { label: 'Task', icon: 'check-circle' },
};

// Pending decision types - decisions from running tasks waiting for user input
export interface DecisionOption {
	id: string;
	label: string;
	description?: string;
	recommended?: boolean;
}

export interface PendingDecision {
	id: string;
	task_id: string;
	question: string;
	options: DecisionOption[];
	created_at: string;
}
