import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createRef } from 'react';
import { Pipeline } from './Pipeline';

const DEFAULT_PHASES = ['Plan', 'Code', 'Test', 'Review', 'Done'];

describe('Pipeline', () => {
	describe('rendering', () => {
		it('renders all 5 phases with correct labels', () => {
			render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={['Plan']} />
			);

			expect(screen.getByText('Plan')).toBeInTheDocument();
			expect(screen.getByText('Code')).toBeInTheDocument();
			expect(screen.getByText('Test')).toBeInTheDocument();
			expect(screen.getByText('Review')).toBeInTheDocument();
			expect(screen.getByText('Done')).toBeInTheDocument();
		});

		it('renders pipeline steps for each phase', () => {
			const { container } = render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={[]} />
			);

			const steps = container.querySelectorAll('.pipeline-step');
			expect(steps).toHaveLength(5);
		});
	});

	describe('completed phases', () => {
		it('renders completed phases with correct visual state (green bar, check icon)', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Test"
					completedPhases={['Plan', 'Code']}
				/>
			);

			const completedSteps = container.querySelectorAll('.pipeline-step--completed');
			expect(completedSteps).toHaveLength(2);

			// Check for check icons in completed phases
			completedSteps.forEach((step) => {
				const icon = step.querySelector('.icon');
				expect(icon).toBeInTheDocument();
			});

			// Verify completed bar fills
			const completedFills = container.querySelectorAll('.pipeline-bar-fill--completed');
			expect(completedFills).toHaveLength(2);
		});

		it('handles case-insensitive phase matching', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="test"
					completedPhases={['plan', 'code']}
				/>
			);

			const completedSteps = container.querySelectorAll('.pipeline-step--completed');
			expect(completedSteps).toHaveLength(2);

			const activeSteps = container.querySelectorAll('.pipeline-step--active');
			expect(activeSteps).toHaveLength(1);
		});
	});

	describe('active phase', () => {
		it('renders active phase with correct visual state (primary color, animation class)', () => {
			const { container } = render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={['Plan']} />
			);

			const activeStep = container.querySelector('.pipeline-step--active');
			expect(activeStep).toBeInTheDocument();

			const activeLabel = container.querySelector('.pipeline-label--active');
			expect(activeLabel).toBeInTheDocument();
			expect(activeLabel).toHaveTextContent('Code');

			const activeFill = container.querySelector('.pipeline-bar-fill--active');
			expect(activeFill).toBeInTheDocument();
		});

		it('displays progress percentage when provided to active phase', () => {
			render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={['Plan']}
					progress={45}
				/>
			);

			expect(screen.getByText('45%')).toBeInTheDocument();
		});

		it('sets progress bar width when progress is provided', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={['Plan']}
					progress={60}
				/>
			);

			const activeFill = container.querySelector('.pipeline-bar-fill--active') as HTMLElement;
			expect(activeFill.style.width).toBe('60%');
		});

		it('does not show progress percentage when not provided', () => {
			const { container } = render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={['Plan']} />
			);

			const progressSpan = container.querySelector('.pipeline-progress');
			expect(progressSpan).not.toBeInTheDocument();
		});
	});

	describe('pending phases', () => {
		it('renders pending phases with correct visual state (muted styling)', () => {
			const { container } = render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Plan" completedPhases={[]} />
			);

			const pendingSteps = container.querySelectorAll('.pipeline-step--pending');
			expect(pendingSteps).toHaveLength(4); // Code, Test, Review, Done

			const pendingLabels = container.querySelectorAll('.pipeline-label--pending');
			expect(pendingLabels).toHaveLength(4);
		});
	});

	describe('failed phase', () => {
		it('renders failed phase with red bar and X icon', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase=""
					completedPhases={['Plan', 'Code']}
					failedPhase="Test"
				/>
			);

			const failedStep = container.querySelector('.pipeline-step--failed');
			expect(failedStep).toBeInTheDocument();

			const failedFill = container.querySelector('.pipeline-bar-fill--failed');
			expect(failedFill).toBeInTheDocument();

			const failedLabel = container.querySelector('.pipeline-label--failed');
			expect(failedLabel).toBeInTheDocument();
			expect(failedLabel).toHaveTextContent('Test');

			// Check for X icon
			const icon = failedStep?.querySelector('.icon');
			expect(icon).toBeInTheDocument();
		});

		it('handles case-insensitive failed phase matching', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase=""
					completedPhases={['Plan']}
					failedPhase="CODE"
				/>
			);

			const failedStep = container.querySelector('.pipeline-step--failed');
			expect(failedStep).toBeInTheDocument();
		});
	});

	describe('compact size', () => {
		it('hides labels when size is compact', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={['Plan']}
					size="compact"
				/>
			);

			expect(container.querySelector('.pipeline--compact')).toBeInTheDocument();

			// Labels should exist but be hidden via CSS
			const labels = container.querySelectorAll('.pipeline-label');
			expect(labels).toHaveLength(5);
		});

		it('shows labels when size is default', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={['Plan']}
					size="default"
				/>
			);

			expect(container.querySelector('.pipeline--compact')).not.toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has progressbar role', () => {
			render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={['Plan']} />
			);

			expect(screen.getByRole('progressbar')).toBeInTheDocument();
		});

		it('sets aria-valuenow to completed phase count', () => {
			render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Test"
					completedPhases={['Plan', 'Code']}
				/>
			);

			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuenow', '2');
		});

		it('sets aria-valuemin to 0', () => {
			render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={[]} />
			);

			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuemin', '0');
		});

		it('sets aria-valuemax to total phase count', () => {
			render(
				<Pipeline phases={DEFAULT_PHASES} currentPhase="Code" completedPhases={[]} />
			);

			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuemax', '5');
		});

		it('sets aria-valuetext describing active phase', () => {
			render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Test"
					completedPhases={['Plan', 'Code']}
				/>
			);

			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute(
				'aria-valuetext',
				'Test phase in progress. 2 of 5 phases completed.'
			);
		});

		it('includes progress in aria-valuetext when provided', () => {
			render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={['Plan']}
					progress={50}
				/>
			);

			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute(
				'aria-valuetext',
				'Code phase in progress (50%). 1 of 5 phases completed.'
			);
		});

		it('sets aria-valuetext for failed phase', () => {
			render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase=""
					completedPhases={['Plan', 'Code']}
					failedPhase="Test"
				/>
			);

			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute(
				'aria-valuetext',
				'Test phase failed. 2 of 5 phases completed.'
			);
		});
	});

	describe('unknown phase names', () => {
		it('handles unknown phase names by showing them as pending', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="UnknownPhase"
					completedPhases={['Plan']}
				/>
			);

			// Plan should be completed
			const completedSteps = container.querySelectorAll('.pipeline-step--completed');
			expect(completedSteps).toHaveLength(1);

			// No active step since current phase doesn't match any
			const activeSteps = container.querySelectorAll('.pipeline-step--active');
			expect(activeSteps).toHaveLength(0);

			// Remaining should be pending
			const pendingSteps = container.querySelectorAll('.pipeline-step--pending');
			expect(pendingSteps).toHaveLength(4);
		});

		it('handles unknown completed phases gracefully', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={['Plan', 'Unknown']}
				/>
			);

			// Only Plan should be completed since Unknown is not in phases
			const completedSteps = container.querySelectorAll('.pipeline-step--completed');
			expect(completedSteps).toHaveLength(1);
		});
	});

	describe('custom className', () => {
		it('preserves custom className', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={[]}
					className="custom-class"
				/>
			);

			const pipeline = container.querySelector('.pipeline');
			expect(pipeline).toHaveClass('custom-class');
			expect(pipeline).toHaveClass('pipeline');
		});

		it('merges multiple custom classes', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={[]}
					className="class-a class-b"
				/>
			);

			const pipeline = container.querySelector('.pipeline');
			expect(pipeline).toHaveClass('class-a');
			expect(pipeline).toHaveClass('class-b');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(
				<Pipeline
					ref={ref}
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={[]}
				/>
			);

			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current).toHaveClass('pipeline');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Code"
					completedPhases={[]}
					data-testid="test-pipeline"
					title="Task progress"
				/>
			);

			const pipeline = screen.getByTestId('test-pipeline');
			expect(pipeline).toHaveAttribute('title', 'Task progress');
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const { container } = render(
				<Pipeline
					phases={DEFAULT_PHASES}
					currentPhase="Test"
					completedPhases={['Plan', 'Code']}
					progress={75}
					size="default"
					className="custom-pipeline"
					data-testid="full-pipeline"
				/>
			);

			const pipeline = container.querySelector('.pipeline');
			expect(pipeline).toHaveClass('pipeline');
			expect(pipeline).toHaveClass('custom-pipeline');
			expect(pipeline).not.toHaveClass('pipeline--compact');

			// 2 completed
			expect(container.querySelectorAll('.pipeline-step--completed')).toHaveLength(2);

			// 1 active with progress
			const activeStep = container.querySelector('.pipeline-step--active');
			expect(activeStep).toBeInTheDocument();
			expect(screen.getByText('75%')).toBeInTheDocument();

			// 2 pending (Review, Done)
			expect(container.querySelectorAll('.pipeline-step--pending')).toHaveLength(2);

			// Accessibility
			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuenow', '2');
			expect(progressbar).toHaveAttribute('aria-valuemax', '5');
			expect(progressbar).toHaveAttribute(
				'aria-valuetext',
				'Test phase in progress (75%). 2 of 5 phases completed.'
			);
		});
	});
});
