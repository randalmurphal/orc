/**
 * TDD Tests for useTaskSubscription - TASK-550
 *
 * Tests for: Connect useEvents hook to taskStore for execution state updates
 *
 * Problem: useTaskSubscription has local state that's always null, but taskStore
 * has execution state that gets updated via events. The hook should read from
 * taskStore instead of maintaining disconnected local state.
 *
 * Success Criteria Coverage:
 * - SC-1: useTaskSubscription returns execution state from taskStore.taskStates
 * - SC-2: When taskStore updates, useTaskSubscription reflects the change
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useTaskSubscription } from './useEvents';
import { EventProvider } from './EventProvider';
import { useTaskStore } from '@/stores/taskStore';
import { create } from '@bufbuild/protobuf';
import { ExecutionStateSchema, type ExecutionState } from '@/gen/orc/v1/task_pb';
import type { ReactNode } from 'react';

// Mock the EventSubscription class to avoid actual network calls
vi.mock('@/lib/events', () => ({
	EventSubscription: vi.fn().mockImplementation(() => ({
		onStatusChange: vi.fn(() => () => {}),
		on: vi.fn(() => () => {}),
		connect: vi.fn(),
		disconnect: vi.fn(),
		isConnected: vi.fn(() => false),
	})),
	handleEvent: vi.fn(),
}));

// Helper to create an ExecutionState
function createMockExecutionState(overrides: Partial<ExecutionState> = {}): ExecutionState {
	return create(ExecutionStateSchema, {
		currentIteration: 1,
		phases: {},
		gates: [],
		...overrides,
	});
}

// Wrapper for the hook that includes EventProvider
function createWrapper() {
	return function Wrapper({ children }: { children: ReactNode }) {
		return <EventProvider autoConnect={false}>{children}</EventProvider>;
	};
}

describe('useTaskSubscription - SC-1: Returns execution state from taskStore', () => {
	beforeEach(() => {
		// Reset the store before each test
		useTaskStore.getState().reset();
		vi.clearAllMocks();
	});

	it('should return null when no execution state exists in store', () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		expect(result.current.state).toBeNull();
	});

	it('should return execution state when it exists in taskStore', () => {
		// Arrange: Put execution state in the store
		const mockState = createMockExecutionState({ currentIteration: 2 });
		useTaskStore.getState().updateTaskState('TASK-001', mockState);

		// Act: Render the hook
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Assert: Should return the state from the store
		expect(result.current.state).not.toBeNull();
		expect(result.current.state?.currentIteration).toBe(2);
	});

	it('should return state for the correct taskId only', () => {
		// Arrange: Put execution states for two different tasks
		const state1 = createMockExecutionState({ currentIteration: 1 });
		const state2 = createMockExecutionState({ currentIteration: 2 });
		useTaskStore.getState().updateTaskState('TASK-001', state1);
		useTaskStore.getState().updateTaskState('TASK-002', state2);

		// Act & Assert: Each hook instance returns the correct state
		const { result: result1 } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});
		const { result: result2 } = renderHook(() => useTaskSubscription('TASK-002'), {
			wrapper: createWrapper(),
		});

		expect(result1.current.state?.currentIteration).toBe(1);
		expect(result2.current.state?.currentIteration).toBe(2);
	});

	it('should return null for undefined taskId', () => {
		const { result } = renderHook(() => useTaskSubscription(undefined), {
			wrapper: createWrapper(),
		});

		expect(result.current.state).toBeNull();
	});
});

describe('useTaskSubscription - SC-2: Updates when taskStore changes', () => {
	beforeEach(() => {
		useTaskStore.getState().reset();
		vi.clearAllMocks();
	});

	it('should update when execution state is added to store', () => {
		// Arrange: Start with no state
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		expect(result.current.state).toBeNull();

		// Act: Add state to store
		act(() => {
			const mockState = createMockExecutionState({ currentIteration: 3 });
			useTaskStore.getState().updateTaskState('TASK-001', mockState);
		});

		// Assert: Hook should reflect the new state
		expect(result.current.state).not.toBeNull();
		expect(result.current.state?.currentIteration).toBe(3);
	});

	it('should update when execution state is modified in store', () => {
		// Arrange: Start with initial state
		const initialState = createMockExecutionState({ currentIteration: 1 });
		useTaskStore.getState().updateTaskState('TASK-001', initialState);

		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		expect(result.current.state?.currentIteration).toBe(1);

		// Act: Update state in store
		act(() => {
			const updatedState = createMockExecutionState({ currentIteration: 5 });
			useTaskStore.getState().updateTaskState('TASK-001', updatedState);
		});

		// Assert: Hook should reflect the updated state
		expect(result.current.state?.currentIteration).toBe(5);
	});

	it('should update to null when execution state is removed from store', () => {
		// Arrange: Start with state
		const mockState = createMockExecutionState({ currentIteration: 2 });
		useTaskStore.getState().updateTaskState('TASK-001', mockState);

		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		expect(result.current.state).not.toBeNull();

		// Act: Remove state from store
		act(() => {
			useTaskStore.getState().removeTaskState('TASK-001');
		});

		// Assert: Hook should return null
		expect(result.current.state).toBeNull();
	});

	it('should not update when a different task state changes', () => {
		// Arrange: Set up initial state for TASK-001
		const state1 = createMockExecutionState({ currentIteration: 1 });
		useTaskStore.getState().updateTaskState('TASK-001', state1);

		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Update a different task
		act(() => {
			const state2 = createMockExecutionState({ currentIteration: 99 });
			useTaskStore.getState().updateTaskState('TASK-002', state2);
		});

		// Assert: TASK-001's state should be unchanged
		expect(result.current.state?.currentIteration).toBe(1);
	});
});

describe('useTaskSubscription - Edge cases', () => {
	beforeEach(() => {
		useTaskStore.getState().reset();
		vi.clearAllMocks();
	});

	it('should handle taskId change', () => {
		// Arrange: Set up states for two tasks
		const state1 = createMockExecutionState({ currentIteration: 1 });
		const state2 = createMockExecutionState({ currentIteration: 2 });
		useTaskStore.getState().updateTaskState('TASK-001', state1);
		useTaskStore.getState().updateTaskState('TASK-002', state2);

		// Act: Start with TASK-001, then switch to TASK-002
		const { result, rerender } = renderHook(
			({ taskId }) => useTaskSubscription(taskId),
			{
				wrapper: createWrapper(),
				initialProps: { taskId: 'TASK-001' },
			}
		);

		expect(result.current.state?.currentIteration).toBe(1);

		rerender({ taskId: 'TASK-002' });

		// Assert: Should now show TASK-002's state
		expect(result.current.state?.currentIteration).toBe(2);
	});

	it('should handle switching from defined taskId to undefined', () => {
		// Arrange
		const mockState = createMockExecutionState({ currentIteration: 5 });
		useTaskStore.getState().updateTaskState('TASK-001', mockState);

		const { result, rerender } = renderHook(
			({ taskId }) => useTaskSubscription(taskId),
			{
				wrapper: createWrapper(),
				initialProps: { taskId: 'TASK-001' as string | undefined },
			}
		);

		expect(result.current.state?.currentIteration).toBe(5);

		// Act: Switch to undefined
		rerender({ taskId: undefined });

		// Assert: Should return null (global subscription doesn't track specific state)
		expect(result.current.state).toBeNull();
	});
});
