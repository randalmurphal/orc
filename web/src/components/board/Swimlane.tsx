/**
 * Swimlane component for Queue column
 *
 * Groups tasks by initiative with:
 * - Collapsible header with chevron, initiative icon, name, count, progress
 * - Smooth height animation on expand/collapse (0.2s)
 * - Support for 'Unassigned' tasks without initiative
 * - Empty state with 'No tasks' message
 */

import { memo, useCallback } from 'react';
import { TaskCard } from './TaskCard';
import { type Task, TaskStatus } from '@/gen/orc/v1/task_pb';
import type { Initiative } from '@/gen/orc/v1/initiative_pb';
import './Swimlane.css';

export interface SwimlaneProps {
	/** Initiative object, or null for unassigned tasks */
	initiative: Initiative | null;
	/** Tasks belonging to this swimlane */
	tasks: Task[];
	/** Whether the swimlane is collapsed */
	isCollapsed: boolean;
	/** Callback to toggle collapsed state */
	onToggle: () => void;
	/** Optional callback when a task card is clicked */
	onTaskClick?: (task: Task) => void;
	/** Optional callback for task context menu */
	onContextMenu?: (task: Task, e: React.MouseEvent) => void;
	/** Map of task ID to pending decision count */
	taskDecisionCounts?: Map<string, number>;
}

/** Color themes for initiative icons and progress bars */
type ColorTheme = 'purple' | 'green' | 'amber' | 'blue' | 'cyan' | 'default';

/** Map initiative ID to consistent color theme */
function getColorTheme(id: string | null): ColorTheme {
	if (!id) return 'default';
	// Use simple hash to get consistent color for each initiative
	const hash = id.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0);
	const colors: ColorTheme[] = ['purple', 'green', 'amber', 'blue', 'cyan'];
	return colors[hash % colors.length];
}

/** Get emoji for initiative (first char of title or fallback) */
function getInitiativeEmoji(initiative: Initiative | null): string {
	if (!initiative) return '?';
	// If title starts with an emoji, use it
	const emojiMatch = initiative.title.match(/^[\p{Emoji}]/u);
	if (emojiMatch) return emojiMatch[0];
	// Otherwise use first character as icon representation
	return initiative.title.charAt(0).toUpperCase();
}

export const Swimlane = memo(function Swimlane({
	initiative,
	tasks,
	isCollapsed,
	onToggle,
	onTaskClick,
	onContextMenu,
	taskDecisionCounts,
}: SwimlaneProps) {
	// Calculate progress
	const completedCount = tasks.filter((t) => t.status === TaskStatus.COMPLETED).length;
	const totalCount = tasks.length;
	const progress = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;

	const swimlaneId = initiative?.id ?? 'unassigned';
	const swimlaneTitle = initiative?.title ?? 'Unassigned';
	const colorTheme = getColorTheme(initiative?.id ?? null);
	const emoji = getInitiativeEmoji(initiative);

	// Handle keyboard for toggle
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onToggle();
			}
		},
		[onToggle]
	);

	const swimlaneClasses = ['swimlane', isCollapsed && 'collapsed'].filter(Boolean).join(' ');

	return (
		<div className={swimlaneClasses} data-testid={`swimlane-${swimlaneId}`}>
			{/* Header */}
			<div
				className="swimlane-header"
				onClick={onToggle}
				onKeyDown={handleKeyDown}
				role="button"
				tabIndex={0}
				aria-expanded={!isCollapsed}
				aria-controls={`swimlane-content-${swimlaneId}`}
			>
				{/* Collapse/expand chevron */}
				<span className="swimlane-chevron" aria-hidden="true">
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="12"
						height="12"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<polyline points="6 9 12 15 18 9" />
					</svg>
				</span>

				{/* Initiative icon (emoji in colored circle) */}
				<span
					className={`swimlane-icon ${initiative ? colorTheme : 'unassigned'}`}
					aria-hidden="true"
				>
					{emoji}
				</span>

				{/* Initiative name and meta */}
				<span className="swimlane-info">
					<span className="swimlane-name" title={swimlaneTitle}>
						{swimlaneTitle}
					</span>
					{initiative && (
						<span className="swimlane-meta">
							{completedCount}/{totalCount} complete
						</span>
					)}
				</span>

				{/* Task count badge */}
				<span className="swimlane-count">{totalCount}</span>

				{/* Progress bar */}
				<span className="swimlane-progress" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100}>
					<span
						className={`swimlane-progress-fill ${colorTheme}`}
						style={{ width: `${progress}%` }}
					/>
				</span>
			</div>

			{/* Content area */}
			<div
				id={`swimlane-content-${swimlaneId}`}
				className="swimlane-tasks swimlane-content"
				aria-hidden={isCollapsed}
			>
				{tasks.length === 0 ? (
					<div className="swimlane-empty">No tasks</div>
				) : (
					tasks.map((task, index) => (
						<TaskCard
							key={task.id}
							task={task}
							position={index + 1}
							onTaskClick={onTaskClick}
							onTaskContextMenu={onContextMenu}
							pendingDecisionCount={taskDecisionCounts?.get(task.id) ?? 0}
						/>
					))
				)}
			</div>
		</div>
	);
});
