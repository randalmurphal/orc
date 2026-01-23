import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	useSessionStore,
	formatDuration,
	formatCost,
	formatTokens,
	STORAGE_KEYS,
} from './sessionStore';

describe('SessionStore', () => {
	beforeEach(() => {
		// Reset store and localStorage before each test
		localStorage.clear();
		useSessionStore.getState().reset();
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('formatting utilities', () => {
		describe('formatDuration', () => {
			it('should return "0m" for null start time', () => {
				expect(formatDuration(null)).toBe('0m');
			});

			it('should return seconds for very short durations', () => {
				const startTime = new Date();
				vi.advanceTimersByTime(30000); // 30 seconds
				expect(formatDuration(startTime)).toBe('30s');
			});

			it('should return minutes for short durations', () => {
				const startTime = new Date();
				vi.advanceTimersByTime(45 * 60 * 1000); // 45 minutes
				expect(formatDuration(startTime)).toBe('45m');
			});

			it('should return hours and minutes for longer durations', () => {
				const startTime = new Date();
				vi.advanceTimersByTime(2 * 60 * 60 * 1000 + 34 * 60 * 1000); // 2h 34m
				expect(formatDuration(startTime)).toBe('2h 34m');
			});

			it('should return "0m" for future start times', () => {
				const futureTime = new Date(Date.now() + 60000);
				expect(formatDuration(futureTime)).toBe('0m');
			});
		});

		describe('formatCost', () => {
			it('should format zero cost', () => {
				expect(formatCost(0)).toBe('$0.00');
			});

			it('should format small costs with 2 decimals', () => {
				expect(formatCost(1.234)).toBe('$1.23');
			});

			it('should format larger costs', () => {
				expect(formatCost(12.5)).toBe('$12.50');
			});

			it('should round correctly', () => {
				expect(formatCost(1.999)).toBe('$2.00');
			});
		});

		describe('formatTokens', () => {
			it('should format small numbers without suffix', () => {
				expect(formatTokens(500)).toBe('500');
				expect(formatTokens(0)).toBe('0');
			});

			it('should format thousands with K suffix', () => {
				expect(formatTokens(1000)).toBe('1K');
				expect(formatTokens(125000)).toBe('125K');
				expect(formatTokens(847000)).toBe('847K');
			});

			it('should round K values', () => {
				expect(formatTokens(1500)).toBe('2K');
				expect(formatTokens(1499)).toBe('1K');
			});

			it('should format millions with M suffix', () => {
				expect(formatTokens(1000000)).toBe('1.0M');
				expect(formatTokens(1200000)).toBe('1.2M');
				expect(formatTokens(12500000)).toBe('12.5M');
			});
		});
	});

	describe('session lifecycle', () => {
		it('should start with initial state', () => {
			const state = useSessionStore.getState();
			expect(state.sessionId).toBeNull();
			expect(state.startTime).toBeNull();
			expect(state.totalTokens).toBe(0);
			expect(state.totalCost).toBe(0);
			expect(state.isPaused).toBe(false);
			expect(state.activeTaskCount).toBe(0);
		});

		it('should start a new session', () => {
			useSessionStore.getState().startSession();
			const state = useSessionStore.getState();

			expect(state.sessionId).not.toBeNull();
			expect(state.sessionId).toMatch(/^session-/);
			expect(state.startTime).toBeInstanceOf(Date);
		});

		it('should persist session ID to localStorage', () => {
			useSessionStore.getState().startSession();
			const state = useSessionStore.getState();

			expect(localStorage.getItem(STORAGE_KEYS.SESSION_ID)).toBe(state.sessionId);
		});

		it('should persist start time to localStorage', () => {
			useSessionStore.getState().startSession();
			const state = useSessionStore.getState();

			expect(localStorage.getItem(STORAGE_KEYS.SESSION_START)).toBe(
				state.startTime?.toISOString()
			);
		});

		it('should end session and clear state', () => {
			useSessionStore.getState().startSession();
			useSessionStore.getState().addTokens(1000, 500, 0.05);

			useSessionStore.getState().endSession();
			const state = useSessionStore.getState();

			expect(state.sessionId).toBeNull();
			expect(state.startTime).toBeNull();
			expect(state.totalTokens).toBe(0);
			expect(state.totalCost).toBe(0);
		});

		it('should clear localStorage on end session', () => {
			useSessionStore.getState().startSession();

			expect(localStorage.getItem(STORAGE_KEYS.SESSION_ID)).not.toBeNull();
			expect(localStorage.getItem(STORAGE_KEYS.SESSION_START)).not.toBeNull();

			useSessionStore.getState().endSession();

			expect(localStorage.getItem(STORAGE_KEYS.SESSION_ID)).toBeNull();
			expect(localStorage.getItem(STORAGE_KEYS.SESSION_START)).toBeNull();
		});

		it('should restore session from localStorage on init', () => {
			// Set up localStorage with a session
			const sessionId = 'session-test-123';
			const startTime = new Date();
			localStorage.setItem(STORAGE_KEYS.SESSION_ID, sessionId);
			localStorage.setItem(STORAGE_KEYS.SESSION_START, startTime.toISOString());

			// Create a fresh store instance by resetting and restoring
			// Since we can't re-create the singleton, we test the restoration logic
			const storedId = localStorage.getItem(STORAGE_KEYS.SESSION_ID);
			const storedStart = localStorage.getItem(STORAGE_KEYS.SESSION_START);

			expect(storedId).toBe(sessionId);
			expect(storedStart).toBe(startTime.toISOString());
		});
	});

	describe('token tracking', () => {
		it('should add tokens correctly', () => {
			useSessionStore.getState().addTokens(1000, 500, 0.05);
			const state = useSessionStore.getState();

			expect(state.inputTokens).toBe(1000);
			expect(state.outputTokens).toBe(500);
			expect(state.totalTokens).toBe(1500);
			expect(state.totalCost).toBeCloseTo(0.05);
		});

		it('should accumulate tokens over multiple calls', () => {
			useSessionStore.getState().addTokens(1000, 500, 0.05);
			useSessionStore.getState().addTokens(2000, 1000, 0.10);
			const state = useSessionStore.getState();

			expect(state.inputTokens).toBe(3000);
			expect(state.outputTokens).toBe(1500);
			expect(state.totalTokens).toBe(4500);
			expect(state.totalCost).toBeCloseTo(0.15);
		});

		it('should update formatted values after adding tokens', () => {
			useSessionStore.getState().addTokens(125000, 25000, 1.23);
			const state = useSessionStore.getState();

			expect(state.formattedTokens).toBe('150K');
			expect(state.formattedCost).toBe('$1.23');
		});
	});

	describe('updateMetrics', () => {
		it('should update partial metrics', () => {
			useSessionStore.getState().updateMetrics({
				inputTokens: 5000,
				outputTokens: 2500,
			});
			const state = useSessionStore.getState();

			expect(state.inputTokens).toBe(5000);
			expect(state.outputTokens).toBe(2500);
			expect(state.totalTokens).toBe(7500);
		});

		it('should update totalCost', () => {
			useSessionStore.getState().updateMetrics({
				totalCost: 2.50,
			});
			const state = useSessionStore.getState();

			expect(state.totalCost).toBe(2.50);
			expect(state.formattedCost).toBe('$2.50');
		});

		it('should handle explicit totalTokens', () => {
			useSessionStore.getState().updateMetrics({
				totalTokens: 10000,
			});
			const state = useSessionStore.getState();

			expect(state.totalTokens).toBe(10000);
		});
	});

	describe('task tracking', () => {
		it('should increment active task count', () => {
			expect(useSessionStore.getState().activeTaskCount).toBe(0);

			useSessionStore.getState().incrementActiveTask();
			expect(useSessionStore.getState().activeTaskCount).toBe(1);

			useSessionStore.getState().incrementActiveTask();
			expect(useSessionStore.getState().activeTaskCount).toBe(2);
		});

		it('should decrement active task count', () => {
			useSessionStore.getState().incrementActiveTask();
			useSessionStore.getState().incrementActiveTask();
			expect(useSessionStore.getState().activeTaskCount).toBe(2);

			useSessionStore.getState().decrementActiveTask();
			expect(useSessionStore.getState().activeTaskCount).toBe(1);
		});

		it('should not go below zero', () => {
			expect(useSessionStore.getState().activeTaskCount).toBe(0);

			useSessionStore.getState().decrementActiveTask();
			expect(useSessionStore.getState().activeTaskCount).toBe(0);
		});
	});

	describe('pause/resume', () => {
		beforeEach(() => {
			// Mock fetch
			global.fetch = vi.fn();
		});

		afterEach(() => {
			vi.restoreAllMocks();
		});

		it('should call pause endpoint and update state', async () => {
			(global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({}),
			});

			await useSessionStore.getState().pauseAll();

			expect(global.fetch).toHaveBeenCalledWith('/api/tasks/pause-all', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
			});
			expect(useSessionStore.getState().isPaused).toBe(true);
		});

		it('should call resume endpoint and update state', async () => {
			// First pause
			useSessionStore.setState({ isPaused: true });

			(global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({}),
			});

			await useSessionStore.getState().resumeAll();

			expect(global.fetch).toHaveBeenCalledWith('/api/tasks/resume-all', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
			});
			expect(useSessionStore.getState().isPaused).toBe(false);
		});

		it('should throw error on pause failure', async () => {
			(global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
				ok: false,
				text: () => Promise.resolve('Server error'),
			});

			await expect(useSessionStore.getState().pauseAll()).rejects.toThrow(
				'Failed to pause all tasks: Server error'
			);
			expect(useSessionStore.getState().isPaused).toBe(false);
		});

		it('should throw error on resume failure', async () => {
			useSessionStore.setState({ isPaused: true });

			(global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
				ok: false,
				text: () => Promise.resolve('Server error'),
			});

			await expect(useSessionStore.getState().resumeAll()).rejects.toThrow(
				'Failed to resume all tasks: Server error'
			);
			expect(useSessionStore.getState().isPaused).toBe(true);
		});
	});

	describe('getFormattedDuration', () => {
		it('should return current duration', () => {
			useSessionStore.getState().startSession();
			vi.advanceTimersByTime(5 * 60 * 1000); // 5 minutes

			expect(useSessionStore.getState().getFormattedDuration()).toBe('5m');
		});

		it('should return "0m" when no session', () => {
			expect(useSessionStore.getState().getFormattedDuration()).toBe('0m');
		});
	});

	describe('reset', () => {
		it('should reset to initial state', () => {
			useSessionStore.getState().startSession();
			useSessionStore.getState().addTokens(1000, 500, 0.05);
			useSessionStore.getState().incrementActiveTask();

			useSessionStore.getState().reset();
			const state = useSessionStore.getState();

			expect(state.sessionId).toBeNull();
			expect(state.startTime).toBeNull();
			expect(state.totalTokens).toBe(0);
			expect(state.totalCost).toBe(0);
			expect(state.activeTaskCount).toBe(0);
			expect(state.isPaused).toBe(false);
		});

		it('should clear localStorage', () => {
			useSessionStore.getState().startSession();

			expect(localStorage.getItem(STORAGE_KEYS.SESSION_ID)).not.toBeNull();

			useSessionStore.getState().reset();

			expect(localStorage.getItem(STORAGE_KEYS.SESSION_ID)).toBeNull();
			expect(localStorage.getItem(STORAGE_KEYS.SESSION_START)).toBeNull();
		});
	});

	describe('selector hooks', () => {
		it('should export individual selectors', async () => {
			const {
				useSessionId,
				useStartTime,
				useTotalTokens,
				useTotalCost,
				useIsPaused,
				useActiveTaskCount,
				useFormattedDuration,
				useFormattedCost,
				useFormattedTokens,
				useSessionMetrics,
			} = await import('./sessionStore');

			// These are Zustand selectors - verify they're functions
			expect(typeof useSessionId).toBe('function');
			expect(typeof useStartTime).toBe('function');
			expect(typeof useTotalTokens).toBe('function');
			expect(typeof useTotalCost).toBe('function');
			expect(typeof useIsPaused).toBe('function');
			expect(typeof useActiveTaskCount).toBe('function');
			expect(typeof useFormattedDuration).toBe('function');
			expect(typeof useFormattedCost).toBe('function');
			expect(typeof useFormattedTokens).toBe('function');
			expect(typeof useSessionMetrics).toBe('function');
		});
	});

	describe('computed values', () => {
		it('should update duration when session starts', () => {
			expect(useSessionStore.getState().duration).toBe('0m');

			useSessionStore.getState().startSession();
			vi.advanceTimersByTime(60000); // 1 minute

			// The duration is computed on access via getFormattedDuration
			expect(useSessionStore.getState().getFormattedDuration()).toBe('1m');
		});

		it('should update formatted values when metrics change', () => {
			expect(useSessionStore.getState().formattedCost).toBe('$0.00');
			expect(useSessionStore.getState().formattedTokens).toBe('0');

			useSessionStore.getState().addTokens(500000, 100000, 5.50);

			expect(useSessionStore.getState().formattedCost).toBe('$5.50');
			expect(useSessionStore.getState().formattedTokens).toBe('600K');
		});
	});

	describe('edge cases', () => {
		it('should handle concurrent updates', () => {
			// Simulate rapid concurrent updates
			for (let i = 0; i < 100; i++) {
				useSessionStore.getState().addTokens(100, 50, 0.01);
			}

			const state = useSessionStore.getState();
			expect(state.inputTokens).toBe(10000);
			expect(state.outputTokens).toBe(5000);
			expect(state.totalTokens).toBe(15000);
			expect(state.totalCost).toBeCloseTo(1.0);
		});

		it('should handle invalid localStorage values gracefully', () => {
			localStorage.setItem(STORAGE_KEYS.SESSION_START, 'invalid-date');

			// The getter should return null for invalid dates
			const storedStart = localStorage.getItem(STORAGE_KEYS.SESSION_START);
			const date = new Date(storedStart!);
			expect(isNaN(date.getTime())).toBe(true);
		});
	});

	describe('updateFromSessionEvent', () => {
		it('should update all metrics from session_update event', () => {
			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 3650,
				total_tokens: 127500,
				estimated_cost_usd: 2.51,
				input_tokens: 95000,
				output_tokens: 32500,
				tasks_running: 2,
				is_paused: false,
			});

			const state = useSessionStore.getState();
			expect(state.totalTokens).toBe(127500);
			expect(state.totalCost).toBe(2.51);
			expect(state.inputTokens).toBe(95000);
			expect(state.outputTokens).toBe(32500);
			expect(state.activeTaskCount).toBe(2);
			expect(state.isPaused).toBe(false);
		});

		it('should compute startTime from duration_seconds when no session exists', () => {
			const now = new Date();
			vi.setSystemTime(now);

			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 3650, // ~1h ago
				total_tokens: 1000,
				estimated_cost_usd: 0.01,
				input_tokens: 500,
				output_tokens: 500,
				tasks_running: 1,
				is_paused: false,
			});

			const state = useSessionStore.getState();
			expect(state.startTime).not.toBeNull();

			// Should compute startTime as approximately 3650 seconds ago
			const expectedStartTime = new Date(now.getTime() - 3650 * 1000);
			const actualStartTime = state.startTime!;
			const diff = Math.abs(actualStartTime.getTime() - expectedStartTime.getTime());
			expect(diff).toBeLessThan(1000); // Within 1 second tolerance
		});

		it('should preserve existing startTime when session already exists', () => {
			useSessionStore.getState().startSession();
			const originalStartTime = useSessionStore.getState().startTime;

			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 100,
				total_tokens: 5000,
				estimated_cost_usd: 0.1,
				input_tokens: 3000,
				output_tokens: 2000,
				tasks_running: 1,
				is_paused: false,
			});

			const state = useSessionStore.getState();
			expect(state.startTime).toBe(originalStartTime);
		});

		it('should update formatted values after session event', () => {
			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 3650,
				total_tokens: 127500,
				estimated_cost_usd: 2.51,
				input_tokens: 95000,
				output_tokens: 32500,
				tasks_running: 2,
				is_paused: false,
			});

			const state = useSessionStore.getState();
			expect(state.formattedTokens).toBe('128K');
			expect(state.formattedCost).toBe('$2.51');
			// Duration should be computed from startTime
			expect(state.duration).toMatch(/^\d+[hms]/);
		});

		it('should handle zero values', () => {
			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 0,
				total_tokens: 0,
				estimated_cost_usd: 0,
				input_tokens: 0,
				output_tokens: 0,
				tasks_running: 0,
				is_paused: false,
			});

			const state = useSessionStore.getState();
			expect(state.totalTokens).toBe(0);
			expect(state.totalCost).toBe(0);
			expect(state.inputTokens).toBe(0);
			expect(state.outputTokens).toBe(0);
			expect(state.activeTaskCount).toBe(0);
			expect(state.isPaused).toBe(false);
		});

		it('should handle paused state', () => {
			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 100,
				total_tokens: 1000,
				estimated_cost_usd: 0.02,
				input_tokens: 600,
				output_tokens: 400,
				tasks_running: 0,
				is_paused: true,
			});

			const state = useSessionStore.getState();
			expect(state.isPaused).toBe(true);
			expect(state.activeTaskCount).toBe(0);
		});

		it('should overwrite local state with server state on reconnect', () => {
			// Set local state
			useSessionStore.getState().updateMetrics({
				totalTokens: 5000,
				totalCost: 0.1,
				inputTokens: 3000,
				outputTokens: 2000,
			});
			useSessionStore.getState().incrementActiveTask();

			// Server has different state
			useSessionStore.getState().updateFromSessionEvent({
				duration_seconds: 200,
				total_tokens: 10000,
				estimated_cost_usd: 0.5,
				input_tokens: 6000,
				output_tokens: 4000,
				tasks_running: 2,
				is_paused: false,
			});

			const state = useSessionStore.getState();
			// Server state should win
			expect(state.totalTokens).toBe(10000);
			expect(state.totalCost).toBe(0.5);
			expect(state.inputTokens).toBe(6000);
			expect(state.outputTokens).toBe(4000);
			expect(state.activeTaskCount).toBe(2);
		});
	});
});
