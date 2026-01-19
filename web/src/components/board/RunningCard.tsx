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
import type { Task, TaskState } from '@/lib/types';
import './RunningCard.css';

export interface RunningCardProps {
	/** The task being displayed */
	task: Task;
	/** Current execution state of the task */
	state: TaskState;
	/** Whether the output section is expanded */
	expanded?: boolean;
	/** Callback when card is clicked to toggle expand */
	onToggleExpand?: () => void;
	/** Additional CSS class names */
	className?: string;
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
function getCompletedPhases(state: TaskState): string[] {
	const completed: string[] = [];
	const phases = state.phases || {};

	for (const [phaseName, phaseState] of Object.entries(phases)) {
		if (phaseState.status === 'completed') {
			const displayName = mapPhaseToDisplay(phaseName);
			if (!completed.includes(displayName)) {
				completed.push(displayName);
			}
		}
	}

	return completed;
}

/** Format elapsed time as MM:SS or H:MM:SS */
function formatElapsedTime(startedAt: string | undefined): string {
	if (!startedAt) return '0:00';

	const start = new Date(startedAt).getTime();
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
function buildAriaLabel(task: Task, state: TaskState, expanded: boolean): string {
	const rawPhase = state.current_phase || task.current_phase || 'starting';
	const displayPhase = mapPhaseToDisplay(rawPhase);
	const parts = [`Running task ${task.id}: ${task.title}`, `phase: ${displayPhase}`];

	if (task.initiative_id) {
		parts.push(`initiative: ${task.initiative_id}`);
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
	className = '',
}: RunningCardProps) {
	// Elapsed time with live updates
	const [elapsedTime, setElapsedTime] = useState(() =>
		formatElapsedTime(state.started_at || task.started_at)
	);

	// Update elapsed time every second while task is running
	useEffect(() => {
		const startedAt = state.started_at || task.started_at;
		if (!startedAt) return;

		// Update immediately
		setElapsedTime(formatElapsedTime(startedAt));

		// Set up interval for live updates
		const interval = setInterval(() => {
			setElapsedTime(formatElapsedTime(startedAt));
		}, 1000);

		return () => clearInterval(interval);
	}, [state.started_at, task.started_at]);

	// Current phase for display
	const currentPhase = useMemo(() => {
		const phase = state.current_phase || task.current_phase || '';
		return mapPhaseToDisplay(phase);
	}, [state.current_phase, task.current_phase]);

	// Completed phases for pipeline
	const completedPhases = useMemo(() => getCompletedPhases(state), [state]);

	// Parse output lines from state (mock for now - actual output comes from parent)
	const outputLines = useMemo(() => {
		// Output would typically be passed in or come from WebSocket subscription
		// For now, return empty array - parent component handles output streaming
		return [] as OutputLine[];
	}, []);

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
	const cardClasses = ['running-card', expanded && 'expanded', className].filter(Boolean).join(' ');

	return (
		<article
			className={cardClasses}
			onClick={handleClick}
			onKeyDown={handleKeyDown}
			tabIndex={0}
			role="button"
			aria-label={buildAriaLabel(task, state, expanded)}
			aria-expanded={expanded}
		>
			{/* Header */}
			<div className="running-header">
				<div className="running-info">
					<div className="running-id">{task.id}</div>
					<h3 className="running-title">{task.title}</h3>
					{task.initiative_id && (
						<div className="running-initiative">
							<span className="running-initiative-dot" />
							{task.initiative_id}
						</div>
					)}
				</div>
				<div className="running-stats">
					<div className="running-phase">{currentPhase || 'Starting'}</div>
					<div className="running-time">{elapsedTime}</div>
				</div>
			</div>

			{/* Pipeline */}
			<div className="running-pipeline">
				<Pipeline
					phases={DISPLAY_PHASES}
					currentPhase={currentPhase}
					completedPhases={completedPhases}
					size="default"
				/>
			</div>

			{/* Output section (collapsible) */}
			<div className={`running-output ${expanded ? 'expanded' : ''}`}>
				{outputLines.length > 0 ? (
					outputLines.slice(-50).map((line, index) => (
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
