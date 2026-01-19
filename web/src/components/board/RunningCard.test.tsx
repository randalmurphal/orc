import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { RunningCard, parseOutputLine, formatElapsedTime, mapPhaseToDisplay } from './RunningCard';
import type { Task, TaskState } from '@/lib/types';

// Sample task for testing
const createTask = (overrides: Partial<Task> = {}): Task => ({
	id: 'TASK-001',
	title: 'Test Task',
	description: 'A test task description',
	weight: 'medium',
	status: 'running',
	current_phase: 'implement',
	branch: 'orc/TASK-001',
	created_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z',
	started_at: '2024-01-01T00:00:00Z',
	...overrides,
});

// Sample task state for testing
const createTaskState = (overrides: Partial<TaskState> = {}): TaskState => ({
	task_id: 'TASK-001',
	current_phase: 'implement',
	current_iteration: 1,
	status: 'running',
	started_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z',
	phases: {
		spec: {
			status: 'completed',
			iterations: 1,
			tokens: { input_tokens: 100, output_tokens: 50, total_tokens: 150 },
		},
	},
	gates: [],
	tokens: { input_tokens: 1000, output_tokens: 500, total_tokens: 1500 },
	...overrides,
});

function renderRunningCard(
	task: Task,
	state: TaskState,
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
			const { container } = renderRunningCard(createTask(), createTaskState());

			expect(container.querySelector('.running-card')).toBeInTheDocument();
		});
	});

	describe('header display', () => {
		it('displays task ID with monospace styling', () => {
			const { container } = renderRunningCard(createTask({ id: 'TASK-123' }), createTaskState());

			const taskId = container.querySelector('.running-id');
			expect(taskId).toBeInTheDocument();
			expect(taskId?.textContent).toBe('TASK-123');
		});

		it('displays task title', () => {
			const { container } = renderRunningCard(
				createTask({ title: 'My Test Task' }),
				createTaskState()
			);

			const title = container.querySelector('.running-title');
			expect(title).toBeInTheDocument();
			expect(title?.textContent).toBe('My Test Task');
		});

		it('displays phase name', () => {
			const { container } = renderRunningCard(
				createTask(),
				createTaskState({ current_phase: 'implement' })
			);

			const phase = container.querySelector('.running-phase');
			expect(phase).toBeInTheDocument();
			expect(phase?.textContent).toBe('Code'); // implement maps to Code
		});

		it('displays elapsed time', () => {
			const { container } = renderRunningCard(
				createTask({ started_at: '2024-01-01T00:00:00Z' }),
				createTaskState({ started_at: '2024-01-01T00:00:00Z' })
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
				createTask(),
				createTaskState({
					current_phase: 'implement',
					phases: {
						spec: {
							status: 'completed',
							iterations: 1,
							tokens: { input_tokens: 100, output_tokens: 50, total_tokens: 150 },
						},
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
		it('renders initiative badge when task.initiative_id is set', () => {
			const { container } = renderRunningCard(
				createTask({ initiative_id: 'INIT-001' }),
				createTaskState()
			);

			const initiative = container.querySelector('.running-initiative');
			expect(initiative).toBeInTheDocument();
			expect(initiative?.textContent).toContain('INIT-001');
		});

		it('does not render initiative badge when initiative_id is not set', () => {
			const { container } = renderRunningCard(
				createTask({ initiative_id: undefined }),
				createTaskState()
			);

			const initiative = container.querySelector('.running-initiative');
			expect(initiative).not.toBeInTheDocument();
		});
	});

	describe('output section visibility', () => {
		it('output section is hidden by default', () => {
			const { container } = renderRunningCard(createTask(), createTaskState());

			const output = container.querySelector('.running-output');
			expect(output).toBeInTheDocument();
			expect(output).not.toHaveClass('expanded');
		});

		it('output section is visible when expanded=true', () => {
			const { container } = renderRunningCard(createTask(), createTaskState(), {
				expanded: true,
			});

			const output = container.querySelector('.running-output');
			expect(output).toHaveClass('expanded');
		});
	});

	describe('expand/collapse interaction', () => {
		it('calls onToggleExpand when card is clicked', () => {
			const onToggleExpand = vi.fn();
			const { container } = renderRunningCard(createTask(), createTaskState(), {
				onToggleExpand,
			});

			const card = container.querySelector('.running-card')!;
			fireEvent.click(card);

			expect(onToggleExpand).toHaveBeenCalledTimes(1);
		});

		it('calls onToggleExpand on Enter key', () => {
			const onToggleExpand = vi.fn();
			const { container } = renderRunningCard(createTask(), createTaskState(), {
				onToggleExpand,
			});

			const card = container.querySelector('.running-card')!;
			fireEvent.keyDown(card, { key: 'Enter' });

			expect(onToggleExpand).toHaveBeenCalledTimes(1);
		});

		it('calls onToggleExpand on Space key', () => {
			const onToggleExpand = vi.fn();
			const { container } = renderRunningCard(createTask(), createTaskState(), {
				onToggleExpand,
			});

			const card = container.querySelector('.running-card')!;
			fireEvent.keyDown(card, { key: ' ' });

			expect(onToggleExpand).toHaveBeenCalledTimes(1);
		});

		it('does not crash when onToggleExpand is not provided', () => {
			const { container } = renderRunningCard(createTask(), createTaskState());

			const card = container.querySelector('.running-card')!;
			expect(() => fireEvent.click(card)).not.toThrow();
		});
	});

	describe('accessibility', () => {
		it('has role="button"', () => {
			renderRunningCard(createTask(), createTaskState());

			const card = screen.getByRole('button');
			expect(card).toBeInTheDocument();
		});

		it('has tabIndex=0 for keyboard focus', () => {
			const { container } = renderRunningCard(createTask(), createTaskState());

			const card = container.querySelector('.running-card');
			expect(card).toHaveAttribute('tabindex', '0');
		});

		it('has aria-expanded attribute matching expanded state', () => {
			const { container, rerender } = renderRunningCard(createTask(), createTaskState(), {
				expanded: false,
			});

			let card = container.querySelector('.running-card');
			expect(card).toHaveAttribute('aria-expanded', 'false');

			// Re-render with expanded=true
			rerender(<RunningCard task={createTask()} state={createTaskState()} expanded={true} />);

			card = container.querySelector('.running-card');
			expect(card).toHaveAttribute('aria-expanded', 'true');
		});

		it('has correct aria-label format', () => {
			renderRunningCard(
				createTask({ id: 'TASK-001', title: 'Test Task', initiative_id: 'INIT-001' }),
				createTaskState({ current_phase: 'implement' })
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
			const { container } = renderRunningCard(createTask(), createTaskState(), {
				className: 'custom-class',
			});

			const card = container.querySelector('.running-card');
			expect(card).toHaveClass('custom-class');
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

	it('returns "0:00" when no started_at provided', () => {
		expect(formatElapsedTime(undefined)).toBe('0:00');
	});

	it('formats seconds correctly', () => {
		vi.setSystemTime(new Date('2024-01-01T00:00:45Z'));
		expect(formatElapsedTime('2024-01-01T00:00:00Z')).toBe('0:45');
	});

	it('formats minutes and seconds correctly', () => {
		vi.setSystemTime(new Date('2024-01-01T00:05:30Z'));
		expect(formatElapsedTime('2024-01-01T00:00:00Z')).toBe('5:30');
	});

	it('formats hours, minutes, and seconds correctly', () => {
		vi.setSystemTime(new Date('2024-01-01T01:30:45Z'));
		expect(formatElapsedTime('2024-01-01T00:00:00Z')).toBe('1:30:45');
	});

	it('handles future times gracefully (returns 0:00)', () => {
		vi.setSystemTime(new Date('2024-01-01T00:00:00Z'));
		expect(formatElapsedTime('2024-01-01T01:00:00Z')).toBe('0:00');
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
