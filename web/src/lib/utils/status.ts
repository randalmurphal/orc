/**
 * Shared status style utilities for consistent styling across components
 */

import type { TaskStatus, TaskWeight, PhaseStatus, ExecutionInfo } from '$lib/types';

// Style definition types
export interface StatusStyle {
	bg: string;
	text: string;
	icon: string;
	glow?: string;
	label: string;
}

export interface PhaseStyle {
	bg: string;
	text: string;
	icon: string;
}

export interface WeightStyle {
	bg: string;
	text: string;
}

/**
 * Task status styles
 * Used by: StatusIndicator, Dashboard activity items, TaskCard
 */
export const taskStatusStyles: Record<TaskStatus, StatusStyle> = {
	created: {
		bg: 'var(--bg-tertiary)',
		text: 'var(--text-muted)',
		icon: '',
		glow: 'transparent',
		label: 'Created'
	},
	classifying: {
		bg: 'var(--status-warning-bg)',
		text: 'var(--status-warning)',
		icon: '',
		glow: 'var(--status-warning-glow)',
		label: 'Classifying'
	},
	planned: {
		bg: 'var(--bg-tertiary)',
		text: 'var(--text-secondary)',
		icon: '',
		glow: 'transparent',
		label: 'Planned'
	},
	running: {
		bg: 'var(--accent-subtle)',
		text: 'var(--accent-primary)',
		icon: '',
		glow: 'var(--accent-glow)',
		label: 'Running'
	},
	paused: {
		bg: 'var(--status-warning-bg)',
		text: 'var(--status-warning)',
		icon: '',
		glow: 'var(--status-warning-glow)',
		label: 'Paused'
	},
	blocked: {
		bg: 'var(--status-danger-bg)',
		text: 'var(--status-danger)',
		icon: '',
		glow: 'var(--status-danger-glow)',
		label: 'Blocked'
	},
	completed: {
		bg: 'var(--status-success-bg)',
		text: 'var(--status-success)',
		icon: '\u2713', // checkmark
		glow: 'transparent',
		label: 'Completed'
	},
	failed: {
		bg: 'var(--status-danger-bg)',
		text: 'var(--status-danger)',
		icon: '\u2717', // X mark
		glow: 'transparent',
		label: 'Failed'
	}
};

/**
 * Phase status styles
 * Used by: Timeline nodes, phase indicators
 */
export const phaseStatusStyles: Record<PhaseStatus, PhaseStyle> = {
	pending: {
		bg: 'var(--bg-tertiary)',
		text: 'var(--text-muted)',
		icon: ''
	},
	running: {
		bg: 'var(--accent-subtle)',
		text: 'var(--accent-primary)',
		icon: ''
	},
	completed: {
		bg: 'var(--status-success-bg)',
		text: 'var(--status-success)',
		icon: '\u2713' // checkmark
	},
	failed: {
		bg: 'var(--status-danger-bg)',
		text: 'var(--status-danger)',
		icon: '\u2717' // X mark
	},
	skipped: {
		bg: 'var(--bg-tertiary)',
		text: 'var(--text-muted)',
		icon: '\u2212' // minus
	}
};

/**
 * Task weight styles
 * Used by: TaskCard weight badges
 */
export const weightStyles: Record<TaskWeight, WeightStyle> = {
	trivial: {
		text: 'var(--weight-trivial)',
		bg: 'rgba(107, 114, 128, 0.15)'
	},
	small: {
		text: 'var(--weight-small)',
		bg: 'var(--status-success-bg)'
	},
	medium: {
		text: 'var(--weight-medium)',
		bg: 'var(--status-info-bg)'
	},
	large: {
		text: 'var(--weight-large)',
		bg: 'var(--status-warning-bg)'
	},
	greenfield: {
		text: 'var(--weight-greenfield)',
		bg: 'var(--accent-subtle)'
	}
};

// Default fallback styles
const defaultTaskStyle: StatusStyle = taskStatusStyles.created;
const defaultPhaseStyle: PhaseStyle = phaseStatusStyles.pending;
const defaultWeightStyle: WeightStyle = weightStyles.small;

/**
 * Get style for a task status with fallback
 */
export function getTaskStatusStyle(status: string): StatusStyle {
	return taskStatusStyles[status as TaskStatus] ?? defaultTaskStyle;
}

/**
 * Get style for a phase status with fallback
 */
export function getPhaseStatusStyle(status: string): PhaseStyle {
	return phaseStatusStyles[status as PhaseStatus] ?? defaultPhaseStyle;
}

/**
 * Get style for a task weight with fallback
 */
export function getWeightStyle(weight: string): WeightStyle {
	return weightStyles[weight as TaskWeight] ?? defaultWeightStyle;
}

/**
 * Check if a status indicates an animated state (running)
 */
export function isAnimatedStatus(status: string): boolean {
	return status === 'running';
}

/**
 * Check if a status indicates a paused/blinking state
 */
export function isPausedStatus(status: string): boolean {
	return status === 'paused';
}

/**
 * Check if a status indicates completion (success or failure)
 */
export function isTerminalStatus(status: string): boolean {
	return status === 'completed' || status === 'failed';
}

/**
 * Check if a status indicates success
 */
export function isSuccessStatus(status: string): boolean {
	return status === 'completed';
}

/**
 * Check if a status indicates failure or blocked state
 */
export function isErrorStatus(status: string): boolean {
	return status === 'failed' || status === 'blocked';
}

/**
 * Check if a task can be run (in runnable state)
 */
export function isRunnableStatus(status: string): boolean {
	return status === 'created' || status === 'planned';
}

/**
 * Check if a task can be paused
 */
export function isPausableStatus(status: string): boolean {
	return status === 'running';
}

/**
 * Check if a task can be resumed
 */
export function isResumableStatus(status: string): boolean {
	return status === 'paused';
}

/**
 * Check if a task appears to be orphaned (running but executor is dead).
 * This is a client-side heuristic - the server performs the actual PID check.
 * @param status The task status
 * @param execution The execution info from state
 * @param staleThresholdMs How long without heartbeat before considered stale (default 5 min)
 */
export function isOrphanedTask(
	status: string,
	execution?: ExecutionInfo,
	staleThresholdMs: number = 5 * 60 * 1000
): boolean {
	// Only running tasks can be orphaned
	if (status !== 'running') {
		return false;
	}

	// No execution info suggests legacy state or orphaned
	if (!execution) {
		return true;
	}

	// Check if heartbeat is stale
	const lastHeartbeat = new Date(execution.last_heartbeat).getTime();
	const now = Date.now();
	return now - lastHeartbeat > staleThresholdMs;
}

/**
 * Get a display label for orphaned status
 */
export function getOrphanReason(execution?: ExecutionInfo): string {
	if (!execution) {
		return 'No executor info';
	}
	return `Executor PID ${execution.pid} on ${execution.hostname} - heartbeat stale`;
}
