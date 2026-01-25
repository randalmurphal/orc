import type {
	Task,
	Plan,
	TaskState,
	Project,
	ReviewComment,
	CreateCommentRequest,
	UpdateCommentRequest,
	PR,
	PRComment,
	CheckRun,
	CheckSummary,
	Attachment,
	TaskComment,
	CreateTaskCommentRequest,
	UpdateTaskCommentRequest,
	TaskCommentStats,
	TestResultsInfo,
	Screenshot,
	TestReport,
	Initiative,
	InitiativeStatus,
	InitiativeIdentity,
	InitiativeTaskRef,
	InitiativeDecision,
	Branch,
	Workflow,
	WorkflowWithDetails,
	PhaseTemplate,
	WorkflowRun,
	WorkflowRunWithDetails,
	WorkflowPhase,
	WorkflowVariable,
} from './types';

const API_BASE = '/api';

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${API_BASE}${path}`, {
		...options,
		headers: {
			'Content-Type': 'application/json',
			...options?.headers,
		},
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

export async function listTasks(options?: {
	page?: number;
	limit?: number;
}): Promise<Task[] | PaginatedTasks> {
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

export async function createTask(
	title: string,
	description?: string,
	weight?: string,
	category?: string,
	attachments?: File[]
): Promise<Task> {
	// If there are attachments, use multipart form
	if (attachments && attachments.length > 0) {
		const formData = new FormData();
		formData.append('title', title);
		if (description) formData.append('description', description);
		if (weight) formData.append('weight', weight);
		if (category) formData.append('category', category);
		for (const file of attachments) {
			formData.append('attachments', file);
		}

		const res = await fetch(`${API_BASE}/tasks`, {
			method: 'POST',
			body: formData,
		});

		if (!res.ok) {
			const error = await res.json().catch(() => ({ error: res.statusText }));
			throw new Error(error.error || 'Request failed');
		}

		return res.json();
	}

	// Otherwise use JSON
	return fetchJSON<Task>('/tasks', {
		method: 'POST',
		body: JSON.stringify({ title, description, weight, category }),
	});
}

export interface UpdateTaskRequest {
	title?: string;
	description?: string;
	weight?: string;
	queue?: string;
	priority?: string;
	category?: string;
	initiative_id?: string;
	target_branch?: string;
	blocked_by?: string[];
	related_to?: string[];
	metadata?: Record<string, string>;
}

export async function updateTask(id: string, update: UpdateTaskRequest): Promise<Task> {
	return fetchJSON<Task>(`/tasks/${id}`, {
		method: 'PATCH',
		body: JSON.stringify(update),
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
export interface RunTaskResponse {
	status: string;
	task_id: string;
	task?: Task; // Task with updated status
}

export async function runTask(id: string): Promise<RunTaskResponse> {
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

// Blocked task actions
export interface SkipBlockResponse {
	status: string;
	task_id: string;
	message: string;
	cleared_blockers: string[];
}

export async function skipBlock(id: string): Promise<SkipBlockResponse> {
	return fetchJSON(`/tasks/${id}/skip-block`, { method: 'POST' });
}

export interface ForceBlockResponse {
	status: string;
	task_id: string;
	task?: Task;
}

export async function forceBlock(id: string): Promise<ForceBlockResponse> {
	return fetchJSON(`/tasks/${id}/run?force=true`, { method: 'POST' });
}

// Transcripts (JSONL-based from Claude Code sessions)
export interface Transcript {
	id: number;
	task_id: string;
	phase: string;
	session_id: string;
	message_uuid: string;
	parent_uuid?: string;
	type: 'user' | 'assistant' | 'queue-operation';
	role: string;
	content: string; // JSON string of content blocks
	model?: string;
	input_tokens: number;
	output_tokens: number;
	cache_creation_tokens: number;
	cache_read_tokens: number;
	tool_calls?: string; // JSON array
	tool_results?: string; // JSON
	timestamp: string;
}

// Legacy type for backwards compatibility
export interface TranscriptFile {
	filename: string;
	content: string;
	created_at: string;
}

export async function getTranscripts(id: string): Promise<Transcript[]> {
	return fetchJSON<Transcript[]>(`/tasks/${id}/transcripts`);
}

// Paginated transcript API
export interface PhaseSummary {
	phase: string;
	transcript_count: number;
}

export interface TranscriptPaginationResult {
	next_cursor: number | null;
	prev_cursor: number | null;
	has_more: boolean;
	total_count: number;
}

export interface PaginatedTranscriptsResponse {
	transcripts: Transcript[];
	pagination: TranscriptPaginationResult;
	phases: PhaseSummary[];
}

export interface GetTranscriptsOptions {
	limit?: number;
	cursor?: number;
	direction?: 'asc' | 'desc';
	phase?: string;
}

export async function getTranscriptsPaginated(
	taskId: string,
	options: GetTranscriptsOptions = {}
): Promise<PaginatedTranscriptsResponse> {
	const params = new URLSearchParams();
	if (options.limit) params.set('limit', String(options.limit));
	if (options.cursor) params.set('cursor', String(options.cursor));
	if (options.direction) params.set('direction', options.direction);
	if (options.phase) params.set('phase', options.phase);
	const query = params.toString() ? `?${params.toString()}` : '';
	return fetchJSON<PaginatedTranscriptsResponse>(`/tasks/${taskId}/transcripts${query}`);
}

// Todos (from Claude's TodoWrite tool calls)
export interface TodoItem {
	content: string;
	status: 'pending' | 'in_progress' | 'completed';
	active_form: string;
}

export interface TodoSnapshot {
	id: number;
	task_id: string;
	phase: string;
	message_uuid?: string;
	items: TodoItem[];
	timestamp: string;
}

export async function getTaskTodos(id: string): Promise<TodoSnapshot | null> {
	return fetchJSON<TodoSnapshot | null>(`/tasks/${id}/todos`);
}

export async function getTaskTodoHistory(id: string): Promise<TodoSnapshot[]> {
	return fetchJSON<TodoSnapshot[]>(`/tasks/${id}/todos/history`);
}

// Metrics (JSONL-based analytics)
export interface ModelMetrics {
	model: string;
	cost: number;
	input_tokens: number;
	output_tokens: number;
	task_count: number;
}

export interface MetricsSummary {
	total_cost: number;
	total_input: number;
	total_output: number;
	task_count: number;
	by_model: Record<string, ModelMetrics>;
}

export interface DailyMetrics {
	date: string;
	total_input: number;
	total_output: number;
	total_cost: number;
	task_count: number;
	models_used: number;
}

export interface TaskMetric {
	id: number;
	task_id: string;
	phase: string;
	model: string;
	input_tokens: number;
	output_tokens: number;
	cache_creation_tokens: number;
	cache_read_tokens: number;
	cost_usd: number;
	duration_ms: number;
	timestamp: string;
}

export interface TokenUsage {
	input_tokens: number;
	output_tokens: number;
	cache_read_tokens: number;
	cache_creation_tokens: number;
	message_count: number;
}

export async function getMetricsSummary(since = '7d'): Promise<MetricsSummary> {
	return fetchJSON<MetricsSummary>(`/metrics/summary?since=${since}`);
}

export async function getDailyMetrics(since = '30d'): Promise<DailyMetrics[]> {
	return fetchJSON<DailyMetrics[]>(`/metrics/daily?since=${since}`);
}

export async function getMetricsByModel(since = '7d'): Promise<ModelMetrics[]> {
	return fetchJSON<ModelMetrics[]>(`/metrics/by-model?since=${since}`);
}

export async function getTaskMetrics(id: string): Promise<TaskMetric[]> {
	return fetchJSON<TaskMetric[]>(`/tasks/${id}/metrics`);
}

export async function getTaskTokenUsage(id: string): Promise<TokenUsage> {
	return fetchJSON<TokenUsage>(`/tasks/${id}/tokens`);
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
		body: JSON.stringify({ content }),
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

export async function listHooks(scope?: 'global' | 'project'): Promise<HooksMap> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<HooksMap>(`/hooks${params}`);
}

export async function getHookTypes(): Promise<HookEvent[]> {
	return fetchJSON<HookEvent[]>('/hooks/types');
}

export async function getHook(event: string, scope?: 'global' | 'project'): Promise<Hook[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Hook[]>(`/hooks/${event}${params}`);
}

export async function createHook(
	event: string,
	hook: Hook,
	scope?: 'global' | 'project'
): Promise<Hook[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Hook[]>(`/hooks${params}`, {
		method: 'POST',
		body: JSON.stringify({ event, hook }),
	});
}

export async function updateHook(
	event: string,
	hooks: Hook[],
	scope?: 'global' | 'project'
): Promise<Hook[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Hook[]>(`/hooks/${event}${params}`, {
		method: 'PUT',
		body: JSON.stringify({ hooks }),
	});
}

export async function deleteHook(event: string, scope?: 'global' | 'project'): Promise<void> {
	const params = scope ? `?scope=${scope}` : '';
	const res = await fetch(`${API_BASE}/hooks/${event}${params}`, { method: 'DELETE' });
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

export async function listSkills(scope?: 'global' | 'project'): Promise<SkillInfo[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<SkillInfo[]>(`/skills${params}`);
}

export async function getSkill(name: string, scope?: 'global' | 'project'): Promise<Skill> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Skill>(`/skills/${name}${params}`);
}

export async function createSkill(skill: Skill, scope?: 'global' | 'project'): Promise<Skill> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Skill>(`/skills${params}`, {
		method: 'POST',
		body: JSON.stringify(skill),
	});
}

export async function updateSkill(
	name: string,
	skill: Skill,
	scope?: 'global' | 'project'
): Promise<Skill> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Skill>(`/skills/${name}${params}`, {
		method: 'PUT',
		body: JSON.stringify(skill),
	});
}

export async function deleteSkill(name: string, scope?: 'global' | 'project'): Promise<void> {
	const params = scope ? `?scope=${scope}` : '';
	const res = await fetch(`${API_BASE}/skills/${name}${params}`, { method: 'DELETE' });
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
	worktree: {
		enabled: boolean;
		dir: string;
		cleanup_on_complete: boolean;
		cleanup_on_fail: boolean;
	};
	completion: {
		action: string;
		target_branch: string;
		delete_branch: boolean;
	};
	timeouts: {
		phase_max: string;
		turn_max: string;
		idle_warning: string;
		heartbeat_interval: string;
		idle_timeout: string;
	};
}

export interface ConfigSourceInfo {
	source: string; // 'default' | 'shared' | 'personal' | 'env' | 'flag'
	path?: string;
}

export interface ConfigWithSources extends Config {
	sources?: Record<string, ConfigSourceInfo>;
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
	worktree?: {
		enabled?: boolean;
		dir?: string;
		cleanup_on_complete?: boolean;
		cleanup_on_fail?: boolean;
	};
	completion?: {
		action?: string;
		target_branch?: string;
		delete_branch?: boolean;
	};
	timeouts?: {
		phase_max?: string;
		turn_max?: string;
		idle_warning?: string;
		heartbeat_interval?: string;
		idle_timeout?: string;
	};
}

export async function getConfig(): Promise<Config> {
	return fetchJSON<Config>('/config');
}

export async function getConfigWithSources(): Promise<ConfigWithSources> {
	return fetchJSON<ConfigWithSources>('/config?with_sources=true');
}

export async function updateConfig(req: ConfigUpdateRequest): Promise<Config> {
	return fetchJSON<Config>('/config', {
		method: 'PUT',
		body: JSON.stringify(req),
	});
}

export interface ConfigStats {
	slashCommandsCount: number;
	claudeMdSize: number;
	mcpServersCount: number;
	permissionsProfile: string;
}

export async function getConfigStats(): Promise<ConfigStats> {
	return fetchJSON<ConfigStats>('/config/stats');
}

// SSE streaming
export function subscribeToTask(
	id: string,
	onEvent: (event: string, data: unknown) => void
): () => void {
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

export async function getDefaultProject(): Promise<string> {
	const response = await fetchJSON<{ default_project: string }>('/projects/default');
	return response.default_project;
}

export async function setDefaultProject(projectId: string): Promise<void> {
	await fetchJSON<{ default_project: string }>('/projects/default', {
		method: 'PUT',
		body: JSON.stringify({ project_id: projectId }),
	});
}

export async function listProjectTasks(projectId: string): Promise<Task[]> {
	return fetchJSON<Task[]>(`/projects/${projectId}/tasks`);
}

export async function createProjectTask(
	projectId: string,
	title: string,
	description?: string,
	weight?: string,
	category?: string,
	attachments?: File[]
): Promise<Task> {
	// If there are attachments, use multipart form
	if (attachments && attachments.length > 0) {
		const formData = new FormData();
		formData.append('title', title);
		if (description) formData.append('description', description);
		if (weight) formData.append('weight', weight);
		if (category) formData.append('category', category);
		for (const file of attachments) {
			formData.append('attachments', file);
		}

		const res = await fetch(`${API_BASE}/projects/${projectId}/tasks`, {
			method: 'POST',
			body: formData,
		});

		if (!res.ok) {
			const error = await res.json().catch(() => ({ error: res.statusText }));
			throw new Error(error.error || 'Request failed');
		}

		return res.json();
	}

	// Otherwise use JSON
	return fetchJSON<Task>(`/projects/${projectId}/tasks`, {
		method: 'POST',
		body: JSON.stringify({ title, description, weight, category }),
	});
}

// Initiatives - see full definitions with options below

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

export async function updateSettings(settings: Settings, scope?: 'global'): Promise<Settings> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Settings>(`/settings${params}`, {
		method: 'PUT',
		body: JSON.stringify(settings),
	});
}

export async function updateGlobalSettings(settings: Settings): Promise<Settings> {
	return fetchJSON<Settings>('/settings/global', {
		method: 'PUT',
		body: JSON.stringify(settings),
	});
}

// Settings Hierarchy (with source tracking)
export interface SettingsSourceInfo {
	source: string; // 'global' | 'project' | 'default'
	path?: string;
}

export interface SettingsLevel {
	path: string;
	settings?: Settings;
}

export interface SettingsHierarchy {
	merged: Settings | null;
	global: SettingsLevel;
	project: SettingsLevel;
	sources: Record<string, SettingsSourceInfo>;
}

export async function getSettingsHierarchy(): Promise<SettingsHierarchy> {
	return fetchJSON<SettingsHierarchy>('/settings/hierarchy');
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

export async function listTools(scope?: 'global'): Promise<ToolInfo[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<ToolInfo[]>(`/tools${params}`);
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
		body: JSON.stringify(perms),
	});
}

// Agents (sub-agent definitions)
export interface SubAgent {
	name: string;
	description: string;
	model?: string;
	tools?: ToolPermissions | string; // string for global agents (from .md files)
	prompt?: string;
	work_dir?: string;
	skill_refs?: string[];
	timeout?: string;
	path?: string; // for global agents discovered from .md files
}

export async function listAgents(scope?: 'global'): Promise<SubAgent[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<SubAgent[]>(`/agents${params}`);
}

export async function getAgent(name: string): Promise<SubAgent> {
	return fetchJSON<SubAgent>(`/agents/${name}`);
}

export async function createAgent(agent: SubAgent): Promise<SubAgent> {
	return fetchJSON<SubAgent>('/agents', {
		method: 'POST',
		body: JSON.stringify(agent),
	});
}

export async function updateAgent(name: string, agent: SubAgent): Promise<SubAgent> {
	return fetchJSON<SubAgent>(`/agents/${name}`, {
		method: 'PUT',
		body: JSON.stringify(agent),
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
		body: JSON.stringify(script),
	});
}

export async function updateScript(name: string, script: ProjectScript): Promise<ProjectScript> {
	return fetchJSON<ProjectScript>(`/scripts/${name}`, {
		method: 'PUT',
		body: JSON.stringify(script),
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
		method: 'POST',
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

export async function getClaudeMD(scope?: 'global' | 'user' | 'project'): Promise<ClaudeMD> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<ClaudeMD>(`/claudemd${params}`);
}

export async function updateClaudeMD(
	content: string,
	scope?: 'global' | 'user' | 'project'
): Promise<void> {
	const params = scope ? `?scope=${scope}` : '';
	const res = await fetch(`${API_BASE}/claudemd${params}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ content }),
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

export async function getClaudeMDHierarchy(): Promise<ClaudeMDHierarchy> {
	return fetchJSON<ClaudeMDHierarchy>('/claudemd/hierarchy');
}

// Constitution (project principles/invariants)
export interface Constitution {
	content: string;
	version: string;
	exists: boolean;
}

export async function getConstitution(): Promise<Constitution> {
	return fetchJSON<Constitution>('/constitution');
}

export async function updateConstitution(content: string, version?: string): Promise<Constitution> {
	const res = await fetch(`${API_BASE}/constitution`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ content, version: version || '1.0.0' }),
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
	return res.json();
}

export async function deleteConstitution(): Promise<void> {
	const res = await fetch(`${API_BASE}/constitution`, { method: 'DELETE' });
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
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

export async function listMCPServers(scope?: 'global'): Promise<MCPServerInfo[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<MCPServerInfo[]>(`/mcp${params}`);
}

export async function getMCPServer(name: string): Promise<MCPServer> {
	return fetchJSON<MCPServer>(`/mcp/${name}`);
}

export async function createMCPServer(server: MCPServerCreate): Promise<MCPServerInfo> {
	return fetchJSON<MCPServerInfo>('/mcp', {
		method: 'POST',
		body: JSON.stringify(server),
	});
}

export async function updateMCPServer(
	name: string,
	server: Partial<MCPServerCreate>
): Promise<MCPServerInfo> {
	return fetchJSON<MCPServerInfo>(`/mcp/${name}`, {
		method: 'PUT',
		body: JSON.stringify(server),
	});
}

export async function deleteMCPServer(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/mcp/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Plugins (.claude/plugins/)
export type PluginScope = 'global' | 'project';

export interface PluginAuthor {
	name: string;
	email?: string;
	url?: string;
}

export interface PluginInfo {
	name: string;
	description: string;
	author: string;
	path: string;
	scope: PluginScope;
	enabled: boolean;
	version?: string;
	has_commands: boolean;
	command_count: number;
}

export interface PluginCommand {
	name: string;
	description: string;
	argument_hint?: string;
	file_path?: string;
}

export interface PluginMCPServer {
	name: string;
	command: string;
	args?: string[];
	env?: Record<string, string>;
	url?: string;
	type?: string;
}

export interface PluginHook {
	event: string;
	type: string;
	command: string;
	matcher?: string;
	description?: string;
}

export interface Plugin {
	name: string;
	description: string;
	author?: PluginAuthor;
	homepage?: string;
	keywords?: string[];
	path: string;
	scope: PluginScope;
	enabled: boolean;
	version?: string;
	installed_at?: string;
	updated_at?: string;
	has_commands: boolean;
	has_hooks: boolean;
	has_scripts: boolean;
	// Discovered resources
	commands?: PluginCommand[];
	mcp_servers?: PluginMCPServer[];
	hooks?: PluginHook[];
}

export interface MarketplacePlugin {
	name: string;
	description: string;
	author: PluginAuthor;
	version: string;
	repository?: string;
	downloads?: number;
	keywords?: string[];
}

export interface PluginUpdateInfo {
	name: string;
	current_version: string;
	latest_version: string;
	scope: PluginScope;
}

export interface PluginResponse {
	plugin?: Plugin;
	requires_restart: boolean;
	message?: string;
}

export interface MarketplaceBrowseResponse {
	plugins: MarketplacePlugin[];
	total: number;
	page: number;
	limit: number;
	cached: boolean;
	cache_age_seconds?: number;
	is_mock?: boolean;
	message?: string;
}

// Local plugin management
export async function listPlugins(scope?: PluginScope): Promise<PluginInfo[]> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginInfo[]>(`/plugins${query}`);
}

export async function getPlugin(name: string, scope?: PluginScope): Promise<Plugin> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<Plugin>(`/plugins/${name}${query}`);
}

export async function enablePlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}/enable${query}`, {
		method: 'POST',
	});
}

export async function disablePlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}/disable${query}`, {
		method: 'POST',
	});
}

export async function uninstallPlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}${query}`, {
		method: 'DELETE',
	});
}

export async function listPluginCommands(
	name: string,
	scope?: PluginScope
): Promise<PluginCommand[]> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginCommand[]>(`/plugins/${name}/commands${query}`);
}

// Plugin resources with source info
export interface PluginMCPServerWithSource extends PluginMCPServer {
	plugin_name: string;
	plugin_scope: PluginScope;
}

export interface PluginHookWithSource extends PluginHook {
	plugin_name: string;
	plugin_scope: PluginScope;
}

export interface PluginCommandWithSource extends PluginCommand {
	plugin_name: string;
	plugin_scope: PluginScope;
}

export interface PluginResourcesResponse {
	mcp_servers: PluginMCPServerWithSource[];
	hooks: PluginHookWithSource[];
	commands: PluginCommandWithSource[];
}

export async function getPluginResources(): Promise<PluginResourcesResponse> {
	return fetchJSON<PluginResourcesResponse>('/plugins/resources');
}

// Marketplace (separate API prefix to avoid route conflicts)
export async function browseMarketplace(
	page?: number,
	limit?: number
): Promise<MarketplaceBrowseResponse> {
	const params = new URLSearchParams();
	if (page) params.set('page', String(page));
	if (limit) params.set('limit', String(limit));
	const query = params.toString();
	return fetchJSON<MarketplaceBrowseResponse>(`/marketplace/plugins${query ? '?' + query : ''}`);
}

export async function searchMarketplace(query: string): Promise<MarketplacePlugin[]> {
	return fetchJSON<MarketplacePlugin[]>(
		`/marketplace/plugins/search?q=${encodeURIComponent(query)}`
	);
}

export async function getMarketplacePlugin(name: string): Promise<MarketplacePlugin> {
	return fetchJSON<MarketplacePlugin>(`/marketplace/plugins/${name}`);
}

export async function installPlugin(
	name: string,
	scope?: PluginScope,
	version?: string
): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/marketplace/plugins/${name}/install${query}`, {
		method: 'POST',
		body: version ? JSON.stringify({ version }) : undefined,
	});
}

// Plugin updates
export async function checkPluginUpdates(): Promise<PluginUpdateInfo[]> {
	return fetchJSON<PluginUpdateInfo[]>('/plugins/updates');
}

export async function updatePlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}/update${query}`, {
		method: 'POST',
	});
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

export async function runProjectTask(
	projectId: string,
	taskId: string
): Promise<RunTaskResponse> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/run`, { method: 'POST' });
}

export async function pauseProjectTask(
	projectId: string,
	taskId: string
): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/pause`, { method: 'POST' });
}

export async function resumeProjectTask(
	projectId: string,
	taskId: string
): Promise<{ status: string; task_id: string }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/resume`, { method: 'POST' });
}

export async function deleteProjectTask(projectId: string, taskId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/projects/${projectId}/tasks/${taskId}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete task');
	}
}

export async function escalateProjectTask(
	projectId: string,
	taskId: string,
	reason: string
): Promise<{ status: string; task_id: string; phase: string; reason: string; attempt: number }> {
	return fetchJSON(`/projects/${projectId}/tasks/${taskId}/escalate`, {
		method: 'POST',
		body: JSON.stringify({ reason }),
	});
}

export async function getProjectTranscripts(
	projectId: string,
	taskId: string
): Promise<Transcript[]> {
	return fetchJSON<Transcript[]>(`/projects/${projectId}/tasks/${taskId}/transcripts`);
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
	cache_creation_input_tokens?: number;
	cache_read_input_tokens?: number;
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

export async function createTemplate(
	taskId: string,
	name: string,
	description?: string,
	global?: boolean
): Promise<Template> {
	return fetchJSON<Template>('/templates', {
		method: 'POST',
		body: JSON.stringify({ task_id: taskId, name, description, global }),
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

export async function createReviewComment(
	taskId: string,
	comment: CreateCommentRequest
): Promise<ReviewComment> {
	return fetchJSON<ReviewComment>(`/tasks/${taskId}/review/comments`, {
		method: 'POST',
		body: JSON.stringify(comment),
	});
}

export async function updateReviewComment(
	taskId: string,
	commentId: string,
	update: UpdateCommentRequest
): Promise<ReviewComment> {
	return fetchJSON<ReviewComment>(`/tasks/${taskId}/review/comments/${commentId}`, {
		method: 'PATCH',
		body: JSON.stringify(update),
	});
}

export async function deleteReviewComment(taskId: string, commentId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/tasks/${taskId}/review/comments/${commentId}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete comment');
	}
}

export async function triggerReviewRetry(taskId: string): Promise<void> {
	await fetchJSON(`/tasks/${taskId}/review/retry`, {
		method: 'POST',
		body: JSON.stringify({ include_comments: true }),
	});
}

// Review Findings (structured review phase output)
export interface ReviewFinding {
	severity: string;
	file?: string;
	line?: number;
	description: string;
	suggestion?: string;
	agent_id?: string;
	constitution_violation?: string;
}

export interface ReviewFindings {
	task_id: string;
	round: number;
	summary: string;
	issues: ReviewFinding[];
	questions?: string[];
	positives?: string[];
	agent_id?: string;
	created_at: string;
}

export async function getReviewFindings(taskId: string): Promise<ReviewFindings[]> {
	return fetchJSON<ReviewFindings[]>(`/tasks/${taskId}/review/findings`);
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
			suggestions: comments.filter((c) => c.severity === 'suggestion' && c.status === 'open')
				.length,
		};
	} catch {
		return null;
	}
}

// Task Comments (general notes/discussion)
export async function getTaskComments(
	taskId: string,
	authorType?: string,
	phase?: string
): Promise<TaskComment[]> {
	let url = `/tasks/${taskId}/comments`;
	const params = new URLSearchParams();
	if (authorType) params.set('author_type', authorType);
	if (phase) params.set('phase', phase);
	if (params.toString()) url += `?${params.toString()}`;
	return fetchJSON<TaskComment[]>(url);
}

export async function getTaskComment(taskId: string, commentId: string): Promise<TaskComment> {
	return fetchJSON<TaskComment>(`/tasks/${taskId}/comments/${commentId}`);
}

export async function createTaskComment(
	taskId: string,
	comment: CreateTaskCommentRequest
): Promise<TaskComment> {
	return fetchJSON<TaskComment>(`/tasks/${taskId}/comments`, {
		method: 'POST',
		body: JSON.stringify(comment),
	});
}

export async function updateTaskComment(
	taskId: string,
	commentId: string,
	update: UpdateTaskCommentRequest
): Promise<TaskComment> {
	return fetchJSON<TaskComment>(`/tasks/${taskId}/comments/${commentId}`, {
		method: 'PATCH',
		body: JSON.stringify(update),
	});
}

export async function deleteTaskComment(taskId: string, commentId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/tasks/${taskId}/comments/${commentId}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete comment');
	}
}

export async function getTaskCommentStats(taskId: string): Promise<TaskCommentStats | null> {
	try {
		return await fetchJSON<TaskCommentStats>(`/tasks/${taskId}/comments/stats`);
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

export async function createPR(
	taskId: string,
	options?: {
		title?: string;
		body?: string;
		base?: string;
		labels?: string[];
		reviewers?: string[];
		draft?: boolean;
	}
): Promise<CreatePRResponse> {
	return fetchJSON<CreatePRResponse>(`/tasks/${taskId}/github/pr`, {
		method: 'POST',
		body: JSON.stringify(options || {}),
	});
}

export async function getPR(taskId: string): Promise<GetPRResponse> {
	return fetchJSON<GetPRResponse>(`/tasks/${taskId}/github/pr`);
}

export async function mergePR(
	taskId: string,
	options?: {
		method?: 'merge' | 'squash' | 'rebase';
		delete_branch?: boolean;
	}
): Promise<MergePRResponse> {
	return fetchJSON<MergePRResponse>(`/tasks/${taskId}/github/pr/merge`, {
		method: 'POST',
		body: JSON.stringify(options || { method: 'squash', delete_branch: true }),
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
		body: JSON.stringify(options),
	});
}

export async function getExportConfig(): Promise<ExportConfig> {
	return fetchJSON<ExportConfig>('/config/export');
}

export async function updateExportConfig(config: Partial<ExportConfig>): Promise<ExportConfig> {
	return fetchJSON<ExportConfig>('/config/export', {
		method: 'PUT',
		body: JSON.stringify(config),
	});
}

// Knowledge Queue
export type KnowledgeType = 'pattern' | 'gotcha' | 'decision';
export type KnowledgeStatus = 'pending' | 'approved' | 'rejected';
export type KnowledgeScope = 'project' | 'global';

export interface KnowledgeEntry {
	id: string;
	type: KnowledgeType;
	name: string;
	description: string;
	scope: KnowledgeScope;
	source_task?: string;
	status: KnowledgeStatus;
	proposed_by?: string;
	proposed_at: string;
	approved_by?: string;
	approved_at?: string;
	rejected_reason?: string;
	validated_at?: string;
	validated_by?: string;
}

export interface KnowledgeStatusResponse {
	pending_count: number;
	stale_count: number;
	approved_count: number;
}

export async function listKnowledge(options?: {
	status?: KnowledgeStatus;
	type?: KnowledgeType;
}): Promise<KnowledgeEntry[]> {
	const params = new URLSearchParams();
	if (options?.status) params.set('status', options.status);
	if (options?.type) params.set('type', options.type);
	const query = params.toString();
	return fetchJSON<KnowledgeEntry[]>(`/knowledge${query ? '?' + query : ''}`);
}

export async function getKnowledgeStatus(): Promise<KnowledgeStatusResponse> {
	return fetchJSON<KnowledgeStatusResponse>('/knowledge/status');
}

export async function listStaleKnowledge(days?: number): Promise<KnowledgeEntry[]> {
	const query = days ? `?days=${days}` : '';
	return fetchJSON<KnowledgeEntry[]>(`/knowledge/stale${query}`);
}

export async function createKnowledge(entry: {
	type: KnowledgeType;
	name: string;
	description: string;
	source_task?: string;
	proposed_by?: string;
}): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>('/knowledge', {
		method: 'POST',
		body: JSON.stringify(entry),
	});
}

export async function getKnowledge(id: string): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>(`/knowledge/${id}`);
}

export async function approveKnowledge(id: string, approvedBy?: string): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>(`/knowledge/${id}/approve`, {
		method: 'POST',
		body: JSON.stringify({ approved_by: approvedBy }),
	});
}

export async function approveAllKnowledge(
	approvedBy?: string
): Promise<{ approved_count: number }> {
	return fetchJSON<{ approved_count: number }>('/knowledge/approve-all', {
		method: 'POST',
		body: JSON.stringify({ approved_by: approvedBy }),
	});
}

export async function validateKnowledge(
	id: string,
	validatedBy?: string
): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>(`/knowledge/${id}/validate`, {
		method: 'POST',
		body: JSON.stringify({ validated_by: validatedBy }),
	});
}

export async function rejectKnowledge(id: string, reason?: string): Promise<void> {
	const res = await fetch(`${API_BASE}/knowledge/${id}/reject`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ reason }),
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

export async function deleteKnowledge(id: string): Promise<void> {
	const res = await fetch(`${API_BASE}/knowledge/${id}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Attachments
export async function listAttachments(taskId: string): Promise<Attachment[]> {
	return fetchJSON<Attachment[]>(`/tasks/${taskId}/attachments`);
}

export async function uploadAttachment(
	taskId: string,
	file: File,
	filename?: string
): Promise<Attachment> {
	const formData = new FormData();
	formData.append('file', file);
	if (filename) {
		formData.append('filename', filename);
	}

	const res = await fetch(`${API_BASE}/tasks/${taskId}/attachments`, {
		method: 'POST',
		body: formData,
	});

	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Upload failed');
	}

	return res.json();
}

export function getAttachmentUrl(taskId: string, filename: string): string {
	return `${API_BASE}/tasks/${taskId}/attachments/${encodeURIComponent(filename)}`;
}

export async function deleteAttachment(taskId: string, filename: string): Promise<void> {
	const res = await fetch(
		`${API_BASE}/tasks/${taskId}/attachments/${encodeURIComponent(filename)}`,
		{
			method: 'DELETE',
		}
	);
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Delete failed');
	}
}

// Test Results (Playwright)
export async function getTestResults(taskId: string): Promise<TestResultsInfo> {
	return fetchJSON<TestResultsInfo>(`/tasks/${taskId}/test-results`);
}

export async function listScreenshots(taskId: string): Promise<Screenshot[]> {
	return fetchJSON<Screenshot[]>(`/tasks/${taskId}/test-results/screenshots`);
}

export function getScreenshotUrl(taskId: string, filename: string): string {
	return `${API_BASE}/tasks/${taskId}/test-results/screenshots/${encodeURIComponent(filename)}`;
}

export async function uploadScreenshot(
	taskId: string,
	file: File,
	filename?: string
): Promise<Screenshot> {
	const formData = new FormData();
	formData.append('file', file);
	if (filename) {
		formData.append('filename', filename);
	}

	const res = await fetch(`${API_BASE}/tasks/${taskId}/test-results/screenshots`, {
		method: 'POST',
		body: formData,
	});

	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Upload failed');
	}

	return res.json();
}

export async function saveTestReport(taskId: string, report: TestReport): Promise<void> {
	await fetchJSON(`/tasks/${taskId}/test-results`, {
		method: 'POST',
		body: JSON.stringify(report),
	});
}

export async function initTestResults(
	taskId: string
): Promise<{ status: string; path: string }> {
	return fetchJSON(`/tasks/${taskId}/test-results/init`, {
		method: 'POST',
	});
}

export function getHTMLReportUrl(taskId: string): string {
	return `${API_BASE}/tasks/${taskId}/test-results/report`;
}

export function getTraceUrl(taskId: string, filename: string): string {
	return `${API_BASE}/tasks/${taskId}/test-results/traces/${encodeURIComponent(filename)}`;
}

// Task Dependencies
export interface DependencyInfo {
	id: string;
	title: string;
	status: string;
	is_met?: boolean;
}

export interface DependencyGraph {
	task_id: string;
	blocked_by: DependencyInfo[];
	blocks: DependencyInfo[];
	related_to: DependencyInfo[];
	referenced_by: DependencyInfo[];
	unmet_dependencies?: string[];
	can_run: boolean;
}

export async function getTaskDependencies(taskId: string): Promise<DependencyGraph> {
	return fetchJSON<DependencyGraph>(`/tasks/${taskId}/dependencies`);
}

export async function addBlocker(taskId: string, blockerId: string): Promise<Task> {
	// Get current task to get existing blockers
	const task = await getTask(taskId);
	const blockedBy = [...(task.blocked_by || [])];
	if (!blockedBy.includes(blockerId)) {
		blockedBy.push(blockerId);
	}
	return updateTask(taskId, { blocked_by: blockedBy });
}

export async function removeBlocker(taskId: string, blockerId: string): Promise<Task> {
	const task = await getTask(taskId);
	const blockedBy = (task.blocked_by || []).filter((id) => id !== blockerId);
	return updateTask(taskId, { blocked_by: blockedBy });
}

export async function addRelated(taskId: string, relatedId: string): Promise<Task> {
	const task = await getTask(taskId);
	const relatedTo = [...(task.related_to || [])];
	if (!relatedTo.includes(relatedId)) {
		relatedTo.push(relatedId);
	}
	return updateTask(taskId, { related_to: relatedTo });
}

export async function removeRelated(taskId: string, relatedId: string): Promise<Task> {
	const task = await getTask(taskId);
	const relatedTo = (task.related_to || []).filter((id) => id !== relatedId);
	return updateTask(taskId, { related_to: relatedTo });
}

// Initiatives
export interface CreateInitiativeRequest {
	title: string;
	vision?: string;
	owner?: InitiativeIdentity;
	branch_base?: string;
	branch_prefix?: string;
	shared?: boolean;
}

export interface UpdateInitiativeRequest {
	title?: string;
	vision?: string;
	status?: InitiativeStatus;
	owner?: InitiativeIdentity;
	branch_base?: string;
	branch_prefix?: string;
}

export async function listInitiatives(options?: {
	status?: InitiativeStatus;
	shared?: boolean;
}): Promise<Initiative[]> {
	const params = new URLSearchParams();
	if (options?.status) params.set('status', options.status);
	if (options?.shared) params.set('shared', 'true');
	const query = params.toString();
	return fetchJSON<Initiative[]>(`/initiatives${query ? '?' + query : ''}`);
}

export async function getInitiative(id: string, shared?: boolean): Promise<Initiative> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<Initiative>(`/initiatives/${id}${query}`);
}

export async function createInitiative(req: CreateInitiativeRequest): Promise<Initiative> {
	return fetchJSON<Initiative>('/initiatives', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export async function updateInitiative(
	id: string,
	req: UpdateInitiativeRequest,
	shared?: boolean
): Promise<Initiative> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<Initiative>(`/initiatives/${id}${query}`, {
		method: 'PUT',
		body: JSON.stringify(req),
	});
}

export async function deleteInitiative(id: string, shared?: boolean): Promise<void> {
	const query = shared ? '?shared=true' : '';
	const res = await fetch(`${API_BASE}/initiatives/${id}${query}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete initiative');
	}
}

// Initiative Tasks
export interface AddInitiativeTaskRequest {
	task_id: string;
	depends_on?: string[];
}

export async function listInitiativeTasks(
	id: string,
	shared?: boolean
): Promise<InitiativeTaskRef[]> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<InitiativeTaskRef[]>(`/initiatives/${id}/tasks${query}`);
}

export async function addInitiativeTask(
	id: string,
	req: AddInitiativeTaskRequest,
	shared?: boolean
): Promise<InitiativeTaskRef[]> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<InitiativeTaskRef[]>(`/initiatives/${id}/tasks${query}`, {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export async function removeInitiativeTask(
	id: string,
	taskId: string,
	shared?: boolean
): Promise<void> {
	const query = shared ? '?shared=true' : '';
	const res = await fetch(`${API_BASE}/initiatives/${id}/tasks/${taskId}${query}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to remove task from initiative');
	}
}

// Initiative Decisions
export interface AddInitiativeDecisionRequest {
	decision: string;
	rationale?: string;
	by?: string;
}

export async function addInitiativeDecision(
	id: string,
	req: AddInitiativeDecisionRequest,
	shared?: boolean
): Promise<InitiativeDecision[]> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<InitiativeDecision[]>(`/initiatives/${id}/decisions${query}`, {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export async function getReadyTasks(id: string, shared?: boolean): Promise<InitiativeTaskRef[]> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<InitiativeTaskRef[]>(`/initiatives/${id}/ready${query}`);
}

// Finalize Operations
export type FinalizeStatus = 'not_started' | 'pending' | 'running' | 'completed' | 'failed';

export interface FinalizeResult {
	synced: boolean;
	conflicts_resolved: number;
	conflict_files?: string[];
	tests_passed: boolean;
	risk_level: string;
	files_changed: number;
	lines_changed: number;
	needs_review: boolean;
	commit_sha?: string;
	target_branch: string;
}

export interface FinalizeState {
	task_id: string;
	status: FinalizeStatus;
	started_at?: string;
	updated_at?: string;
	completed_at?: string;
	step?: string;
	progress?: string;
	step_percent?: number;
	result?: FinalizeResult;
	error?: string;
	commit_sha?: string;
	message?: string;
}

export interface FinalizeRequest {
	force?: boolean;
	gate_override?: boolean;
}

export interface FinalizeResponse {
	task_id: string;
	status: FinalizeStatus;
	message?: string;
}

export async function triggerFinalize(
	taskId: string,
	options?: FinalizeRequest
): Promise<FinalizeResponse> {
	return fetchJSON<FinalizeResponse>(`/tasks/${taskId}/finalize`, {
		method: 'POST',
		body: JSON.stringify(options || {}),
	});
}

export async function getFinalizeStatus(taskId: string): Promise<FinalizeState> {
	return fetchJSON<FinalizeState>(`/tasks/${taskId}/finalize`);
}

// Dependency Graph
export interface DependencyGraphNode {
	id: string;
	title: string;
	status: 'done' | 'running' | 'blocked' | 'ready' | 'pending' | 'paused' | 'failed';
}

export interface DependencyGraphEdge {
	from: string;
	to: string;
}

export interface DependencyGraphData {
	nodes: DependencyGraphNode[];
	edges: DependencyGraphEdge[];
}

export async function getInitiativeDependencyGraph(
	id: string,
	shared?: boolean
): Promise<DependencyGraphData> {
	const query = shared ? '?shared=true' : '';
	return fetchJSON<DependencyGraphData>(`/initiatives/${id}/dependency-graph${query}`);
}

export async function getTasksDependencyGraph(taskIds: string[]): Promise<DependencyGraphData> {
	const ids = taskIds.join(',');
	return fetchJSON<DependencyGraphData>(`/tasks/dependency-graph?ids=${encodeURIComponent(ids)}`);
}

// Decisions (gate approval in headless mode)
export interface SubmitDecisionRequest {
	approved: boolean;
	reason?: string;
}

export interface SubmitDecisionResponse {
	decision_id: string;
	task_id: string;
	approved: boolean;
	new_status: string;
}

export async function submitDecision(
	decisionId: string,
	request: SubmitDecisionRequest
): Promise<SubmitDecisionResponse> {
	return fetchJSON<SubmitDecisionResponse>(`/decisions/${decisionId}`, {
		method: 'POST',
		body: JSON.stringify(request),
	});
}

// Branch Registry Operations
export interface BranchListOptions {
	type?: 'initiative' | 'staging' | 'task';
	status?: 'active' | 'merged' | 'stale' | 'orphaned';
}

export async function listBranches(options?: BranchListOptions): Promise<Branch[]> {
	const params = new URLSearchParams();
	if (options?.type) params.set('type', options.type);
	if (options?.status) params.set('status', options.status);
	const query = params.toString() ? `?${params.toString()}` : '';
	return fetchJSON<Branch[]>(`/branches${query}`);
}

export async function getBranch(name: string): Promise<Branch> {
	return fetchJSON<Branch>(`/branches/${encodeURIComponent(name)}`);
}

export async function updateBranchStatus(
	name: string,
	status: 'active' | 'merged' | 'stale' | 'orphaned'
): Promise<void> {
	const res = await fetch(`${API_BASE}/branches/${encodeURIComponent(name)}/status`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ status }),
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to update branch status');
	}
}

export async function deleteBranch(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/branches/${encodeURIComponent(name)}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete branch');
	}
}

// Workflows
export async function listWorkflows(options?: {
	builtin?: boolean;
	custom?: boolean;
}): Promise<Workflow[]> {
	const params = new URLSearchParams();
	if (options?.builtin) params.set('builtin', 'true');
	if (options?.custom) params.set('custom', 'true');
	const query = params.toString() ? `?${params.toString()}` : '';
	return fetchJSON<Workflow[]>(`/workflows${query}`);
}

export async function getWorkflow(id: string): Promise<WorkflowWithDetails> {
	return fetchJSON<WorkflowWithDetails>(`/workflows/${id}`);
}

export interface CreateWorkflowRequest {
	id: string;
	name?: string;
	description?: string;
	workflow_type?: string;
	default_model?: string;
	default_thinking?: boolean;
	based_on?: string;
}

export async function createWorkflow(req: CreateWorkflowRequest): Promise<Workflow> {
	return fetchJSON<Workflow>('/workflows', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export interface UpdateWorkflowRequest {
	name?: string;
	description?: string;
	default_model?: string;
	default_thinking?: boolean;
}

export async function updateWorkflow(id: string, req: UpdateWorkflowRequest): Promise<Workflow> {
	return fetchJSON<Workflow>(`/workflows/${id}`, {
		method: 'PUT',
		body: JSON.stringify(req),
	});
}

export async function deleteWorkflow(id: string): Promise<void> {
	const res = await fetch(`${API_BASE}/workflows/${id}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete workflow');
	}
}

export interface CloneWorkflowRequest {
	new_id: string;
	name?: string;
	description?: string;
}

export async function cloneWorkflow(id: string, req: CloneWorkflowRequest): Promise<Workflow> {
	return fetchJSON<Workflow>(`/workflows/${id}/clone`, {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

// Workflow Phases
export interface AddWorkflowPhaseRequest {
	phase_template_id: string;
	sequence: number;
	max_iterations_override?: number;
	model_override?: string;
	gate_type_override?: string;
}

export async function addWorkflowPhase(
	workflowId: string,
	req: AddWorkflowPhaseRequest
): Promise<WorkflowPhase> {
	return fetchJSON<WorkflowPhase>(`/workflows/${workflowId}/phases`, {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export async function removeWorkflowPhase(
	workflowId: string,
	phaseTemplateId: string
): Promise<void> {
	const res = await fetch(`${API_BASE}/workflows/${workflowId}/phases/${phaseTemplateId}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to remove phase');
	}
}

// Workflow Variables
export interface AddWorkflowVariableRequest {
	name: string;
	description?: string;
	source_type: string;
	source_config: Record<string, unknown>;
	required?: boolean;
	default_value?: string;
	cache_ttl_seconds?: number;
}

export async function addWorkflowVariable(
	workflowId: string,
	req: AddWorkflowVariableRequest
): Promise<WorkflowVariable> {
	return fetchJSON<WorkflowVariable>(`/workflows/${workflowId}/variables`, {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export async function removeWorkflowVariable(workflowId: string, name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/workflows/${workflowId}/variables/${name}`, {
		method: 'DELETE',
	});
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to remove variable');
	}
}

// Phase Templates
export async function listPhaseTemplates(options?: {
	builtin?: boolean;
	custom?: boolean;
}): Promise<PhaseTemplate[]> {
	const params = new URLSearchParams();
	if (options?.builtin) params.set('builtin', 'true');
	if (options?.custom) params.set('custom', 'true');
	const query = params.toString() ? `?${params.toString()}` : '';
	return fetchJSON<PhaseTemplate[]>(`/phase-templates${query}`);
}

export async function getPhaseTemplate(id: string): Promise<PhaseTemplate> {
	return fetchJSON<PhaseTemplate>(`/phase-templates/${id}`);
}

export interface CreatePhaseTemplateRequest {
	id: string;
	name?: string;
	description?: string;
	prompt_source?: string;
	prompt_content?: string;
	prompt_path?: string;
	max_iterations?: number;
	model_override?: string;
	gate_type?: string;
	produces_artifact?: boolean;
	artifact_type?: string;
}

export async function createPhaseTemplate(
	req: CreatePhaseTemplateRequest
): Promise<PhaseTemplate> {
	return fetchJSON<PhaseTemplate>('/phase-templates', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export interface UpdatePhaseTemplateRequest {
	name?: string;
	description?: string;
	prompt_content?: string;
	max_iterations?: number;
	model_override?: string;
	gate_type?: string;
}

export async function updatePhaseTemplate(
	id: string,
	req: UpdatePhaseTemplateRequest
): Promise<PhaseTemplate> {
	return fetchJSON<PhaseTemplate>(`/phase-templates/${id}`, {
		method: 'PUT',
		body: JSON.stringify(req),
	});
}

export async function deletePhaseTemplate(id: string): Promise<void> {
	const res = await fetch(`${API_BASE}/phase-templates/${id}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Failed to delete phase template');
	}
}

// Workflow Runs
export async function listWorkflowRuns(options?: {
	status?: string;
	workflow_id?: string;
	task_id?: string;
	limit?: number;
}): Promise<WorkflowRun[]> {
	const params = new URLSearchParams();
	if (options?.status) params.set('status', options.status);
	if (options?.workflow_id) params.set('workflow_id', options.workflow_id);
	if (options?.task_id) params.set('task_id', options.task_id);
	if (options?.limit) params.set('limit', String(options.limit));
	const query = params.toString() ? `?${params.toString()}` : '';
	return fetchJSON<WorkflowRun[]>(`/workflow-runs${query}`);
}

export async function getWorkflowRun(id: string): Promise<WorkflowRunWithDetails> {
	return fetchJSON<WorkflowRunWithDetails>(`/workflow-runs/${id}`);
}

export async function cancelWorkflowRun(id: string): Promise<{ status: string }> {
	return fetchJSON<{ status: string }>(`/workflow-runs/${id}/cancel`, {
		method: 'POST',
	});
}

// Timeline Events
import type { EventsListResponse, GetEventsOptions } from './types';

export async function getEvents(options: GetEventsOptions = {}): Promise<EventsListResponse> {
	const params = new URLSearchParams();
	if (options.task_id) params.set('task_id', options.task_id);
	if (options.initiative_id) params.set('initiative_id', options.initiative_id);
	if (options.since) params.set('since', options.since);
	if (options.until) params.set('until', options.until);
	if (options.types?.length) params.set('types', options.types.join(','));
	if (options.limit) params.set('limit', String(options.limit));
	if (options.offset) params.set('offset', String(options.offset));
	const query = params.toString() ? `?${params.toString()}` : '';
	return fetchJSON<EventsListResponse>(`/events${query}`);
}

// Automation API
export interface TriggerConfig {
	metric?: string;
	threshold?: number;
	event?: string;
	operator?: string;
	value?: number;
	weights?: string[];
	categories?: string[];
	filter?: Record<string, unknown>;
}

export interface Trigger {
	id: string;
	type: string;
	description: string;
	enabled: boolean;
	config: TriggerConfig;
	last_triggered_at: string | null;
	trigger_count: number;
	created_at: string;
}

export interface TriggerExecution {
	id: number;
	trigger_id: string;
	task_id: string | null;
	triggered_at: string;
	trigger_reason: string;
	status: string;
	completed_at: string | null;
	error_message: string | null;
}

export async function listTriggers(): Promise<Trigger[]> {
	const data = await fetchJSON<{ triggers: Trigger[] }>('/automation/triggers');
	return data.triggers || [];
}

export async function getTriggerHistory(
	triggerId: string,
	limit = 10
): Promise<TriggerExecution[]> {
	const data = await fetchJSON<{ executions: TriggerExecution[] }>(
		`/automation/triggers/${triggerId}/history?limit=${limit}`
	);
	return data.executions || [];
}

export async function runTrigger(triggerId: string): Promise<{ task_id: string }> {
	return fetchJSON<{ task_id: string }>(`/automation/triggers/${triggerId}/run`, {
		method: 'POST',
	});
}

export async function toggleTrigger(triggerId: string, enabled: boolean): Promise<void> {
	await fetchJSON(`/automation/triggers/${triggerId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ enabled }),
	});
}

export async function resetTrigger(triggerId: string): Promise<void> {
	await fetchJSON(`/automation/triggers/${triggerId}/reset`, { method: 'POST' });
}

// Notifications API
export interface Notification {
	id: string;
	type: string;
	title: string;
	message?: string;
	source_type?: string;
	source_id?: string;
	created_at: string;
	expires_at?: string;
}

export async function listNotifications(): Promise<Notification[]> {
	const data = await fetchJSON<{ notifications: Notification[] }>('/notifications');
	return data.notifications || [];
}

export async function dismissNotification(id: string): Promise<void> {
	await fetchJSON(`/notifications/${id}/dismiss`, { method: 'POST' });
}

export async function dismissAllNotifications(): Promise<void> {
	await fetchJSON('/notifications/dismiss-all', { method: 'POST' });
}

// Session API
export async function pauseAllTasks(): Promise<void> {
	await fetchJSON('/tasks/pause-all', { method: 'POST' });
}

export async function resumeAllTasks(): Promise<void> {
	await fetchJSON('/tasks/resume-all', { method: 'POST' });
}
