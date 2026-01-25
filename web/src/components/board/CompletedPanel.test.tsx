/**
 * CompletedPanel Component Tests
 *
 * Tests for:
 * - Basic rendering with completed count and stats
 * - Empty state when count is 0
 * - Token number formatting (K/M suffix)
 * - Cost formatting
 * - Expand/collapse behavior
 * - Task list display
 * - Accessibility
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CompletedPanel } from './CompletedPanel';
import { formatNumber, formatCost } from '@/lib/format';
import type { Task } from '@/lib/types';

// Helper to create mock task
function createMockTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'TASK-001',
		title: 'Test task',
		weight: 'small',
		status: 'completed',
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		...overrides,
	};
}

describe('CompletedPanel', () => {
	describe('basic rendering', () => {
		it('renders panel with completed count and badge', () => {
			render(
				<CompletedPanel
					completedCount={34}
					todayTokens={127000}
					todayCost={2.34}
				/>
			);

			expect(screen.getByText('Completed')).toBeInTheDocument();
			expect(screen.getByText('34')).toBeInTheDocument();
		});

		it('renders green-themed header icon', () => {
			render(
				<CompletedPanel
					completedCount={5}
					todayTokens={50000}
					todayCost={1.0}
				/>
			);

			const iconContainer = document.querySelector('.panel-title-icon.green');
			expect(iconContainer).toBeInTheDocument();
		});

		it('renders green-themed badge', () => {
			render(
				<CompletedPanel
					completedCount={10}
					todayTokens={100000}
					todayCost={2.0}
				/>
			);

			const badge = document.querySelector('.panel-badge.green');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveTextContent('10');
		});
	});

	describe('empty state', () => {
		it('shows empty message when count is 0', () => {
			render(
				<CompletedPanel completedCount={0} todayTokens={0} todayCost={0} />
			);

			expect(screen.getByText('No tasks completed today')).toBeInTheDocument();
		});

		it('does not show badge when count is 0', () => {
			render(
				<CompletedPanel completedCount={0} todayTokens={0} todayCost={0} />
			);

			const badge = document.querySelector('.panel-badge');
			expect(badge).not.toBeInTheDocument();
		});
	});

	describe('expand/collapse', () => {
		it('starts collapsed', () => {
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			expect(
				screen.queryByRole('region', { name: /completed/i })
			).not.toBeInTheDocument();
		});

		it('expands when clicked (with tasks)', async () => {
			const user = userEvent.setup();
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			await user.click(header);

			expect(screen.getByRole('region')).toBeInTheDocument();
		});

		it('collapses when clicked again', async () => {
			const user = userEvent.setup();
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			await user.click(header); // expand
			await user.click(header); // collapse

			expect(
				screen.queryByRole('region')
			).not.toBeInTheDocument();
		});

		it('shows chevron only when tasks are present', () => {
			const { rerender } = render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={[]}
				/>
			);

			expect(document.querySelector('.panel-chevron')).not.toBeInTheDocument();

			rerender(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={[createMockTask()]}
				/>
			);

			expect(document.querySelector('.panel-chevron')).toBeInTheDocument();
		});

		it('disables button when no expandable content', () => {
			render(
				<CompletedPanel
					completedCount={5}
					todayTokens={50000}
					todayCost={1.0}
					recentTasks={[]}
				/>
			);

			const header = screen.getByRole('button');
			expect(header).toBeDisabled();
		});
	});

	describe('expanded content', () => {
		it('shows stats detail when expanded', async () => {
			const user = userEvent.setup();
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={127000}
					todayCost={2.34}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			await user.click(header);

			expect(screen.getByText('127K tokens')).toBeInTheDocument();
			expect(screen.getByText('$2.34')).toBeInTheDocument();
		});

		it('shows task list when expanded', async () => {
			const user = userEvent.setup();
			const tasks = [
				createMockTask({ id: 'TASK-001', title: 'First task' }),
				createMockTask({ id: 'TASK-002', title: 'Second task' }),
			];

			render(
				<CompletedPanel
					completedCount={2}
					todayTokens={50000}
					todayCost={1.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			await user.click(header);

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('First task')).toBeInTheDocument();
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('Second task')).toBeInTheDocument();
		});

		it('shows task title with tooltip', async () => {
			const user = userEvent.setup();
			const longTitle = 'This is a very long task title that should be truncated';
			const tasks = [createMockTask({ title: longTitle })];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			await user.click(header);

			const titleElement = screen.getByText(longTitle);
			expect(titleElement).toHaveAttribute('title', longTitle);
		});
	});

	describe('keyboard navigation', () => {
		it('expands on Enter key', async () => {
			const user = userEvent.setup();
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			header.focus();
			await user.keyboard('{Enter}');

			expect(screen.getByRole('region')).toBeInTheDocument();
		});

		it('expands on Space key', async () => {
			const user = userEvent.setup();
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			header.focus();
			await user.keyboard(' ');

			expect(screen.getByRole('region')).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has aria-expanded attribute when expandable', async () => {
			const user = userEvent.setup();
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			expect(header).toHaveAttribute('aria-expanded', 'false');

			await user.click(header);
			expect(header).toHaveAttribute('aria-expanded', 'true');
		});

		it('has aria-controls linking to body', () => {
			const tasks = [createMockTask()];

			render(
				<CompletedPanel
					completedCount={1}
					todayTokens={10000}
					todayCost={0.5}
					recentTasks={tasks}
				/>
			);

			const header = screen.getByRole('button');
			expect(header).toHaveAttribute('aria-controls', 'completed-panel-body');
		});

		it('has descriptive aria-label', () => {
			render(
				<CompletedPanel
					completedCount={5}
					todayTokens={50000}
					todayCost={1.25}
				/>
			);

			const header = screen.getByRole('button');
			expect(header).toHaveAttribute(
				'aria-label',
				expect.stringContaining('5 tasks')
			);
		});

		it('badge has aria-label for count', () => {
			render(
				<CompletedPanel
					completedCount={10}
					todayTokens={100000}
					todayCost={2.0}
				/>
			);

			const badge = screen.getByLabelText('10 tasks completed');
			expect(badge).toBeInTheDocument();
		});
	});
});

describe('formatNumber', () => {
	it('returns number as-is for values under 1000', () => {
		expect(formatNumber(0)).toBe('0');
		expect(formatNumber(1)).toBe('1');
		expect(formatNumber(999)).toBe('999');
	});

	it('formats thousands with K suffix', () => {
		expect(formatNumber(1000)).toBe('1K');
		expect(formatNumber(1234)).toBe('1.2K');
		expect(formatNumber(5678)).toBe('5.7K');
		expect(formatNumber(9999)).toBe('10K');
	});

	it('formats larger thousands with K suffix', () => {
		expect(formatNumber(10000)).toBe('10K');
		expect(formatNumber(12345)).toBe('12.3K');
		expect(formatNumber(127000)).toBe('127K');
		expect(formatNumber(999999)).toBe('1000K');
	});

	it('formats millions with M suffix', () => {
		expect(formatNumber(1000000)).toBe('1M');
		expect(formatNumber(1234567)).toBe('1.2M');
		expect(formatNumber(5678901)).toBe('5.7M');
	});

	it('formats larger millions with M suffix', () => {
		expect(formatNumber(10000000)).toBe('10M');
		expect(formatNumber(12345678)).toBe('12.3M');
		expect(formatNumber(100000000)).toBe('100M');
	});
});

describe('formatCost', () => {
	it('formats with $ prefix and 2 decimal places', () => {
		expect(formatCost(0)).toBe('$0.00');
		expect(formatCost(1)).toBe('$1.00');
		expect(formatCost(2.34)).toBe('$2.34');
	});

	it('rounds to 2 decimal places', () => {
		expect(formatCost(1.234)).toBe('$1.23');
		expect(formatCost(1.235)).toBe('$1.24');
		expect(formatCost(1.999)).toBe('$2.00');
	});

	it('pads with zeros', () => {
		expect(formatCost(0.1)).toBe('$0.10');
		expect(formatCost(5.5)).toBe('$5.50');
	});

	it('formats large values with K/M suffix', () => {
		expect(formatCost(100.00)).toBe('$100.00');
		expect(formatCost(1234.56)).toBe('$1.2K');
		expect(formatCost(1500000)).toBe('$1.50M');
	});
});
