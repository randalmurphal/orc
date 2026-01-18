/**
 * Tooltip Component Tests
 *
 * Tests for the CSS-only Tooltip component including:
 * - Basic rendering
 * - Position variants
 * - Delay configuration
 * - Disabled state
 * - Accessibility attributes
 * - Ref forwarding
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createRef } from 'react';
import { Tooltip, type TooltipPosition } from './Tooltip';

describe('Tooltip', () => {
	describe('rendering', () => {
		it('renders the trigger children', () => {
			render(
				<Tooltip content="Test tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
		});

		it('renders tooltip content element', () => {
			const { container } = render(
				<Tooltip content="Test tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			const tooltipContent = container.querySelector('.tooltip-content');
			expect(tooltipContent).toBeInTheDocument();
			expect(tooltipContent).toHaveTextContent('Test tooltip');
		});

		it('wraps children in tooltip-wrapper', () => {
			const { container } = render(
				<Tooltip content="Test tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toBeInTheDocument();
			expect(wrapper?.querySelector('button')).toBeInTheDocument();
		});
	});

	describe('disabled state', () => {
		it('renders just children when disabled', () => {
			const { container } = render(
				<Tooltip content="Test tooltip" disabled>
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
			expect(container.querySelector('.tooltip-wrapper')).not.toBeInTheDocument();
			expect(container.querySelector('.tooltip-content')).not.toBeInTheDocument();
		});

		it('renders just children when content is empty', () => {
			const { container } = render(
				<Tooltip content="">
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
			expect(container.querySelector('.tooltip-wrapper')).not.toBeInTheDocument();
		});
	});

	describe('position variants', () => {
		const positions: TooltipPosition[] = ['top', 'bottom', 'left', 'right'];

		it.each(positions)('applies %s position via data attribute', (position) => {
			const { container } = render(
				<Tooltip content="Positioned tooltip" position={position}>
					<button>Hover me</button>
				</Tooltip>
			);

			const tooltipContent = container.querySelector('.tooltip-content');
			expect(tooltipContent).toHaveAttribute('data-position', position);
		});

		it('defaults to top position', () => {
			const { container } = render(
				<Tooltip content="Default position">
					<button>Hover me</button>
				</Tooltip>
			);

			const tooltipContent = container.querySelector('.tooltip-content');
			expect(tooltipContent).toHaveAttribute('data-position', 'top');
		});
	});

	describe('delay configuration', () => {
		it('sets delay via CSS custom property', () => {
			const { container } = render(
				<Tooltip content="Delayed tooltip" delay={500}>
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveStyle({ '--tooltip-delay': '500ms' });
		});

		it('defaults to 300ms delay', () => {
			const { container } = render(
				<Tooltip content="Default delay">
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveStyle({ '--tooltip-delay': '300ms' });
		});

		it('supports zero delay', () => {
			const { container } = render(
				<Tooltip content="No delay" delay={0}>
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveStyle({ '--tooltip-delay': '0ms' });
		});
	});

	describe('accessibility', () => {
		it('has role="tooltip" on content', () => {
			const { container } = render(
				<Tooltip content="Accessible tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			const tooltipContent = container.querySelector('.tooltip-content');
			expect(tooltipContent).toHaveAttribute('role', 'tooltip');
		});

		it('adds aria-describedby to child element linking to tooltip', () => {
			const { container } = render(
				<Tooltip content="Accessible tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			const button = container.querySelector('button');
			const tooltipContent = container.querySelector('.tooltip-content');

			expect(button).toHaveAttribute('aria-describedby', tooltipContent?.id);
		});

		it('generates unique id for tooltip content', () => {
			const { container } = render(
				<>
					<Tooltip content="First tooltip">
						<button>First</button>
					</Tooltip>
					<Tooltip content="Second tooltip">
						<button>Second</button>
					</Tooltip>
				</>
			);

			const tooltips = container.querySelectorAll('.tooltip-content');
			const ids = Array.from(tooltips).map((t) => t.id);

			expect(ids[0]).toBeTruthy();
			expect(ids[1]).toBeTruthy();
			expect(ids[0]).not.toBe(ids[1]);
		});

		it('links each trigger to its own tooltip', () => {
			const { container } = render(
				<>
					<Tooltip content="First tooltip">
						<button>First</button>
					</Tooltip>
					<Tooltip content="Second tooltip">
						<button>Second</button>
					</Tooltip>
				</>
			);

			const buttons = container.querySelectorAll('button');
			const tooltips = container.querySelectorAll('.tooltip-content');

			expect(buttons[0]).toHaveAttribute('aria-describedby', tooltips[0].id);
			expect(buttons[1]).toHaveAttribute('aria-describedby', tooltips[1].id);
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref to wrapper element', () => {
			const ref = createRef<HTMLDivElement>();
			render(
				<Tooltip ref={ref} content="Test tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current).toHaveClass('tooltip-wrapper');
		});
	});

	describe('custom className', () => {
		it('applies custom className to wrapper', () => {
			const { container } = render(
				<Tooltip content="Styled tooltip" className="custom-class">
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveClass('tooltip-wrapper');
			expect(wrapper).toHaveClass('custom-class');
		});

		it('merges multiple custom classes', () => {
			const { container } = render(
				<Tooltip content="Styled tooltip" className="class-a class-b">
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveClass('tooltip-wrapper');
			expect(wrapper).toHaveClass('class-a');
			expect(wrapper).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			const { container } = render(
				<Tooltip content="Test" data-testid="test-tooltip" title="Native title">
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveAttribute('data-testid', 'test-tooltip');
			expect(wrapper).toHaveAttribute('title', 'Native title');
		});

		it('merges custom style with delay variable', () => {
			const { container } = render(
				<Tooltip content="Styled" delay={200} style={{ marginTop: '10px' }}>
					<button>Hover me</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveStyle({
				'--tooltip-delay': '200ms',
				marginTop: '10px',
			});
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const ref = createRef<HTMLDivElement>();
			const { container } = render(
				<Tooltip
					ref={ref}
					content="Combined tooltip"
					position="right"
					delay={400}
					className="custom"
					data-testid="combined"
				>
					<button>Combined</button>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			const tooltipContent = container.querySelector('.tooltip-content');

			expect(wrapper).toHaveClass('tooltip-wrapper');
			expect(wrapper).toHaveClass('custom');
			expect(wrapper).toHaveStyle({ '--tooltip-delay': '400ms' });
			expect(wrapper).toHaveAttribute('data-testid', 'combined');

			expect(tooltipContent).toHaveAttribute('data-position', 'right');
			expect(tooltipContent).toHaveTextContent('Combined tooltip');

			expect(ref.current).toBe(wrapper);
		});
	});

	describe('children types', () => {
		it('works with button children', () => {
			render(
				<Tooltip content="Button tooltip">
					<button>Click me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument();
		});

		it('works with link children', () => {
			render(
				<Tooltip content="Link tooltip">
					<a href="/test">Link</a>
				</Tooltip>
			);

			expect(screen.getByRole('link', { name: 'Link' })).toBeInTheDocument();
		});

		it('works with span children', () => {
			const { container } = render(
				<Tooltip content="Span tooltip">
					<span>Text</span>
				</Tooltip>
			);

			expect(container.querySelector('.tooltip-wrapper span')).toHaveTextContent('Text');
		});

		it('works with multiple children (no aria-describedby enhancement)', () => {
			const { container } = render(
				<Tooltip content="Multiple children">
					<span>First</span>
					<span>Second</span>
				</Tooltip>
			);

			const wrapper = container.querySelector('.tooltip-wrapper');
			const spans = wrapper?.querySelectorAll('span:not(.tooltip-content)');
			expect(spans?.length).toBe(2);
			// Multiple children are not enhanced with aria-describedby
			expect(spans?.[0]).not.toHaveAttribute('aria-describedby');
		});

		it('works with text node children (no aria-describedby enhancement)', () => {
			const { container } = render(<Tooltip content="Text tooltip">Plain text</Tooltip>);

			const wrapper = container.querySelector('.tooltip-wrapper');
			expect(wrapper).toHaveTextContent('Plain text');
			// The tooltip still works, just without aria-describedby on the text
			expect(container.querySelector('.tooltip-content')).toBeInTheDocument();
		});
	});
});
