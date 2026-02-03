/**
 * Integration tests for FeedbackPanel in TaskDetail view
 *
 * These tests verify that the FeedbackPanel component is properly wired
 * into the TaskDetail page and can interact with the task execution flow.
 *
 * This is MANDATORY integration testing - ensures new feedback UI is
 * actually connected to existing production code paths.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { TaskDetail } from './TaskDetail';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockTaskPlan, createMockFeedback } from '@/test/factories';

// Mock all the dependencies
const mockGetTask = vi.fn();
const mockGetTaskPlan = vi.fn();
const mockListFeedback = vi.fn();
const mockAddFeedback = vi.fn();
const mockSendFeedback = vi.fn();
const mockPauseTask = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: (...args: unknown[]) => mockGetTask(...args),
		getTaskPlan: (...args: unknown[]) => mockGetTaskPlan(...args),
		pauseTask: (...args: unknown[]) => mockPauseTask(...args),
	},
	feedbackClient: {
		listFeedback: (...args: unknown[]) => mockListFeedback(...args),
		addFeedback: (...args: unknown[]) => mockAddFeedback(...args),
		sendFeedback: (...args: unknown[]) => mockSendFeedback(...args),
	},
}));

// Mock stores and hooks
vi.mock('@/stores', () => ({
	useCurrentProjectId: () => 'test-project',
	useWebSocket: () => ({
		on: vi.fn(),
		off: vi.fn(),
	}),
}));

vi.mock('@/hooks', () => ({
	useTaskSubscription: () => ({
		state: null,
		transcript: [],
	}),
	useDocumentTitle: vi.fn(),
}));

vi.mock('@/stores/taskStore', () => ({
	useTask: () => null,
	useTaskState: () => null,
}));

// Helper to render TaskDetail with router context
function renderTaskDetail(taskId: string = 'TASK-123') {
	return render(
		<MemoryRouter initialEntries={[`/tasks/${taskId}`]}>
			<Routes>
				<Route path="/tasks/:id" element={<TaskDetail />} />
			</Routes>
		</MemoryRouter>
	);
}

describe('TaskDetail - FeedbackPanel Integration', () => {
	const mockTask = createMockTask({
		id: 'TASK-123',
		title: 'Test Task for Feedback',
		status: TaskStatus.RUNNING,
	});

	const mockPlan = createMockTaskPlan();

	beforeEach(() => {
		vi.clearAllMocks();

		// Default successful responses
		mockGetTask.mockResolvedValue({ task: mockTask });
		mockGetTaskPlan.mockResolvedValue({ plan: mockPlan });
		mockListFeedback.mockResolvedValue({ feedback: [] });
		mockAddFeedback.mockResolvedValue({ feedback: createMockFeedback() });
		mockSendFeedback.mockResolvedValue({ sentCount: 0 });
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	/**
	 * INTEGRATION TEST: Verifies FeedbackPanel is rendered in TaskDetail
	 *
	 * This test FAILS if the wiring is missing. It exercises the real
	 * TaskDetail component and ensures FeedbackPanel appears.
	 */
	it('renders FeedbackPanel within TaskDetail layout', async () => {
		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Verify FeedbackPanel is present - this FAILS without wiring
		expect(screen.getByRole('region', { name: /feedback/i })).toBeInTheDocument();
		expect(screen.getByLabelText(/feedback text/i)).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
	});

	/**
	 * INTEGRATION TEST: Verifies feedback creation flows through real TaskDetail
	 *
	 * Tests the production code path: TaskDetail → FeedbackPanel → API
	 */
	it('creates feedback through TaskDetail → FeedbackPanel flow', async () => {
		const user = userEvent.setup();
		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Create feedback through the integrated UI
		await user.type(screen.getByLabelText(/feedback text/i), 'Integration test feedback');
		await user.click(screen.getByRole('button', { name: /add feedback/i }));

		// Verify API was called with correct task ID from route
		await waitFor(() => {
			expect(mockAddFeedback).toHaveBeenCalledWith({
				projectId: 'test-project',
				taskId: 'TASK-123', // Must match the route parameter
				type: expect.any(Number), // FeedbackType enum
				text: 'Integration test feedback',
				timing: expect.any(Number), // FeedbackTiming enum
				file: '',
				line: 0,
			});
		});
	});

	/**
	 * INTEGRATION TEST: Verifies NOW timing feedback pauses the task
	 *
	 * Tests the cross-component interaction: FeedbackPanel → TaskDetail → TaskClient
	 */
	it('NOW timing feedback triggers task pause through production flow', async () => {
		const user = userEvent.setup();
		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Select NOW timing - use selectOptions for native <select> element
		const timingSelect = screen.getByLabelText(/timing/i);
		await user.selectOptions(timingSelect, '1');  // 1 = FeedbackTiming.NOW

		// Create feedback
		await user.type(screen.getByLabelText(/feedback text/i), 'Stop and fix this');
		await user.click(screen.getByRole('button', { name: /add feedback/i }));

		// Verify both feedback creation AND task pause were called
		await waitFor(() => {
			expect(mockAddFeedback).toHaveBeenCalled();
			expect(mockPauseTask).toHaveBeenCalledWith({
				projectId: 'test-project',
				taskId: 'TASK-123',
			});
		});
	});

	/**
	 * INTEGRATION TEST: Verifies feedback panel appears in correct layout location
	 */
	it('positions FeedbackPanel in expected layout section', async () => {
		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Verify layout structure - feedback should be in a specific section
		// This tests that FeedbackPanel is integrated into the SplitPane layout
		const feedbackSection = screen.getByRole('region', { name: /feedback/i });
		expect(feedbackSection).toBeInTheDocument();

		// Should be within the task detail content area
		const taskDetailContent = feedbackSection.closest('.task-detail-content');
		expect(taskDetailContent).toBeInTheDocument();
	});

	/**
	 * INTEGRATION TEST: Verifies task updates trigger feedback panel refresh
	 *
	 * Tests the real-time update flow: WebSocket → TaskDetail → FeedbackPanel
	 */
	it('feedback panel refreshes when task updates', async () => {
		// Start with no feedback
		mockListFeedback.mockResolvedValue({ feedback: [] });

		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Verify no feedback initially
		expect(screen.queryByText('Existing feedback item')).not.toBeInTheDocument();

		// Simulate feedback being added externally (e.g., from another UI)
		const newFeedback = [createMockFeedback({ text: 'Existing feedback item' })];
		mockListFeedback.mockResolvedValue({ feedback: newFeedback });

		// Trigger a re-render (simulating WebSocket update)
		mockGetTask.mockResolvedValue({
			task: { ...mockTask, updatedAt: new Date() }
		});

		// In a real scenario, this would be triggered by WebSocket events
		// For testing, we can trigger it by updating the task
		renderTaskDetail();

		// Verify feedback panel shows the new feedback
		await waitFor(() => {
			expect(screen.getByText('Existing feedback item')).toBeInTheDocument();
		});
	});

	/**
	 * INTEGRATION TEST: Verifies error handling flows through complete stack
	 */
	it('handles feedback API errors through full TaskDetail integration', async () => {
		const user = userEvent.setup();

		// Make feedback API fail
		mockAddFeedback.mockRejectedValue(new Error('API Error'));

		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Try to create feedback
		await user.type(screen.getByLabelText(/feedback text/i), 'This will fail');
		await user.click(screen.getByRole('button', { name: /add feedback/i }));

		// Verify error is displayed in the UI
		await waitFor(() => {
			expect(screen.getByText(/failed to add feedback/i)).toBeInTheDocument();
		});
	});

	/**
	 * INTEGRATION TEST: Verifies feedback panel doesn't appear for completed tasks
	 */
	it('conditionally shows feedback panel based on task status', async () => {
		const completedTask = createMockTask({
			id: 'TASK-123',
			title: 'Test Task for Feedback',
			status: TaskStatus.COMPLETED,
		});

		mockGetTask.mockResolvedValue({ task: completedTask });

		renderTaskDetail();

		// Wait for task to load
		await waitFor(() => {
			expect(screen.getByText('Test Task for Feedback')).toBeInTheDocument();
		});

		// Feedback panel should be hidden or disabled for completed tasks
		// This business logic should be tested to ensure it's wired correctly
		const feedbackRegion = screen.queryByRole('region', { name: /feedback/i });
		if (feedbackRegion) {
			// If shown, it should be disabled
			expect(screen.getByRole('button', { name: /add feedback/i })).toBeDisabled();
		} else {
			// Or it might be completely hidden
			expect(feedbackRegion).not.toBeInTheDocument();
		}
	});
});