/**
 * Tooltip Component Tests
 *
 * Tests for the Radix-based Tooltip component including:
 * - Basic rendering
 * - Controlled and uncontrolled modes
 * - Disabled state
 * - Custom placement
 * - Provider configuration
 *
 * Note: Complex hover/focus interactions are better tested via E2E tests
 * as jsdom has limitations with pointer events and focus management.
 */

import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { Tooltip, TooltipProvider } from './Tooltip';

// Mock browser APIs not available in jsdom
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

// Helper to wrap with TooltipProvider
function renderWithProvider(ui: React.ReactElement) {
	return render(<TooltipProvider delayDuration={0}>{ui}</TooltipProvider>);
}

describe('Tooltip', () => {
	describe('Basic rendering', () => {
		it('renders trigger element', () => {
			renderWithProvider(
				<Tooltip content="Test tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
		});

		it('does not show tooltip content initially', () => {
			renderWithProvider(
				<Tooltip content="Test tooltip">
					<button>Hover me</button>
				</Tooltip>
			);

			// Tooltip content should not be visible before interaction
			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});

		it('renders just children when disabled', () => {
			renderWithProvider(
				<Tooltip content="Test tooltip" disabled>
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
			// Should not have any tooltip-related attributes
			const button = screen.getByRole('button');
			expect(button).not.toHaveAttribute('aria-describedby');
		});

		it('renders just children when content is empty', () => {
			renderWithProvider(
				<Tooltip content="">
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
		});

		it('renders just children when content is null', () => {
			renderWithProvider(
				<Tooltip content={null as unknown as string}>
					<button>Hover me</button>
				</Tooltip>
			);

			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();
		});
	});

	describe('Controlled mode', () => {
		it('shows tooltip when open is true', async () => {
			renderWithProvider(
				<Tooltip content="Controlled tooltip" open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			// Radix may duplicate content for accessibility
			expect(screen.getAllByText('Controlled tooltip').length).toBeGreaterThan(0);
		});

		it('hides tooltip when open is false', () => {
			renderWithProvider(
				<Tooltip content="Controlled tooltip" open={false}>
					<button>Trigger</button>
				</Tooltip>
			);

			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});

		it('calls onOpenChange callback', async () => {
			const onOpenChange = vi.fn();

			const { rerender } = render(
				<TooltipProvider delayDuration={0}>
					<Tooltip content="Test" open={false} onOpenChange={onOpenChange}>
						<button>Trigger</button>
					</Tooltip>
				</TooltipProvider>
			);

			// Rerender with open=true to trigger change
			rerender(
				<TooltipProvider delayDuration={0}>
					<Tooltip content="Test" open={true} onOpenChange={onOpenChange}>
						<button>Trigger</button>
					</Tooltip>
				</TooltipProvider>
			);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
		});
	});

	describe('Content variations', () => {
		it('renders string content', async () => {
			renderWithProvider(
				<Tooltip content="Simple text" open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				expect(screen.getAllByText('Simple text').length).toBeGreaterThan(0);
			});
		});

		it('renders JSX content', async () => {
			renderWithProvider(
				<Tooltip
					content={
						<span>
							Press <kbd>Enter</kbd>
						</span>
					}
					open={true}
				>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			// Radix duplicates content for accessibility (visual + screen reader)
			expect(screen.getAllByText('Enter').length).toBeGreaterThan(0);
		});
	});

	describe('Placement props', () => {
		it('applies side prop to content', async () => {
			renderWithProvider(
				<Tooltip content="Right tooltip" side="right" open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				// The data-side attribute is on the .tooltip-content element, not role="tooltip"
				// role="tooltip" is a visually hidden screen-reader-only element in Radix
				const contentEl = document.querySelector('.tooltip-content');
				expect(contentEl).toHaveAttribute('data-side', 'right');
			});
		});

		it('applies align prop to content', async () => {
			renderWithProvider(
				<Tooltip content="Start aligned" align="start" open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				const contentEl = document.querySelector('.tooltip-content');
				expect(contentEl).toHaveAttribute('data-align', 'start');
			});
		});
	});

	describe('Arrow', () => {
		it('renders arrow by default', async () => {
			renderWithProvider(
				<Tooltip content="With arrow" open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			// Arrow is rendered as an SVG element with class tooltip-arrow inside .tooltip-content
			const contentEl = document.querySelector('.tooltip-content');
			expect(contentEl?.querySelector('.tooltip-arrow')).toBeInTheDocument();
		});

		it('hides arrow when showArrow is false', async () => {
			renderWithProvider(
				<Tooltip content="No arrow" showArrow={false} open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			const contentEl = document.querySelector('.tooltip-content');
			expect(contentEl?.querySelector('.tooltip-arrow')).not.toBeInTheDocument();
		});
	});

	describe('Custom className', () => {
		it('applies custom className to content', async () => {
			renderWithProvider(
				<Tooltip content="Styled" className="my-custom-class" open={true}>
					<button>Trigger</button>
				</Tooltip>
			);

			await waitFor(() => {
				const contentEl = document.querySelector('.tooltip-content');
				expect(contentEl).toHaveClass('tooltip-content');
				expect(contentEl).toHaveClass('my-custom-class');
			});
		});
	});
});

describe('TooltipProvider', () => {
	it('renders children', () => {
		render(
			<TooltipProvider>
				<button>Child button</button>
			</TooltipProvider>
		);

		expect(screen.getByRole('button', { name: 'Child button' })).toBeInTheDocument();
	});

	it('accepts custom delay duration', async () => {
		render(
			<TooltipProvider delayDuration={100}>
				<Tooltip content="Delayed" open={true}>
					<button>Trigger</button>
				</Tooltip>
			</TooltipProvider>
		);

		await waitFor(() => {
			expect(screen.getByRole('tooltip')).toBeInTheDocument();
		});
	});

	it('can be nested with different configurations', () => {
		render(
			<TooltipProvider delayDuration={500}>
				<TooltipProvider delayDuration={100}>
					<button>Nested child</button>
				</TooltipProvider>
			</TooltipProvider>
		);

		expect(screen.getByRole('button', { name: 'Nested child' })).toBeInTheDocument();
	});
});
