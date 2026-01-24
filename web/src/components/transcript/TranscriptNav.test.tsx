import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TranscriptNav, type TranscriptNavPhase } from './TranscriptNav';

// Helper to create phase data
const createPhase = (overrides: Partial<TranscriptNavPhase> = {}): TranscriptNavPhase => ({
	phase: 'implement',
	iterations: 1,
	transcript_count: 10,
	status: 'completed',
	...overrides,
});

describe('TranscriptNav', () => {
	const defaultPhases: TranscriptNavPhase[] = [
		createPhase({ phase: 'spec', iterations: 1, status: 'completed' }),
		createPhase({ phase: 'implement', iterations: 3, status: 'completed' }),
		createPhase({ phase: 'test', iterations: 1, status: 'running' }),
		createPhase({ phase: 'review', iterations: 0, status: 'pending' }),
		createPhase({ phase: 'docs', iterations: 0, status: 'pending' }),
	];

	const defaultProps = {
		phases: defaultPhases,
		onNavigate: vi.fn(),
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('rendering phases (SC-1)', () => {
		it('renders all phases', () => {
			render(<TranscriptNav {...defaultProps} />);

			expect(screen.getByText('spec')).toBeInTheDocument();
			expect(screen.getByText('implement')).toBeInTheDocument();
			expect(screen.getByText('test')).toBeInTheDocument();
			expect(screen.getByText('review')).toBeInTheDocument();
			expect(screen.getByText('docs')).toBeInTheDocument();
		});

		it('renders completed status indicator (checkmark)', () => {
			const phases = [createPhase({ phase: 'spec', status: 'completed' })];
			const { container } = render(<TranscriptNav phases={phases} onNavigate={vi.fn()} />);

			// Should have the completed class/indicator
			const phaseItem = container.querySelector('[data-phase="spec"]');
			expect(phaseItem).toBeInTheDocument();
			expect(phaseItem).toHaveAttribute('data-status', 'completed');

			// Visual indicator - look for checkmark icon or class
			const statusIndicator = container.querySelector('.nav-phase-status--completed');
			expect(statusIndicator).toBeInTheDocument();
		});

		it('renders failed status indicator (x)', () => {
			const phases = [createPhase({ phase: 'implement', status: 'failed' })];
			const { container } = render(<TranscriptNav phases={phases} onNavigate={vi.fn()} />);

			const phaseItem = container.querySelector('[data-phase="implement"]');
			expect(phaseItem).toHaveAttribute('data-status', 'failed');

			const statusIndicator = container.querySelector('.nav-phase-status--failed');
			expect(statusIndicator).toBeInTheDocument();
		});

		it('renders running status indicator (dot)', () => {
			const phases = [createPhase({ phase: 'test', status: 'running' })];
			const { container } = render(<TranscriptNav phases={phases} onNavigate={vi.fn()} />);

			const phaseItem = container.querySelector('[data-phase="test"]');
			expect(phaseItem).toHaveAttribute('data-status', 'running');

			const statusIndicator = container.querySelector('.nav-phase-status--running');
			expect(statusIndicator).toBeInTheDocument();
		});

		it('renders pending status indicator (empty circle)', () => {
			const phases = [createPhase({ phase: 'review', status: 'pending' })];
			const { container } = render(<TranscriptNav phases={phases} onNavigate={vi.fn()} />);

			const phaseItem = container.querySelector('[data-phase="review"]');
			expect(phaseItem).toHaveAttribute('data-status', 'pending');

			const statusIndicator = container.querySelector('.nav-phase-status--pending');
			expect(statusIndicator).toBeInTheDocument();
		});
	});

	describe('iterations display', () => {
		it('shows iterations under expanded phases', () => {
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'completed' })];
			render(
				<TranscriptNav phases={phases} onNavigate={vi.fn()} currentPhase="implement" />
			);

			// When phase is current/expanded, should show iteration items
			expect(screen.getByText('Iteration 1')).toBeInTheDocument();
			expect(screen.getByText('Iteration 2')).toBeInTheDocument();
			expect(screen.getByText('Iteration 3')).toBeInTheDocument();
		});

		it('does not show iterations for collapsed phases', () => {
			const phases = [
				createPhase({ phase: 'spec', iterations: 1, status: 'completed' }),
				createPhase({ phase: 'implement', iterations: 3, status: 'completed' }),
			];
			// currentPhase is spec, so implement should be collapsed
			render(
				<TranscriptNav phases={phases} onNavigate={vi.fn()} currentPhase="spec" />
			);

			// Spec iterations should be visible
			expect(screen.getByText('Iteration 1')).toBeInTheDocument();
			// Implement iterations should not be visible when collapsed
			expect(screen.queryByText('Iteration 2')).not.toBeInTheDocument();
			expect(screen.queryByText('Iteration 3')).not.toBeInTheDocument();
		});

		it('toggles iterations visibility on phase click', () => {
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'completed' })];
			render(<TranscriptNav phases={phases} onNavigate={vi.fn()} />);

			const phaseButton = screen.getByText('implement').closest('button');
			expect(phaseButton).toBeInTheDocument();

			// Initially collapsed (no currentPhase)
			expect(screen.queryByText('Iteration 1')).not.toBeInTheDocument();

			// Click to expand
			fireEvent.click(phaseButton!);
			expect(screen.getByText('Iteration 1')).toBeInTheDocument();

			// Click to collapse
			fireEvent.click(phaseButton!);
			expect(screen.queryByText('Iteration 1')).not.toBeInTheDocument();
		});
	});

	describe('navigation callback (SC-2)', () => {
		it('calls onNavigate with phase when phase clicked', () => {
			const onNavigate = vi.fn();
			render(<TranscriptNav {...defaultProps} onNavigate={onNavigate} />);

			const phaseButton = screen.getByText('spec').closest('button');
			fireEvent.click(phaseButton!);

			expect(onNavigate).toHaveBeenCalledWith('spec', undefined);
		});

		it('calls onNavigate with phase and iteration when iteration clicked', () => {
			const onNavigate = vi.fn();
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'completed' })];
			render(
				<TranscriptNav
					phases={phases}
					onNavigate={onNavigate}
					currentPhase="implement"
				/>
			);

			// Click on iteration 2
			const iteration2 = screen.getByText('Iteration 2');
			fireEvent.click(iteration2);

			expect(onNavigate).toHaveBeenCalledWith('implement', 2);
		});

		it('calls onNavigate on keyboard Enter', () => {
			const onNavigate = vi.fn();
			render(<TranscriptNav {...defaultProps} onNavigate={onNavigate} />);

			const phaseButton = screen.getByText('spec').closest('button');
			fireEvent.keyDown(phaseButton!, { key: 'Enter' });

			expect(onNavigate).toHaveBeenCalledWith('spec', undefined);
		});
	});

	describe('current position highlighting (SC-3)', () => {
		it('highlights current phase with active class', () => {
			const { container } = render(
				<TranscriptNav {...defaultProps} currentPhase="implement" />
			);

			const implementPhase = container.querySelector('[data-phase="implement"]');
			expect(implementPhase).toHaveClass('nav-phase--active');

			// Other phases should not be active
			const specPhase = container.querySelector('[data-phase="spec"]');
			expect(specPhase).not.toHaveClass('nav-phase--active');
		});

		it('highlights current iteration', () => {
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'running' })];
			const { container } = render(
				<TranscriptNav
					phases={phases}
					onNavigate={vi.fn()}
					currentPhase="implement"
					currentIteration={2}
				/>
			);

			// Iteration 2 should have active class
			const iteration2 = container.querySelector('[data-iteration="2"]');
			expect(iteration2).toHaveClass('nav-iteration--active');

			// Iteration 1 should not
			const iteration1 = container.querySelector('[data-iteration="1"]');
			expect(iteration1).not.toHaveClass('nav-iteration--active');
		});

		it('expands phase containing current iteration', () => {
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'running' })];
			render(
				<TranscriptNav
					phases={phases}
					onNavigate={vi.fn()}
					currentPhase="implement"
					currentIteration={2}
				/>
			);

			// Iterations should be visible because the phase is current
			expect(screen.getByText('Iteration 1')).toBeInTheDocument();
			expect(screen.getByText('Iteration 2')).toBeInTheDocument();
			expect(screen.getByText('Iteration 3')).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('phase buttons have appropriate roles', () => {
			render(<TranscriptNav {...defaultProps} />);

			const buttons = screen.getAllByRole('button');
			expect(buttons.length).toBeGreaterThanOrEqual(defaultPhases.length);
		});

		it('supports keyboard navigation between phases', () => {
			render(<TranscriptNav {...defaultProps} />);

			const firstPhase = screen.getByText('spec').closest('button');
			expect(firstPhase).toHaveAttribute('tabindex', '0');
		});

		it('has aria-expanded for collapsible phases', () => {
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'completed' })];
			render(
				<TranscriptNav phases={phases} onNavigate={vi.fn()} currentPhase="implement" />
			);

			const phaseButton = screen.getByText('implement').closest('button');
			expect(phaseButton).toHaveAttribute('aria-expanded', 'true');
		});
	});

	describe('CSS structure', () => {
		it('renders with transcript-nav root class', () => {
			const { container } = render(<TranscriptNav {...defaultProps} />);

			const nav = container.querySelector('.transcript-nav');
			expect(nav).toBeInTheDocument();
		});

		it('applies nav-phase class to phase items', () => {
			const { container } = render(<TranscriptNav {...defaultProps} />);

			const phases = container.querySelectorAll('.nav-phase');
			expect(phases.length).toBe(defaultPhases.length);
		});

		it('applies nav-iteration class to iteration items', () => {
			const phases = [createPhase({ phase: 'implement', iterations: 3, status: 'completed' })];
			const { container } = render(
				<TranscriptNav phases={phases} onNavigate={vi.fn()} currentPhase="implement" />
			);

			const iterations = container.querySelectorAll('.nav-iteration');
			expect(iterations.length).toBe(3);
		});
	});

	describe('empty states', () => {
		it('renders with empty phases array', () => {
			const { container } = render(<TranscriptNav phases={[]} onNavigate={vi.fn()} />);

			const nav = container.querySelector('.transcript-nav');
			expect(nav).toBeInTheDocument();
		});

		it('renders phase with 0 iterations without iteration list', () => {
			const phases = [createPhase({ phase: 'review', iterations: 0, status: 'pending' })];
			const { container } = render(
				<TranscriptNav phases={phases} onNavigate={vi.fn()} currentPhase="review" />
			);

			// Should not have any iteration items
			const iterations = container.querySelectorAll('.nav-iteration');
			expect(iterations.length).toBe(0);
		});
	});

	describe('testId support', () => {
		it('applies testId to root element', () => {
			render(<TranscriptNav {...defaultProps} testId="transcript-nav" />);

			expect(screen.getByTestId('transcript-nav')).toBeInTheDocument();
		});
	});
});
