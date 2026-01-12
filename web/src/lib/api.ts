import type { Task, Plan, TaskState, Project, ReviewComment, CreateCommentRequest, UpdateCommentRequest, PR, PRComment, CheckRun, CheckSummary } from './types';

const API_BASE = '/api';

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${API_BASE}${path}`, {
		...options,
		headers: {
			'Content-Type': 'application/json',
			...options?.headers
		}
	});

	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}

	return res.json();
}

// Tasks
export interface PaginatedTasks {
	tasks: Task[];
	total: number;
	page: number;
	limit: number;
	total_pages: number;
}

export async function listTasks(options?: { page?: number; limit?: number }): Promise<Task[] | PaginatedTasks> {
	if (options?.page || options?.limit) {
		const params = new URLSearchParams();
		if (options.page) params.set('page', String(options.page));
		if (options.limit) params.set('limit', String(options.limit));
		return fetchJSON<PaginatedTasks>(`/tasks?${params}`);
	}
	return fetchJSON<Task[]>('/tasks');
}

export async function getTask(id: string): Promise<Task> {
	return fetchJSON<Task>(`/tasks/${id}`);
}

export async function createTask(title: string, description?: string, weight?: string): Promise<Task> {
	return fetchJSON<Task>('/tasks', {
		method: 'POST',
		body: JSON.stringify({ title, description, weight })
	});
}

// Task state and plan
export async function getTaskState(id: string): Promise<TaskState> {
	return fetchJSON<TaskState>(`/tasks/${id}/state`);
}

export async function getTaskPlan(id: string): Promise<Plan> {
	return fetchJSON<Plan>(`/tasks/${id}/plan`);
}

// Task control
export async function runTask(id: string): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/tasks/${id}/run`, { method: 'POST' });
}

export async function pauseTask(id: string): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/tasks/${id}/pause`, { method: 'POST' });
}

export async function resumeTask(id: string): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/tasks/${id}/resume`, { method: 'POST' });
}

export async function deleteTask(id: string): Promise<void> {
	const res = await fetch(`${API_BASE}/tasks/${id}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete task');
	}
}

// Transcripts
export interface TranscriptFile {
	filename: string;
	content: string;
	created_at: string;
}

export async function getTranscripts(id: string): Promise<TranscriptFile[]> {
	return fetchJSON<TranscriptFile[]>(`/tasks/${id}/transcripts`);
}

// Prompts
export interface PromptInfo {
	phase: string;
	source: 'project' | 'embedded' | 'inline';
	has_override: boolean;
	variables: string[];
}

export interface Prompt {
	phase: string;
	content: string;
	source: 'project' | 'embedded' | 'inline';
	variables: string[];
}

export async function listPrompts(): Promise<PromptInfo[]> {
	return fetchJSON<PromptInfo[]>('/prompts');
}

export async function getPrompt(phase: string): Promise<Prompt> {
	return fetchJSON<Prompt>(`/prompts/${phase}`);
}

export async function getPromptDefault(phase: string): Promise<Prompt> {
	return fetchJSON<Prompt>(`/prompts/${phase}/default`);
}

export async function getPromptVariables(): Promise<Record<string, string>> {
	return fetchJSON<Record<string, string>>('/prompts/variables');
}

export async function savePrompt(phase: string, content: string): Promise<Prompt> {
	return fetchJSON<Prompt>(`/prompts/${phase}`, {
		method: 'PUT',
		body: JSON.stringify({ content })
	});
}

export async function deletePrompt(phase: string): Promise<void> {
	const res = await fetch(`${API_BASE}/prompts/${phase}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Hooks (settings.json format)
export type HookEvent = 'PreToolUse' | 'PostToolUse' | 'PreCompact' | 'PrePrompt' | 'Stop';

export interface HookEntry {
	type: string;
	command: string;
}

export interface Hook {
	matcher: string;
	hooks: HookEntry[];
}

// Hooks are stored as map[event][]Hook
export type HooksMap = Record<string, Hook[]>;

export async function listHooks(): Promise<HooksMap> {
	return fetchJSON<HooksMap>('/hooks');
}

export async function getHookTypes(): Promise<HookEvent[]> {
	return fetchJSON<HookEvent[]>('/hooks/types');
}

export async function getHook(event: string): Promise<Hook[]> {
	return fetchJSON<Hook[]>(`/hooks/${event}`);
}

export async function createHook(event: string, hook: Hook): Promise<Hook[]> {
	return fetchJSON<Hook[]>('/hooks', {
		method: 'POST',
		body: JSON.stringify({ event, hook })
	});
}

export async function updateHook(event: string, hooks: Hook[]): Promise<Hook[]> {
	return fetchJSON<Hook[]>(`/hooks/${event}`, {
		method: 'PUT',
		body: JSON.stringify({ hooks })
	});
}

export async function deleteHook(event: string): Promise<void> {
	const res = await fetch(`${API_BASE}/hooks/${event}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Skills (SKILL.md format)
export interface SkillInfo {
	name: string;
	description: string;
	path: string;
}

export interface Skill {
	name: string;
	description: string;
	content: string;
	allowed_tools?: string[];
	version?: string;
	path?: string;
	has_references?: boolean;
	has_scripts?: boolean;
	has_assets?: boolean;
}

export async function listSkills(): Promise<SkillInfo[]> {
	return fetchJSON<SkillInfo[]>('/skills');
}

export async function getSkill(name: string): Promise<Skill> {
	return fetchJSON<Skill>(`/skills/${name}`);
}

export async function createSkill(skill: Skill): Promise<Skill> {
	return fetchJSON<Skill>('/skills', {
		method: 'POST',
		body: JSON.stringify(skill)
	});
}

export async function updateSkill(name: string, skill: Skill): Promise<Skill> {
	return fetchJSON<Skill>(`/skills/${name}`, {
		method: 'PUT',
		body: JSON.stringify(skill)
	});
}

export async function deleteSkill(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/skills/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Config
export interface Config {
	version: string;
	profile: string;
	automation: {
		profile: string;
		gates_default: string;
		retry_enabled: boolean;
		retry_max: number;
	};
	execution: {
		model: string;
		max_iterations: number;
		timeout: string;
	};
	git: {
		branch_prefix: string;
		commit_prefix: string;
	};
}

export interface ConfigUpdateRequest {
	profile?: string;
	automation?: {
		gates_default?: string;
		retry_enabled?: boolean;
		retry_max?: number;
	};
	execution?: {
		model?: string;
		max_iterations?: number;
		timeout?: string;
	};
	git?: {
		branch_prefix?: string;
		commit_prefix?: string;
	};
}

export async function getConfig(): Promise<Config> {
	return fetchJSON<Config>('/config');
}

export async function updateConfig(req: ConfigUpdateRequest): Promise<Config> {
	return fetchJSON<Config>('/config', {
		method: 'PUT',
		body: JSON.stringify(req)
	});
}

// SSE streaming
export function subscribeToTask(id: string, onEvent: (event: string, data: unknown) => void): () => void {
	const eventSource = new EventSource(`${API_BASE}/tasks/${id}/stream`);

	eventSource.onmessage = (e) => {
		try {
			const data = JSON.parse(e.data);
			onEvent('message', data);
		} catch {
			onEvent('message', e.data);
		}
	};

	eventSource.addEventListener('state', (e) => {
		try {
			const data = JSON.parse((e as MessageEvent).data);
			onEvent('state', data);
		} catch {
			// Ignore parse errors
		}
	});

	eventSource.addEventListener('transcript', (e) => {
		try {
			const data = JSON.parse((e as MessageEvent).data);
			onEvent('transcript', data);
		} catch {
			// Ignore parse errors
		}
	});

	eventSource.addEventListener('phase', (e) => {
		try {
			const data = JSON.parse((e as MessageEvent).data);
			onEvent('phase', data);
		} catch {
			// Ignore parse errors
		}
	});

	eventSource.onerror = () => {
		onEvent('error', { message: 'Connection lost' });
	};

	// Return cleanup function
	return () => eventSource.close();
}

// Projects
export async function listProjects(): Promise<Project[]> {
	return fetchJSON<Project[]>('/projects');
}

export async function getProject(id: string): Promise<Project> {
	return fetchJSON<Project>(`/projects/${id}`);
}

export async function listProjectTasks(projectId: string): Promise<Task[]> {
	return fetchJSON<Task[]>(`/projects/${projectId}/tasks`);
}

export async function createProjectTask(projectId: string, title: string, description?: string, weight?: string): Promise<Task> {
	return fetchJSON<Task>(`/projects/${projectId}/tasks`, {
		method: 'POST',
		body: JSON.stringify({ title, description, weight })
	});
}

// Settings (Claude Code settings.json)
export interface Settings {
	env?: Record<string, string>;
	hooks?: HooksMap;
	enabledPlugins?: Record<string, boolean>;
	statusLine?: {
		type?: string;
		command?: string;
	};
	extensions?: Record<string, unknown>;
}

export async function getSettings(): Promise<Settings> {
	return fetchJSON<Settings>('/settings');
}

export async function getGlobalSettings(): Promise<Settings> {
	return fetchJSON<Settings>('/settings/global');
}

export async function getProjectSettings(): Promise<Settings> {
	return fetchJSON<Settings>('/settings/project');
}

export async function updateSettings(settings: Settings): Promise<Settings> {
	return fetchJSON<Settings>('/settings', {
		method: 'PUT',
		body: JSON.stringify(settings)
	});
}

// Tools
export interface ToolInfo {
	name: string;
	description: string;
	category: string;
}

export interface ToolPermissions {
	allow?: string[];
	deny?: string[];
}

export interface ToolsByCategory {
	[category: string]: ToolInfo[];
}

export async function listTools(): Promise<ToolInfo[]> {
	return fetchJSON<ToolInfo[]>('/tools');
}

export async function listToolsByCategory(): Promise<ToolsByCategory> {
	return fetchJSON<ToolsByCategory>('/tools?by_category=true');
}

export async function getToolPermissions(): Promise<ToolPermissions> {
	return fetchJSON<ToolPermissions>('/tools/permissions');
}

export async function updateToolPermissions(perms: ToolPermissions): Promise<ToolPermissions> {
	return fetchJSON<ToolPermissions>('/tools/permissions', {
		method: 'PUT',
		body: JSON.stringify(perms)
	});
}

// Agents (sub-agent definitions)
export interface SubAgent {
	name: string;
	description: string;
	model?: string;
	tools?: ToolPermissions;
	prompt?: string;
	work_dir?: string;
	skill_refs?: string[];
	timeout?: string;
}

export async function listAgents(): Promise<SubAgent[]> {
	return fetchJSON<SubAgent[]>('/agents');
}

export async function getAgent(name: string): Promise<SubAgent> {
	return fetchJSON<SubAgent>(`/agents/${name}`);
}

export async function createAgent(agent: SubAgent): Promise<SubAgent> {
	return fetchJSON<SubAgent>('/agents', {
		method: 'POST',
		body: JSON.stringify(agent)
	});
}

export async function updateAgent(name: string, agent: SubAgent): Promise<SubAgent> {
	return fetchJSON<SubAgent>(`/agents/${name}`, {
		method: 'PUT',
		body: JSON.stringify(agent)
	});
}

export async function deleteAgent(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/agents/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Scripts (project script registry)
export interface ProjectScript {
	name: string;
	path: string;
	description: string;
	language?: string;
}

export async function listScripts(): Promise<ProjectScript[]> {
	return fetchJSON<ProjectScript[]>('/scripts');
}

export async function getScript(name: string): Promise<ProjectScript> {
	return fetchJSON<ProjectScript>(`/scripts/${name}`);
}

export async function createScript(script: ProjectScript): Promise<ProjectScript> {
	return fetchJSON<ProjectScript>('/scripts', {
		method: 'POST',
		body: JSON.stringify(script)
	});
}

export async function updateScript(name: string, script: ProjectScript): Promise<ProjectScript> {
	return fetchJSON<ProjectScript>(`/scripts/${name}`, {
		method: 'PUT',
		body: JSON.stringify(script)
	});
}

export async function deleteScript(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/scripts/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

export async function discoverScripts(): Promise<ProjectScript[]> {
	return fetchJSON<ProjectScript[]>('/scripts/discover', {
		method: 'POST'
	});
}

// CLAUDE.md
export interface ClaudeMD {
	path: string;
	content: string;
	is_global: boolean;
	source: 'global' | 'user' | 'project' | 'local';
}

export interface ClaudeMDHierarchy {
	global?: ClaudeMD;
	user?: ClaudeMD;
	project?: ClaudeMD;
	local?: ClaudeMD[];
}

export async function getClaudeMD(): Promise<ClaudeMD> {
	return fetchJSON<ClaudeMD>('/claudemd');
}

export async function updateClaudeMD(content: string): Promise<void> {
	const res = await fetch(`${API_BASE}/claudemd`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ content })
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

export async function getClaudeMDHierarchy(): Promise<ClaudeMDHierarchy> {
	return fetchJSON<ClaudeMDHierarchy>('/claudemd/hierarchy');
}

// MCP Servers (.mcp.json)
export interface MCPServerInfo {
	name: string;
	type: string;
	command?: string;
	url?: string;
	disabled: boolean;
	has_env: boolean;
	env_count: number;
	args_count: number;
}

export interface MCPServer {
	name: string;
	type: string;
	command?: string;
	args?: string[];
	env?: Record<string, string>;
	url?: string;
	headers?: string[];
	disabled: boolean;
}

export interface MCPServerCreate {
	name: string;
	type?: string;
	command?: string;
	args?: string[];
	env?: Record<string, string>;
	url?: string;
	headers?: string[];
	disabled?: boolean;
}

export async function listMCPServers(): Promise<MCPServerInfo[]> {
	return fetchJSON<MCPServerInfo[]>('/mcp');
}

export async function getMCPServer(name: string): Promise<MCPServer> {
	return fetchJSON<MCPServer>(`/mcp/${name}`);
}

export async function createMCPServer(server: MCPServerCreate): Promise<MCPServerInfo> {
	return fetchJSON<MCPServerInfo>('/mcp', {
		method: 'POST',
		body: JSON.stringify(server)
	});
}

export async function updateMCPServer(name: string, server: Partial<MCPServerCreate>): Promise<MCPServerInfo> {
	return fetchJSON<MCPServerInfo>(`/mcp/${name}`, {
		method: 'PUT',
		body: JSON.stringify(server)
	});
}

export async function deleteMCPServer(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/mcp/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Project Task operations
export async function getProjectTask(projectId: string, taskId: string): Promise<Task> {
	return fetchJSON<Task>(`/projects/${projectId}/tasks/${taskId}`);
}

export async function getProjectTaskState(projectId: string, taskId: string): Promise<TaskState> {
	return fetchJSON<TaskState>(`/projects/${projectId}/tasks/${taskId}/state`);
}

export async function getProjectTaskPlan(projectId: string, taskId: string): Promise<Plan> {
	return fetchJSON<Plan>(`/projects/${projectId}/tasks/${taskId}/plan`);
}

export async function runProjectTask(projectId: string, taskId: string): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/run`, { method: 'POST' });
}

export async function pauseProjectTask(projectId: string, taskId: string): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/pause`, { method: 'POST' });
}

export async function resumeProjectTask(projectId: string, taskId: string): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/resume`, { method: 'POST' });
}

export async function deleteProjectTask(projectId: string, taskId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/projects/${projectId}/tasks/${taskId}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete task');
	}
}

export async function escalateProjectTask(projectId: string, taskId: string, reason: string): Promise<{ status: string; task_id: string; phase: string; reason: string; attempt: number }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/escalate`, {
		method: 'POST',
		body: JSON.stringify({ reason })
	});
}

export async function getProjectTranscripts(projectId: string, taskId: string): Promise<TranscriptFile[]> {
	return fetchJSON<TranscriptFile[]>(`/projects/${projectId}/tasks/${taskId}/transcripts`);
}

// Dashboard
export interface DashboardStats {
	running: number;
	paused: number;
	blocked: number;
	completed: number;
	failed: number;
	today: number;
	total: number;
	tokens: number;
	cost: number;
}

export async function getDashboardStats(): Promise<DashboardStats> {
	return fetchJSON<DashboardStats>('/dashboard/stats');
}

// Templates
export interface TemplateInfo {
	name: string;
	description?: string;
	weight: string;
	phases: string[];
	scope: 'project' | 'global' | 'builtin';
	variables?: { name: string; description?: string; required: boolean; default?: string }[];
}

export interface Template extends TemplateInfo {
	version: number;
	prompts?: Record<string, string>;
	defaults?: { branch_prefix?: string };
	created_from?: string;
	created_at?: string;
	author?: string;
}

export async function listTemplates(): Promise<TemplateInfo[]> {
	return fetchJSON<TemplateInfo[]>('/templates');
}

export async function getTemplate(name: string): Promise<Template> {
	return fetchJSON<Template>(`/templates/${name}`);
}

export async function createTemplate(taskId: string, name: string, description?: string, global?: boolean): Promise<Template> {
	return fetchJSON<Template>('/templates', {
		method: 'POST',
		body: JSON.stringify({ task_id: taskId, name, description, global })
	});
}

export async function deleteTemplate(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/templates/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete template');
	}
}

// Review Comments
export async function getReviewComments(taskId: string): Promise<ReviewComment[]> {
	return fetchJSON<ReviewComment[]>(`/tasks/${taskId}/review/comments`);
}

export async function createReviewComment(taskId: string, comment: CreateCommentRequest): Promise<ReviewComment> {
	return fetchJSON<ReviewComment>(`/tasks/${taskId}/review/comments`, {
		method: 'POST',
		body: JSON.stringify(comment)
	});
}

export async function updateReviewComment(taskId: string, commentId: string, update: UpdateCommentRequest): Promise<ReviewComment> {
	return fetchJSON<ReviewComment>(`/tasks/${taskId}/review/comments/${commentId}`, {
		method: 'PATCH',
		body: JSON.stringify(update)
	});
}

export async function deleteReviewComment(taskId: string, commentId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/tasks/${taskId}/review/comments/${commentId}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete comment');
	}
}

export async function triggerReviewRetry(taskId: string): Promise<void> {
	await fetchJSON(`/tasks/${taskId}/review/retry`, {
		method: 'POST',
		body: JSON.stringify({ include_comments: true })
	});
}

// Diff Stats
export interface DiffStatsResponse {
	files_changed: number;
	additions: number;
	deletions: number;
}

export async function getDiffStats(taskId: string): Promise<DiffStatsResponse | null> {
	try {
		return await fetchJSON<DiffStatsResponse>(`/tasks/${taskId}/diff/stats`);
	} catch {
		return null;
	}
}

// Review Stats
export interface ReviewStatsResponse {
	open_comments: number;
	resolved_comments: number;
	total_comments: number;
	blockers: number;
	issues: number;
	suggestions: number;
}

export async function getReviewStats(taskId: string): Promise<ReviewStatsResponse | null> {
	try {
		const comments = await fetchJSON<ReviewComment[]>(`/tasks/${taskId}/review/comments`);
		const openComments = comments.filter((c) => c.status === 'open');
		const resolvedComments = comments.filter(
			(c) => c.status === 'resolved' || c.status === 'wont_fix'
		);

		return {
			open_comments: openComments.length,
			resolved_comments: resolvedComments.length,
			total_comments: comments.length,
			blockers: comments.filter((c) => c.severity === 'blocker' && c.status === 'open').length,
			issues: comments.filter((c) => c.severity === 'issue' && c.status === 'open').length,
			suggestions: comments.filter((c) => c.severity === 'suggestion' && c.status === 'open').length
		};
	} catch {
		return null;
	}
}

// GitHub PR Operations
export interface CreatePRResponse {
	pr: PR;
	created: boolean;
	message?: string;
}

export interface GetPRResponse {
	pr: PR;
	comments: PRComment[];
	checks: CheckRun[];
}

export interface MergePRResponse {
	merged: boolean;
	pr_number: number;
	method: string;
	message: string;
	warning?: string;
}

export interface GetChecksResponse {
	checks: CheckRun[];
	summary: CheckSummary;
}

export async function createPR(taskId: string, options?: {
	title?: string;
	body?: string;
	base?: string;
	labels?: string[];
	reviewers?: string[];
	draft?: boolean;
}): Promise<CreatePRResponse> {
	return fetchJSON<CreatePRResponse>(`/tasks/${taskId}/github/pr`, {
		method: 'POST',
		body: JSON.stringify(options || {})
	});
}

export async function getPR(taskId: string): Promise<GetPRResponse> {
	return fetchJSON<GetPRResponse>(`/tasks/${taskId}/github/pr`);
}

export async function mergePR(taskId: string, options?: {
	method?: 'merge' | 'squash' | 'rebase';
	delete_branch?: boolean;
}): Promise<MergePRResponse> {
	return fetchJSON<MergePRResponse>(`/tasks/${taskId}/github/pr/merge`, {
		method: 'POST',
		body: JSON.stringify(options || { method: 'squash', delete_branch: true })
	});
}

export async function getPRChecks(taskId: string): Promise<GetChecksResponse> {
	return fetchJSON<GetChecksResponse>(`/tasks/${taskId}/github/pr/checks`);
}

// Export
export interface ExportConfig {
	enabled: boolean;
	preset: string;
	task_definition: boolean;
	final_state: boolean;
	transcripts: boolean;
	context_summary: boolean;
}

export interface ExportRequest {
	task_definition?: boolean;
	final_state?: boolean;
	transcripts?: boolean;
	context_summary?: boolean;
	to_branch?: boolean;
}

export interface ExportResponse {
	success: boolean;
	task_id: string;
	exported_to: string;
	files?: string[];
	committed_sha?: string;
}

export async function exportTask(taskId: string, options: ExportRequest): Promise<ExportResponse> {
	return fetchJSON<ExportResponse>(`/tasks/${taskId}/export`, {
		method: 'POST',
		body: JSON.stringify(options)
	});
}

export async function getExportConfig(): Promise<ExportConfig> {
	return fetchJSON<ExportConfig>('/config/export');
}

export async function updateExportConfig(config: Partial<ExportConfig>): Promise<ExportConfig> {
	return fetchJSON<ExportConfig>('/config/export', {
		method: 'PUT',
		body: JSON.stringify(config)
	});
}
