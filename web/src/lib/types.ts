// Task types matching Go structs
export type TaskWeight = 'trivial' | 'small' | 'medium' | 'large' | 'greenfield';
export type TaskStatus = 'created' | 'classifying' | 'planned' | 'running' | 'paused' | 'blocked' | 'completed' | 'failed';
export type PhaseStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';

export interface Task {
	id: string;
	title: string;
	description?: string;
	weight: TaskWeight;
	status: TaskStatus;
	current_phase?: string;
	branch: string;
	created_at: string;
	updated_at: string;
	started_at?: string;
	completed_at?: string;
	metadata?: Record<string, string>;
}

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
	cache_read_input_tokens?: number;
	total_tokens: number;
}

export interface TranscriptLine {
	timestamp: string;
	type: 'prompt' | 'response' | 'tool' | 'error';
	content: string;
}

// Full transcript file from API
export interface TranscriptFile {
	filename: string;
	content: string;
	created_at: string;
}

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
