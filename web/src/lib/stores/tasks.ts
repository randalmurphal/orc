import { writable, derived, get, type Readable } from 'svelte/store';
import type { Task, TaskState } from '$lib/types';
import { listProjectTasks, getProjectTask, getProjectTaskState } from '$lib/api';
import { currentProjectId } from './project';

// Store for all tasks
export const tasks = writable<Task[]>([]);

// Store for task states (keyed by task ID)
export const taskStates = writable<Map<string, TaskState>>(new Map());

// Loading and error states
export const tasksLoading = writable<boolean>(false);
export const tasksError = writable<string | null>(null);

// Derived stores for filtered views
export const activeTasks: Readable<Task[]> = derived(
	tasks,
	($tasks) => $tasks.filter(t => ['running', 'blocked', 'paused'].includes(t.status))
);

export const recentTasks: Readable<Task[]> = derived(
	tasks,
	($tasks) => $tasks
		.filter(t => ['completed', 'failed'].includes(t.status))
		.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
		.slice(0, 10)
);

export const runningTasks: Readable<Task[]> = derived(
	tasks,
	($tasks) => $tasks.filter(t => t.status === 'running')
);

// Status counts
export const statusCounts: Readable<{
	all: number;
	active: number;
	completed: number;
	failed: number;
	running: number;
	blocked: number;
}> = derived(
	tasks,
	($tasks) => ({
		all: $tasks.length,
		active: $tasks.filter(t => !['completed', 'failed'].includes(t.status)).length,
		completed: $tasks.filter(t => t.status === 'completed').length,
		failed: $tasks.filter(t => t.status === 'failed').length,
		running: $tasks.filter(t => t.status === 'running').length,
		blocked: $tasks.filter(t => t.status === 'blocked').length
	})
);

// Load tasks from API
export async function loadTasks(): Promise<void> {
	tasksLoading.set(true);
	tasksError.set(null);

	try {
		const projectId = get(currentProjectId);

		// No project selected - return empty list
		// Tasks are always project-scoped, so we need a project context
		if (!projectId) {
			tasks.set([]);
			return;
		}

		const loaded = await listProjectTasks(projectId);
		tasks.set(loaded);
	} catch (e) {
		const errorMsg = e instanceof Error ? e.message : 'Failed to load tasks';
		tasksError.set(errorMsg);
		console.error('Failed to load tasks:', e);
	} finally {
		tasksLoading.set(false);
	}
}

// Track pending task fetches to avoid duplicate requests
const pendingFetches = new Set<string>();

// Update a single task in the store (from WebSocket event)
export function updateTask(taskId: string, updates: Partial<Task>): void {
	let taskFound = false;
	tasks.update(current => {
		const idx = current.findIndex(t => t.id === taskId);
		if (idx >= 0) {
			// Update existing task (create new object to avoid mutation issues)
			taskFound = true;
			return current.map((t, i) => i === idx ? { ...t, ...updates } : t);
		}
		return current;
	});

	// Task not found - fetch it from API (if not already fetching)
	if (!taskFound && !pendingFetches.has(taskId)) {
		pendingFetches.add(taskId);
		refreshTask(taskId).finally(() => {
			pendingFetches.delete(taskId);
		});
	}
}

// Update task status (common operation from WebSocket)
export function updateTaskStatus(taskId: string, status: Task['status'], currentPhase?: string): void {
	let taskFound = false;
	tasks.update(current => {
		const idx = current.findIndex(t => t.id === taskId);
		if (idx >= 0) {
			taskFound = true;
			return current.map((t, i) => i === idx ? {
				...t,
				status,
				current_phase: currentPhase ?? t.current_phase,
				updated_at: new Date().toISOString()
			} : t);
		}
		return current;
	});

	// Task not found - fetch it from API (if not already fetching)
	if (!taskFound && !pendingFetches.has(taskId)) {
		pendingFetches.add(taskId);
		refreshTask(taskId).finally(() => {
			pendingFetches.delete(taskId);
		});
	}
}

// Update task state (from WebSocket state event)
export function updateTaskState(taskId: string, state: TaskState): void {
	taskStates.update(current => {
		const newMap = new Map(current);
		newMap.set(taskId, state);
		return newMap;
	});

	// Also update the task status if we have state info
	if (state.status) {
		updateTaskStatus(taskId, state.status as Task['status'], state.current_phase);
	}
}

// Remove a task from the store
export function removeTask(taskId: string): void {
	tasks.update(current => current.filter(t => t.id !== taskId));
	taskStates.update(current => {
		const newMap = new Map(current);
		newMap.delete(taskId);
		return newMap;
	});
}

// Add a new task to the store
export function addTask(task: Task): void {
	tasks.update(current => {
		// Check if task already exists
		if (current.some(t => t.id === task.id)) {
			return current;
		}
		return [task, ...current];
	});
}

// Fetch and update a single task from the API
export async function refreshTask(taskId: string): Promise<Task | null> {
	try {
		const projectId = get(currentProjectId);

		// No project selected - can't refresh task
		if (!projectId) {
			return null;
		}

		const task = await getProjectTask(projectId, taskId);
		updateTask(taskId, task);
		return task;
	} catch (e) {
		console.error('Failed to refresh task:', taskId, e);
		return null;
	}
}

// Fetch and update task state from the API
export async function refreshTaskState(taskId: string): Promise<TaskState | null> {
	try {
		const projectId = get(currentProjectId);

		// No project selected - can't refresh task state
		if (!projectId) {
			return null;
		}

		const state = await getProjectTaskState(projectId, taskId);
		updateTaskState(taskId, state);
		return state;
	} catch (e) {
		console.error('Failed to refresh task state:', taskId, e);
		return null;
	}
}

// Get a task by ID from the store
export function getTaskFromStore(taskId: string): Task | undefined {
	return get(tasks).find(t => t.id === taskId);
}

// Get task state by ID from the store
export function getTaskStateFromStore(taskId: string): TaskState | undefined {
	return get(taskStates).get(taskId);
}
