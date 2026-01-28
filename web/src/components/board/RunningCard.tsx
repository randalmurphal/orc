/* eslint-disable react-refresh/only-export-components */
/**
 * RunningCard - Expanded card component for actively executing tasks.
 *
 * Displays rich execution context including:
 * - Header with task ID, title, initiative badge, phase name, elapsed timer
 * - Pipeline visualization showing current phase progress
 * - Collapsible output section with color-coded lines
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Pipeline } from './Pipeline';
import { type Task, type ExecutionState, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import './RunningCard.css';

export interface RunningCardProps {
	/** The task being displayed */
	task: Task;
	/** Current execution state of the task (from WebSocket or task.execution) */
	state?: ExecutionState;
	/** Whether the output section is expanded */
	expanded?: boolean;
	/** Callback when card is clicked to toggle expand */
	onToggleExpand?: () => void;
	/** Output lines to display (passed from parent, typically via WebSocket) */
	outputLines?: string[];
	/** Additional CSS class names */
	className?: string;
	/** Number of pending decisions for this task */
	pendingDecisionCount?: number;
}

/** Output line with type for color coding */
export interface OutputLine {
	type: 'success' | 'error' | 'info' | 'default';
	content: string;
}

/** Standard 5 phases for pipeline display */
const DISPLAY_PHASES = ['Plan', 'Code', 'Test', 'Review', 'Done'];

/** Map internal phase names to display names */
function mapPhaseToDisplay(phase: string): string {
	const mapping: Record<string, string> = {
		spec: 'Plan',
		design: 'Plan',
		research: 'Plan',
		implement: 'Code',
		review: 'Review',
		test: 'Test',
		docs: 'Done',
		validate: 'Done',
	};
	return mapping[phase.toLowerCase()] || phase;
}

/** Get completed phases based on task state */
function getCompletedPhases(state: ExecutionState | undefined): string[] {
	const completed: string[] = [];
	if (!state) return completed;
	const phases = state.phases || {};

	for (const [phaseName, phaseState] of Object.entries(phases)) {
		if (phaseState.status === PhaseStatus.COMPLETED) {
			const displayName = mapPhaseToDisplay(phaseName);
			if (!completed.includes(displayName)) {
				completed.push(displayName);
			}
		}
	}

	return completed;
}

/** Format elapsed time as MM:SS or H:MM:SS */
function formatElapsedTime(startedAt: Date | null): string {
	if (!startedAt) return '0:00';

	const start = startedAt.getTime();
	const now = Date.now();
	const elapsed = Math.max(0, Math.floor((now - start) / 1000));

	const hours = Math.floor(elapsed / 3600);
	const minutes = Math.floor((elapsed % 3600) / 60);
	const seconds = elapsed % 60;

	if (hours > 0) {
		return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
	}
	return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

/** Parse output content and determine line type */
function parseOutputLine(line: string): OutputLine {
	const trimmed = line.trim();

	// Success: lines with checkmarks or containing 'success'
	if (trimmed.startsWith('✓') || trimmed.toLowerCase().includes('success')) {
		return { type: 'success', content: line };
	}

	// Error: lines with X marks or containing 'error'/'fail'
	if (
		trimmed.startsWith('✗') ||
		trimmed.toLowerCase().includes('error') ||
		trimmed.toLowerCase().includes('fail')
	) {
		return { type: 'error', content: line };
	}

	// Info: lines with arrows or containing 'info'
	if (trimmed.startsWith('→') || trimmed.startsWith('◐') || trimmed.toLowerCase().includes('info')) {
		return { type: 'info', content: line };
	}

	return { type: 'default', content: line };
}

/** Build accessible aria-label for running card */
function buildAriaLabel(task: Task, expanded: boolean): string {
	const rawPhase = task.currentPhase || 'starting';
	const displayPhase = mapPhaseToDisplay(rawPhase);
	const parts = [`Running task ${task.id}: ${task.title}`, `phase: ${displayPhase}`];

	if (task.initiativeId) {
		parts.push(`initiative: ${task.initiativeId}`);
	}

	parts.push(expanded ? 'expanded' : 'collapsed');

	return parts.join(', ');
}

/**
 * RunningCard component for displaying active task execution.
 */
export function RunningCard({
	task,
	state,
	expanded = false,
	onToggleExpand,
	outputLines: rawOutputLines = [],
	className = '',
	pendingDecisionCount = 0,
}: RunningCardProps) {
	// Get started timestamp from task (proto Timestamp -> Date)
	const startedAt = useMemo(() => timestampToDate(task.startedAt), [task.startedAt]);

	// Elapsed time with live updates
	const [elapsedTime, setElapsedTime] = useState(() =>
		formatElapsedTime(startedAt)
	);

	// Update elapsed time every second while task is running
	useEffect(() => {
		if (!startedAt) return;

		// Update immediately
		setElapsedTime(formatElapsedTime(startedAt));

		// Set up interval for live updates
		const interval = setInterval(() => {
			setElapsedTime(formatElapsedTime(startedAt));
		}, 1000);

		return () => clearInterval(interval);
	}, [startedAt]);

	// Current phase for display
	const currentPhase = useMemo(() => {
		const phase = task.currentPhase || '';
		return mapPhaseToDisplay(phase);
	}, [task.currentPhase]);

	// Completed phases for pipeline
	const completedPhases = useMemo(() => getCompletedPhases(state), [state]);

	// Parse raw output lines into typed output lines with color coding
	const parsedOutputLines = useMemo(() => {
		return rawOutputLines.map(parseOutputLine);
	}, [rawOutputLines]);

	// Click handler
	const handleClick = useCallback(() => {
		onToggleExpand?.();
	}, [onToggleExpand]);

	// Keyboard handler for accessibility
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onToggleExpand?.();
			}
		},
		[onToggleExpand]
	);

	// Build class names
	const hasPendingDecision = pendingDecisionCount > 0;
	const cardClasses = [
		'running-card',
		expanded && 'expanded',
		hasPendingDecision && 'has-pending-decision',
		className
	].filter(Boolean).join(' ');

	return (
		<article
			className={cardClasses}
			data-task-id={task.id}
			onClick={handleClick}
			onKeyDown={handleKeyDown}
			tabIndex={0}
			role="button"
			aria-label={buildAriaLabel(task, expanded)}
			aria-expanded={expanded}
		>
			{/* Header */}
			<div className="running-header">
				<div className="running-info">
					<div className="running-id">{task.id}</div>
					<h3 className="running-title">{task.title}</h3>
					{task.initiativeId && (
						<div className="running-initiative">
							<span className="running-initiative-dot" />
							{task.initiativeId}
						</div>
					)}
				</div>
				<div className="running-stats">
					<div className="running-phase">{currentPhase || 'Starting'}</div>
					<div className="running-time">{elapsedTime}</div>
				</div>
			</div>

			{/* Pipeline */}
			<div className="running-card__pipeline">
				<Pipeline
					phases={DISPLAY_PHASES}
					currentPhase={currentPhase}
					completedPhases={completedPhases}
					size="default"
				/>
			</div>

			{/* Output section (collapsible) */}
			<div className={`running-output ${expanded ? 'expanded' : ''}`}>
				{parsedOutputLines.length > 0 ? (
					parsedOutputLines.slice(-50).map((line, index) => (
						<span key={index} className={`output-line ${line.type}`}>
							{line.content}
						</span>
					))
				) : (
					<span className="output-line output-empty">No output yet</span>
				)}
			</div>

			{/* Expand toggle indicator */}
			<div className="running-expand-toggle" aria-hidden="true">
				<Icon name={expanded ? 'chevron-up' : 'chevron-down'} size={12} />
			</div>
		</article>
	);
}

// Export utilities for parent components
export { parseOutputLine, formatElapsedTime, mapPhaseToDisplay };
