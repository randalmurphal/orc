import type { Task, Plan, TaskState, Project, ReviewComment, CreateCommentRequest, UpdateCommentRequest, PR, PRComment, CheckRun, CheckSummary, TaskComment, CreateTaskCommentRequest, UpdateTaskCommentRequest, TaskCommentStats } from './types';

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

export async function createHook(event: string, hook: Hook, scope?: 'global' | 'project'): Promise<Hook[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Hook[]>(`/hooks${params}`, {
		method: 'POST',
		body: JSON.stringify({ event, hook })
	});
}

export async function updateHook(event: string, hooks: Hook[], scope?: 'global' | 'project'): Promise<Hook[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Hook[]>(`/hooks/${event}${params}`, {
		method: 'PUT',
		body: JSON.stringify({ hooks })
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
		body: JSON.stringify(skill)
	});
}

export async function updateSkill(name: string, skill: Skill, scope?: 'global' | 'project'): Promise<Skill> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<Skill>(`/skills/${name}${params}`, {
		method: 'PUT',
		body: JSON.stringify(skill)
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
		body: JSON.stringify(perms)
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

export async function getClaudeMD(scope?: 'global' | 'user' | 'project'): Promise<ClaudeMD> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<ClaudeMD>(`/claudemd${params}`);
}

export async function updateClaudeMD(content: string, scope?: 'global' | 'user' | 'project'): Promise<void> {
	const params = scope ? `?scope=${scope}` : '';
	const res = await fetch(`${API_BASE}/claudemd${params}`, {
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
		method: 'POST'
	});
}

export async function disablePlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}/disable${query}`, {
		method: 'POST'
	});
}

export async function uninstallPlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}${query}`, {
		method: 'DELETE'
	});
}

export async function listPluginCommands(name: string, scope?: PluginScope): Promise<PluginCommand[]> {
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
export async function browseMarketplace(page?: number, limit?: number): Promise<MarketplaceBrowseResponse> {
	const params = new URLSearchParams();
	if (page) params.set('page', String(page));
	if (limit) params.set('limit', String(limit));
	const query = params.toString();
	return fetchJSON<MarketplaceBrowseResponse>(`/marketplace/plugins${query ? '?' + query : ''}`);
}

export async function searchMarketplace(query: string): Promise<MarketplacePlugin[]> {
	return fetchJSON<MarketplacePlugin[]>(`/marketplace/plugins/search?q=${encodeURIComponent(query)}`);
}

export async function getMarketplacePlugin(name: string): Promise<MarketplacePlugin> {
	return fetchJSON<MarketplacePlugin>(`/marketplace/plugins/${name}`);
}

export async function installPlugin(name: string, scope?: PluginScope, version?: string): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/marketplace/plugins/${name}/install${query}`, {
		method: 'POST',
		body: version ? JSON.stringify({ version }) : undefined
	});
}

// Plugin updates
export async function checkPluginUpdates(): Promise<PluginUpdateInfo[]> {
	return fetchJSON<PluginUpdateInfo[]>('/plugins/updates');
}

export async function updatePlugin(name: string, scope?: PluginScope): Promise<PluginResponse> {
	const query = scope ? `?scope=${scope}` : '';
	return fetchJSON<PluginResponse>(`/plugins/${name}/update${query}`, {
		method: 'POST'
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

// Task Comments (general notes/discussion)
export async function getTaskComments(taskId: string, authorType?: string, phase?: string): Promise<TaskComment[]> {
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

export async function createTaskComment(taskId: string, comment: CreateTaskCommentRequest): Promise<TaskComment> {
	return fetchJSON<TaskComment>(`/tasks/${taskId}/comments`, {
		method: 'POST',
		body: JSON.stringify(comment)
	});
}

export async function updateTaskComment(taskId: string, commentId: string, update: UpdateTaskCommentRequest): Promise<TaskComment> {
	return fetchJSON<TaskComment>(`/tasks/${taskId}/comments/${commentId}`, {
		method: 'PATCH',
		body: JSON.stringify(update)
	});
}

export async function deleteTaskComment(taskId: string, commentId: string): Promise<void> {
	const res = await fetch(`${API_BASE}/tasks/${taskId}/comments/${commentId}`, { method: 'DELETE' });
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

export async function listKnowledge(options?: { status?: KnowledgeStatus; type?: KnowledgeType }): Promise<KnowledgeEntry[]> {
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
		body: JSON.stringify(entry)
	});
}

export async function getKnowledge(id: string): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>(`/knowledge/${id}`);
}

export async function approveKnowledge(id: string, approvedBy?: string): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>(`/knowledge/${id}/approve`, {
		method: 'POST',
		body: JSON.stringify({ approved_by: approvedBy })
	});
}

export async function approveAllKnowledge(approvedBy?: string): Promise<{ approved_count: number }> {
	return fetchJSON<{ approved_count: number }>('/knowledge/approve-all', {
		method: 'POST',
		body: JSON.stringify({ approved_by: approvedBy })
	});
}

export async function validateKnowledge(id: string, validatedBy?: string): Promise<KnowledgeEntry> {
	return fetchJSON<KnowledgeEntry>(`/knowledge/${id}/validate`, {
		method: 'POST',
		body: JSON.stringify({ validated_by: validatedBy })
	});
}

export async function rejectKnowledge(id: string, reason?: string): Promise<void> {
	const res = await fetch(`${API_BASE}/knowledge/${id}/reject`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ reason })
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
