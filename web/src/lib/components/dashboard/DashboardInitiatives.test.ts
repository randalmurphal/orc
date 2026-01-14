import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import DashboardInitiatives from './DashboardInitiatives.svelte';
import type { Initiative } from '$lib/types';

// Mock goto
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

import { goto } from '$app/navigation';

describe('DashboardInitiatives', () => {
	const mockInitiatives: Initiative[] = [
		{
			version: 1,
			id: 'INIT-001',
			title: 'Frontend Migration',
			status: 'active',
			vision: 'Migrate from Vue to Svelte',
			tasks: [
				{ id: 'TASK-001', title: 'Task 1', status: 'completed' },
				{ id: 'TASK-002', title: 'Task 2', status: 'completed' },
				{ id: 'TASK-003', title: 'Task 3', status: 'completed' },
				{ id: 'TASK-004', title: 'Task 4', status: 'completed' },
				{ id: 'TASK-005', title: 'Task 5', status: 'completed' },
				{ id: 'TASK-006', title: 'Task 6', status: 'completed' },
				{ id: 'TASK-007', title: 'Task 7', status: 'running' },
				{ id: 'TASK-008', title: 'Task 8', status: 'planned' }
			],
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z'
		},
		{
			version: 1,
			id: 'INIT-002',
			title: 'Auth System Rework',
			status: 'active',
			vision: 'Implement OAuth2 flow',
			tasks: [
				{ id: 'TASK-010', title: 'Task 10', status: 'completed' },
				{ id: 'TASK-011', title: 'Task 11', status: 'completed' },
				{ id: 'TASK-012', title: 'Task 12', status: 'planned' },
				{ id: 'TASK-013', title: 'Task 13', status: 'planned' },
				{ id: 'TASK-014', title: 'Task 14', status: 'planned' },
				{ id: 'TASK-015', title: 'Task 15', status: 'planned' },
				{ id: 'TASK-016', title: 'Task 16', status: 'planned' },
				{ id: 'TASK-017', title: 'Task 17', status: 'planned' },
				{ id: 'TASK-018', title: 'Task 18', status: 'planned' },
				{ id: 'TASK-019', title: 'Task 19', status: 'planned' }
			],
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-10T00:00:00Z'
		},
		{
			version: 1,
			id: 'INIT-003',
			title: 'API Cleanup',
			status: 'active',
			tasks: [
				{ id: 'TASK-020', title: 'Task 20', status: 'completed' },
				{ id: 'TASK-021', title: 'Task 21', status: 'completed' },
				{ id: 'TASK-022', title: 'Task 22', status: 'completed' },
				{ id: 'TASK-023', title: 'Task 23', status: 'completed' },
				{ id: 'TASK-024', title: 'Task 24', status: 'completed' },
				{ id: 'TASK-025', title: 'Task 25', status: 'planned' },
				{ id: 'TASK-026', title: 'Task 26', status: 'planned' },
				{ id: 'TASK-027', title: 'Task 27', status: 'planned' }
			],
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-12T00:00:00Z'
		}
	];

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('does not render when no initiatives', () => {
			const { container } = render(DashboardInitiatives, {
				props: { initiatives: [] }
			});

			expect(container.querySelector('.initiatives-section')).toBeNull();
		});

		it('renders section with Active Initiatives header', () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			expect(screen.getByText('Active Initiatives')).toBeInTheDocument();
		});

		it('displays count of initiatives', () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			expect(screen.getByText('3')).toBeInTheDocument();
		});

		it('displays initiative titles', () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			expect(screen.getByText('Frontend Migration')).toBeInTheDocument();
			expect(screen.getByText('Auth System Rework')).toBeInTheDocument();
			expect(screen.getByText('API Cleanup')).toBeInTheDocument();
		});

		it('displays progress counts', () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			// Frontend Migration: 6/8 completed
			expect(screen.getByText('6/8')).toBeInTheDocument();
			// Auth System Rework: 2/10 completed
			expect(screen.getByText('2/10')).toBeInTheDocument();
			// API Cleanup: 5/8 completed
			expect(screen.getByText('5/8')).toBeInTheDocument();
		});
	});

	describe('progress colors', () => {
		it('applies high progress color for >75%', () => {
			const highProgress: Initiative[] = [
				{
					...mockInitiatives[0],
					tasks: [
						{ id: 'T1', title: 'T1', status: 'completed' },
						{ id: 'T2', title: 'T2', status: 'completed' },
						{ id: 'T3', title: 'T3', status: 'completed' },
						{ id: 'T4', title: 'T4', status: 'planned' }
					]
				}
			];

			const { container } = render(DashboardInitiatives, {
				props: { initiatives: highProgress }
			});

			const progressFill = container.querySelector('.progress-fill');
			expect(progressFill?.classList.contains('progress-high')).toBe(true);
		});

		it('applies medium progress color for 25-75%', () => {
			const mediumProgress: Initiative[] = [
				{
					...mockInitiatives[0],
					tasks: [
						{ id: 'T1', title: 'T1', status: 'completed' },
						{ id: 'T2', title: 'T2', status: 'planned' },
						{ id: 'T3', title: 'T3', status: 'planned' },
						{ id: 'T4', title: 'T4', status: 'planned' }
					]
				}
			];

			const { container } = render(DashboardInitiatives, {
				props: { initiatives: mediumProgress }
			});

			const progressFill = container.querySelector('.progress-fill');
			expect(progressFill?.classList.contains('progress-medium')).toBe(true);
		});

		it('applies low progress color for <25%', () => {
			const lowProgress: Initiative[] = [
				{
					...mockInitiatives[0],
					tasks: [
						{ id: 'T1', title: 'T1', status: 'completed' },
						{ id: 'T2', title: 'T2', status: 'planned' },
						{ id: 'T3', title: 'T3', status: 'planned' },
						{ id: 'T4', title: 'T4', status: 'planned' },
						{ id: 'T5', title: 'T5', status: 'planned' },
						{ id: 'T6', title: 'T6', status: 'planned' },
						{ id: 'T7', title: 'T7', status: 'planned' },
						{ id: 'T8', title: 'T8', status: 'planned' }
					]
				}
			];

			const { container } = render(DashboardInitiatives, {
				props: { initiatives: lowProgress }
			});

			const progressFill = container.querySelector('.progress-fill');
			expect(progressFill?.classList.contains('progress-low')).toBe(true);
		});
	});

	describe('navigation', () => {
		it('navigates to board filtered by initiative when clicked', async () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			const initiativeRow = screen.getByText('Frontend Migration').closest('button');
			if (initiativeRow) {
				await fireEvent.click(initiativeRow);
			}

			expect(goto).toHaveBeenCalledWith('/board?initiative=INIT-001');
		});
	});

	describe('view all link', () => {
		it('does not show View All when 5 or fewer initiatives', () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives.slice(0, 3) }
			});

			expect(screen.queryByText('View All →')).not.toBeInTheDocument();
		});

		it('shows View All when more than 5 initiatives', () => {
			const manyInitiatives = [
				...mockInitiatives,
				{ ...mockInitiatives[0], id: 'INIT-004', title: 'Init 4' },
				{ ...mockInitiatives[0], id: 'INIT-005', title: 'Init 5' },
				{ ...mockInitiatives[0], id: 'INIT-006', title: 'Init 6' }
			];

			render(DashboardInitiatives, {
				props: { initiatives: manyInitiatives }
			});

			expect(screen.getByText('View All →')).toBeInTheDocument();
		});

		it('navigates to board when View All is clicked', async () => {
			const manyInitiatives = [
				...mockInitiatives,
				{ ...mockInitiatives[0], id: 'INIT-004', title: 'Init 4' },
				{ ...mockInitiatives[0], id: 'INIT-005', title: 'Init 5' },
				{ ...mockInitiatives[0], id: 'INIT-006', title: 'Init 6' }
			];

			render(DashboardInitiatives, {
				props: { initiatives: manyInitiatives }
			});

			const viewAllLink = screen.getByText('View All →');
			await fireEvent.click(viewAllLink);

			expect(goto).toHaveBeenCalledWith('/board');
		});

		it('only shows top 5 initiatives when more exist', () => {
			const manyInitiatives = [
				{ ...mockInitiatives[0], id: 'INIT-001', title: 'Init 1', updated_at: '2024-01-06T00:00:00Z' },
				{ ...mockInitiatives[0], id: 'INIT-002', title: 'Init 2', updated_at: '2024-01-05T00:00:00Z' },
				{ ...mockInitiatives[0], id: 'INIT-003', title: 'Init 3', updated_at: '2024-01-04T00:00:00Z' },
				{ ...mockInitiatives[0], id: 'INIT-004', title: 'Init 4', updated_at: '2024-01-03T00:00:00Z' },
				{ ...mockInitiatives[0], id: 'INIT-005', title: 'Init 5', updated_at: '2024-01-02T00:00:00Z' },
				{ ...mockInitiatives[0], id: 'INIT-006', title: 'Init 6', updated_at: '2024-01-01T00:00:00Z' }
			];

			render(DashboardInitiatives, {
				props: { initiatives: manyInitiatives }
			});

			// Should show first 5 by most recent updated
			expect(screen.getByText('Init 1')).toBeInTheDocument();
			expect(screen.getByText('Init 5')).toBeInTheDocument();
			expect(screen.queryByText('Init 6')).not.toBeInTheDocument();
		});
	});

	describe('sorting', () => {
		it('sorts initiatives by updated_at descending', () => {
			const { container } = render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			const titles = container.querySelectorAll('.initiative-title');
			// mockInitiatives[0] updated_at: 2024-01-15 (most recent)
			// mockInitiatives[2] updated_at: 2024-01-12
			// mockInitiatives[1] updated_at: 2024-01-10
			expect(titles[0].textContent).toBe('Frontend Migration');
			expect(titles[1].textContent).toBe('API Cleanup');
			expect(titles[2].textContent).toBe('Auth System Rework');
		});
	});

	describe('title truncation', () => {
		it('truncates long titles', () => {
			const longTitleInitiative: Initiative[] = [
				{
					...mockInitiatives[0],
					title: 'This is a very long initiative title that should be truncated'
				}
			];

			const { container } = render(DashboardInitiatives, {
				props: { initiatives: longTitleInitiative }
			});

			// Title should be truncated to 30 chars (29 + ellipsis)
			const title = container.querySelector('.initiative-title');
			expect(title?.textContent?.length).toBeLessThanOrEqual(30);
			expect(title?.textContent).toContain('…');
		});
	});

	describe('tooltip', () => {
		it('shows title and vision in tooltip', () => {
			render(DashboardInitiatives, {
				props: { initiatives: mockInitiatives }
			});

			const initiativeRow = screen.getByText('Frontend Migration').closest('button');
			expect(initiativeRow?.getAttribute('title')).toContain('Frontend Migration');
			expect(initiativeRow?.getAttribute('title')).toContain('Migrate from Vue to Svelte');
		});

		it('shows only title in tooltip when no vision', () => {
			const noVisionInitiative: Initiative[] = [
				{
					...mockInitiatives[0],
					vision: undefined
				}
			];

			render(DashboardInitiatives, {
				props: { initiatives: noVisionInitiative }
			});

			const initiativeRow = screen.getByText('Frontend Migration').closest('button');
			expect(initiativeRow?.getAttribute('title')).toBe('Frontend Migration');
		});
	});

	describe('empty tasks', () => {
		it('shows 0/0 for initiative with no tasks', () => {
			const emptyTasksInitiative: Initiative[] = [
				{
					...mockInitiatives[0],
					tasks: []
				}
			];

			render(DashboardInitiatives, {
				props: { initiatives: emptyTasksInitiative }
			});

			expect(screen.getByText('0/0')).toBeInTheDocument();
		});

		it('handles undefined tasks gracefully', () => {
			const undefinedTasksInitiative: Initiative[] = [
				{
					...mockInitiatives[0],
					tasks: undefined
				}
			];

			render(DashboardInitiatives, {
				props: { initiatives: undefinedTasksInitiative }
			});

			expect(screen.getByText('0/0')).toBeInTheDocument();
		});
	});

	describe('finished status', () => {
		it('counts finished tasks as completed', () => {
			const finishedTasksInitiative: Initiative[] = [
				{
					...mockInitiatives[0],
					tasks: [
						{ id: 'T1', title: 'T1', status: 'finished' },
						{ id: 'T2', title: 'T2', status: 'completed' },
						{ id: 'T3', title: 'T3', status: 'planned' },
						{ id: 'T4', title: 'T4', status: 'planned' }
					]
				}
			];

			render(DashboardInitiatives, {
				props: { initiatives: finishedTasksInitiative }
			});

			// 2 completed (finished + completed) out of 4
			expect(screen.getByText('2/4')).toBeInTheDocument();
		});
	});
});
