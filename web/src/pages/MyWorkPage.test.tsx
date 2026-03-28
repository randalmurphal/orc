import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { MyWorkPage } from './MyWorkPage';
import { TooltipProvider } from '@/components/ui';
import { attentionDashboardClient, projectClient } from '@/lib/client';
import {
	createMockAttentionDashboardResponse,
	createMockAttentionItem,
	createMockGetAllProjectsStatusResponse,
	createMockProjectStatus,
	createMockRecentCompletion,
	createMockRunningTask,
	createTimestamp,
} from '@/test/factories';
import { AttentionAction, AttentionItemType } from '@/gen/orc/v1/attention_dashboard_pb';
import {
	PerformAttentionActionResponseSchema,
	RunningSummarySchema,
} from '@/gen/orc/v1/attention_dashboard_pb';
import { emitAttentionDashboardSignal } from '@/lib/events/attentionDashboardSignals';
import { emitRecommendationSignal } from '@/lib/events/recommendationSignals';

const mockNavigate = vi.fn();
const mockSelectProject = vi.fn();
const mockToastError = vi.fn();
const mockToastWarning = vi.fn();

vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

vi.mock('@/lib/client', () => ({
	projectClient: {
		getAllProjectsStatus: vi.fn(),
	},
	attentionDashboardClient: {
		getAttentionDashboardData: vi.fn(),
		performAttentionAction: vi.fn(),
	},
}));

vi.mock('@/stores/projectStore', () => ({
	useProjectStore: Object.assign(
		vi.fn((selector?: (state: Record<string, unknown>) => unknown) => {
			const state = {
				selectProject: mockSelectProject,
			};
			return selector ? selector(state) : state;
		}),
		{
			getState: vi.fn(() => ({
				selectProject: mockSelectProject,
			})),
		},
	),
}));

vi.mock('@/stores', () => ({
	toast: {
		error: (...args: unknown[]) => mockToastError(...args),
		warning: (...args: unknown[]) => mockToastWarning(...args),
	},
}));

function renderPage() {
	return render(
		<MemoryRouter initialEntries={['/']}>
			<TooltipProvider delayDuration={0}>
				<MyWorkPage />
			</TooltipProvider>
		</MemoryRouter>,
	);
}

describe('CommandCenter MyWorkPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.useRealTimers();
		vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue(
			createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({
					projectId: 'proj-alpha',
					projectName: 'Project Alpha',
					activeThreadCount: 2,
					pendingRecommendations: 2,
					recentCompletions: [
						createMockRecentCompletion({
							id: 'TASK-099',
							title: 'Finished alpha task',
						}),
					],
				}),
			]),
		);
		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockResolvedValue(
			createMockAttentionDashboardResponse(),
		);
		vi.mocked(attentionDashboardClient.performAttentionAction).mockResolvedValue({
			...create(PerformAttentionActionResponseSchema, {
				success: true,
				errorMessage: '',
			}),
		});
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('renders the five command center sections from the cross-project API responses', async () => {
		vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue(
			createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({
					projectId: 'proj-alpha',
					projectName: 'Project Alpha',
					activeThreadCount: 2,
					pendingRecommendations: 2,
					recentCompletions: [
						createMockRecentCompletion({
							id: 'TASK-099',
							title: 'Finished alpha task',
						}),
					],
				}),
			]),
		);
		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockResolvedValue(
			createMockAttentionDashboardResponse({
				runningSummary: create(RunningSummarySchema, {
					taskCount: 1,
					tasks: [
						createMockRunningTask({
							id: 'TASK-001',
							title: 'Alpha run',
							projectId: 'proj-alpha',
							projectName: 'Project Alpha',
						}),
					],
				}),
				attentionItems: [
					createMockAttentionItem({
						id: 'proj-alpha::failed-TASK-002',
						taskId: 'TASK-002',
						title: 'Needs retry',
						type: AttentionItemType.FAILED_TASK,
						projectId: 'proj-alpha',
						signalKind: 'blocker',
					}),
					createMockAttentionItem({
						id: 'proj-alpha::decision-TASK-003',
						taskId: 'TASK-003',
						title: 'Discuss release plan',
						type: AttentionItemType.PENDING_DECISION,
						projectId: 'proj-alpha',
						signalKind: 'decision_request',
					}),
				],
				pendingRecommendations: 2,
			}),
		);

		renderPage();

		await screen.findByText('Command Center');

		expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(1);
		expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledWith({ projectId: '' });
		expect(screen.getByText('Running', { selector: '.command-center-section__title' })).toBeInTheDocument();
		expect(screen.getByText('Attention', { selector: '.command-center-section__title' })).toBeInTheDocument();
		expect(screen.getByText('Discussions', { selector: '.command-center-section__title' })).toBeInTheDocument();
		expect(screen.getByText('Recommendations', { selector: '.command-center-section__title' })).toBeInTheDocument();
		expect(screen.getByText('Recently Completed', { selector: '.command-center-section__title' })).toBeInTheDocument();
		expect(screen.getByText('Alpha run')).toBeInTheDocument();
		expect(screen.getByText('Needs retry')).toBeInTheDocument();
		expect(screen.getByText('Discuss release plan')).toBeInTheDocument();
		expect(screen.getByText('Finished alpha task')).toBeInTheDocument();
		expect(screen.getAllByText('Project Alpha').length).toBeGreaterThan(0);
		expect(screen.getByText('2 pending')).toBeInTheDocument();
	});

	it('renders explicit empty states for every command center section', async () => {
		vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue(
			createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({
					projectId: 'proj-empty',
					projectName: 'Quiet Project',
				}),
			]),
		);
		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockResolvedValue(
			createMockAttentionDashboardResponse(),
		);

		renderPage();

		await screen.findByText('Quiet Project');

		expect(screen.getByText('No tasks running')).toBeInTheDocument();
		expect(screen.getByText('Nothing needs attention')).toBeInTheDocument();
		expect(screen.getByText('No active discussions')).toBeInTheDocument();
		expect(screen.getByText('No pending recommendations')).toBeInTheDocument();
		expect(screen.getByText('No recent completions')).toBeInTheDocument();
	});

	it('renders only the 10 most recent completions across projects', async () => {
		const alphaCompletions = Array.from({ length: 8 }, (_, index) => {
			return createMockRecentCompletion({
				id: `ALPHA-${index + 1}`,
				title: `Alpha completion ${index + 1}`,
				completedAt: createTimestamp(new Date(Date.UTC(2024, 0, 1, 12, 20-index))),
			});
		});
		const betaCompletions = Array.from({ length: 8 }, (_, index) => {
			return createMockRecentCompletion({
				id: `BETA-${index + 1}`,
				title: `Beta completion ${index + 1}`,
				completedAt: createTimestamp(new Date(Date.UTC(2024, 0, 1, 11, 20-index))),
			});
		});

		vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue(
			createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({
					projectId: 'proj-alpha',
					projectName: 'Project Alpha',
					recentCompletions: alphaCompletions,
				}),
				createMockProjectStatus({
					projectId: 'proj-beta',
					projectName: 'Project Beta',
					recentCompletions: betaCompletions,
				}),
			]),
		);

		renderPage();

		await screen.findByText('Command Center');

		expect(screen.getByText('10', { selector: '.command-center-section__count' })).toBeInTheDocument();
		expect(screen.getByText('Alpha completion 1')).toBeInTheDocument();
		expect(screen.getByText('Alpha completion 8')).toBeInTheDocument();
		expect(screen.getByText('Beta completion 1')).toBeInTheDocument();
		expect(screen.getByText('Beta completion 2')).toBeInTheDocument();
		expect(screen.queryByText('Beta completion 3')).not.toBeInTheDocument();
		expect(screen.queryByText('Beta completion 8')).not.toBeInTheDocument();
	});

	it('polls every 15 seconds and refreshes on recommendation and attention signals', async () => {
		vi.mocked(projectClient.getAllProjectsStatus)
			.mockResolvedValueOnce(createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({ projectName: 'Project Alpha' }),
			]))
			.mockResolvedValue(createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({ projectName: 'Project Alpha' }),
			]));
		const setIntervalSpy = vi.spyOn(globalThis, 'setInterval');

		renderPage();

		await screen.findByText('Project Alpha');

		expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 15_000);
		const intervalCallback = setIntervalSpy.mock.calls[0]?.[0];
		expect(intervalCallback).toBeTypeOf('function');

		await act(async () => {
			await intervalCallback?.();
		});

		await waitFor(() => {
			expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(2);
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(2);
		});

		await act(async () => {
			emitRecommendationSignal({
				projectId: 'proj-alpha',
				recommendationId: 'REC-001',
				type: 'created',
			});
		});

		await waitFor(() => {
			expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(3);
		});

		await act(async () => {
			emitAttentionDashboardSignal({
				projectId: 'proj-alpha',
				taskId: 'TASK-001',
				type: 'task-updated',
			});
		});

		await waitFor(() => {
			expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(4);
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(4);
		});
	});

	it('uses composite attention IDs for actions and refreshes after success', async () => {
		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockResolvedValue(
			createMockAttentionDashboardResponse({
				attentionItems: [
					createMockAttentionItem({
						id: 'proj-alpha::failed-TASK-001',
						taskId: 'TASK-001',
						title: 'Retry me',
						projectId: 'proj-alpha',
						signalKind: 'blocker',
					}),
				],
			}),
		);

		renderPage();

		await screen.findByText('Retry me');

		fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		await waitFor(() => {
			expect(attentionDashboardClient.performAttentionAction).toHaveBeenCalledWith({
				projectId: 'proj-alpha',
				attentionItemId: 'proj-alpha::failed-TASK-001',
				action: AttentionAction.RETRY,
				decisionOptionId: '',
			});
		});

		await waitFor(() => {
			expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(2);
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(2);
		});
	});
});
