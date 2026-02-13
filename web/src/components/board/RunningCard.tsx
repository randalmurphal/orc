/* eslint-disable react-refresh/only-export-components */
/**
 * RunningCard - Enhanced card component for actively executing tasks with real-time progress updates.
 *
 * Displays rich execution context including:
 * - Header with task ID, title, initiative badge, phase name, elapsed timer
 * - Real-time activity indicators with current state
 * - Pipeline visualization showing current phase progress
 * - Live output section with color-coded lines
 * - Session metrics including tokens, cost, and duration
 */

import { memo, useCallback, useEffect, useMemo, useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Pipeline } from './Pipeline';
import { ActivityIndicator } from '../common/ActivityIndicator';
import { RealTimeMetrics } from '../common/RealTimeMetrics';
import { LiveOutput } from '../common/LiveOutput';
import { type Task, type ExecutionState, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { providerLabel, parseProviderModelTuple } from '@/lib/providerUtils';
import { timestampToDate } from '@/lib/time';
import { useTaskStore } from '@/stores/taskStore';
import './RunningCard.css';

export interface RunningCardProps {
	/** The task being displayed */
	task: Task;
	/** Current execution state of the task (from WebSocket or task.execution) */
	executionState?: ExecutionState;
	/** Whether the output section is expanded */
	isExpanded?: boolean;
	/** Callback when card is clicked to toggle expand */
	onToggleExpand?: (taskId: string) => void;
	/** Additional CSS class names */
	className?: string;
	/** Number of pending decisions for this task */
	pendingDecisionCount?: number;
}

/** Standard 5 phases for pipeline display */
const DISPLAY_PHASES = ['Plan', 'Code', 'Test', 'Review', 'Done'];

/** Map internal phase names to display names */
function mapPhaseToDisplay(phase: string): string {
	const mapping: Record<string, string> = {
		spec: 'Plan',
		tiny_spec: 'Plan',
		design: 'Plan',
		research: 'Plan',
		tdd_write: 'Plan',
		breakdown: 'Plan',
		implement: 'Code',
		review: 'Review',
		test: 'Test',
		docs: 'Done',
		validate: 'Done',
	};
	return mapping[phase.toLowerCase()] || phase;
}

/** Get completed phases based on execution state */
function getCompletedPhases(executionState: ExecutionState | undefined): string[] {
	const completed: string[] = [];
	const phases = executionState?.phases || {};

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

/** Build accessible aria-label for running card */
function buildAriaLabel(task: Task, isExpanded: boolean): string {
	const rawPhase = task.currentPhase || 'starting';
	const displayPhase = mapPhaseToDisplay(rawPhase);
	const parts = [`Running task ${task.id}: ${task.title}`, `phase: ${displayPhase}`];

	if (task.initiativeId) {
		parts.push(`initiative: ${task.initiativeId}`);
	}

	parts.push(isExpanded ? 'expanded' : 'collapsed');

	return parts.join(', ');
}

/**
 * RunningCard component for displaying active task execution with real-time updates.
 */
export const RunningCard = memo(function RunningCard({
	task,
	executionState,
	isExpanded = false,
	onToggleExpand,
	className = '',
	pendingDecisionCount = 0,
}: RunningCardProps) {
	const {
		getTaskActivity,
		getTaskOutputLines,
		getSessionMetrics,
		getPhaseProgress
	} = useTaskStore();

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
	const completedPhases = useMemo(() => getCompletedPhases(executionState), [executionState]);

	// Get real-time data from stores
	const taskActivity = getTaskActivity(task.id);
	const outputLines = getTaskOutputLines(task.id) || [];
	const sessionMetrics = getSessionMetrics(task.id);
	const phaseProgress = getPhaseProgress(task.id);

	// Extract provider from session model (may be "provider:model" tuple)
	const sessionModel = executionState?.session?.model ?? '';
	const { provider: sessionProvider } = parseProviderModelTuple(sessionModel);

	// Click handler
	const handleClick = useCallback(() => {
		onToggleExpand?.(task.id);
	}, [onToggleExpand, task.id]);

	// Keyboard handler for accessibility
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onToggleExpand?.(task.id);
			}
		},
		[onToggleExpand, task.id]
	);

	// Build class names
	const hasPendingDecision = pendingDecisionCount > 0;
	const cardClasses = [
		'running-card',
		isExpanded && 'expanded',
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
			aria-label={buildAriaLabel(task, isExpanded)}
			aria-expanded={isExpanded}
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
					{sessionProvider && sessionProvider !== 'claude' && (
						<span
							className="running-provider-badge"
							style={{
								fontSize: '0.7rem',
								padding: '1px 6px',
								borderRadius: '3px',
								background: 'var(--color-surface-alt, #333)',
								color: 'var(--color-text-secondary, #aaa)',
							}}
						>
							{providerLabel(sessionProvider)}
						</span>
					)}
				</div>
				<div className="running-stats">
					<div className="running-phase">{currentPhase || 'Starting'}</div>
					<div className="running-time">{elapsedTime}</div>
				</div>
			</div>

			{/* Real-time Activity Indicator */}
			{taskActivity && (
				<div className="running-activity">
					<ActivityIndicator
						activity={taskActivity.activity.toString()}
						phase={taskActivity.phase}
						className="mb-2"
					/>
				</div>
			)}

			{/* Pipeline */}
			<div className="running-card__pipeline">
				<Pipeline
					phases={DISPLAY_PHASES}
					currentPhase={currentPhase}
					completedPhases={completedPhases}
					size="default"
				/>
			</div>

			{/* Session Metrics (when expanded) */}
			{isExpanded && sessionMetrics && (
				<div className="running-metrics">
					<RealTimeMetrics
						taskId={task.id}
						sessionMetrics={sessionMetrics}
						phaseProgress={phaseProgress}
						showDetailed={false}
					/>
				</div>
			)}

			{/* Live Output section (when expanded) */}
			{isExpanded && (
				<div className="running-output-section">
					<LiveOutput
						taskId={task.id}
						outputLines={outputLines}
						maxLines={50}
						showTimestamps={false}
						autoScroll={true}
						searchable={false}
						allowCopy={true}
					/>
				</div>
			)}

			{/* Expand toggle indicator */}
			<div className="running-expand-toggle" aria-hidden="true">
				<Icon name={isExpanded ? 'chevron-up' : 'chevron-down'} size={12} />
			</div>
		</article>
	);
});

/** Output line type classification */
export interface OutputLine {
	type: 'success' | 'error' | 'info' | 'default';
	content: string;
}

/** Parse output line and classify by content */
function parseOutputLine(line: string): OutputLine {
	const content = line.trim();

	// Success patterns
	if (content.includes('✓') || /\b(success|successful|completed|passed)\b/i.test(content)) {
		return { type: 'success', content };
	}

	// Error patterns
	if (content.includes('✗') || /\b(error|failed?|exception)\b/i.test(content)) {
		return { type: 'error', content };
	}

	// Info patterns (arrows, spinners, processing indicators)
	if (content.includes('→') || content.includes('◐') || /\b(processing|running|analyzing)\b/i.test(content)) {
		return { type: 'info', content };
	}

	// Default
	return { type: 'default', content };
}

// Export utilities for parent components
export { formatElapsedTime, mapPhaseToDisplay, parseOutputLine };