/**
 * Unit Tests for ProjectCard component
 *
 * Success Criteria Coverage:
 * - SC-2: ProjectCard renders task rows for each active task in the project
 * - SC-10: "View all" link on ProjectCard sets project context and navigates to /board
 *
 * Edge Cases:
 * - Empty active_tasks array: card shows "No active tasks" message
 * - Project with multiple tasks: all TaskRow children render
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ProjectCard } from './ProjectCard';
import { createMockProjectStatus, createMockTaskSummary } from '@/test/factories';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { TooltipProvider } from '@/components/ui';

function renderProjectCard(
	props: Partial<React.ComponentProps<typeof ProjectCard>> = {}
) {
	const defaultProps = {
		project: createMockProjectStatus(),
		onTaskClick: vi.fn(),
		onViewAll: vi.fn(),
		...props,
	};
	return render(
		<TooltipProvider delayDuration={0}>
			<ProjectCard {...defaultProps} />
		</TooltipProvider>
	);
}

describe('ProjectCard', () => {
	describe('SC-2: renders task rows', () => {
		it('should render project name', () => {
			renderProjectCard({
				project: createMockProjectStatus({ projectName: 'My Awesome Project' }),
			});
			expect(screen.getByText('My Awesome Project')).toBeInTheDocument();
		});

		it('should render active task count', () => {
			renderProjectCard({
				project: createMockProjectStatus({
					projectName: 'Test',
					activeTasks: [
						createMockTaskSummary({ id: 'TASK-001' }),
						createMockTaskSummary({ id: 'TASK-002' }),
						createMockTaskSummary({ id: 'TASK-003' }),
					],
				}),
			});
			// Should show task count somewhere in the card header
			expect(screen.getByText(/3/)).toBeInTheDocument();
		});

		it('should render TaskRow for each active task', () => {
			renderProjectCard({
				project: createMockProjectStatus({
					activeTasks: [
						createMockTaskSummary({ id: 'TASK-001', title: 'First task' }),
						createMockTaskSummary({ id: 'TASK-002', title: 'Second task' }),
						createMockTaskSummary({ id: 'TASK-003', title: 'Third task' }),
					],
				}),
			});

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('TASK-003')).toBeInTheDocument();
			expect(screen.getByText('First task')).toBeInTheDocument();
			expect(screen.getByText('Second task')).toBeInTheDocument();
			expect(screen.getByText('Third task')).toBeInTheDocument();
		});

		it('should show total tasks and completed today stats', () => {
			renderProjectCard({
				project: createMockProjectStatus({
					totalTasks: 15,
					completedToday: 3,
				}),
			});

			// Stats should be visible in the card
			expect(screen.getByText(/15/)).toBeInTheDocument();
			expect(screen.getByText(/3/)).toBeInTheDocument();
		});
	});

	describe('SC-10: view all link', () => {
		it('should call onViewAll with project ID when "view all" is clicked', () => {
			const onViewAll = vi.fn();
			renderProjectCard({
				project: createMockProjectStatus({ projectId: 'proj-abc' }),
				onViewAll,
			});

			const viewAllLink = screen.getByText(/view all/i);
			fireEvent.click(viewAllLink);
			expect(onViewAll).toHaveBeenCalledWith('proj-abc');
		});
	});

	describe('task click propagation', () => {
		it('should call onTaskClick when a task row is clicked', () => {
			const onTaskClick = vi.fn();
			renderProjectCard({
				project: createMockProjectStatus({
					projectId: 'proj-abc',
					activeTasks: [
						createMockTaskSummary({ id: 'TASK-042', title: 'Click me' }),
					],
				}),
				onTaskClick,
			});

			const taskEl = screen.getByText('TASK-042').closest('[role="button"]') ||
				screen.getByText('TASK-042').closest('.task-row');
			expect(taskEl).toBeInTheDocument();
			fireEvent.click(taskEl!);
			expect(onTaskClick).toHaveBeenCalledWith('proj-abc', 'TASK-042');
		});
	});

	describe('edge cases', () => {
		it('should show "No active tasks" when activeTasks is empty', () => {
			renderProjectCard({
				project: createMockProjectStatus({
					activeTasks: [],
				}),
			});

			expect(screen.getByText(/no active tasks/i)).toBeInTheDocument();
		});
	});
});
