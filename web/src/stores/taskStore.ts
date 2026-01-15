import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Task, TaskState, TaskStatus, StatusCounts } from '@/lib/types';

// Active statuses for filtering (running/blocked/paused for getActiveTasks)
const ACTIVE_STATUSES: TaskStatus[] = ['running', 'blocked', 'paused'];
const RECENT_STATUSES: TaskStatus[] = ['completed', 'failed', 'finished'];
// Terminal statuses (not active anymore)
const TERMINAL_STATUSES: TaskStatus[] = ['completed', 'failed', 'finished'];

interface TaskStore {
	// State
	tasks: Task[];
	taskStates: Map<string, TaskState>;
	loading: boolean;
	error: string | null;

	// Derived state (computed on access)
	getActiveTasks: () => Task[];
	getRecentTasks: () => Task[];
	getRunningTasks: () => Task[];
	getStatusCounts: () => StatusCounts;

	// Actions
	setTasks: (tasks: Task[]) => void;
	addTask: (task: Task) => void;
	updateTask: (taskId: string, updates: Partial<Task>) => void;
	updateTaskStatus: (taskId: string, status: TaskStatus, currentPhase?: string) => void;
	removeTask: (taskId: string) => void;
	updateTaskState: (taskId: string, state: TaskState) => void;
	removeTaskState: (taskId: string) => void;
	getTask: (taskId: string) => Task | undefined;
	getTaskState: (taskId: string) => TaskState | undefined;
	setLoading: (loading: boolean) => void;
	setError: (error: string | null) => void;
	reset: () => void;
}

const initialState = {
	tasks: [] as Task[],
	taskStates: new Map<string, TaskState>(),
	loading: false,
	error: null as string | null,
};

export const useTaskStore = create<TaskStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		// Derived state - computed on each access
		getActiveTasks: () => {
			const { tasks } = get();
			return tasks.filter((task) => ACTIVE_STATUSES.includes(task.status));
		},

		getRecentTasks: () => {
			const { tasks } = get();
			return tasks
				.filter((task) => RECENT_STATUSES.includes(task.status))
				.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
				.slice(0, 10);
		},

		getRunningTasks: () => {
			const { tasks } = get();
			return tasks.filter((task) => task.status === 'running');
		},

		getStatusCounts: () => {
			const { tasks } = get();
			return tasks.reduce(
				(counts, task) => {
					counts.all++;
					// Active = not terminal (matches Svelte: !['completed', 'failed'].includes)
					// Note: Svelte missed 'finished', we include it in terminal
					if (!TERMINAL_STATUSES.includes(task.status)) counts.active++;
					if (task.status === 'completed' || task.status === 'finished') counts.completed++;
					if (task.status === 'failed') counts.failed++;
					if (task.status === 'running') counts.running++;
					if (task.status === 'blocked') counts.blocked++;
					return counts;
				},
				{ all: 0, active: 0, completed: 0, failed: 0, running: 0, blocked: 0 } as StatusCounts
			);
		},

		// Actions
		setTasks: (tasks) => set({ tasks, error: null }),

		addTask: (task) =>
			set((state) => {
				// Prevent duplicates
				if (state.tasks.some((t) => t.id === task.id)) {
					return state;
				}
				return { tasks: [...state.tasks, task] };
			}),

		updateTask: (taskId, updates) =>
			set((state) => ({
				tasks: state.tasks.map((task) =>
					task.id === taskId ? { ...task, ...updates } : task
				),
			})),

		updateTaskStatus: (taskId, status, currentPhase) =>
			set((state) => ({
				tasks: state.tasks.map((task) =>
					task.id === taskId
						? { ...task, status, ...(currentPhase !== undefined && { current_phase: currentPhase }) }
						: task
				),
			})),

		removeTask: (taskId) =>
			set((state) => ({
				tasks: state.tasks.filter((task) => task.id !== taskId),
				taskStates: (() => {
					const newStates = new Map(state.taskStates);
					newStates.delete(taskId);
					return newStates;
				})(),
			})),

		updateTaskState: (taskId, taskState) =>
			set((state) => {
				const newStates = new Map(state.taskStates);
				newStates.set(taskId, taskState);

				// Sync status to task if task exists
				const taskIndex = state.tasks.findIndex((t) => t.id === taskId);
				if (taskIndex !== -1 && taskState.status) {
					const updatedTasks = [...state.tasks];
					updatedTasks[taskIndex] = {
						...updatedTasks[taskIndex],
						status: taskState.status as TaskStatus,
						current_phase: taskState.current_phase,
					};
					return { taskStates: newStates, tasks: updatedTasks };
				}

				return { taskStates: newStates };
			}),

		removeTaskState: (taskId) =>
			set((state) => {
				const newStates = new Map(state.taskStates);
				newStates.delete(taskId);
				return { taskStates: newStates };
			}),

		getTask: (taskId) => get().tasks.find((task) => task.id === taskId),

		getTaskState: (taskId) => get().taskStates.get(taskId),

		setLoading: (loading) => set({ loading }),

		setError: (error) => set({ error }),

		reset: () => set(initialState),
	}))
);

// Selector hooks for derived state (memoized via subscribeWithSelector)
export const useActiveTasks = () => useTaskStore((state) => state.getActiveTasks());
export const useRecentTasks = () => useTaskStore((state) => state.getRecentTasks());
export const useRunningTasks = () => useTaskStore((state) => state.getRunningTasks());
export const useStatusCounts = () => useTaskStore((state) => state.getStatusCounts());

// Individual task selector
export const useTask = (taskId: string) =>
	useTaskStore((state) => state.tasks.find((t) => t.id === taskId));

// Individual task state selector
export const useTaskState = (taskId: string) =>
	useTaskStore((state) => state.taskStates.get(taskId));
