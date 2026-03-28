import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { MyWorkPage } from './MyWorkPage';
import { TooltipProvider } from '@/components/ui';
import { attentionDashboardClient, projectClient } from '@/lib/client';
import {
	createMockAttentionDashboardResponse,
	createMockAttentionItem,
	createMockGetAllProjectsStatusResponse,
	createMockProjectStatus,
	createMockRunningTask,
} from '@/test/factories';
import {
	AttentionItemType,
	RunningSummarySchema,
} from '@/gen/orc/v1/attention_dashboard_pb';

const mockNavigate = vi.fn();
const mockSelectProject = vi.fn();

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
		error: vi.fn(),
		warning: vi.fn(),
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

describe('CommandCenter navigation', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue(
			createMockGetAllProjectsStatusResponse([
				createMockProjectStatus({
					projectId: 'proj-alpha',
					projectName: 'Project Alpha',
					pendingRecommendations: 2,
				}),
				createMockProjectStatus({
					projectId: 'proj-beta',
					projectName: 'Project Beta',
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
							title: 'Running alpha task',
							projectId: 'proj-alpha',
							projectName: 'Project Alpha',
						}),
					],
				}),
				attentionItems: [
					createMockAttentionItem({
						id: 'proj-beta::failed-TASK-002',
						taskId: 'TASK-002',
						title: 'Blocked beta task',
						projectId: 'proj-beta',
						type: AttentionItemType.FAILED_TASK,
						signalKind: 'blocker',
					}),
				],
				pendingRecommendations: 2,
			}),
		);
	});

	it('selects the project before navigating when a running task is clicked', async () => {
		renderPage();

		await screen.findByText('Running alpha task');

		fireEvent.click(screen.getByRole('button', { name: /Running alpha task/i }));

		expect(mockSelectProject).toHaveBeenCalledWith('proj-alpha');
		expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		expect(mockSelectProject.mock.invocationCallOrder[0]).toBeLessThan(
			mockNavigate.mock.invocationCallOrder[0],
		);
	});

	it('selects the project before navigating when an attention item is opened', async () => {
		renderPage();

		await screen.findByText('Blocked beta task');

		fireEvent.click(screen.getByRole('button', { name: /Blocked beta task/i }));

		expect(mockSelectProject).toHaveBeenCalledWith('proj-beta');
		expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-002');
		expect(mockSelectProject.mock.invocationCallOrder[0]).toBeLessThan(
			mockNavigate.mock.invocationCallOrder[0],
		);
	});

	it('navigates project summary rows to the project home in the selected project scope', async () => {
		renderPage();

		const recommendationSummary = (await screen.findByText('2 pending')).closest('button');
		expect(recommendationSummary).toBeInTheDocument();

		fireEvent.click(recommendationSummary!);

		expect(mockSelectProject).toHaveBeenCalledWith('proj-alpha');
		expect(mockNavigate).toHaveBeenCalledWith('/project');
	});
});
