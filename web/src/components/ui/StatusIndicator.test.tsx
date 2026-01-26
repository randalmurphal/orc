import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusIndicator } from './StatusIndicator';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

describe('StatusIndicator', () => {
	it('renders an orb element', () => {
		const { container } = render(<StatusIndicator status={TaskStatus.CREATED} />);
		const orb = container.querySelector('.orb');
		expect(orb).toBeInTheDocument();
	});

	it('applies size class correctly', () => {
		const { container: sm } = render(<StatusIndicator status={TaskStatus.CREATED} size="sm" />);
		expect(sm.querySelector('.status-indicator')).toHaveClass('size-sm');

		const { container: md } = render(<StatusIndicator status={TaskStatus.CREATED} size="md" />);
		expect(md.querySelector('.status-indicator')).toHaveClass('size-md');

		const { container: lg } = render(<StatusIndicator status={TaskStatus.CREATED} size="lg" />);
		expect(lg.querySelector('.status-indicator')).toHaveClass('size-lg');
	});

	it('uses medium size by default', () => {
		const { container } = render(<StatusIndicator status={TaskStatus.CREATED} />);
		expect(container.querySelector('.status-indicator')).toHaveClass('size-md');
	});

	it('does not show label by default', () => {
		render(<StatusIndicator status={TaskStatus.RUNNING} />);
		expect(screen.queryByText('Running')).not.toBeInTheDocument();
	});

	it('shows label when showLabel is true', () => {
		render(<StatusIndicator status={TaskStatus.RUNNING} showLabel />);
		expect(screen.getByText('Running')).toBeInTheDocument();
	});

	it('applies animated class for running status', () => {
		const { container } = render(<StatusIndicator status={TaskStatus.RUNNING} />);
		expect(container.querySelector('.status-indicator')).toHaveClass('animated');
	});

	it('applies paused class for paused status', () => {
		const { container } = render(<StatusIndicator status={TaskStatus.PAUSED} />);
		expect(container.querySelector('.status-indicator')).toHaveClass('paused');
	});

	it('does not apply animated/paused class for static statuses', () => {
		const staticStatuses: TaskStatus[] = [TaskStatus.CREATED, TaskStatus.PLANNED, TaskStatus.COMPLETED, TaskStatus.FAILED];

		staticStatuses.forEach((status) => {
			const { container, unmount } = render(<StatusIndicator status={status} />);
			const indicator = container.querySelector('.status-indicator');
			expect(indicator).not.toHaveClass('animated');
			expect(indicator).not.toHaveClass('paused');
			unmount();
		});
	});

	it('renders correct labels for all statuses', () => {
		const statusLabels: Record<TaskStatus, string> = {
			[TaskStatus.UNSPECIFIED]: 'Unknown',
			[TaskStatus.CREATED]: 'Created',
			[TaskStatus.CLASSIFYING]: 'Classifying',
			[TaskStatus.PLANNED]: 'Planned',
			[TaskStatus.RUNNING]: 'Running',
			[TaskStatus.PAUSED]: 'Paused',
			[TaskStatus.BLOCKED]: 'Blocked',
			[TaskStatus.FINALIZING]: 'Finalizing',
			[TaskStatus.COMPLETED]: 'Completed',
			[TaskStatus.FAILED]: 'Failed',
			[TaskStatus.RESOLVED]: 'Resolved',
		};

		Object.entries(statusLabels).forEach(([status, label]) => {
			const { unmount } = render(<StatusIndicator status={Number(status) as TaskStatus} showLabel />);
			expect(screen.getByText(label)).toBeInTheDocument();
			unmount();
		});
	});

	it('sets CSS custom properties for colors', () => {
		const { container } = render(<StatusIndicator status={TaskStatus.RUNNING} />);
		const orb = container.querySelector('.orb');
		expect(orb).toHaveStyle({
			'--status-color': 'var(--primary)',
			'--status-glow': 'var(--primary-glow)',
		});
	});

	it('applies inline color style to label', () => {
		const { container } = render(<StatusIndicator status={TaskStatus.COMPLETED} showLabel />);
		const label = container.querySelector('.label');
		expect(label).toHaveStyle({ color: 'var(--status-success)' });
	});
});
