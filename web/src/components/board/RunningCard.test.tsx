import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { RunningCard, parseOutputLine, formatElapsedTime, mapPhaseToDisplay } from './RunningCard';
import type { Task, ExecutionState, PhaseState } from '@/gen/orc/v1/task_pb';
import { TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createTimestamp } from '@/test/factories';

// Helper to create mock ExecutionState
function createMockExecutionState(overrides: Partial<ExecutionState> = {}): ExecutionState {
	return {
		currentIteration: 1,
		phases: {
			spec: {
				status: PhaseStatus.COMPLETED,
				iterations: 1,
				startedAt: createTimestamp('2024-01-01T00:00:00Z'),
			} as PhaseState,
		},
		gates: [],
		tokens: { inputTokens: 1000, outputTokens: 500, totalTokens: 1500 } as ExecutionState['tokens'],
		...overrides,
	} as ExecutionState;
}

function renderRunningCard(
	task: Task,
	state?: ExecutionState,
	props: Partial<Parameters<typeof RunningCard>[0]> = {}
) {
	return render(<RunningCard task={task} state={state} {...props} />);
}

describe('RunningCard', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Mock Date.now for elapsed time calculations
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2024-01-01T00:05:30Z'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('rendering with minimal props', () => {
		it('renders with minimal props (task + state)', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			expect(container.querySelector('.running-card')).toBeInTheDocument();
		});
	});

	describe('header display', () => {
		it('displays task ID with monospace styling', () => {
			const { container } = renderRunningCard(
				createMockTask({ id: 'TASK-123', status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			const taskId = container.querySelector('.running-id');
			expect(taskId).toBeInTheDocument();
			expect(taskId?.textContent).toBe('TASK-123');
		});

		it('displays task title', () => {
			const { container } = renderRunningCard(
				createMockTask({ title: 'My Test Task', status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			const title = container.querySelector('.running-title');
			expect(title).toBeInTheDocument();
			expect(title?.textContent).toBe('My Test Task');
		});

		it('displays phase name', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING, currentPhase: 'implement' }),
				createMockExecutionState()
			);

			const phase = container.querySelector('.running-phase');
			expect(phase).toBeInTheDocument();
			expect(phase?.textContent).toBe('Code'); // implement maps to Code
		});

		it('displays elapsed time', () => {
			const { container } = renderRunningCard(
				createMockTask({
					status: TaskStatus.RUNNING,
					startedAt: createTimestamp('2024-01-01T00:00:00Z')
				}),
				createMockExecutionState()
			);

			const time = container.querySelector('.running-time');
			expect(time).toBeInTheDocument();
			// At 00:05:30, elapsed from 00:00:00 should be 5:30
			expect(time?.textContent).toBe('5:30');
		});
	});

	describe('Pipeline component integration', () => {
		it('renders Pipeline component with correct props', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING, currentPhase: 'implement' }),
				createMockExecutionState({
					phases: {
						spec: {
							status: PhaseStatus.COMPLETED,
							iterations: 1,
						} as PhaseState,
					},
				})
			);

			// Pipeline should be present
			const pipeline = container.querySelector('.pipeline');
			expect(pipeline).toBeInTheDocument();

			// Should have 5 steps (Plan, Code, Test, Review, Done)
			const steps = container.querySelectorAll('.pipeline-step');
			expect(steps.length).toBe(5);
		});
	});

	describe('initiative badge', () => {
		it('renders initiative badge when task.initiativeId is set', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING, initiativeId: 'INIT-001' }),
				createMockExecutionState()
			);

			const initiative = container.querySelector('.running-initiative');
			expect(initiative).toBeInTheDocument();
			expect(initiative?.textContent).toContain('INIT-001');
		});

		it('does not render initiative badge when initiativeId is not set', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING, initiativeId: undefined }),
				createMockExecutionState()
			);

			const initiative = container.querySelector('.running-initiative');
			expect(initiative).not.toBeInTheDocument();
		});
	});

	describe('output section visibility', () => {
		it('output section is hidden by default', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			const output = container.querySelector('.running-output');
			expect(output).toBeInTheDocument();
			expect(output).not.toHaveClass('expanded');
		});

		it('output section is visible when expanded=true', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{ expanded: true }
			);

			const output = container.querySelector('.running-output');
			expect(output).toHaveClass('expanded');
		});
	});

	describe('expand/collapse interaction', () => {
		it('calls onToggleExpand when card is clicked', () => {
			const onToggleExpand = vi.fn();
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{ onToggleExpand }
			);

			const card = container.querySelector('.running-card')!;
			fireEvent.click(card);

			expect(onToggleExpand).toHaveBeenCalledTimes(1);
		});

		it('calls onToggleExpand on Enter key', () => {
			const onToggleExpand = vi.fn();
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{ onToggleExpand }
			);

			const card = container.querySelector('.running-card')!;
			fireEvent.keyDown(card, { key: 'Enter' });

			expect(onToggleExpand).toHaveBeenCalledTimes(1);
		});

		it('calls onToggleExpand on Space key', () => {
			const onToggleExpand = vi.fn();
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{ onToggleExpand }
			);

			const card = container.querySelector('.running-card')!;
			fireEvent.keyDown(card, { key: ' ' });

			expect(onToggleExpand).toHaveBeenCalledTimes(1);
		});

		it('does not crash when onToggleExpand is not provided', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			const card = container.querySelector('.running-card')!;
			expect(() => fireEvent.click(card)).not.toThrow();
		});
	});

	describe('accessibility', () => {
		it('has role="button"', () => {
			renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			const card = screen.getByRole('button');
			expect(card).toBeInTheDocument();
		});

		it('has tabIndex=0 for keyboard focus', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState()
			);

			const card = container.querySelector('.running-card');
			expect(card).toHaveAttribute('tabindex', '0');
		});

		it('has aria-expanded attribute matching expanded state', () => {
			const { container, rerender } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{ expanded: false }
			);

			let card = container.querySelector('.running-card');
			expect(card).toHaveAttribute('aria-expanded', 'false');

			// Re-render with expanded=true
			rerender(
				<RunningCard
					task={createMockTask({ status: TaskStatus.RUNNING })}
					state={createMockExecutionState()}
					expanded={true}
				/>
			);

			card = container.querySelector('.running-card');
			expect(card).toHaveAttribute('aria-expanded', 'true');
		});

		it('has correct aria-label format', () => {
			renderRunningCard(
				createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					currentPhase: 'implement',
					initiativeId: 'INIT-001'
				}),
				createMockExecutionState()
			);

			const card = screen.getByRole('button');
			const ariaLabel = card.getAttribute('aria-label');
			expect(ariaLabel).toContain('TASK-001');
			expect(ariaLabel).toContain('Test Task');
			expect(ariaLabel).toContain('Code'); // implement maps to Code
			expect(ariaLabel).toContain('INIT-001');
		});
	});

	describe('className prop', () => {
		it('applies additional className', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{ className: 'custom-class' }
			);

			const card = container.querySelector('.running-card');
			expect(card).toHaveClass('custom-class');
		});
	});

	describe('output lines', () => {
		it('renders output lines with correct color classes', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{
					expanded: true,
					outputLines: [
						'✓ Success message',
						'✗ Error message',
						'→ Info message',
						'Regular message',
					],
				}
			);

			const outputLines = container.querySelectorAll('.output-line:not(.output-empty)');
			expect(outputLines.length).toBe(4);

			expect(outputLines[0]).toHaveClass('success');
			expect(outputLines[1]).toHaveClass('error');
			expect(outputLines[2]).toHaveClass('info');
			expect(outputLines[3]).toHaveClass('default');
		});

		it('truncates output to last 50 lines when content exceeds limit', () => {
			// Create 60 lines
			const manyLines = Array.from({ length: 60 }, (_, i) => `Line ${i + 1}`);

			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{
					expanded: true,
					outputLines: manyLines,
				}
			);

			const outputLines = container.querySelectorAll('.output-line:not(.output-empty)');
			expect(outputLines.length).toBe(50);

			// Should show lines 11-60 (last 50), not lines 1-50
			expect(outputLines[0].textContent).toBe('Line 11');
			expect(outputLines[49].textContent).toBe('Line 60');
		});

		it('shows "No output yet" when outputLines is empty', () => {
			const { container } = renderRunningCard(
				createMockTask({ status: TaskStatus.RUNNING }),
				createMockExecutionState(),
				{
					expanded: true,
					outputLines: [],
				}
			);

			const emptyMessage = container.querySelector('.output-empty');
			expect(emptyMessage).toBeInTheDocument();
			expect(emptyMessage?.textContent).toBe('No output yet');
		});
	});
});

describe('parseOutputLine', () => {
	it('identifies success lines (checkmark)', () => {
		const result = parseOutputLine('✓ Task completed');
		expect(result.type).toBe('success');
		expect(result.content).toBe('✓ Task completed');
	});

	it('identifies success lines (keyword)', () => {
		const result = parseOutputLine('Operation success');
		expect(result.type).toBe('success');
	});

	it('identifies error lines (X mark)', () => {
		const result = parseOutputLine('✗ Test failed');
		expect(result.type).toBe('error');
	});

	it('identifies error lines (error keyword)', () => {
		const result = parseOutputLine('Error: Something went wrong');
		expect(result.type).toBe('error');
	});

	it('identifies error lines (fail keyword)', () => {
		const result = parseOutputLine('Build failed');
		expect(result.type).toBe('error');
	});

	it('identifies info lines (arrow)', () => {
		const result = parseOutputLine('→ Processing...');
		expect(result.type).toBe('info');
	});

	it('identifies info lines (spinner)', () => {
		const result = parseOutputLine('◐ Running tests...');
		expect(result.type).toBe('info');
	});

	it('returns default for unmatched lines', () => {
		const result = parseOutputLine('Just a regular line');
		expect(result.type).toBe('default');
	});
});

describe('formatElapsedTime', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2024-01-01T01:30:45Z'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('returns "0:00" when no startedAt provided', () => {
		expect(formatElapsedTime(null)).toBe('0:00');
	});

	it('formats seconds correctly', () => {
		vi.setSystemTime(new Date('2024-01-01T00:00:45Z'));
		expect(formatElapsedTime(new Date('2024-01-01T00:00:00Z'))).toBe('0:45');
	});

	it('formats minutes and seconds correctly', () => {
		vi.setSystemTime(new Date('2024-01-01T00:05:30Z'));
		expect(formatElapsedTime(new Date('2024-01-01T00:00:00Z'))).toBe('5:30');
	});

	it('formats hours, minutes, and seconds correctly', () => {
		vi.setSystemTime(new Date('2024-01-01T01:30:45Z'));
		expect(formatElapsedTime(new Date('2024-01-01T00:00:00Z'))).toBe('1:30:45');
	});

	it('handles future times gracefully (returns 0:00)', () => {
		vi.setSystemTime(new Date('2024-01-01T00:00:00Z'));
		expect(formatElapsedTime(new Date('2024-01-01T01:00:00Z'))).toBe('0:00');
	});
});

describe('mapPhaseToDisplay', () => {
	it('maps spec to Plan', () => {
		expect(mapPhaseToDisplay('spec')).toBe('Plan');
	});

	it('maps design to Plan', () => {
		expect(mapPhaseToDisplay('design')).toBe('Plan');
	});

	it('maps research to Plan', () => {
		expect(mapPhaseToDisplay('research')).toBe('Plan');
	});

	it('maps implement to Code', () => {
		expect(mapPhaseToDisplay('implement')).toBe('Code');
	});

	it('maps review to Review', () => {
		expect(mapPhaseToDisplay('review')).toBe('Review');
	});

	it('maps test to Test', () => {
		expect(mapPhaseToDisplay('test')).toBe('Test');
	});

	it('maps docs to Done', () => {
		expect(mapPhaseToDisplay('docs')).toBe('Done');
	});

	it('maps validate to Done', () => {
		expect(mapPhaseToDisplay('validate')).toBe('Done');
	});

	it('handles case-insensitive input', () => {
		expect(mapPhaseToDisplay('IMPLEMENT')).toBe('Code');
		expect(mapPhaseToDisplay('Spec')).toBe('Plan');
	});

	it('passes through unknown phases', () => {
		expect(mapPhaseToDisplay('custom')).toBe('custom');
	});
});
