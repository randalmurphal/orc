import { create } from 'zustand';
import { useShallow } from 'zustand/react/shallow';
import { subscribeWithSelector } from 'zustand/middleware';
import { create as createProto } from '@bufbuild/protobuf';
import { taskClient } from '@/lib/client';
import {
	PauseAllTasksRequestSchema,
	ResumeAllTasksRequestSchema,
} from '@/gen/orc/v1/task_pb';
import { formatNumber, formatCost } from '@/lib/format';

// Storage key for session persistence
const SESSION_ID_KEY = 'orc-session-id';
const SESSION_START_KEY = 'orc-session-start';

// Types
export interface SessionMetrics {
	totalTokens: number;
	totalCost: number;
	inputTokens: number;
	outputTokens: number;
}

export interface SessionState extends SessionMetrics {
	// Session identity
	sessionId: string | null;
	startTime: Date | null;

	// Status
	isPaused: boolean;
	activeTaskCount: number;

	// Computed values (derived from state)
	duration: string;
	formattedCost: string;
	formattedTokens: string;
}

export interface SessionActions {
	// Session lifecycle
	startSession: () => void;
	endSession: () => void;

	// Control
	pauseAll: (projectId?: string) => Promise<void>;
	resumeAll: (projectId?: string) => Promise<void>;

	// Updates
	updateMetrics: (metrics: Partial<SessionMetrics>) => void;
	addTokens: (input: number, output: number, cost: number) => void;
	updateFromMetricsEvent: (data: {
		durationSeconds: number | bigint;
		totalTokens: number;
		estimatedCostUsd: number;
		inputTokens: number;
		outputTokens: number;
		tasksRunning: number;
		isPaused: boolean;
	}) => void;

	// Task tracking
	incrementActiveTask: () => void;
	decrementActiveTask: () => void;
	setActiveTaskCount: (count: number) => void;

	// Computed getters
	getFormattedDuration: () => string;

	// Reset
	reset: () => void;
}

export type SessionStore = SessionState & SessionActions;

// Formatting utilities

/**
 * Format duration from start time to now
 * @returns "2h 34m" or "45m" or "5s"
 */
export function formatDuration(startTime: Date | null): string {
	if (!startTime) return '0m';

	const now = new Date();
	const diffMs = now.getTime() - startTime.getTime();

	if (diffMs < 0) return '0m';

	const diffSeconds = Math.floor(diffMs / 1000);
	const diffMinutes = Math.floor(diffSeconds / 60);
	const diffHours = Math.floor(diffMinutes / 60);

	if (diffHours > 0) {
		const remainingMinutes = diffMinutes % 60;
		return `${diffHours}h ${remainingMinutes}m`;
	}

	if (diffMinutes > 0) {
		return `${diffMinutes}m`;
	}

	return `${diffSeconds}s`;
}

// localStorage helpers

function getStoredSessionId(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		return localStorage.getItem(SESSION_ID_KEY);
	} catch {
		return null;
	}
}

function setStoredSessionId(sessionId: string | null): void {
	if (typeof window === 'undefined') return;
	try {
		if (sessionId) {
			localStorage.setItem(SESSION_ID_KEY, sessionId);
		} else {
			localStorage.removeItem(SESSION_ID_KEY);
		}
	} catch {
		// Ignore localStorage errors
	}
}

function getStoredStartTime(): Date | null {
	if (typeof window === 'undefined') return null;
	try {
		const stored = localStorage.getItem(SESSION_START_KEY);
		if (stored) {
			const date = new Date(stored);
			// Validate the date is reasonable (not in the future, not too old)
			if (!isNaN(date.getTime())) {
				return date;
			}
		}
		return null;
	} catch {
		return null;
	}
}

function setStoredStartTime(startTime: Date | null): void {
	if (typeof window === 'undefined') return;
	try {
		if (startTime) {
			localStorage.setItem(SESSION_START_KEY, startTime.toISOString());
		} else {
			localStorage.removeItem(SESSION_START_KEY);
		}
	} catch {
		// Ignore localStorage errors
	}
}

function generateSessionId(): string {
	return `session-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

// Initial state
const initialState: SessionState = {
	sessionId: null,
	startTime: null,
	totalTokens: 0,
	totalCost: 0,
	inputTokens: 0,
	outputTokens: 0,
	isPaused: false,
	activeTaskCount: 0,
	duration: '0m',
	formattedCost: '$0.00',
	formattedTokens: '0',
};

// Helper to compute derived state
function computeDerivedState(state: SessionState): Partial<SessionState> {
	return {
		duration: formatDuration(state.startTime),
		formattedCost: formatCost(state.totalCost),
		formattedTokens: formatNumber(state.totalTokens),
	};
}

export const useSessionStore = create<SessionStore>()(
	subscribeWithSelector((set, get) => {
		// Restore session from localStorage on init
		const storedSessionId = getStoredSessionId();
		const storedStartTime = getStoredStartTime();

		const restoredState: Partial<SessionState> = {};
		if (storedSessionId && storedStartTime) {
			restoredState.sessionId = storedSessionId;
			restoredState.startTime = storedStartTime;
			restoredState.duration = formatDuration(storedStartTime);
		}

		return {
			...initialState,
			...restoredState,

			// Session lifecycle
			startSession: () => {
				const sessionId = generateSessionId();
				const startTime = new Date();

				setStoredSessionId(sessionId);
				setStoredStartTime(startTime);

				set({
					sessionId,
					startTime,
					duration: formatDuration(startTime),
				});
			},

			endSession: () => {
				setStoredSessionId(null);
				setStoredStartTime(null);

				set({
					sessionId: null,
					startTime: null,
					totalTokens: 0,
					totalCost: 0,
					inputTokens: 0,
					outputTokens: 0,
					isPaused: false,
					activeTaskCount: 0,
					duration: '0m',
					formattedCost: '$0.00',
					formattedTokens: '0',
				});
			},

			// Control - pause/resume all tasks via Connect RPC
			pauseAll: async (projectId?: string) => {
				await taskClient.pauseAllTasks(createProto(PauseAllTasksRequestSchema, { projectId: projectId ?? '' }));
				set({ isPaused: true });
			},

			resumeAll: async (projectId?: string) => {
				await taskClient.resumeAllTasks(createProto(ResumeAllTasksRequestSchema, { projectId: projectId ?? '' }));
				set({ isPaused: false });
			},

			// Updates
			updateMetrics: (metrics: Partial<SessionMetrics>) => {
				set((state) => {
					const newState = {
						...state,
						...metrics,
						totalTokens:
							metrics.totalTokens !== undefined
								? metrics.totalTokens
								: (metrics.inputTokens ?? state.inputTokens) +
									(metrics.outputTokens ?? state.outputTokens),
					};
					return {
						...newState,
						...computeDerivedState(newState),
					};
				});
			},

			addTokens: (input: number, output: number, cost: number) => {
				set((state) => {
					const newInputTokens = state.inputTokens + input;
					const newOutputTokens = state.outputTokens + output;
					const newTotalTokens = newInputTokens + newOutputTokens;
					const newTotalCost = state.totalCost + cost;

					return {
						inputTokens: newInputTokens,
						outputTokens: newOutputTokens,
						totalTokens: newTotalTokens,
						totalCost: newTotalCost,
						formattedCost: formatCost(newTotalCost),
						formattedTokens: formatNumber(newTotalTokens),
					};
				});
			},

			updateFromMetricsEvent: (data) => {
				set((state) => {
					// Compute startTime from durationSeconds if no session exists
					let newStartTime = state.startTime;
					const durationSeconds = Number(data.durationSeconds);
					if (!newStartTime && durationSeconds > 0) {
						const now = new Date();
						newStartTime = new Date(now.getTime() - durationSeconds * 1000);
					}

					const newState = {
						...state,
						startTime: newStartTime,
						totalTokens: data.totalTokens,
						totalCost: data.estimatedCostUsd,
						inputTokens: data.inputTokens,
						outputTokens: data.outputTokens,
						activeTaskCount: data.tasksRunning,
						isPaused: data.isPaused,
					};

					return {
						...newState,
						...computeDerivedState(newState),
					};
				});
			},

			// Task tracking
			incrementActiveTask: () => {
				set((state) => ({
					activeTaskCount: state.activeTaskCount + 1,
				}));
			},

			decrementActiveTask: () => {
				set((state) => ({
					activeTaskCount: Math.max(0, state.activeTaskCount - 1),
				}));
			},

			setActiveTaskCount: (count: number) => {
				set({ activeTaskCount: count });
			},

			// Computed getters
			getFormattedDuration: () => {
				return formatDuration(get().startTime);
			},

			// Reset
			reset: () => {
				setStoredSessionId(null);
				setStoredStartTime(null);
				set(initialState);
			},
		};
	})
);

// Selector hooks
export const useSessionId = () => useSessionStore((state) => state.sessionId);
export const useStartTime = () => useSessionStore((state) => state.startTime);
export const useTotalTokens = () => useSessionStore((state) => state.totalTokens);
export const useTotalCost = () => useSessionStore((state) => state.totalCost);
export const useIsPaused = () => useSessionStore((state) => state.isPaused);
export const useActiveTaskCount = () => useSessionStore((state) => state.activeTaskCount);
export const useFormattedDuration = () => useSessionStore((state) => state.duration);
export const useFormattedCost = () => useSessionStore((state) => state.formattedCost);
export const useFormattedTokens = () => useSessionStore((state) => state.formattedTokens);

// Session metrics as a single object (for components that need all metrics).
// useShallow prevents re-renders when individual values haven't changed —
// without it, a new object is created every render, failing reference equality.
export const useSessionMetrics = () =>
	useSessionStore(useShallow((state) => ({
		duration: state.duration,
		formattedCost: state.formattedCost,
		formattedTokens: state.formattedTokens,
		totalTokens: state.totalTokens,
		totalCost: state.totalCost,
		inputTokens: state.inputTokens,
		outputTokens: state.outputTokens,
	})));

// Export storage keys for testing
export const STORAGE_KEYS = {
	SESSION_ID: SESSION_ID_KEY,
	SESSION_START: SESSION_START_KEY,
} as const;
