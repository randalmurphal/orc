/**
 * Pipeline component - horizontal phase visualization for task execution.
 * Displays 5 phases (Plan, Code, Test, Review, Done) with status indicators.
 */

import { forwardRef, useMemo, type HTMLAttributes } from 'react';
import { Check, X } from 'lucide-react';
import { Icon } from '../core/Icon';
import './Pipeline.css';

export type PhaseStatus = 'pending' | 'active' | 'completed' | 'failed';

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

interface PhaseState {
	name: string;
	status: PhaseStatus;
	progress?: number;
}

/**
 * Computes the status of each phase based on current, completed, and failed phases.
 */
function computePhaseStates(
	phases: string[],
	currentPhase: string,
	completedPhases: string[],
	failedPhase?: string,
	progress?: number
): PhaseState[] {
	const completedSet = new Set(completedPhases.map((p) => p.toLowerCase()));

	return phases.map((name) => {
		const nameLower = name.toLowerCase();

		if (failedPhase && failedPhase.toLowerCase() === nameLower) {
			return { name, status: 'failed' as const };
		}

		if (completedSet.has(nameLower)) {
			return { name, status: 'completed' as const };
		}

		if (currentPhase.toLowerCase() === nameLower) {
			return { name, status: 'active' as const, progress };
		}

		return { name, status: 'pending' as const };
	});
}

/**
 * Generates the aria-valuetext for accessibility.
 */
function getAriaValueText(phaseStates: PhaseState[]): string {
	const activePhase = phaseStates.find((p) => p.status === 'active');
	const completedCount = phaseStates.filter((p) => p.status === 'completed').length;
	const failedPhase = phaseStates.find((p) => p.status === 'failed');

	if (failedPhase) {
		return `${failedPhase.name} phase failed. ${completedCount} of ${phaseStates.length} phases completed.`;
	}

	if (activePhase) {
		const progressText = activePhase.progress !== undefined ? ` (${activePhase.progress}%)` : '';
		return `${activePhase.name} phase in progress${progressText}. ${completedCount} of ${phaseStates.length} phases completed.`;
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
		const phaseStates = useMemo(
			() => computePhaseStates(phases, currentPhase, completedPhases, failedPhase, progress),
			[phases, currentPhase, completedPhases, failedPhase, progress]
		);

		const completedCount = phaseStates.filter((p) => p.status === 'completed').length;
		const ariaValueText = getAriaValueText(phaseStates);

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
									phase.status === 'active' && phase.progress !== undefined
										? { width: `${phase.progress}%` }
										: undefined
								}
							/>
						</div>
						<span className={`pipeline-label pipeline-label--${phase.status}`}>
							{phase.status === 'completed' && (
								<Icon name={Check} size="xs" color="success" aria-hidden />
							)}
							{phase.status === 'failed' && <Icon name={X} size="xs" color="error" aria-hidden />}
							{phase.name}
							{phase.status === 'active' && phase.progress !== undefined && (
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
