import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { WebSocketProvider, useWebSocket } from './useWebSocket';
import { useSessionStore } from '@/stores/sessionStore';
import type { ReactNode } from 'react';

describe('WebSocket session_update integration', () => {
	beforeEach(() => {
		// Reset session store
		useSessionStore.getState().reset();
		vi.clearAllMocks();
	});

	const wrapper = ({ children }: { children: ReactNode }) => (
		<WebSocketProvider autoConnect={false}>{children}</WebSocketProvider>
	);

	it('should update sessionStore with all fields from session_update event', async () => {
		renderHook(() => useWebSocket(), { wrapper });

		// Simulate session_update WebSocket event by directly calling the store method
		// This is how the WebSocket handler would call it when an event is received
		useSessionStore.getState().updateFromSessionEvent({
			duration_seconds: 3650,
			total_tokens: 127500,
			estimated_cost_usd: 2.51,
			input_tokens: 95000,
			output_tokens: 32500,
			tasks_running: 2,
			is_paused: false,
		});

		await waitFor(() => {
			const sessionState = useSessionStore.getState();
			expect(sessionState.totalTokens).toBe(127500);
			expect(sessionState.totalCost).toBe(2.51);
			expect(sessionState.inputTokens).toBe(95000);
			expect(sessionState.outputTokens).toBe(32500);
			expect(sessionState.activeTaskCount).toBe(2);
			expect(sessionState.isPaused).toBe(false);
		});
	});

	it('should compute duration from duration_seconds', async () => {
		renderHook(() => useWebSocket(), { wrapper });

		// Simulate session_update WebSocket event
		useSessionStore.getState().updateFromSessionEvent({
			duration_seconds: 3650, // 1h 0m 50s
			total_tokens: 1000,
			estimated_cost_usd: 0.01,
			input_tokens: 500,
			output_tokens: 500,
			tasks_running: 1,
			is_paused: false,
		});

		// The handler should compute startTime from duration_seconds
		// Expected: duration_seconds of 3650 = ~1h 0m
		const expectedDuration = '1h 0m';

		await waitFor(() => {
			const state = useSessionStore.getState();
			// Duration should be computed from the server-provided duration_seconds
			expect(state.duration).toBe(expectedDuration);
		});
	});

	it('should handle reconnection by syncing from first event', async () => {
		renderHook(() => useWebSocket(), { wrapper });

		// Set initial local state
		useSessionStore.getState().updateMetrics({
			totalTokens: 5000,
			totalCost: 0.1,
			inputTokens: 3000,
			outputTokens: 2000,
		});

		// Simulate reconnection with fresh session_update
		useSessionStore.getState().updateFromSessionEvent({
			duration_seconds: 100,
			total_tokens: 10000, // Server has more tokens
			estimated_cost_usd: 0.5,
			input_tokens: 6000,
			output_tokens: 4000,
			tasks_running: 1,
			is_paused: false,
		});

		await waitFor(() => {
			const state = useSessionStore.getState();
			// After reconnect, server state should win
			expect(state.totalTokens).toBe(10000);
			expect(state.totalCost).toBe(0.5);
			expect(state.inputTokens).toBe(6000);
			expect(state.outputTokens).toBe(4000);
		});
	});

	it('should update metrics within 100ms of event receipt', async () => {
		renderHook(() => useWebSocket(), { wrapper });

		const startTime = performance.now();

		// Trigger the event
		useSessionStore.getState().updateFromSessionEvent({
			duration_seconds: 50,
			total_tokens: 1000,
			estimated_cost_usd: 0.02,
			input_tokens: 600,
			output_tokens: 400,
			tasks_running: 1,
			is_paused: false,
		});

		await waitFor(
			() => {
				const state = useSessionStore.getState();
				expect(state.totalTokens).toBe(1000);

				const elapsed = performance.now() - startTime;
				expect(elapsed).toBeLessThan(100);
			},
			{ timeout: 100 }
		);
	});

	it('should preserve local startTime if it exists', async () => {
		renderHook(() => useWebSocket(), { wrapper });

		// Start a local session
		useSessionStore.getState().startSession();
		const originalStartTime = useSessionStore.getState().startTime;

		// Simulate session_update
		useSessionStore.getState().updateFromSessionEvent({
			duration_seconds: 100,
			total_tokens: 5000,
			estimated_cost_usd: 0.1,
			input_tokens: 3000,
			output_tokens: 2000,
			tasks_running: 1,
			is_paused: false,
		});

		await waitFor(() => {
			const state = useSessionStore.getState();
			// startTime should be preserved if it already exists
			expect(state.startTime).toBe(originalStartTime);
		});
	});

	it('should handle is_paused flag updates', async () => {
		renderHook(() => useWebSocket(), { wrapper });

		// Simulate session_update with paused state
		useSessionStore.getState().updateFromSessionEvent({
			duration_seconds: 100,
			total_tokens: 1000,
			estimated_cost_usd: 0.02,
			input_tokens: 600,
			output_tokens: 400,
			tasks_running: 0,
			is_paused: true,
		});

		await waitFor(() => {
			const state = useSessionStore.getState();
			expect(state.isPaused).toBe(true);
			expect(state.activeTaskCount).toBe(0);
		});
	});
});
