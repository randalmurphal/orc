import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TooltipProvider } from '@/components/ui';
import { ProjectHomePage } from './ProjectHomePage';
import { attentionDashboardClient, dashboardClient } from '@/lib/client';
import { listRecommendations } from '@/lib/api/recommendation';
import { emitAttentionDashboardSignal } from '@/lib/events/attentionDashboardSignals';
import { emitRecommendationSignal } from '@/lib/events/recommendationSignals';
import { useProjectStore } from '@/stores/projectStore';
import { useThreadStore } from '@/stores/threadStore';
import {
	createMockAttentionDashboardResponse,
	createMockAttentionItem,
	createMockDashboardStats,
	createMockRecommendation,
	createMockRecentCompletion,
	createMockRunningTask,
	createMockThread,
	createTimestamp,
} from '@/test/factories';
import type { GetAttentionDashboardDataResponse } from '@/gen/orc/v1/attention_dashboard_pb';
import { AttentionAction, AttentionItemType, PerformAttentionActionResponseSchema, RunningSummarySchema } from '@/gen/orc/v1/attention_dashboard_pb';
import type { GetStatsResponse } from '@/gen/orc/v1/dashboard_pb';
import { GetStatsResponseSchema } from '@/gen/orc/v1/dashboard_pb';
import type { ListRecommendationsResponse } from '@/gen/orc/v1/recommendation_pb';
import { RecommendationStatus, ListRecommendationsResponseSchema } from '@/gen/orc/v1/recommendation_pb';
import { HandoffSourceType } from '@/gen/orc/v1/handoff_pb';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';

const mockToastError = vi.fn();
const mockToastWarning = vi.fn();
let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

vi.mock('@/lib/client', () => ({
	attentionDashboardClient: {
		getAttentionDashboardData: vi.fn(),
		performAttentionAction: vi.fn(),
	},
	dashboardClient: {
		getStats: vi.fn(),
	},
}));

vi.mock('@/lib/api/recommendation', () => ({
	listRecommendations: vi.fn(),
}));

vi.mock('@/stores', () => ({
	toast: {
		error: (...args: unknown[]) => mockToastError(...args),
		warning: (...args: unknown[]) => mockToastWarning(...args),
	},
}));

vi.mock('@/components/handoff/HandoffActions', () => ({
	HandoffActions: ({
		projectId,
		sourceType,
		sourceId,
	}: {
		projectId?: string;
		sourceType: number;
		sourceId: string;
	}) => (
		<div
			data-testid="handoff-actions"
			data-project-id={projectId ?? ''}
			data-source-type={String(sourceType)}
			data-source-id={sourceId}
		/>
	),
}));

async function renderPage() {
	await act(async () => {
		render(
			<MemoryRouter initialEntries={['/project']}>
				<TooltipProvider delayDuration={0}>
					<ProjectHomePage />
				</TooltipProvider>
			</MemoryRouter>,
		);
	});
}

function deferred<T>() {
	let resolve: (value: T) => void;
	let reject: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return {
		promise,
		resolve: resolve!,
		reject: reject!,
	};
}

describe('ProjectHomePage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		useProjectStore.setState({
			projects: [
				create(ProjectSchema, {
					id: 'proj-alpha',
					name: 'Project Alpha',
					path: '/tmp/project-alpha',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
				create(ProjectSchema, {
					id: 'proj-beta',
					name: 'Project Beta',
					path: '/tmp/project-beta',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
			],
			currentProjectId: 'proj-alpha',
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useThreadStore.setState({
			threads: [
				createMockThread({
					id: 'thread-alpha',
					title: 'Release review',
					taskId: 'TASK-020',
				}),
			],
			selectedThreadId: null,
			loading: false,
			error: null,
		});
		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockResolvedValue(
			createMockAttentionDashboardResponse({
				runningSummary: create(RunningSummarySchema, {
					taskCount: 1,
					tasks: [
						createMockRunningTask({
							id: 'TASK-010',
							title: 'Ship project home',
							projectId: 'proj-alpha',
							projectName: 'Project Alpha',
						}),
					],
				}),
				attentionItems: [
					createMockAttentionItem({
						id: 'proj-alpha::failed-TASK-011',
						taskId: 'TASK-011',
						title: 'Retry browser QA',
						projectId: 'proj-alpha',
						type: AttentionItemType.FAILED_TASK,
						signalKind: 'blocker',
					}),
					createMockAttentionItem({
						id: 'proj-alpha::decision-TASK-012',
						taskId: 'TASK-012',
						title: 'Discuss the rollout copy',
						projectId: 'proj-alpha',
						type: AttentionItemType.PENDING_DECISION,
						signalKind: 'decision_request',
					}),
				],
			}),
		);
		vi.mocked(listRecommendations).mockResolvedValue(
			create(ListRecommendationsResponseSchema, {
				recommendations: [
					createMockRecommendation({
						id: 'REC-100',
						title: 'Document the operator handoff',
						sourceTaskId: 'TASK-010',
						status: RecommendationStatus.PENDING,
					}),
				],
			}),
		);
		vi.mocked(dashboardClient.getStats).mockResolvedValue(
			create(GetStatsResponseSchema, {
				stats: createMockDashboardStats({
					recentCompletions: [
						createMockRecentCompletion({
							id: 'TASK-013',
							title: 'Finished the review pass',
						}),
					],
				}),
			}),
		);
		vi.mocked(attentionDashboardClient.performAttentionAction).mockResolvedValue(
			create(PerformAttentionActionResponseSchema, {
				success: true,
				errorMessage: '',
			}),
		);
	});

	afterEach(() => {
		consoleErrorSpy.mockRestore();
	});

	it('renders all five sections with project-scoped data and empty states', async () => {
		await renderPage();

		await screen.findByRole('heading', { name: 'Project Alpha' });

		expect(screen.getByRole('heading', { name: 'Running Tasks' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Needs Attention' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Recommendations' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Discussions' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Recently Completed' })).toBeInTheDocument();
		expect(screen.getByText('Ship project home')).toBeInTheDocument();
		expect(screen.getByText('Retry browser QA')).toBeInTheDocument();
		expect(screen.getByText('Document the operator handoff')).toBeInTheDocument();
		expect(screen.getByText('Release review')).toBeInTheDocument();
		expect(screen.getByText('Finished the review pass')).toBeInTheDocument();

		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockResolvedValueOnce(
			createMockAttentionDashboardResponse(),
		);
		vi.mocked(listRecommendations).mockResolvedValueOnce(
			create(ListRecommendationsResponseSchema, { recommendations: [] }),
		);
		vi.mocked(dashboardClient.getStats).mockResolvedValueOnce(
			create(GetStatsResponseSchema, {
				stats: createMockDashboardStats({ recentCompletions: [] }),
			}),
		);
		useThreadStore.setState({ threads: [] });

		await act(async () => {
			await useProjectStore.getState().selectProject('proj-beta');
		});

		await screen.findByRole('heading', { name: 'Project Beta' });

		expect(screen.getByText('No tasks running')).toBeInTheDocument();
		expect(screen.getByText('Nothing needs attention')).toBeInTheDocument();
		expect(screen.getByText('No pending recommendations')).toBeInTheDocument();
		expect(screen.getByText('No active discussions')).toBeInTheDocument();
		expect(screen.getByText('No recent completions')).toBeInTheDocument();
	});

	it('renders handoff actions for running tasks, attention items, and recommendations with the correct source wiring', async () => {
		await renderPage();

		await screen.findByText('Ship project home');

		const handoffActions = screen.getAllByTestId('handoff-actions');
		expect(handoffActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					dataset: expect.objectContaining({
						projectId: 'proj-alpha',
						sourceType: String(HandoffSourceType.TASK),
						sourceId: 'TASK-010',
					}),
				}),
				expect.objectContaining({
					dataset: expect.objectContaining({
						projectId: 'proj-alpha',
						sourceType: String(HandoffSourceType.ATTENTION_ITEM),
						sourceId: 'proj-alpha::failed-TASK-011',
					}),
				}),
				expect.objectContaining({
					dataset: expect.objectContaining({
						projectId: 'proj-alpha',
						sourceType: String(HandoffSourceType.RECOMMENDATION),
						sourceId: 'REC-100',
					}),
				}),
			]),
		);
	});

	it('dispatches attention actions with the current project scope and refreshes after success', async () => {
		await renderPage();

		await screen.findByText('Retry browser QA');

		fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		await waitFor(() => {
			expect(attentionDashboardClient.performAttentionAction).toHaveBeenCalledWith({
				projectId: 'proj-alpha',
				attentionItemId: 'proj-alpha::failed-TASK-011',
				action: AttentionAction.RETRY,
				decisionOptionId: '',
			});
		});

		await waitFor(() => {
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(2);
		});
	});

	it('ignores stale responses from the previous project after a project switch', async () => {
		const alphaAttention = deferred<ReturnType<typeof createMockAttentionDashboardResponse>>();
		const betaAttention = deferred<ReturnType<typeof createMockAttentionDashboardResponse>>();
		const alphaRecommendations = deferred<ReturnType<typeof create<typeof ListRecommendationsResponseSchema>>>();
		const betaRecommendations = deferred<ReturnType<typeof create<typeof ListRecommendationsResponseSchema>>>();
		const alphaStats = deferred<ReturnType<typeof create<typeof GetStatsResponseSchema>>>();
		const betaStats = deferred<ReturnType<typeof create<typeof GetStatsResponseSchema>>>();

		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockImplementation(({ projectId }) => {
			return projectId === 'proj-alpha' ? alphaAttention.promise : betaAttention.promise;
		});
		vi.mocked(listRecommendations).mockImplementation((projectId) => {
			return projectId === 'proj-alpha' ? alphaRecommendations.promise : betaRecommendations.promise;
		});
		vi.mocked(dashboardClient.getStats).mockImplementation(({ projectId }) => {
			return projectId === 'proj-alpha' ? alphaStats.promise : betaStats.promise;
		});

		await renderPage();

		await act(async () => {
			useProjectStore.getState().selectProject('proj-beta');
		});

		await act(async () => {
			alphaAttention.resolve(createMockAttentionDashboardResponse({
				runningSummary: create(RunningSummarySchema, {
					taskCount: 1,
					tasks: [createMockRunningTask({ id: 'TASK-ALPHA', title: 'Alpha only', projectId: 'proj-alpha', projectName: 'Project Alpha' })],
				}),
			}));
			alphaRecommendations.resolve(create(ListRecommendationsResponseSchema, {
				recommendations: [createMockRecommendation({ id: 'REC-ALPHA', title: 'Alpha recommendation' })],
			}));
			alphaStats.resolve(create(GetStatsResponseSchema, {
				stats: createMockDashboardStats({
					recentCompletions: [createMockRecentCompletion({ id: 'TASK-ALPHA-DONE', title: 'Alpha completion' })],
				}),
			}));
		});

		expect(screen.queryByText('Alpha only')).not.toBeInTheDocument();
		expect(screen.queryByText('Alpha recommendation')).not.toBeInTheDocument();
		expect(screen.queryByText('Alpha completion')).not.toBeInTheDocument();

		await act(async () => {
			betaAttention.resolve(createMockAttentionDashboardResponse({
				runningSummary: create(RunningSummarySchema, {
					taskCount: 1,
					tasks: [createMockRunningTask({ id: 'TASK-BETA', title: 'Beta only', projectId: 'proj-beta', projectName: 'Project Beta' })],
				}),
			}));
			betaRecommendations.resolve(create(ListRecommendationsResponseSchema, {
				recommendations: [createMockRecommendation({ id: 'REC-BETA', title: 'Beta recommendation' })],
			}));
			betaStats.resolve(create(GetStatsResponseSchema, {
				stats: createMockDashboardStats({
					recentCompletions: [createMockRecentCompletion({ id: 'TASK-BETA-DONE', title: 'Beta completion' })],
				}),
			}));
		});

		await screen.findByRole('heading', { name: 'Project Beta' });
		expect(screen.getByText('Beta only')).toBeInTheDocument();
		expect(screen.getByText('Beta recommendation')).toBeInTheDocument();
		expect(screen.getByText('Beta completion')).toBeInTheDocument();
	});

	it('coalesces rapid attention and recommendation signals into one in-flight refresh plus one queued refresh', async () => {
		const refreshAttentionOne = deferred<GetAttentionDashboardDataResponse>();
		const refreshAttentionTwo = deferred<GetAttentionDashboardDataResponse>();
		const refreshRecommendationsOne = deferred<ListRecommendationsResponse>();
		const refreshRecommendationsTwo = deferred<ListRecommendationsResponse>();
		const refreshStatsOne = deferred<GetStatsResponse>();
		const refreshStatsTwo = deferred<GetStatsResponse>();

		let refreshCount = 0;
		vi.mocked(attentionDashboardClient.getAttentionDashboardData).mockImplementation(() => {
			refreshCount += 1;
			if (refreshCount === 1) {
				return Promise.resolve(createMockAttentionDashboardResponse());
			}
			return refreshCount === 2 ? refreshAttentionOne.promise : refreshAttentionTwo.promise;
		});
		vi.mocked(listRecommendations).mockImplementation(() => {
			if (refreshCount <= 1) {
				return Promise.resolve(create(ListRecommendationsResponseSchema, { recommendations: [] }));
			}
			return refreshCount === 2 ? refreshRecommendationsOne.promise : refreshRecommendationsTwo.promise;
		});
		vi.mocked(dashboardClient.getStats).mockImplementation(() => {
			if (refreshCount <= 1) {
				return Promise.resolve(create(GetStatsResponseSchema, { stats: createMockDashboardStats() }));
			}
			return refreshCount === 2 ? refreshStatsOne.promise : refreshStatsTwo.promise;
		});

		await renderPage();
		await screen.findByRole('heading', { name: 'Project Alpha' });

		await act(async () => {
			for (let index = 0; index < 3; index += 1) {
				emitAttentionDashboardSignal({
					projectId: 'proj-alpha',
					type: 'task-updated',
				});
			}
			for (let index = 0; index < 2; index += 1) {
				emitRecommendationSignal({
					projectId: 'proj-alpha',
					recommendationId: `REC-${index}`,
					type: 'created',
				});
			}
		});

		await waitFor(() => {
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(2);
		});

		await act(async () => {
			refreshAttentionOne.resolve(createMockAttentionDashboardResponse());
			refreshRecommendationsOne.resolve(create(ListRecommendationsResponseSchema, { recommendations: [] }));
			refreshStatsOne.resolve(create(GetStatsResponseSchema, { stats: createMockDashboardStats() }));
		});

		await waitFor(() => {
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(3);
		});

		await act(async () => {
			refreshAttentionTwo.resolve(createMockAttentionDashboardResponse());
			refreshRecommendationsTwo.resolve(create(ListRecommendationsResponseSchema, { recommendations: [] }));
			refreshStatsTwo.resolve(create(GetStatsResponseSchema, { stats: createMockDashboardStats() }));
		});

		await waitFor(() => {
			expect(attentionDashboardClient.getAttentionDashboardData).toHaveBeenCalledTimes(3);
			expect(listRecommendations).toHaveBeenCalledTimes(3);
			expect(dashboardClient.getStats).toHaveBeenCalledTimes(3);
		});
	});

	it('renders the remaining sections when one API fails instead of showing a full-page error', async () => {
		vi.mocked(listRecommendations).mockRejectedValue(new Error('Recommendation service unavailable'));

		await renderPage();

		await screen.findByText('Ship project home');

		expect(screen.getByText('Recommendation service unavailable')).toBeInTheDocument();
		expect(screen.getByText('Retry browser QA')).toBeInTheDocument();
		expect(screen.getByText('Release review')).toBeInTheDocument();
		expect(screen.getByText('Finished the review pass')).toBeInTheDocument();
		expect(screen.queryByText('Select a project to open the project home.')).not.toBeInTheDocument();
	});

	it('shows thread items from the thread store and selects a thread when clicked', async () => {
		await renderPage();

		await screen.findByText('Release review');

		fireEvent.click(screen.getByRole('button', { name: /release review/i }));

		expect(useThreadStore.getState().selectedThreadId).toBe('thread-alpha');
	});
});
