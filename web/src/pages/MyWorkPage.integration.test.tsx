/**
 * Integration Tests for MyWorkPage - Task Click Wiring
 *
 * Success Criteria Coverage:
 * - SC-3: Clicking a task row sets project context via selectProject() AND navigates to /tasks/:id
 * - SC-4: MyWorkPage passes onTaskClick handler through ProjectCard to TaskRow
 * - SC-10: "View all" link sets project context and navigates to /board
 *
 * INTEGRATION TEST PATTERN:
 * These tests render MyWorkPage (the parent) and verify that clicking a TaskRow
 * (a deeply nested child) triggers the correct actions:
 * 1. selectProject() is called with the task's project ID
 * 2. Navigation to /tasks/:id occurs
 *
 * This catches wiring bugs where:
 * - onTaskClick handler is not passed from MyWorkPage to ProjectCard
 * - ProjectCard doesn't forward the handler to TaskRow
 * - Handler is wired but doesn't call selectProject or navigate
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import { MyWorkPage } from './MyWorkPage';
import { projectClient } from '@/lib/client';
import {
	createMockProjectStatus,
	createMockTaskSummary,
} from '@/test/factories';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { TooltipProvider } from '@/components/ui';

// Track navigate calls
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async (importOriginal) => {
	const actual = await importOriginal<typeof import('react-router-dom')>();
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

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

// Track selectProject calls
const mockSelectProject = vi.fn();
vi.mock('@/stores/projectStore', () => ({
	useProjectStore: Object.assign(
		vi.fn((selector?: (state: Record<string, unknown>) => unknown) => {
			const state = {
				projects: [],
				currentProjectId: null,
				selectProject: mockSelectProject,
				loading: false,
			};
			return selector ? selector(state) : state;
		}),
		{
			getState: vi.fn(() => ({
				selectProject: mockSelectProject,
			})),
		}
	),
	useCurrentProjectId: vi.fn(() => null),
	useProjectLoading: vi.fn(() => false),
	useCurrentProject: vi.fn(() => undefined),
	useProjects: vi.fn(() => []),
}));

const mockProjects = [
	createMockProjectStatus({
		projectId: 'proj-orc',
		projectName: 'orc',
		activeTasks: [
			createMockTaskSummary({
				id: 'TASK-042',
				title: 'Build dashboard',
				status: TaskStatus.RUNNING,
			}),
			createMockTaskSummary({
				id: 'TASK-043',
				title: 'Fix bug',
				status: TaskStatus.BLOCKED,
			}),
		],
		totalTasks: 10,
		completedToday: 2,
	}),
	createMockProjectStatus({
		projectId: 'proj-llmkit',
		projectName: 'llmkit',
		activeTasks: [
			createMockTaskSummary({
				id: 'TASK-100',
				title: 'Add streaming',
				status: TaskStatus.CREATED,
			}),
		],
		totalTasks: 5,
		completedToday: 0,
	}),
];

function renderMyWorkPage() {
	return render(
		<MemoryRouter initialEntries={['/']}>
			<TooltipProvider delayDuration={0}>
				<MyWorkPage />
			</TooltipProvider>
		</MemoryRouter>
	);
}

describe('MyWorkPage Integration - Task Click Wiring', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(projectClient.getAllProjectsStatus).mockResolvedValue({
			projects: mockProjects,
		});
	});

	describe('SC-3 & SC-4: clicking task row sets project context and navigates', () => {
		it('should call selectProject with correct project ID when task is clicked', async () => {
			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-042')).toBeInTheDocument();
			});

			// Click on TASK-042 which belongs to proj-orc
			const taskEl = screen.getByText('TASK-042').closest('[role="button"]') ||
				screen.getByText('TASK-042').closest('.task-row');
			expect(taskEl).toBeInTheDocument();
			fireEvent.click(taskEl!);

			expect(mockSelectProject).toHaveBeenCalledWith('proj-orc');
		});

		it('should navigate to /tasks/:id when task is clicked', async () => {
			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-042')).toBeInTheDocument();
			});

			const taskEl = screen.getByText('TASK-042').closest('[role="button"]') ||
				screen.getByText('TASK-042').closest('.task-row');
			fireEvent.click(taskEl!);

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-042');
		});

		it('should set correct project context for tasks in different projects', async () => {
			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-100')).toBeInTheDocument();
			});

			// Click TASK-100 from llmkit project
			const taskEl = screen.getByText('TASK-100').closest('[role="button"]') ||
				screen.getByText('TASK-100').closest('.task-row');
			fireEvent.click(taskEl!);

			// Should set llmkit project, not orc
			expect(mockSelectProject).toHaveBeenCalledWith('proj-llmkit');
			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-100');
		});

		it('should call BOTH selectProject and navigate (not just one)', async () => {
			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-043')).toBeInTheDocument();
			});

			const taskEl = screen.getByText('TASK-043').closest('[role="button"]') ||
				screen.getByText('TASK-043').closest('.task-row');
			fireEvent.click(taskEl!);

			// Both must be called - this catches partial wiring
			expect(mockSelectProject).toHaveBeenCalledTimes(1);
			expect(mockNavigate).toHaveBeenCalledTimes(1);
		});
	});

	describe('SC-10: view all link navigates to board with project context', () => {
		it('should call selectProject and navigate to /board when "view all" is clicked', async () => {
			renderMyWorkPage();

			await waitFor(() => {
				expect(screen.getByText('orc')).toBeInTheDocument();
			});

			// Find the "view all" link for the orc project card
			const viewAllLinks = screen.getAllByText(/view all/i);
			expect(viewAllLinks.length).toBeGreaterThanOrEqual(1);

			// Click the first "view all" (orc project)
			fireEvent.click(viewAllLinks[0]);

			expect(mockSelectProject).toHaveBeenCalledWith('proj-orc');
			expect(mockNavigate).toHaveBeenCalledWith('/board');
		});
	});
});
