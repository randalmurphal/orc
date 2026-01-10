import type { Task, Plan, TaskState } from './types';

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

// Hooks
export type HookType = 'pre:tool' | 'post:tool' | 'pre:command' | 'post:command' | 'prompt:submit';

export interface HookInfo {
	name: string;
	type: HookType;
	pattern?: string;
	disabled: boolean;
}

export interface Hook {
	name: string;
	type: HookType;
	pattern?: string;
	command: string;
	timeout?: number;
	disabled?: boolean;
}

export async function listHooks(): Promise<HookInfo[]> {
	return fetchJSON<HookInfo[]>('/hooks');
}

export async function getHookTypes(): Promise<HookType[]> {
	return fetchJSON<HookType[]>('/hooks/types');
}

export async function getHook(name: string): Promise<Hook> {
	return fetchJSON<Hook>(`/hooks/${name}`);
}

export async function createHook(hook: Hook): Promise<Hook> {
	return fetchJSON<Hook>('/hooks', {
		method: 'POST',
		body: JSON.stringify(hook)
	});
}

export async function updateHook(name: string, hook: Hook): Promise<Hook> {
	return fetchJSON<Hook>(`/hooks/${name}`, {
		method: 'PUT',
		body: JSON.stringify(hook)
	});
}

export async function deleteHook(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/hooks/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// Skills
export interface SkillInfo {
	name: string;
	description: string;
}

export interface Skill {
	name: string;
	description: string;
	prompt: string;
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
