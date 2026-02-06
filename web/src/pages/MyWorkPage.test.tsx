/**
 * Unit Tests for MyWorkPage
 *
 * Success Criteria Coverage:
 * - SC-1: MyWorkPage fetches and renders cross-project data from getAllProjectsStatus
 * - SC-8: Filter dropdown filters tasks by status category
 *
 * Failure Modes:
 * - API error: page shows error message with retry affordance
 * - Empty projects: page shows empty state with guidance
 * - Network timeout: loading then error state
 *
 * Edge Cases:
 * - Single project with no active tasks
 * - All tasks filtered out by status filter
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { MyWorkPage } from './MyWorkPage';
import { projectClient } from '@/lib/client';
import {
	createMockProjectStatus,
	createMockTaskSummary,
} from '@/test/factories';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { TooltipProvider } from '@/components/ui';

// Mock the API client
vi.mock('@/lib/client', () => ({
	projectClient: {
		getAllProjectsStatus: vi.fn(),
		listProjects: vi.fn().mockResolvedValue({ projects: [] }),
	},
	taskClient: {
		listTasks: vi.fn().mockResolvedValue({ tasks: [] }),
	},
}));

// Mock project store
vi.mock('@/stores/projectStore', () => ({
	useProjectStore: Object.assign(
		vi.fn((selector?: (state: Record<string, unknown>) => unknown) => {
			const state = {
				projects: [],
				currentProjectId: null,
				selectProject: vi.fn(),
				loading: false,
			};
			return selector ? selector(state) : state;
		}),
		{
			getState: vi.fn(() => ({
				selectProject: vi.fn(),
			})),
		}
	),
	useCurrentProjectId: vi.fn(() => null),
	useProjectLoading: vi.fn(() => false),
	useCurrentProject: vi.fn(() => undefined),
	useProjects: vi.fn(() => []),
}));

function renderMyWorkPage() {
	return render(
		<MemoryRouter initialEntries={['/']}>
			<TooltipProvider delayDuration={0}>
				<MyWorkPage />
			</TooltipProvider>
		</MemoryRouter>
	);
}

describe('MyWorkPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-1: fetches and renders cross-project data', () => {
		it('should call getAllProjectsStatus on mount', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: [],
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(1);
			});
		});

		it('should render project cards for each project with active tasks', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: [
					createMockProjectStatus({
						projectId: 'proj-1',
						projectName: 'Project Alpha',
						activeTasks: [
							createMockTaskSummary({ id: 'TASK-001', title: 'Alpha task 1' }),
						],
					}),
					createMockProjectStatus({
						projectId: 'proj-2',
						projectName: 'Project Beta',
						activeTasks: [
							createMockTaskSummary({ id: 'TASK-010', title: 'Beta task 1' }),
							createMockTaskSummary({ id: 'TASK-011', title: 'Beta task 2' }),
						],
					}),
				],
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('Project Alpha')).toBeInTheDocument();
				expect(screen.getByText('Project Beta')).toBeInTheDocument();
			});

			// Task rows should be visible
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('TASK-010')).toBeInTheDocument();
			expect(screen.getByText('TASK-011')).toBeInTheDocument();
		});

		it('should show loading state while fetching', () => {
			// Create a promise that never resolves to keep loading state
			vi.mocked(projectClient.getAllProjectsStatus).mockReturnValue(
				new Promise(() => {})
			);

			renderMyWorkPage();

			// Should show some loading indicator
			const loader = screen.queryByRole('progressbar') ||
				screen.queryByText(/loading/i) ||
				document.querySelector('.page-loader');
			expect(loader).toBeInTheDocument();
		});
	});

	describe('SC-8: status filter', () => {
		const projectsWithMixedStatuses = [
			createMockProjectStatus({
				projectId: 'proj-1',
				projectName: 'Project One',
				activeTasks: [
					createMockTaskSummary({
						id: 'TASK-001',
						title: 'Running task',
						status: TaskStatus.RUNNING,
					}),
					createMockTaskSummary({
						id: 'TASK-002',
						title: 'Blocked task',
						status: TaskStatus.BLOCKED,
					}),
				],
			}),
			createMockProjectStatus({
				projectId: 'proj-2',
				projectName: 'Project Two',
				activeTasks: [
					createMockTaskSummary({
						id: 'TASK-003',
						title: 'Created task',
						status: TaskStatus.CREATED,
					}),
				],
			}),
		];

		it('should show all tasks by default (All filter)', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: projectsWithMixedStatuses,
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('TASK-003')).toBeInTheDocument();
		});

		it('should filter to show only running tasks when Running is selected', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: projectsWithMixedStatuses,
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Find and click the filter dropdown, select "Running"
			const filterSelect = screen.getByRole('combobox') ||
				screen.getByLabelText(/filter/i);
			fireEvent.change(filterSelect, { target: { value: 'running' } });

			// Only running task should be visible
			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
				expect(screen.queryByText('TASK-003')).not.toBeInTheDocument();
			});
		});

		it('should filter to show only blocked tasks when Blocked is selected', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: projectsWithMixedStatuses,
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			const filterSelect = screen.getByRole('combobox') ||
				screen.getByLabelText(/filter/i);
			fireEvent.change(filterSelect, { target: { value: 'blocked' } });

			await waitFor(() => {
				expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
				expect(screen.queryByText('TASK-003')).not.toBeInTheDocument();
			});
		});

		it('should hide projects with no matching tasks after filtering', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: projectsWithMixedStatuses,
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('Project One')).toBeInTheDocument();
				expect(screen.getByText('Project Two')).toBeInTheDocument();
			});

			// Filter to Running - only Project One has running tasks
			const filterSelect = screen.getByRole('combobox') ||
				screen.getByLabelText(/filter/i);
			fireEvent.change(filterSelect, { target: { value: 'running' } });

			await waitFor(() => {
				expect(screen.getByText('Project One')).toBeInTheDocument();
				// Project Two has no running tasks, should be hidden
				expect(screen.queryByText('Project Two')).not.toBeInTheDocument();
			});
		});

		it('should show "No matching tasks" when all tasks are filtered out', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: [
					createMockProjectStatus({
						activeTasks: [
							createMockTaskSummary({ status: TaskStatus.CREATED }),
						],
					}),
				],
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Filter to "Running" - but no running tasks exist
			const filterSelect = screen.getByRole('combobox') ||
				screen.getByLabelText(/filter/i);
			fireEvent.change(filterSelect, { target: { value: 'running' } });

			await waitFor(() => {
				expect(screen.getByText(/no matching tasks/i)).toBeInTheDocument();
			});
		});

		it('should show all tasks again when filter is reset to All', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: projectsWithMixedStatuses,
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			const filterSelect = screen.getByRole('combobox') ||
				screen.getByLabelText(/filter/i);

			// Filter to Running
			fireEvent.change(filterSelect, { target: { value: 'running' } });
			await waitFor(() => {
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
			});

			// Reset to All
			fireEvent.change(filterSelect, { target: { value: 'all' } });
			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
				expect(screen.getByText('TASK-003')).toBeInTheDocument();
			});
		});
	});

	describe('failure modes', () => {
		it('should show error message when API call fails', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockRejectedValue(
				new Error('Network error')
			);

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText(/error/i)).toBeInTheDocument();
			});
		});

		it('should show retry button when API call fails', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockRejectedValue(
				new Error('Network error')
			);

			renderMyWorkPage();

			await waitFor(() => {
				const retryButton = screen.getByRole('button', { name: /retry/i });
				expect(retryButton).toBeInTheDocument();
			});
		});

		it('should re-fetch when retry button is clicked', async () => {
			vi.mocked(projectClient.getAllProjectsStatus)
				.mockRejectedValueOnce(new Error('Network error'))
				.mockResolvedValueOnce({
					projects: [
						createMockProjectStatus({ projectName: 'Recovered Project' }),
					],
				});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(projectClient.getAllProjectsStatus).toHaveBeenCalledTimes(2);
			});
		});

		it('should show empty state when API returns no projects', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: [],
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText(/no projects/i)).toBeInTheDocument();
			});
			// Should show guidance about running orc init
			expect(screen.getByText(/orc init/i)).toBeInTheDocument();
		});
	});

	describe('edge cases', () => {
		it('should render single project with no active tasks', async () => {
			vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
				projects: [
					createMockProjectStatus({
						projectName: 'Empty Project',
						activeTasks: [],
						totalTasks: 5,
					}),
				],
			});

			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('Empty Project')).toBeInTheDocument();
			});
			expect(screen.getByText(/no active tasks/i)).toBeInTheDocument();
		});
	});
});
