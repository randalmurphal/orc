/**
 * Unit Tests for TaskRow component
 *
 * Success Criteria Coverage:
 * - SC-2: TaskRow renders task ID, title, and status indicator
 * - SC-9: TaskRow displays stale claim warning when isStale is true
 *
 * Edge Cases:
 * - Task with no claimer (claimedByName empty)
 * - Task with stale claim but no claimer name
 * - Very long task title (CSS concern, not tested here)
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TaskRow } from './TaskRow';
import { createMockTaskSummary } from '@/test/factories';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { TooltipProvider } from '@/components/ui';

function renderTaskRow(
	props: Partial<React.ComponentProps<typeof TaskRow>> = {}
) {
	const defaultProps = {
		task: createMockTaskSummary(),
		onClick: vi.fn(),
		...props,
	};
	return render(
		<TooltipProvider delayDuration={0}>
			<TaskRow {...defaultProps} />
		</TooltipProvider>
	);
}

describe('TaskRow', () => {
	describe('SC-2: renders task data correctly', () => {
		it('should render task ID', () => {
			renderTaskRow({
				task: createMockTaskSummary({ id: 'TASK-042' }),
			});
			expect(screen.getByText('TASK-042')).toBeInTheDocument();
		});

		it('should render task title', () => {
			renderTaskRow({
				task: createMockTaskSummary({ title: 'Implement feature X' }),
			});
			expect(screen.getByText('Implement feature X')).toBeInTheDocument();
		});

		it('should render status indicator for running task', () => {
			const { container } = renderTaskRow({
				task: createMockTaskSummary({ status: TaskStatus.RUNNING }),
			});
			// Status indicator should be present - exact styling is CSS
			const statusEl = container.querySelector('[data-status="running"]') ||
				container.querySelector('.task-row__status');
			expect(statusEl).toBeInTheDocument();
		});

		it('should render status indicator for blocked task', () => {
			const { container } = renderTaskRow({
				task: createMockTaskSummary({ status: TaskStatus.BLOCKED }),
			});
			const statusEl = container.querySelector('[data-status="blocked"]') ||
				container.querySelector('.task-row__status');
			expect(statusEl).toBeInTheDocument();
		});

		it('should render status indicator for created task', () => {
			const { container } = renderTaskRow({
				task: createMockTaskSummary({ status: TaskStatus.CREATED }),
			});
			const statusEl = container.querySelector('[data-status="created"]') ||
				container.querySelector('.task-row__status');
			expect(statusEl).toBeInTheDocument();
		});

		it('should call onClick when clicked', () => {
			const onClick = vi.fn();
			renderTaskRow({
				task: createMockTaskSummary({ id: 'TASK-099' }),
				onClick,
			});

			const row = screen.getByText('TASK-099').closest('[role="button"]') ||
				screen.getByText('TASK-099').closest('.task-row');
			expect(row).toBeInTheDocument();
			fireEvent.click(row!);
			expect(onClick).toHaveBeenCalledTimes(1);
		});
	});

	describe('SC-9: stale claim warning', () => {
		it('should show stale warning icon when isStale is true', () => {
			const { container } = renderTaskRow({
				task: createMockTaskSummary({
					isStale: true,
					status: TaskStatus.RUNNING,
					claimedByName: 'bob',
				}),
			});

			// Look for warning indicator - could be an icon, data attribute, or class
			const staleIndicator =
				container.querySelector('[data-stale="true"]') ||
				container.querySelector('.task-row__stale') ||
				container.querySelector('.task-row--stale');
			expect(staleIndicator).toBeInTheDocument();
		});

		it('should NOT show stale warning icon when isStale is false', () => {
			const { container } = renderTaskRow({
				task: createMockTaskSummary({
					isStale: false,
					status: TaskStatus.RUNNING,
					claimedByName: 'bob',
				}),
			});

			const staleIndicator =
				container.querySelector('[data-stale="true"]') ||
				container.querySelector('.task-row__stale');
			expect(staleIndicator).not.toBeInTheDocument();
		});

		it('should show claimer name when present', () => {
			renderTaskRow({
				task: createMockTaskSummary({ claimedByName: 'bob' }),
			});

			expect(screen.getByText('bob')).toBeInTheDocument();
		});
	});

	describe('edge cases', () => {
		it('should not show user indicator when claimedByName is empty', () => {
			renderTaskRow({
				task: createMockTaskSummary({ claimedByName: '' }),
			});

			// No claimer text should appear
			const { container } = renderTaskRow({
				task: createMockTaskSummary({ claimedByName: '' }),
			});
			const claimerEl = container.querySelector('.task-row__claimer');
			// Either not present or empty
			if (claimerEl) {
				expect(claimerEl.textContent).toBe('');
			}
		});

		it('should show stale icon but no claimer text when stale with no claimer', () => {
			const { container } = renderTaskRow({
				task: createMockTaskSummary({
					isStale: true,
					claimedByName: '',
					status: TaskStatus.RUNNING,
				}),
			});

			// Stale indicator should be present
			const staleIndicator =
				container.querySelector('[data-stale="true"]') ||
				container.querySelector('.task-row__stale') ||
				container.querySelector('.task-row--stale');
			expect(staleIndicator).toBeInTheDocument();

			// But no claimer name rendered
			expect(screen.queryByText(/\w+/i, {
				selector: '.task-row__claimer',
			})).not.toBeInTheDocument();
		});
	});
});
