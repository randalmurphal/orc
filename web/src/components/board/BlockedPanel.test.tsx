import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BlockedPanel } from './BlockedPanel';
import { createMockTask } from '@/test/factories';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

describe('BlockedPanel', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('empty state', () => {
		it('returns null when no blocked tasks', () => {
			const onSkip = vi.fn();
			const onForce = vi.fn();
			const { container } = render(
				<BlockedPanel tasks={[]} onSkip={onSkip} onForce={onForce} />
			);

			expect(container.firstChild).toBeNull();
		});

		it('does not render when tasks array is empty', () => {
			const onSkip = vi.fn();
			const onForce = vi.fn();
			render(<BlockedPanel tasks={[]} onSkip={onSkip} onForce={onForce} />);

			expect(screen.queryByText('Blocked')).not.toBeInTheDocument();
		});
	});

	describe('rendering with blocked tasks', () => {
		it('renders section header with "Blocked" title', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: 'Blocked task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			expect(screen.getByText('Blocked')).toBeInTheDocument();
		});

		it('renders count badge with correct number of tasks', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
				createMockTask({
					id: 'TASK-003',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-004'],
				}),
				createMockTask({
					id: 'TASK-005',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-006'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			expect(screen.getByText('3')).toBeInTheDocument();
		});

		it('renders task ID in monospace format', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-123',
					title: 'Test task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-456'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const taskIdElement = container.querySelector('.blocked-id');
			expect(taskIdElement).toHaveTextContent('TASK-123');
		});

		it('renders task title', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: 'Implement user authentication',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			expect(screen.getByText('Implement user authentication')).toBeInTheDocument();
		});

		it('renders multiple blocked tasks', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: 'First blocked task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-100'],
				}),
				createMockTask({
					id: 'TASK-002',
					title: 'Second blocked task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-200'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			expect(screen.getByText('First blocked task')).toBeInTheDocument();
			expect(screen.getByText('Second blocked task')).toBeInTheDocument();
		});
	});

	describe('blocking reason formatting', () => {
		it('formats single task ID blocker with "Waiting for" prefix', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-999'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			// Check the full text content of the reason element
			const reasonElement = container.querySelector('.blocked-reason');
			expect(reasonElement?.textContent).toContain('Waiting for');
			expect(reasonElement?.textContent).toContain('TASK-999');
		});

		it('formats single initiative ID blocker with "Waiting for" prefix', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['INIT-001'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const reasonElement = container.querySelector('.blocked-reason');
			expect(reasonElement?.textContent).toContain('Waiting for');
			expect(reasonElement?.textContent).toContain('INIT-001');
		});

		it('renders task ID in code tags for single blocker', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-999'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const codeElement = container.querySelector('.blocked-reason code');
			expect(codeElement).toHaveTextContent('TASK-999');
		});

		it('renders multiple blockers as a list', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-100', 'TASK-200', 'TASK-300'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const listElement = container.querySelector('.blocked-reason-list');
			expect(listElement).toBeInTheDocument();

			const listItems = listElement?.querySelectorAll('li');
			expect(listItems).toHaveLength(3);
		});

		it('renders multiple blocker IDs in code tags within list', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-100', 'INIT-200'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const codeElements = container.querySelectorAll('.blocked-reason-list code');
			expect(codeElements).toHaveLength(2);
			expect(codeElements[0]).toHaveTextContent('TASK-100');
			expect(codeElements[1]).toHaveTextContent('INIT-200');
		});

		it('renders non-ID blockers as plain text', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['External dependency not met'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const reasonElement = container.querySelector('.blocked-reason');
			expect(reasonElement?.textContent).toBe('External dependency not met');
		});

		it('shows "Unknown blocker" when blockers arrays are empty', () => {
			// Create a task with no blockers set (will default to empty arrays)
			const task = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.BLOCKED,
			});
			// Ensure both arrays are empty
			task.blockedBy = [];
			task.unmetBlockers = [];

			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={[task]} onSkip={onSkip} onForce={onForce} />);

			expect(screen.getByText('Unknown blocker')).toBeInTheDocument();
		});

		it('prefers unmetBlockers over blockedBy', () => {
			const task = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.BLOCKED,
			});
			// Set both arrays - unmetBlockers should be used
			task.unmetBlockers = ['TASK-555'];
			task.blockedBy = ['TASK-666'];

			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={[task]} onSkip={onSkip} onForce={onForce} />
			);

			const codeElement = container.querySelector('.blocked-reason code');
			expect(codeElement).toHaveTextContent('TASK-555');
		});
	});

	describe('collapsible behavior', () => {
		it('starts expanded by default', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: 'Test task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const panel = container.querySelector('.blocked-panel');
			expect(panel).not.toHaveClass('collapsed');
		});

		it('collapses when header is clicked', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: 'Test task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const header = container.querySelector('.panel-header');
			fireEvent.click(header!);

			const panel = container.querySelector('.blocked-panel');
			expect(panel).toHaveClass('collapsed');
		});

		it('expands when header is clicked again', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: 'Test task',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const header = container.querySelector('.panel-header');
			fireEvent.click(header!);
			fireEvent.click(header!);

			const panel = container.querySelector('.blocked-panel');
			expect(panel).not.toHaveClass('collapsed');
		});

		it('updates aria-expanded when collapsed', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const header = container.querySelector('.panel-header');
			expect(header).toHaveAttribute('aria-expanded', 'true');

			fireEvent.click(header!);

			expect(header).toHaveAttribute('aria-expanded', 'false');
		});
	});

	describe('action buttons', () => {
		it('renders Skip button for each task', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
				createMockTask({
					id: 'TASK-003',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-004'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const skipButtons = screen.getAllByRole('button', { name: /skip block for/i });
			expect(skipButtons).toHaveLength(2);
		});

		it('renders Force button for each task', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
				createMockTask({
					id: 'TASK-003',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-004'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButtons = screen.getAllByRole('button', { name: /force run/i });
			expect(forceButtons).toHaveLength(2);
		});

		it('calls onSkip with task ID when Skip button is clicked', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const skipButton = screen.getByRole('button', { name: /skip block for TASK-001/i });
			fireEvent.click(skipButton);

			expect(onSkip).toHaveBeenCalledTimes(1);
			expect(onSkip).toHaveBeenCalledWith('TASK-001');
		});

		it('calls onSkip with correct task ID for multiple tasks', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-010'],
				}),
				createMockTask({
					id: 'TASK-002',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-020'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const skipButton2 = screen.getByRole('button', { name: /skip block for TASK-002/i });
			fireEvent.click(skipButton2);

			expect(onSkip).toHaveBeenCalledWith('TASK-002');
		});
	});

	describe('force confirmation modal', () => {
		it('shows confirmation modal when Force button is clicked', async () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButton = screen.getByRole('button', { name: /force run TASK-001/i });
			fireEvent.click(forceButton);

			await waitFor(() => {
				expect(screen.getByText('Force Run Task?')).toBeInTheDocument();
			});
		});

		it('displays task ID in confirmation modal', async () => {
			const tasks = [
				createMockTask({
					id: 'TASK-123',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButton = screen.getByRole('button', { name: /force run TASK-123/i });
			fireEvent.click(forceButton);

			await waitFor(() => {
				const modalCode = screen.getByRole('dialog').querySelector('code');
				expect(modalCode).toHaveTextContent('TASK-123');
			});
		});

		it('displays warning message about blocked dependencies', async () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButton = screen.getByRole('button', { name: /force run/i });
			fireEvent.click(forceButton);

			await waitFor(() => {
				expect(
					screen.getByText(/blocked by incomplete dependencies/i)
				).toBeInTheDocument();
			});
		});

		it('dismisses modal when Cancel is clicked', async () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButton = screen.getByRole('button', { name: /force run/i });
			fireEvent.click(forceButton);

			await waitFor(() => {
				expect(screen.getByText('Force Run Task?')).toBeInTheDocument();
			});

			const cancelButton = screen.getByRole('button', { name: 'Cancel' });
			fireEvent.click(cancelButton);

			await waitFor(() => {
				expect(screen.queryByText('Force Run Task?')).not.toBeInTheDocument();
			});

			expect(onForce).not.toHaveBeenCalled();
		});

		it('calls onForce and closes modal when Force Run is confirmed', async () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButton = screen.getByRole('button', { name: /force run TASK-001/i });
			fireEvent.click(forceButton);

			await waitFor(() => {
				expect(screen.getByText('Force Run Task?')).toBeInTheDocument();
			});

			const confirmButton = screen.getByRole('button', { name: 'Force Run' });
			fireEvent.click(confirmButton);

			await waitFor(() => {
				expect(onForce).toHaveBeenCalledTimes(1);
				expect(onForce).toHaveBeenCalledWith('TASK-001');
			});

			await waitFor(() => {
				expect(screen.queryByText('Force Run Task?')).not.toBeInTheDocument();
			});
		});

		it('confirms force for correct task when multiple tasks exist', async () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-010'],
				}),
				createMockTask({
					id: 'TASK-002',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-020'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			// Click force on second task
			const forceButton = screen.getByRole('button', { name: /force run TASK-002/i });
			fireEvent.click(forceButton);

			await waitFor(() => {
				const modalCode = screen.getByRole('dialog').querySelector('code');
				expect(modalCode).toHaveTextContent('TASK-002');
			});

			const confirmButton = screen.getByRole('button', { name: 'Force Run' });
			fireEvent.click(confirmButton);

			await waitFor(() => {
				expect(onForce).toHaveBeenCalledWith('TASK-002');
			});
		});
	});

	describe('accessibility', () => {
		it('has aria-expanded attribute on header button', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const header = container.querySelector('.panel-header');
			expect(header).toHaveAttribute('aria-expanded');
		});

		it('has aria-controls attribute on header button', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const header = container.querySelector('.panel-header');
			expect(header).toHaveAttribute('aria-controls', 'blocked-panel-body');
		});

		it('has role="region" on panel body', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const body = container.querySelector('#blocked-panel-body');
			expect(body).toHaveAttribute('role', 'region');
		});

		it('has aria-label on Skip button', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const skipButton = screen.getByRole('button', { name: /skip block for TASK-001/i });
			expect(skipButton).toHaveAttribute('aria-label', 'Skip block for TASK-001');
		});

		it('has aria-label on Force button', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			render(<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />);

			const forceButton = screen.getByRole('button', { name: /force run TASK-001/i });
			expect(forceButton).toHaveAttribute('aria-label', 'Force run TASK-001');
		});

		it('has aria-label on count badge', () => {
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
				createMockTask({
					id: 'TASK-003',
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-004'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const badge = container.querySelector('.panel-badge');
			expect(badge).toHaveAttribute('aria-label', '2 blocked tasks');
		});
	});

	describe('title tooltip', () => {
		it('sets title attribute on task title for long text', () => {
			const longTitle = 'This is a very long task title that should show a tooltip on hover';
			const tasks = [
				createMockTask({
					id: 'TASK-001',
					title: longTitle,
					status: TaskStatus.BLOCKED,
					unmetBlockers: ['TASK-002'],
				}),
			];
			const onSkip = vi.fn();
			const onForce = vi.fn();

			const { container } = render(
				<BlockedPanel tasks={tasks} onSkip={onSkip} onForce={onForce} />
			);

			const titleElement = container.querySelector('.blocked-title');
			expect(titleElement).toHaveAttribute('title', longTitle);
		});
	});
});
