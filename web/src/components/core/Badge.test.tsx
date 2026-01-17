import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createRef } from 'react';
import { Badge, type BadgeVariant, type BadgeStatus } from './Badge';

describe('Badge', () => {
	describe('rendering', () => {
		it('renders a span element', () => {
			render(<Badge>Active</Badge>);
			expect(screen.getByText('Active')).toBeInTheDocument();
		});

		it('renders children content', () => {
			render(<Badge>Test Content</Badge>);
			expect(screen.getByText('Test Content')).toBeInTheDocument();
		});

		it('renders nothing for empty children', () => {
			const { container } = render(<Badge>{''}</Badge>);
			expect(container.querySelector('.badge')).not.toBeInTheDocument();
		});

		it('renders nothing for null children', () => {
			const { container } = render(<Badge>{null}</Badge>);
			expect(container.querySelector('.badge')).not.toBeInTheDocument();
		});

		it('renders nothing for undefined children', () => {
			const { container } = render(<Badge>{undefined}</Badge>);
			expect(container.querySelector('.badge')).not.toBeInTheDocument();
		});

		it('renders numeric children', () => {
			render(<Badge variant="count">{27}</Badge>);
			expect(screen.getByText('27')).toBeInTheDocument();
		});
	});

	describe('variants', () => {
		const variants: BadgeVariant[] = ['status', 'count', 'tool'];

		it.each(variants)('renders %s variant with correct class', (variant) => {
			const { container } = render(<Badge variant={variant}>Badge</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass(`badge-${variant}`);
		});

		it('uses status variant by default', () => {
			const { container } = render(<Badge>Badge</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('badge-status');
		});
	});

	describe('status colors', () => {
		const statuses: BadgeStatus[] = ['active', 'paused', 'completed', 'failed', 'idle'];

		it.each(statuses)('renders %s status with correct class', (status) => {
			const { container } = render(
				<Badge variant="status" status={status}>
					Badge
				</Badge>
			);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass(`badge-status-${status}`);
		});

		it('uses idle status by default', () => {
			const { container } = render(<Badge variant="status">Badge</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('badge-status-idle');
		});

		it('only applies status class when variant is status', () => {
			const { container } = render(
				<Badge variant="count" status="active">
					27
				</Badge>
			);
			const badge = container.querySelector('.badge');
			expect(badge).not.toHaveClass('badge-status-active');
		});
	});

	describe('count variant', () => {
		it('renders numeric value correctly', () => {
			render(<Badge variant="count">42</Badge>);
			expect(screen.getByText('42')).toBeInTheDocument();
		});

		it('applies count variant class', () => {
			const { container } = render(<Badge variant="count">10</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('badge-count');
		});
	});

	describe('tool variant', () => {
		it('renders tool label correctly', () => {
			render(<Badge variant="tool">File Read/Write</Badge>);
			expect(screen.getByText('File Read/Write')).toBeInTheDocument();
		});

		it('applies tool variant class', () => {
			const { container } = render(<Badge variant="tool">Bash</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('badge-tool');
		});
	});

	describe('text overflow', () => {
		it('wraps content in badge-content span for truncation', () => {
			const { container } = render(
				<Badge variant="tool">Very Long Tool Name That Should Be Truncated</Badge>
			);
			const content = container.querySelector('.badge-content');
			expect(content).toBeInTheDocument();
			expect(content).toHaveTextContent('Very Long Tool Name That Should Be Truncated');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLSpanElement>();
			render(<Badge ref={ref}>Badge</Badge>);
			expect(ref.current).toBeInstanceOf(HTMLSpanElement);
			expect(ref.current?.tagName).toBe('SPAN');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<Badge className="custom-class">Badge</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('custom-class');
			expect(badge).toHaveClass('badge');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(
				<Badge className="class-a class-b" variant="status" status="active">
					Active
				</Badge>
			);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('badge');
			expect(badge).toHaveClass('badge-status');
			expect(badge).toHaveClass('badge-status-active');
			expect(badge).toHaveClass('class-a');
			expect(badge).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native span attributes', () => {
			render(<Badge data-testid="test-badge" title="Test tooltip">Badge</Badge>);
			const badge = screen.getByTestId('test-badge');
			expect(badge).toHaveAttribute('title', 'Test tooltip');
		});

		it('supports aria attributes', () => {
			const { container } = render(<Badge aria-label="Status: Active">Active</Badge>);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveAttribute('aria-label', 'Status: Active');
		});
	});

	describe('flex container compatibility', () => {
		it('has min-width: 0 via CSS for flex truncation', () => {
			const { container } = render(
				<div style={{ display: 'flex', width: '50px' }}>
					<Badge variant="tool">Very Long Text</Badge>
				</div>
			);
			const badge = container.querySelector('.badge');
			expect(badge).toBeInTheDocument();
		});
	});

	describe('combined props', () => {
		it('handles all status props together', () => {
			const { container } = render(
				<Badge variant="status" status="completed" className="custom" data-testid="test-badge">
					Completed
				</Badge>
			);
			const badge = container.querySelector('.badge');
			expect(badge).toHaveClass('badge');
			expect(badge).toHaveClass('badge-status');
			expect(badge).toHaveClass('badge-status-completed');
			expect(badge).toHaveClass('custom');
			expect(badge).toHaveAttribute('data-testid', 'test-badge');
		});
	});
});
