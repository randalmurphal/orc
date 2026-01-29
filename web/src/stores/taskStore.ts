import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { type Task, TaskStatus, type ExecutionState } from '@/gen/orc/v1/task_pb';
import { ActivityState } from '@/gen/orc/v1/events_pb';
import { type StatusCounts } from '@/gen/orc/v1/dashboard_pb';
import { timestampToDate } from '@/lib/time';

// Active statuses for filtering (running/blocked/paused for getActiveTasks)
const ACTIVE_STATUSES: TaskStatus[] = [TaskStatus.RUNNING, TaskStatus.BLOCKED, TaskStatus.PAUSED];
const RECENT_STATUSES: TaskStatus[] = [TaskStatus.COMPLETED, TaskStatus.FAILED];
// Terminal statuses (not active anymore)
const TERMINAL_STATUSES: TaskStatus[] = [TaskStatus.COMPLETED, TaskStatus.FAILED];

// Activity state for a task (ephemeral, from event stream)
export interface TaskActivity {
	phase: string;
	activity: ActivityState;
	timestamp: number;
}

interface TaskStore {
	// State
	tasks: Task[];
	taskStates: Map<string, ExecutionState>;
	taskActivities: Map<string, TaskActivity>;
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
	updateTaskState: (taskId: string, state: ExecutionState) => void;
	removeTaskState: (taskId: string) => void;
	getTask: (taskId: string) => Task | undefined;
	getTaskState: (taskId: string) => ExecutionState | undefined;
	updateTaskActivity: (taskId: string, phase: string, activity: ActivityState) => void;
	clearTaskActivity: (taskId: string) => void;
	getTaskActivity: (taskId: string) => TaskActivity | undefined;
	setLoading: (loading: boolean) => void;
	setError: (error: string | null) => void;
	reset: () => void;
}

const initialState = {
	tasks: [] as Task[],
	taskStates: new Map<string, ExecutionState>(),
	taskActivities: new Map<string, TaskActivity>(),
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
				.sort((a, b) => {
					const dateA = timestampToDate(a.updatedAt);
					const dateB = timestampToDate(b.updatedAt);
					if (!dateA || !dateB) return 0;
					return dateB.getTime() - dateA.getTime();
				})
				.slice(0, 10);
		},

		getRunningTasks: () => {
			const { tasks } = get();
			return tasks.filter((task) => task.status === TaskStatus.RUNNING);
		},

		getStatusCounts: () => {
			const { tasks } = get();
			return tasks.reduce(
				(counts, task) => {
					counts.all++;
					// Active = not terminal (completed or failed)
					if (!TERMINAL_STATUSES.includes(task.status)) counts.active++;
					if (task.status === TaskStatus.COMPLETED) counts.completed++;
					if (task.status === TaskStatus.FAILED) counts.failed++;
					if (task.status === TaskStatus.RUNNING) counts.running++;
					if (task.status === TaskStatus.BLOCKED) counts.blocked++;
					return counts;
				},
				{ all: 0, active: 0, completed: 0, failed: 0, running: 0, blocked: 0 } as StatusCounts
			);
		},

		// Actions
		setTasks: (tasks) => {
			// Deduplicate by task ID to prevent React duplicate key warnings
			const seen = new Map<string, Task>();
			for (const task of tasks) {
				seen.set(task.id, task);
			}
			const deduplicated = seen.size === tasks.length ? tasks : Array.from(seen.values());
			set({ tasks: deduplicated, error: null });
		},

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
						? { ...task, status, ...(currentPhase !== undefined && { currentPhase }) }
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

		updateTaskActivity: (taskId, phase, activity) =>
			set((state) => {
				const newActivities = new Map(state.taskActivities);
				newActivities.set(taskId, { phase, activity, timestamp: Date.now() });
				return { taskActivities: newActivities };
			}),

		clearTaskActivity: (taskId) =>
			set((state) => {
				const newActivities = new Map(state.taskActivities);
				newActivities.delete(taskId);
				return { taskActivities: newActivities };
			}),

		getTaskActivity: (taskId) => get().taskActivities.get(taskId),

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

// Individual task activity selector
export const useTaskActivity = (taskId: string) =>
	useTaskStore((state) => state.taskActivities.get(taskId));
