/**
 * TDD Tests for useLayoutPersistence hook
 *
 * Tests for TASK-640: Debounced layout save functionality
 *
 * Success Criteria Coverage:
 * - SC-10: Node drag calls saveWorkflowLayout with debounce (1 second)
 *
 * Behaviors:
 * - Debounces rapid position changes
 * - Collects all node positions for batch save
 * - Handles save errors gracefully
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useLayoutPersistence } from './useLayoutPersistence';
import { workflowClient } from '@/lib/client';
import { createMockSaveWorkflowLayoutResponse } from '@/test/factories';

// Mock the workflow client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		saveWorkflowLayout: vi.fn(),
	},
}));

describe('useLayoutPersistence', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('debounce behavior', () => {
		it('does not call API immediately on position update', () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			// Should not be called immediately
			expect(mockSave).not.toHaveBeenCalled();
		});

		it('calls API after 1 second debounce period', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			// Advance timer by 1 second
			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			expect(mockSave).toHaveBeenCalledTimes(1);
		});

		it('resets debounce timer on each position update', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			// First update
			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			// Wait 500ms
			await act(async () => {
				vi.advanceTimersByTime(500);
			});

			// Second update - should reset timer
			act(() => {
				result.current.savePosition('spec', 150, 250);
			});

			// Wait another 500ms (total 1000ms since start, but only 500ms since last update)
			await act(async () => {
				vi.advanceTimersByTime(500);
			});

			// Should not have been called yet
			expect(mockSave).not.toHaveBeenCalled();

			// Wait the remaining 500ms
			await act(async () => {
				vi.advanceTimersByTime(500);
			});

			// Now it should be called with the latest position
			expect(mockSave).toHaveBeenCalledTimes(1);
		});

		it('makes single API call for multiple rapid position updates', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			// Rapid updates
			act(() => {
				result.current.savePosition('spec', 100, 200);
				result.current.savePosition('spec', 110, 210);
				result.current.savePosition('spec', 120, 220);
				result.current.savePosition('spec', 130, 230);
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			// Should only call once with final position
			expect(mockSave).toHaveBeenCalledTimes(1);
		});
	});

	describe('position collection', () => {
		it('includes workflowId in save request', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'my-workflow' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			expect(mockSave).toHaveBeenCalledWith(
				expect.objectContaining({
					workflowId: 'my-workflow',
				})
			);
		});

		it('includes all phase positions in single save request', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
				result.current.savePosition('implement', 300, 200);
				result.current.savePosition('review', 500, 200);
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			expect(mockSave).toHaveBeenCalledWith(
				expect.objectContaining({
					positions: expect.arrayContaining([
						expect.objectContaining({ phaseTemplateId: 'spec', positionX: 100, positionY: 200 }),
						expect.objectContaining({ phaseTemplateId: 'implement', positionX: 300, positionY: 200 }),
						expect.objectContaining({ phaseTemplateId: 'review', positionX: 500, positionY: 200 }),
					]),
				})
			);
		});

		it('uses latest position when same phase updated multiple times', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
				result.current.savePosition('spec', 150, 250); // Update same phase
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			expect(mockSave).toHaveBeenCalledWith(
				expect.objectContaining({
					positions: [
						expect.objectContaining({ phaseTemplateId: 'spec', positionX: 150, positionY: 250 }),
					],
				})
			);
		});
	});

	describe('error handling', () => {
		it('calls onError callback when save fails', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockRejectedValue(new Error('Network error'));

			const onError = vi.fn();

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf', onError })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			expect(onError).toHaveBeenCalledWith(expect.any(Error));
		});

		it('clears pending positions after successful save', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			expect(mockSave).toHaveBeenCalledTimes(1);

			// New position update after successful save
			act(() => {
				result.current.savePosition('implement', 300, 200);
			});

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			// Should only include the new position, not the old one
			expect(mockSave).toHaveBeenCalledTimes(2);
			expect(mockSave).toHaveBeenLastCalledWith(
				expect.objectContaining({
					positions: [
						expect.objectContaining({ phaseTemplateId: 'implement' }),
					],
				})
			);
		});
	});

	describe('cleanup', () => {
		it('cancels pending save on unmount', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result, unmount } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			// Unmount before debounce completes
			unmount();

			await act(async () => {
				vi.advanceTimersByTime(1000);
			});

			// Should not have been called since hook was unmounted
			expect(mockSave).not.toHaveBeenCalled();
		});
	});

	describe('flush functionality', () => {
		it('provides flush function for immediate save', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			act(() => {
				result.current.savePosition('spec', 100, 200);
			});

			// Flush immediately instead of waiting
			await act(async () => {
				await result.current.flush();
			});

			expect(mockSave).toHaveBeenCalledTimes(1);
		});
	});

	describe('saveAllPositions helper', () => {
		it('saves all provided positions in single call', async () => {
			const mockSave = vi.mocked(workflowClient.saveWorkflowLayout);
			mockSave.mockResolvedValue(createMockSaveWorkflowLayoutResponse(true));

			const { result } = renderHook(() =>
				useLayoutPersistence({ workflowId: 'test-wf' })
			);

			const positions = [
				{ phaseTemplateId: 'spec', x: 100, y: 200 },
				{ phaseTemplateId: 'implement', x: 300, y: 200 },
			];

			await act(async () => {
				await result.current.saveAllPositions(positions);
			});

			expect(mockSave).toHaveBeenCalledWith(
				expect.objectContaining({
					positions: expect.arrayContaining([
						expect.objectContaining({ phaseTemplateId: 'spec' }),
						expect.objectContaining({ phaseTemplateId: 'implement' }),
					]),
				})
			);
		});
	});
});
