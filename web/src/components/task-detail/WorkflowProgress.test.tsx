/**
 * Tests for WorkflowProgress component
 *
 * TDD tests for the workflow progress visualization that shows
 * phase progression with gate symbols between phases.
 *
 * Success Criteria Coverage:
 * - SC-1: Page header displays back link, task ID, title, workflow name, branch, elapsed time
 * - SC-2: Workflow progress component renders phases as nodes with gate diamonds between them
 * - SC-3: Gate symbols show correct status colors
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { WorkflowProgress } from './WorkflowProgress';
import { TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import {
	createMockTask,
	createMockPhase,
	createMockTaskPlan,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';

describe('WorkflowProgress', () => {
	describe('SC-2: Renders phases as nodes with gate diamonds', () => {
		it('renders all phases from the plan as nodes', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-3', name: 'review', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			render(<WorkflowProgress task={task} plan={plan} />);

			// All phase names should be visible
			expect(screen.getByText('spec')).toBeInTheDocument();
			expect(screen.getByText('implement')).toBeInTheDocument();
			expect(screen.getByText('review')).toBeInTheDocument();
		});

		it('renders gate diamond symbols between phases', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-3', name: 'review', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// Gate diamonds should be present between phases (entry gate + 2 between gates + exit gate = 4)
			const gateDiamonds = container.querySelectorAll('.workflow-progress__gate');
			expect(gateDiamonds.length).toBeGreaterThanOrEqual(3);
		});

		it('renders checkmark (✓) for completed phases', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// Completed phase should have completed indicator
			const completedPhase = container.querySelector('.workflow-progress__phase--completed');
			expect(completedPhase).toBeInTheDocument();
		});

		it('renders dot (●) for running phase', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// Running phase should have running indicator
			const runningPhase = container.querySelector('.workflow-progress__phase--running');
			expect(runningPhase).toBeInTheDocument();
		});

		it('renders circle (○) for pending phases', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-3', name: 'review', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'spec',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// Pending phases should have pending indicators
			const pendingPhases = container.querySelectorAll('.workflow-progress__phase--pending');
			expect(pendingPhases.length).toBe(2); // implement and review are pending
		});

		it('renders skipped phases with distinct indicator and class', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'review', status: PhaseStatus.SKIPPED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'spec',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			const skippedPhase = container.querySelector('.workflow-progress__phase--skipped');
			expect(skippedPhase).toBeInTheDocument();

			const skippedIndicator = skippedPhase?.querySelector(
				'.workflow-progress__phase-indicator'
			);
			expect(skippedIndicator).toHaveTextContent('⊘');
			expect(skippedIndicator).not.toHaveTextContent('○');
		});

		it('renders single phase workflow correctly', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			render(<WorkflowProgress task={task} plan={plan} />);

			expect(screen.getByText('implement')).toBeInTheDocument();
		});

		it('renders fallback when plan is not available', () => {
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			render(<WorkflowProgress task={task} plan={null} />);

			// Should show simple text fallback
			expect(screen.getByText(/implement/i)).toBeInTheDocument();
		});
	});

	describe('SC-3: Gate symbols show correct status colors', () => {
		// Gate types come from workflow phase definitions, not plan phases
		// The component needs workflow phase data to render gate types correctly

		it('renders passthrough gates with gray color when gate type is unspecified', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const workflowPhases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'spec',
					sequence: 1,
				}),
				createMockWorkflowPhase({
					id: 2,
					phaseTemplateId: 'implement',
					sequence: 2,
				}),
			];
			const phaseTemplates = [
				createMockPhaseTemplate({ id: 'spec', gateType: GateType.UNSPECIFIED }),
				createMockPhaseTemplate({ id: 'implement', gateType: GateType.UNSPECIFIED }),
			];
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(
				<WorkflowProgress
					task={task}
					plan={plan}
					workflowPhases={workflowPhases}
					phaseTemplates={phaseTemplates}
				/>
			);

			const passthroughGate = container.querySelector('.workflow-progress__gate--passthrough');
			expect(passthroughGate).toBeInTheDocument();
		});

		it('renders auto gates with blue color', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const workflowPhases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];
			const phaseTemplates = [
				createMockPhaseTemplate({ id: 'spec', gateType: GateType.AUTO }),
				createMockPhaseTemplate({ id: 'implement', gateType: GateType.AUTO }),
			];
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(
				<WorkflowProgress
					task={task}
					plan={plan}
					workflowPhases={workflowPhases}
					phaseTemplates={phaseTemplates}
				/>
			);

			const autoGate = container.querySelector('.workflow-progress__gate--auto');
			expect(autoGate).toBeInTheDocument();
		});

		it('renders human gates with yellow color', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const workflowPhases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];
			const phaseTemplates = [
				createMockPhaseTemplate({ id: 'spec', gateType: GateType.HUMAN }),
				createMockPhaseTemplate({ id: 'implement', gateType: GateType.HUMAN }),
			];
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(
				<WorkflowProgress
					task={task}
					plan={plan}
					workflowPhases={workflowPhases}
					phaseTemplates={phaseTemplates}
				/>
			);

			const humanGate = container.querySelector('.workflow-progress__gate--human');
			expect(humanGate).toBeInTheDocument();
		});

		it('renders AI gates with purple color', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const workflowPhases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];
			const phaseTemplates = [
				createMockPhaseTemplate({ id: 'spec', gateType: GateType.AI }),
				createMockPhaseTemplate({ id: 'implement', gateType: GateType.AI }),
			];
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			const { container } = render(
				<WorkflowProgress
					task={task}
					plan={plan}
					workflowPhases={workflowPhases}
					phaseTemplates={phaseTemplates}
				/>
			);

			const aiGate = container.querySelector('.workflow-progress__gate--ai');
			expect(aiGate).toBeInTheDocument();
		});

		it('renders passed gates with green color override (status overrides type)', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.COMPLETED }),
				],
			});
			const workflowPhases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];
			const phaseTemplates = [
				createMockPhaseTemplate({ id: 'spec', gateType: GateType.HUMAN }),
				createMockPhaseTemplate({ id: 'implement', gateType: GateType.HUMAN }),
			];
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});

			const { container } = render(
				<WorkflowProgress
					task={task}
					plan={plan}
					workflowPhases={workflowPhases}
					phaseTemplates={phaseTemplates}
				/>
			);

			// Gates between completed phases should show passed status (green overrides yellow)
			const passedGate = container.querySelector('.workflow-progress__gate--passed');
			expect(passedGate).toBeInTheDocument();
		});

		it('renders blocked gates with red color override', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const workflowPhases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];
			const phaseTemplates = [
				createMockPhaseTemplate({ id: 'spec', gateType: GateType.HUMAN }),
				createMockPhaseTemplate({ id: 'implement', gateType: GateType.HUMAN }),
			];
			const task = createMockTask({
				status: TaskStatus.BLOCKED,
				currentPhase: 'implement',
			});

			const { container } = render(
				<WorkflowProgress
					task={task}
					plan={plan}
					workflowPhases={workflowPhases}
					phaseTemplates={phaseTemplates}
				/>
			);

			// Gate before blocked phase should show blocked status (red overrides yellow)
			const blockedGate = container.querySelector('.workflow-progress__gate--blocked');
			expect(blockedGate).toBeInTheDocument();
		});

		it('renders gate as passed between completed and skipped phases', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'review', status: PhaseStatus.SKIPPED }),
					createMockPhase({ id: 'phase-3', name: 'docs', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'docs',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			const passedGates = container.querySelectorAll('.workflow-progress__gate--passed');
			expect(passedGates.length).toBeGreaterThanOrEqual(1);
		});
	});

	describe('Edge Cases', () => {
		it('handles task with many phases (10+) gracefully', () => {
			const phases = Array.from({ length: 10 }, (_, i) =>
				createMockPhase({
					id: `phase-${i}`,
					name: `phase-${i}`,
					status: i < 3 ? PhaseStatus.COMPLETED : PhaseStatus.PENDING,
				})
			);
			const plan = createMockTaskPlan({ phases });
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'phase-3',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// Should render all phases or provide scrolling/compact mode
			const phaseNodes = container.querySelectorAll('.workflow-progress__phase');
			expect(phaseNodes.length).toBe(10);
		});

		it('handles task just created (no phase started)', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.CREATED,
				currentPhase: '',
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// All phases should show as pending
			const pendingPhases = container.querySelectorAll('.workflow-progress__phase--pending');
			expect(pendingPhases.length).toBe(2);
		});

		it('handles completed task (all phases done)', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// All phases should show as completed
			const completedPhases = container.querySelectorAll('.workflow-progress__phase--completed');
			expect(completedPhases.length).toBe(2);
		});
	});
});

/**
 * TASK-740: Phase click interaction tests
 *
 * These tests verify the click-to-navigate functionality for workflow phases.
 * Clicking a phase should call onPhaseClick callback with the phase name.
 */
describe('Phase Click Interaction (TASK-740)', () => {
	describe('SC-1: WorkflowProgress calls onPhaseClick when phase is clicked', () => {
		it('calls onPhaseClick with phase name when phase node is clicked', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
					createMockPhase({ id: 'phase-3', name: 'review', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});
			const onPhaseClick = vi.fn();

			render(<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />);

			// Click on the 'spec' phase
			const specPhase = screen.getByText('spec').closest('.workflow-progress__phase');
			expect(specPhase).toBeInTheDocument();
			fireEvent.click(specPhase!);

			expect(onPhaseClick).toHaveBeenCalledTimes(1);
			expect(onPhaseClick).toHaveBeenCalledWith('spec');
		});

		it('calls onPhaseClick with correct phase name for each phase', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});
			const onPhaseClick = vi.fn();

			render(<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />);

			// Click on 'implement' phase
			const implPhase = screen.getByText('implement').closest('.workflow-progress__phase');
			fireEvent.click(implPhase!);

			expect(onPhaseClick).toHaveBeenCalledWith('implement');
		});

		it('does not throw when onPhaseClick is not provided', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'spec',
			});

			render(<WorkflowProgress task={task} plan={plan} />);

			// Click should not throw
			const specPhase = screen.getByText('spec').closest('.workflow-progress__phase');
			expect(() => fireEvent.click(specPhase!)).not.toThrow();
		});
	});

	describe('SC-2: Phase nodes display cursor:pointer when clickable', () => {
		it('adds clickable class when onPhaseClick is provided', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});
			const onPhaseClick = vi.fn();

			const { container } = render(
				<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />
			);

			// Phase should have clickable class
			const phase = container.querySelector('.workflow-progress__phase--clickable');
			expect(phase).toBeInTheDocument();
		});

		it('does not add clickable class when onPhaseClick is not provided', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});

			const { container } = render(<WorkflowProgress task={task} plan={plan} />);

			// Phase should NOT have clickable class
			const phase = container.querySelector('.workflow-progress__phase--clickable');
			expect(phase).not.toBeInTheDocument();
		});
	});

	describe('SC-3: Keyboard accessibility', () => {
		it('triggers onPhaseClick when Enter key is pressed on phase', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});
			const onPhaseClick = vi.fn();

			render(<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />);

			const specPhase = screen.getByText('spec').closest('.workflow-progress__phase');
			fireEvent.keyDown(specPhase!, { key: 'Enter' });

			expect(onPhaseClick).toHaveBeenCalledWith('spec');
		});

		it('triggers onPhaseClick when Space key is pressed on phase', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});
			const onPhaseClick = vi.fn();

			render(<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />);

			const specPhase = screen.getByText('spec').closest('.workflow-progress__phase');
			fireEvent.keyDown(specPhase!, { key: ' ' });

			expect(onPhaseClick).toHaveBeenCalledWith('spec');
		});

		it('phase has tabIndex when clickable', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});
			const onPhaseClick = vi.fn();

			render(<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />);

			const specPhase = screen.getByText('spec').closest('.workflow-progress__phase');
			expect(specPhase).toHaveAttribute('tabIndex', '0');
		});

		it('phase has role="button" when clickable', () => {
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
				],
			});
			const task = createMockTask({
				status: TaskStatus.COMPLETED,
			});
			const onPhaseClick = vi.fn();

			render(<WorkflowProgress task={task} plan={plan} onPhaseClick={onPhaseClick} />);

			const specPhase = screen.getByText('spec').closest('.workflow-progress__phase');
			expect(specPhase).toHaveAttribute('role', 'button');
		});
	});
});
