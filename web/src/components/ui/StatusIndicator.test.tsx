import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusIndicator } from './StatusIndicator';
import type { TaskStatus } from '@/lib/types';

describe('StatusIndicator', () => {
	it('renders an orb element', () => {
		const { container } = render(<StatusIndicator status="created" />);
		const orb = container.querySelector('.orb');
		expect(orb).toBeInTheDocument();
	});

	it('applies size class correctly', () => {
		const { container: sm } = render(<StatusIndicator status="created" size="sm" />);
		expect(sm.querySelector('.status-indicator')).toHaveClass('size-sm');

		const { container: md } = render(<StatusIndicator status="created" size="md" />);
		expect(md.querySelector('.status-indicator')).toHaveClass('size-md');

		const { container: lg } = render(<StatusIndicator status="created" size="lg" />);
		expect(lg.querySelector('.status-indicator')).toHaveClass('size-lg');
	});

	it('uses medium size by default', () => {
		const { container } = render(<StatusIndicator status="created" />);
		expect(container.querySelector('.status-indicator')).toHaveClass('size-md');
	});

	it('does not show label by default', () => {
		render(<StatusIndicator status="running" />);
		expect(screen.queryByText('Running')).not.toBeInTheDocument();
	});

	it('shows label when showLabel is true', () => {
		render(<StatusIndicator status="running" showLabel />);
		expect(screen.getByText('Running')).toBeInTheDocument();
	});

	it('applies animated class for running status', () => {
		const { container } = render(<StatusIndicator status="running" />);
		expect(container.querySelector('.status-indicator')).toHaveClass('animated');
	});

	it('applies paused class for paused status', () => {
		const { container } = render(<StatusIndicator status="paused" />);
		expect(container.querySelector('.status-indicator')).toHaveClass('paused');
	});

	it('does not apply animated/paused class for static statuses', () => {
		const staticStatuses: TaskStatus[] = ['created', 'planned', 'completed', 'failed'];

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
			created: 'Created',
			classifying: 'Classifying',
			planned: 'Planned',
			running: 'Running',
			paused: 'Paused',
			blocked: 'Blocked',
			finalizing: 'Finalizing',
			completed: 'Completed',
			failed: 'Failed',
		};

		Object.entries(statusLabels).forEach(([status, label]) => {
			const { unmount } = render(<StatusIndicator status={status as TaskStatus} showLabel />);
			expect(screen.getByText(label)).toBeInTheDocument();
			unmount();
		});
	});

	it('sets CSS custom properties for colors', () => {
		const { container } = render(<StatusIndicator status="running" />);
		const orb = container.querySelector('.orb');
		expect(orb).toHaveStyle({
			'--status-color': 'var(--primary)',
			'--status-glow': 'var(--primary-glow)',
		});
	});

	it('applies inline color style to label', () => {
		const { container } = render(<StatusIndicator status="completed" showLabel />);
		const label = container.querySelector('.label');
		expect(label).toHaveStyle({ color: 'var(--status-success)' });
	});
});
