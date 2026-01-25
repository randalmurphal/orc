import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { pauseAllTasks, resumeAllTasks } from '@/lib/api';
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

// WebSocket session_update event data
export interface SessionUpdateData {
	duration_seconds: number;
	total_tokens: number;
	estimated_cost_usd: number;
	input_tokens: number;
	output_tokens: number;
	tasks_running: number;
	is_paused: boolean;
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
	pauseAll: () => Promise<void>;
	resumeAll: () => Promise<void>;

	// Updates
	updateMetrics: (metrics: Partial<SessionMetrics>) => void;
	addTokens: (input: number, output: number, cost: number) => void;
	updateFromSessionEvent: (data: SessionUpdateData) => void;

	// Task tracking
	incrementActiveTask: () => void;
	decrementActiveTask: () => void;

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

// Format utilities imported from @/lib/format

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

			// Control
			pauseAll: async () => {
				await pauseAllTasks();
				set({ isPaused: true });
			},

			resumeAll: async () => {
				await resumeAllTasks();
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

			updateFromSessionEvent: (data: SessionUpdateData) => {
				set((state) => {
					// Compute startTime from duration_seconds if no session exists
					let newStartTime = state.startTime;
					if (!newStartTime) {
						const now = new Date();
						newStartTime = new Date(now.getTime() - data.duration_seconds * 1000);
					}
					// If startTime exists, preserve it (local time reference wins)

					const newState = {
						...state,
						startTime: newStartTime,
						totalTokens: data.total_tokens,
						totalCost: data.estimated_cost_usd,
						inputTokens: data.input_tokens,
						outputTokens: data.output_tokens,
						activeTaskCount: data.tasks_running,
						isPaused: data.is_paused,
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

// Session metrics as a single object (for components that need all metrics)
export const useSessionMetrics = () =>
	useSessionStore((state) => ({
		duration: state.duration,
		formattedCost: state.formattedCost,
		formattedTokens: state.formattedTokens,
		totalTokens: state.totalTokens,
		totalCost: state.totalCost,
		inputTokens: state.inputTokens,
		outputTokens: state.outputTokens,
	}));

// Export storage keys for testing
export const STORAGE_KEYS = {
	SESSION_ID: SESSION_ID_KEY,
	SESSION_START: SESSION_START_KEY,
} as const;
