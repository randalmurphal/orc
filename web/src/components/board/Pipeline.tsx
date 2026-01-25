/**
 * Pipeline component - horizontal phase visualization for task execution.
 * Displays 5 phases (Plan, Code, Test, Review, Done) with status indicators.
 */

import { forwardRef, useMemo, type HTMLAttributes } from 'react';
import { Check, X, Pause, AlertCircle } from 'lucide-react';
import './Pipeline.css';

export interface PipelineProps extends HTMLAttributes<HTMLDivElement> {
	/** Array of phase names to display */
	phases: string[];
	/** Currently active phase name */
	currentPhase: string;
	/** List of completed phase names */
	completedPhases: string[];
	/** Phase that failed (if any) */
	failedPhase?: string;
	/** Progress percentage (0-100) for current phase */
	progress?: number;
	/** Size variant: compact hides labels */
	size?: 'compact' | 'default';
}

/** Visual status type for pipeline rendering */
type PipelineVisualStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped' | 'paused' | 'interrupted' | 'blocked';

/** Internal representation of a phase with computed status. */
interface PhaseState {
	name: string;
	status: PipelineVisualStatus;
	progress?: number;
}

/** Return type for computePhaseStates including computed count. */
interface PhaseStatesResult {
	phases: PhaseState[];
	completedCount: number;
}

/**
 * Computes the status of each phase based on current, completed, and failed phases.
 * Returns both phase states and completed count in a single pass.
 *
 * Handles all backend PhaseStatus values:
 * - pending: not started yet (gray)
 * - running: currently executing (pulsing blue)
 * - completed: finished successfully (green)
 * - failed: encountered error (red)
 * - skipped: intentionally skipped (gray with indicator)
 * - paused: temporarily stopped (yellow)
 * - interrupted: execution was interrupted (orange)
 * - blocked: waiting on external dependency (gray with lock)
 */
function computePhaseStates(
	phases: string[],
	currentPhase: string,
	completedPhases: string[],
	failedPhase?: string,
	progress?: number
): PhaseStatesResult {
	const completedSet = new Set(completedPhases.map((p) => p.toLowerCase()));
	let completedCount = 0;

	const phaseStates = phases.map((name) => {
		const nameLower = name.toLowerCase();

		if (failedPhase && failedPhase.toLowerCase() === nameLower) {
			return { name, status: 'failed' as const };
		}

		if (completedSet.has(nameLower)) {
			completedCount++;
			return { name, status: 'completed' as const };
		}

		if (currentPhase.toLowerCase() === nameLower) {
			return { name, status: 'running' as const, progress };
		}

		return { name, status: 'pending' as const };
	});

	return { phases: phaseStates, completedCount };
}

/**
 * Generates the aria-valuetext for accessibility.
 */
function getAriaValueText(phaseStates: PhaseState[], completedCount: number): string {
	const runningPhase = phaseStates.find((p) => p.status === 'running');
	const failedPhase = phaseStates.find((p) => p.status === 'failed');
	const pausedPhase = phaseStates.find((p) => p.status === 'paused');
	const interruptedPhase = phaseStates.find((p) => p.status === 'interrupted');
	const blockedPhase = phaseStates.find((p) => p.status === 'blocked');

	if (failedPhase) {
		return `${failedPhase.name} phase failed. ${completedCount} of ${phaseStates.length} phases completed.`;
	}

	if (pausedPhase) {
		return `${pausedPhase.name} phase paused. ${completedCount} of ${phaseStates.length} phases completed.`;
	}

	if (interruptedPhase) {
		return `${interruptedPhase.name} phase interrupted. ${completedCount} of ${phaseStates.length} phases completed.`;
	}

	if (blockedPhase) {
		return `${blockedPhase.name} phase blocked. ${completedCount} of ${phaseStates.length} phases completed.`;
	}

	if (runningPhase) {
		const progressText = runningPhase.progress !== undefined ? ` (${runningPhase.progress}%)` : '';
		return `${runningPhase.name} phase in progress${progressText}. ${completedCount} of ${phaseStates.length} phases completed.`;
	}

	return `${completedCount} of ${phaseStates.length} phases completed.`;
}

/**
 * Pipeline component for visualizing task execution phases.
 *
 * @example
 * // Basic usage
 * <Pipeline
 *   phases={["Plan", "Code", "Test", "Review", "Done"]}
 *   currentPhase="Code"
 *   completedPhases={["Plan"]}
 * />
 *
 * @example
 * // With progress
 * <Pipeline
 *   phases={["Plan", "Code", "Test", "Review", "Done"]}
 *   currentPhase="Code"
 *   completedPhases={["Plan"]}
 *   progress={45}
 * />
 *
 * @example
 * // Compact variant (no labels)
 * <Pipeline
 *   phases={["Plan", "Code", "Test", "Review", "Done"]}
 *   currentPhase="Test"
 *   completedPhases={["Plan", "Code"]}
 *   size="compact"
 * />
 *
 * @example
 * // Failed phase
 * <Pipeline
 *   phases={["Plan", "Code", "Test", "Review", "Done"]}
 *   currentPhase=""
 *   completedPhases={["Plan", "Code"]}
 *   failedPhase="Test"
 * />
 */
export const Pipeline = forwardRef<HTMLDivElement, PipelineProps>(
	(
		{
			phases,
			currentPhase,
			completedPhases,
			failedPhase,
			progress,
			size = 'default',
			className = '',
			...props
		},
		ref
	) => {
		const { phases: phaseStates, completedCount } = useMemo(
			() => computePhaseStates(phases, currentPhase, completedPhases, failedPhase, progress),
			[phases, currentPhase, completedPhases, failedPhase, progress]
		);

		const ariaValueText = useMemo(
			() => getAriaValueText(phaseStates, completedCount),
			[phaseStates, completedCount]
		);

		const classes = ['pipeline', size === 'compact' && 'pipeline--compact', className]
			.filter(Boolean)
			.join(' ');

		return (
			<div
				ref={ref}
				className={classes}
				role="progressbar"
				aria-valuenow={completedCount}
				aria-valuemin={0}
				aria-valuemax={phases.length}
				aria-valuetext={ariaValueText}
				{...props}
			>
				{phaseStates.map((phase) => (
					<div key={phase.name} className={`pipeline-step pipeline-step--${phase.status}`}>
						<div className="pipeline-bar">
							<div
								className={`pipeline-bar-fill pipeline-bar-fill--${phase.status}`}
								style={
									phase.status === 'running' && phase.progress !== undefined
										? { width: `${phase.progress}%` }
										: undefined
								}
							/>
						</div>
						<span className={`pipeline-label pipeline-label--${phase.status}`}>
							{phase.status === 'completed' && (
								<Check size={12} className="pipeline-icon pipeline-icon--success" aria-hidden="true" />
							)}
							{phase.status === 'skipped' && (
								<Check size={12} className="pipeline-icon pipeline-icon--muted" aria-hidden="true" />
							)}
							{phase.status === 'failed' && (
								<X size={12} className="pipeline-icon pipeline-icon--error" aria-hidden="true" />
							)}
							{phase.status === 'paused' && (
								<Pause size={12} className="pipeline-icon pipeline-icon--warning" aria-hidden="true" />
							)}
							{phase.status === 'interrupted' && (
								<AlertCircle size={12} className="pipeline-icon pipeline-icon--warning" aria-hidden="true" />
							)}
							{phase.status === 'blocked' && (
								<AlertCircle size={12} className="pipeline-icon pipeline-icon--muted" aria-hidden="true" />
							)}
							{phase.name}
							{phase.status === 'running' && phase.progress !== undefined && (
								<span className="pipeline-progress">{phase.progress}%</span>
							)}
						</span>
					</div>
				))}
			</div>
		);
	}
);

Pipeline.displayName = 'Pipeline';
