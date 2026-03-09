/**
 * WorkflowProgress component
 *
 * Visualizes phase progression with gate symbols between phases.
 * Shows completed (✓), running (●), and pending (○) phase states.
 * Gate diamonds show type colors with status overrides.
 */

import type { Task, TaskPlan, PlanPhase } from '@/gen/orc/v1/task_pb';
import { TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import type { WorkflowPhase, PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import './WorkflowProgress.css';

interface WorkflowProgressProps {
	task: Task;
	plan: TaskPlan | null;
	workflowPhases?: WorkflowPhase[];
	phaseTemplates?: PhaseTemplate[];
	onPhaseClick?: (phaseName: string) => void;
}

/**
 * Determine phase visual state based on task and phase status
 */
function getPhaseState(
	phase: PlanPhase,
	task: Task
): 'completed' | 'running' | 'pending' | 'failed' | 'skipped' {
	// Check phase status first
	if (phase.status === PhaseStatus.COMPLETED) {
		return 'completed';
	}
	if (phase.status === PhaseStatus.SKIPPED) {
		return 'skipped';
	}

	// Check if this is the current phase and task failed
	if (task.currentPhase === phase.name && task.status === TaskStatus.FAILED) {
		return 'failed';
	}

	// Check if this is the current running phase
	if (task.currentPhase === phase.name && task.status === TaskStatus.RUNNING) {
		return 'running';
	}

	return 'pending';
}

/**
 * Get the gate type for a phase from workflow/template data
 */
function getGateType(
	phaseName: string,
	workflowPhases?: WorkflowPhase[],
	phaseTemplates?: PhaseTemplate[]
): GateType {
	if (!workflowPhases || !phaseTemplates) {
		return GateType.UNSPECIFIED;
	}

	// Find the workflow phase
	const workflowPhase = workflowPhases.find(
		(wp) => wp.phaseTemplateId === phaseName
	);
	if (!workflowPhase) {
		return GateType.UNSPECIFIED;
	}

	// Find the phase template to get gate type
	const template = phaseTemplates.find((t) => t.id === phaseName);
	return template?.gateType ?? GateType.UNSPECIFIED;
}

/**
 * Determine gate visual state based on adjacent phases
 */
function getGateState(
	prevPhaseState: 'completed' | 'running' | 'pending' | 'failed' | 'skipped' | null,
	nextPhaseState: 'completed' | 'running' | 'pending' | 'failed' | 'skipped',
	task: Task
): 'passed' | 'blocked' | null {
	// If task is blocked and this is the gate before the current phase, show blocked
	if (
		task.status === TaskStatus.BLOCKED &&
		(nextPhaseState === 'running' || nextPhaseState === 'pending')
	) {
		// Check if we're at the gate before the blocked phase
		if (prevPhaseState === 'completed') {
			return 'blocked';
		}
	}

	// If the previous phase is completed and current is running/completed, gate passed
	if (
		(prevPhaseState === 'completed' || prevPhaseState === 'skipped') &&
		(nextPhaseState === 'completed' ||
			nextPhaseState === 'running' ||
			nextPhaseState === 'skipped')
	) {
		return 'passed';
	}

	return null;
}

/**
 * Get CSS class for gate type
 */
function getGateTypeClass(gateType: GateType): string {
	switch (gateType) {
		case GateType.AUTO:
			return 'workflow-progress__gate--auto';
		case GateType.HUMAN:
			return 'workflow-progress__gate--human';
		case GateType.AI:
			return 'workflow-progress__gate--ai';
		case GateType.SKIP:
		case GateType.UNSPECIFIED:
		default:
			return 'workflow-progress__gate--passthrough';
	}
}

export function WorkflowProgress({
	task,
	plan,
	workflowPhases,
	phaseTemplates,
	onPhaseClick,
}: WorkflowProgressProps) {
	// Fallback when plan is not available
	if (!plan || plan.phases.length === 0) {
		return (
			<div className="workflow-progress workflow-progress--fallback">
				<span className="workflow-progress__fallback-text">
					{task.currentPhase || 'No phases'}
				</span>
			</div>
		);
	}

	const phases = plan.phases;
	const phaseStates = phases.map((phase) => getPhaseState(phase, task));

	return (
		<div className="workflow-progress">
			{phases.map((phase, index) => {
				const phaseState = phaseStates[index];
				const prevPhaseState = index > 0 ? phaseStates[index - 1] : null;

				// Get gate info for this phase
				const gateType = getGateType(
					phase.name,
					workflowPhases,
					phaseTemplates
				);
				const gateState = getGateState(prevPhaseState, phaseState, task);
				const gateTypeClass = getGateTypeClass(gateType);

				return (
					<div key={phase.id} className="workflow-progress__segment">
						{/* Gate before phase (except for first phase, which has entry gate) */}
						<div
							className={`workflow-progress__gate ${gateTypeClass} ${
								gateState === 'passed'
									? 'workflow-progress__gate--passed'
									: gateState === 'blocked'
									? 'workflow-progress__gate--blocked'
									: ''
							}`}
							title={`Gate: ${GateType[gateType]}`}
						>
							<span className="workflow-progress__gate-diamond">◆</span>
						</div>

						{/* Phase node */}
						<div
							className={`workflow-progress__phase workflow-progress__phase--${phaseState}${onPhaseClick ? ' workflow-progress__phase--clickable' : ''}`}
							title={`${phase.name}: ${phaseState}`}
							onClick={() => onPhaseClick?.(phase.name)}
							onKeyDown={(e) => {
								if (onPhaseClick && (e.key === 'Enter' || e.key === ' ')) {
									e.preventDefault();
									onPhaseClick(phase.name);
								}
							}}
							tabIndex={onPhaseClick ? 0 : undefined}
							role={onPhaseClick ? 'button' : undefined}
						>
							<span className="workflow-progress__phase-indicator">
								{phaseState === 'completed' && '✓'}
								{phaseState === 'running' && '●'}
								{phaseState === 'pending' && '○'}
								{phaseState === 'failed' && '✗'}
								{phaseState === 'skipped' && '⊘'}
							</span>
							<span className="workflow-progress__phase-name">{phase.name}</span>
						</div>

						{/* Exit gate after last phase */}
						{index === phases.length - 1 && (
							<div
								className={`workflow-progress__gate ${gateTypeClass} ${
									phaseState === 'completed'
										? 'workflow-progress__gate--passed'
										: ''
								}`}
							>
								<span className="workflow-progress__gate-diamond">◆</span>
							</div>
						)}
					</div>
				);
			})}
		</div>
	);
}
