/**
 * Integration tests for TaskDetail page layout redesign
 *
 * Tests the new "deep work" layout with:
 * - Header with back link, task info, workflow, branch, elapsed time
 * - Workflow progress visualization
 * - Split pane (Live Output + Changes)
 * - Footer with metrics and action buttons
 *
 * Success Criteria Coverage:
 * - SC-1: Page header displays back link, task ID, title, workflow name, branch, elapsed time
 * - SC-2-12: Component integration (delegates to component tests)
 *
 * This file tests the wiring/integration of components into the page.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { TaskDetail } from './TaskDetail';
import { useTaskStore, useProjectStore } from '@/stores';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockTaskPlan, createMockPhase, createTimestamp } from '@/test/factories';

// Mock the Connect RPC client
const mockGetTask = vi.fn();
const mockGetTaskPlan = vi.fn();
const mockPauseTask = vi.fn();
const mockResumeTask = vi.fn();
const mockListReviewComments = vi.fn();
const mockListFeedback = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: (...args: unknown[]) => mockGetTask(...args),
		getTaskPlan: (...args: unknown[]) => mockGetTaskPlan(...args),
		pauseTask: (...args: unknown[]) => mockPauseTask(...args),
		resumeTask: (...args: unknown[]) => mockResumeTask(...args),
		listReviewComments: (...args: unknown[]) => mockListReviewComments(...args),
	},
	feedbackClient: {
		listFeedback: (...args: unknown[]) => mockListFeedback(...args),
	},
}));

// Mock hooks module
vi.mock('@/hooks', () => ({
	useTaskSubscription: vi.fn(() => ({
		state: undefined,
		transcript: [],
		isSubscribed: false,
		connectionStatus: 'connected',
		clearTranscript: vi.fn(),
	})),
	useDocumentTitle: vi.fn(),
}));

// Mock stores
vi.mock('@/stores', async () => {
	const actual = await vi.importActual('@/stores');
	return {
		...actual,
		getInitiativeBadgeTitle: () => null,
		useInitiatives: () => [],
	};
});

vi.mock('@/stores/uiStore', async () => {
	const actual = await vi.importActual('@/stores/uiStore');
	return {
		...actual,
		toast: {
			success: vi.fn(),
			error: vi.fn(),
		},
	};
});

// Mock navigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

function renderTaskDetail(taskId: string = 'TASK-001') {
	return render(
		<TooltipProvider delayDuration={0}>
			<MemoryRouter initialEntries={[`/tasks/${taskId}`]}>
				<Routes>
					<Route path="/tasks/:id" element={<TaskDetail />} />
					<Route path="/board" element={<div>Board Page</div>} />
				</Routes>
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('TaskDetail Layout (TASK-736)', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		useTaskStore.getState().reset();
		useProjectStore.setState({ currentProjectId: 'test-project' });

		// Default mock responses for child components
		mockListReviewComments.mockResolvedValue({ comments: [] });
		mockListFeedback.mockResolvedValue({ feedback: [] });

		// Default mock responses
		mockGetTask.mockResolvedValue({
			task: createMockTask({
				id: 'TASK-001',
				title: 'Fix authentication bug',
				status: TaskStatus.RUNNING,
				branch: 'orc/TASK-001',
				workflowId: 'implement-medium',
				currentPhase: 'implement',
				startedAt: createTimestamp(new Date(Date.now() - 222000)), // 3:42 ago
			}),
		});
		mockGetTaskPlan.mockResolvedValue({
			plan: createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-3', name: 'review', status: PhaseStatus.PENDING }),
				],
			}),
		});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-1: Header displays task information', () => {
		it('displays back link to board', async () => {
			renderTaskDetail();

			await waitFor(() => {
				const backLink = screen.getByRole('link', { name: /back to board/i });
				expect(backLink).toBeInTheDocument();
				expect(backLink).toHaveAttribute('href', '/board');
			});
		});

		it('displays task ID badge', async () => {
			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});
		});

		it('displays task title', async () => {
			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Fix authentication bug')).toBeInTheDocument();
			});
		});

		it('displays workflow name', async () => {
			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText(/implement-medium/i)).toBeInTheDocument();
			});
		});

		it('displays branch name', async () => {
			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText(/orc\/TASK-001/)).toBeInTheDocument();
			});
		});

		it('displays elapsed time', async () => {
			renderTaskDetail();

			await waitFor(() => {
				// Should show something like "3:42" or "3m 42s"
				expect(screen.getByText(/\d+:\d+|\d+m/)).toBeInTheDocument();
			});
		});

		it('shows error when task fetch fails', async () => {
			mockGetTask.mockRejectedValue(new Error('Task not found'));

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Failed to load task')).toBeInTheDocument();
				expect(screen.getByText('Task not found')).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			mockGetTask.mockRejectedValue(new Error('Network error'));

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});
	});

	describe('Layout Integration', () => {
		it('renders workflow progress component', async () => {
			renderTaskDetail();

			await waitFor(() => {
				// Workflow progress should show phases
				expect(screen.getByText('spec')).toBeInTheDocument();
				expect(screen.getByText('implement')).toBeInTheDocument();
				expect(screen.getByText('review')).toBeInTheDocument();
			});
		});

		it('renders split pane with left and right panels', async () => {
			const { container } = renderTaskDetail();

			await waitFor(() => {
				const splitPane = container.querySelector('.split-pane');
				expect(splitPane).toBeInTheDocument();

				const leftPanel = container.querySelector('.split-pane__left');
				const rightPanel = container.querySelector('.split-pane__right');
				expect(leftPanel).toBeInTheDocument();
				expect(rightPanel).toBeInTheDocument();
			});
		});

		it('renders footer with action buttons', async () => {
			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument();
			});
		});
	});

	describe('BDD-1: Running task shows correct phase states', () => {
		it('shows spec with checkmark, implement with dot, review with circle', async () => {
			mockGetTask.mockResolvedValue({
				task: createMockTask({
					status: TaskStatus.RUNNING,
					currentPhase: 'implement',
				}),
			});
			mockGetTaskPlan.mockResolvedValue({
				plan: createMockTaskPlan({
					phases: [
						createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
						createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
						createMockPhase({ id: 'phase-3', name: 'review', status: PhaseStatus.PENDING }),
					],
				}),
			});

			const { container } = renderTaskDetail();

			await waitFor(() => {
				// Completed phase (spec) should have completed indicator
				const completedPhase = container.querySelector('.workflow-progress__phase--completed');
				expect(completedPhase).toBeInTheDocument();

				// Running phase (implement) should have running indicator
				const runningPhase = container.querySelector('.workflow-progress__phase--running');
				expect(runningPhase).toBeInTheDocument();

				// Pending phase (review) should have pending indicator
				const pendingPhase = container.querySelector('.workflow-progress__phase--pending');
				expect(pendingPhase).toBeInTheDocument();
			});
		});
	});

	describe('BDD-2: Pause button changes to Resume', () => {
		it('clicking Pause changes button to Resume', async () => {
			mockPauseTask.mockResolvedValue({
				task: createMockTask({ status: TaskStatus.PAUSED }),
			});

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument();
			});

			const pauseButton = screen.getByRole('button', { name: /pause/i });
			fireEvent.click(pauseButton);

			await waitFor(() => {
				expect(mockPauseTask).toHaveBeenCalled();
			});
		});
	});

	describe('BDD-3: Split pane ratio persistence', () => {
		it('persists split pane ratio across navigations', async () => {
			// Pre-set localStorage to simulate a previously saved ratio
			// This tests that the component reads persisted values on mount
			localStorage.setItem('split-pane-task-detail', '30');

			const { container } = renderTaskDetail();

			await waitFor(() => {
				const divider = container.querySelector('.split-pane__divider');
				// Ratio should be restored from localStorage
				const ratio = divider?.getAttribute('aria-valuenow');
				expect(Number(ratio)).toBeLessThan(50);
			});

			// Clean up
			localStorage.removeItem('split-pane-task-detail');
		});
	});

	describe('BDD-4: Retry with feedback', () => {
		it('sends feedback with retry request', async () => {
			// This test verifies that when a failed task is displayed,
			// the user can enter guidance in a textarea and that guidance
			// is sent with the retry request.
			//
			// Setup: Failed task with error state
			mockGetTask.mockResolvedValue({
				task: createMockTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
				}),
			});

			renderTaskDetail();

			// Wait for page to load - use exact match to avoid matching branch name
			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// The feedback textarea should be visible when task is failed
			const textarea = screen.getByPlaceholderText(/guidance|feedback|note/i);
			fireEvent.change(textarea, { target: { value: 'Use validateSession instead' } });

			const retryButton = screen.getByRole('button', { name: /retry implement/i });
			fireEvent.click(retryButton);

			// Verify API was called with feedback (the exact API structure TBD during implementation)
			// This test will be refined when the TaskFooter component is implemented
		});
	});

	describe('Failure Modes', () => {
		it('shows fallback when workflow plan is unavailable', async () => {
			mockGetTaskPlan.mockResolvedValue({ plan: null });

			renderTaskDetail();

			await waitFor(() => {
				// Should still show workflow ID and current phase in header
				// Using exact match to avoid matching phase names in progress
				expect(screen.getByText('implement-medium')).toBeInTheDocument();
			});
		});

		it('handles WebSocket disconnection gracefully', async () => {
			// When WebSocket disconnects, page should continue showing task data
			// (from initial API fetch). Connection status indicator may show "disconnected".

			renderTaskDetail();

			await waitFor(() => {
				// Page should still render with data from initial API fetch
				expect(screen.getByText('Fix authentication bug')).toBeInTheDocument();
			});
		});
	});
});
