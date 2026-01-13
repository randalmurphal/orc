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
