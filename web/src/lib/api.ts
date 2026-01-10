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
export async function listTasks(): Promise<Task[]> {
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

// Transcripts
export interface TranscriptFile {
	filename: string;
	content: string;
	created_at: string;
}

export async function getTranscripts(id: string): Promise<TranscriptFile[]> {
	return fetchJSON<TranscriptFile[]>(`/tasks/${id}/transcripts`);
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
