import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';

import { AttentionDashboard } from './AttentionDashboard';
import { TooltipProvider } from '@/components/ui/Tooltip';
import {
	AttentionAction,
	AttentionItemType,
	GetAttentionDashboardDataResponseSchema,
	QueueSummarySchema,
	RunningSummarySchema,
} from '@/gen/orc/v1/attention_dashboard_pb';
import { TaskPriority } from '@/gen/orc/v1/task_pb';
import { emitAttentionDashboardSignal } from '@/lib/events/attentionDashboardSignals';

const mockGetAttentionDashboardData = vi.fn();
const mockPerformAttentionAction = vi.fn();
const mockToastError = vi.fn();
const mockToastWarning = vi.fn();
const mockNavigate = vi.fn();
const currentProjectId = 'proj-001';

vi.mock('@/stores/projectStore', () => ({
	useCurrentProjectId: () => currentProjectId,
}));

vi.mock('@/stores', () => ({
	toast: {
		error: (...args: unknown[]) => mockToastError(...args),
		warning: (...args: unknown[]) => mockToastWarning(...args),
	},
}));

vi.mock('@/lib/client', () => ({
	attentionDashboardClient: {
		getAttentionDashboardData: (...args: unknown[]) => mockGetAttentionDashboardData(...args),
		performAttentionAction: (...args: unknown[]) => mockPerformAttentionAction(...args),
	},
}));

vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

function createDashboardResponse() {
	return create(GetAttentionDashboardDataResponseSchema, {
		runningSummary: create(RunningSummarySchema, {
			taskCount: 0,
			tasks: [],
		}),
		attentionItems: [
			{
				id: 'failed-TASK-001',
				type: AttentionItemType.FAILED_TASK,
				taskId: 'TASK-001',
				title: 'Failed task',
				description: 'Needs a retry',
				priority: TaskPriority.NORMAL,
				availableActions: [AttentionAction.RETRY, AttentionAction.VIEW],
			},
		],
		queueSummary: create(QueueSummarySchema, {
			taskCount: 0,
			swimlanes: [],
			unassignedTasks: [],
		}),
	});
}

function renderDashboard() {
	return render(
		<TooltipProvider>
			<MemoryRouter>
				<AttentionDashboard />
			</MemoryRouter>
		</TooltipProvider>,
	);
}

describe('AttentionDashboard actions', () => {
	beforeEach(() => {
		mockGetAttentionDashboardData.mockResolvedValue(createDashboardResponse());
		mockPerformAttentionAction.mockResolvedValue({ success: true, errorMessage: '' });
	});

	afterEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	it('performs attention actions and reloads the dashboard', async () => {
		renderDashboard();

		await screen.findByText('Failed task');

		fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		expect(screen.getByRole('button', { name: 'Working…' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'View' })).toBeDisabled();

		await waitFor(() => {
			expect(mockPerformAttentionAction).toHaveBeenCalledWith({
				projectId: currentProjectId,
				attentionItemId: 'failed-TASK-001',
				action: AttentionAction.RETRY,
				decisionOptionId: '',
			});
		});

		await waitFor(() => {
			expect(mockGetAttentionDashboardData).toHaveBeenCalledTimes(2);
		});
	});

	it('shows an error toast and restores buttons when an attention action fails', async () => {
		mockPerformAttentionAction.mockRejectedValueOnce(new Error('backend exploded'));
		renderDashboard();

		await screen.findByText('Failed task');
		const initialLoadCalls = mockGetAttentionDashboardData.mock.calls.length;

		fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		await waitFor(() => {
			expect(mockToastError).toHaveBeenCalledWith('backend exploded');
		});

		expect(mockGetAttentionDashboardData).toHaveBeenCalledTimes(initialLoadCalls);
		expect(screen.getByRole('button', { name: 'Retry' })).toBeEnabled();
		expect(screen.getByRole('button', { name: 'View' })).toBeEnabled();
	});

	it('warns when an action succeeds but the dashboard refresh fails', async () => {
		mockGetAttentionDashboardData
			.mockResolvedValueOnce(createDashboardResponse())
			.mockRejectedValueOnce(new Error('refresh failed'));

		renderDashboard();

		await screen.findByText('Failed task');

		fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		await waitFor(() => {
			expect(mockPerformAttentionAction).toHaveBeenCalledWith({
				projectId: currentProjectId,
				attentionItemId: 'failed-TASK-001',
				action: AttentionAction.RETRY,
				decisionOptionId: '',
			});
		});

		await waitFor(() => {
			expect(mockToastWarning).toHaveBeenCalledWith(
				'Action succeeded, but the dashboard did not refresh.',
			);
		});
		expect(mockToastError).not.toHaveBeenCalled();
	});

	it('keeps current content visible during a background signal refresh', async () => {
		let resolveReload: ((value: ReturnType<typeof createDashboardResponse>) => void) | undefined;
		mockGetAttentionDashboardData
			.mockResolvedValueOnce(createDashboardResponse())
			.mockImplementationOnce(
				() =>
					new Promise((resolve) => {
						resolveReload = resolve;
					}),
			);

		renderDashboard();

		await screen.findByText('Failed task');

		await act(async () => {
			emitAttentionDashboardSignal({
				projectId: currentProjectId,
				taskId: 'TASK-001',
				type: 'task-updated',
			});
		});

		await waitFor(() => {
			expect(screen.getByText('Failed task')).toBeInTheDocument();
		});
		expect(screen.queryByText('Loading attention dashboard...')).not.toBeInTheDocument();

		resolveReload?.(createDashboardResponse());

		await waitFor(() => {
			expect(mockGetAttentionDashboardData).toHaveBeenCalledTimes(2);
		});
	});

	it('keeps current content visible when a background refresh fails', async () => {
		mockGetAttentionDashboardData
			.mockResolvedValueOnce(createDashboardResponse())
			.mockRejectedValueOnce(new Error('refresh failed'));

		renderDashboard();

		await screen.findByText('Failed task');
		const initialLoadCalls = mockGetAttentionDashboardData.mock.calls.length;

		await act(async () => {
			emitAttentionDashboardSignal({
				projectId: currentProjectId,
				taskId: 'TASK-001',
				type: 'task-updated',
			});
		});

		await waitFor(() => {
			expect(mockGetAttentionDashboardData.mock.calls.length).toBeGreaterThan(initialLoadCalls);
		});

		expect(screen.getByText('Failed task')).toBeInTheDocument();
		expect(screen.queryByText(/Error loading dashboard:/)).not.toBeInTheDocument();
	});

	it('submits selected decision options as approve actions', async () => {
		mockGetAttentionDashboardData.mockResolvedValueOnce(
			create(GetAttentionDashboardDataResponseSchema, {
				runningSummary: create(RunningSummarySchema, {
					taskCount: 0,
					tasks: [],
				}),
				attentionItems: [
					{
						id: 'decision-DEC-001',
						type: AttentionItemType.PENDING_DECISION,
						taskId: 'TASK-010',
						title: 'Choose a rollout path',
						description: 'One of these has to be true',
						priority: TaskPriority.NORMAL,
						availableActions: [AttentionAction.APPROVE, AttentionAction.REJECT],
						decisionOptions: [
							{
								id: 'ship-now',
								label: 'Ship now',
								recommended: true,
							},
						],
					},
				],
				queueSummary: create(QueueSummarySchema, {
					taskCount: 0,
					swimlanes: [],
					unassignedTasks: [],
				}),
			}),
		);

		renderDashboard();

		await screen.findByText('Choose a rollout path');
		fireEvent.click(screen.getByRole('button', { name: 'Ship now' }));

		await waitFor(() => {
			expect(mockPerformAttentionAction).toHaveBeenCalledWith({
				projectId: currentProjectId,
				attentionItemId: 'decision-DEC-001',
				action: AttentionAction.APPROVE,
				decisionOptionId: 'ship-now',
			});
		});
	});

	it('reloads when a dashboard signal arrives for the current project', async () => {
		renderDashboard();

		await screen.findByText('Failed task');
		const initialLoadCalls = mockGetAttentionDashboardData.mock.calls.length;

		await act(async () => {
			emitAttentionDashboardSignal({
				projectId: currentProjectId,
				taskId: 'TASK-001',
				type: 'task-updated',
			});
		});

		await waitFor(() => {
			expect(mockGetAttentionDashboardData.mock.calls.length).toBeGreaterThan(initialLoadCalls);
		});
	});

	it('coalesces overlapping background refresh requests', async () => {
		let resolveRefresh: ((value: ReturnType<typeof createDashboardResponse>) => void) | undefined;
		mockGetAttentionDashboardData
			.mockResolvedValueOnce(createDashboardResponse())
			.mockImplementationOnce(
				() =>
					new Promise((resolve) => {
						resolveRefresh = resolve;
					}),
			)
			.mockResolvedValue(createDashboardResponse());

		renderDashboard();

		await screen.findByText('Failed task');

		await act(async () => {
			emitAttentionDashboardSignal({
				projectId: currentProjectId,
				taskId: 'TASK-001',
				type: 'task-updated',
			});
			emitAttentionDashboardSignal({
				projectId: currentProjectId,
				taskId: 'TASK-001',
				type: 'task-updated',
			});
		});

		await waitFor(() => {
			expect(mockGetAttentionDashboardData).toHaveBeenCalledTimes(2);
		});

		resolveRefresh?.(createDashboardResponse());

		await waitFor(() => {
			expect(mockGetAttentionDashboardData).toHaveBeenCalledTimes(3);
		});
	});

	it('ignores dashboard signals for other projects', async () => {
		renderDashboard();

		await screen.findByText('Failed task');
		const initialLoadCalls = mockGetAttentionDashboardData.mock.calls.length;

		await act(async () => {
			emitAttentionDashboardSignal({
				projectId: 'proj-002',
				taskId: 'TASK-001',
				type: 'task-updated',
			});
		});

		await waitFor(() => {
			expect(mockGetAttentionDashboardData).toHaveBeenCalledTimes(initialLoadCalls);
		});
	});
});
