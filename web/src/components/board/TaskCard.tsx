/**
 * TaskCard - Compact card component for queue display
 *
 * Displays task information in a compact format optimized for queue columns:
 * - Task ID badge (monospace)
 * - Title (2-line max with ellipsis)
 * - Priority dot
 * - Category icon
 * - Status indicators (blocked warning, running progress)
 */

import { useCallback } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import { Tooltip } from '@/components/ui/Tooltip';
import { type Task, TaskStatus, TaskPriority, TaskCategory } from '@/gen/orc/v1/task_pb';
import './TaskCard.css';

interface TaskCardProps {
	task: Task;
	onClick?: () => void;
	onContextMenu?: (e: React.MouseEvent) => void;
	isSelected?: boolean;
	showInitiative?: boolean;
	className?: string;
	/** Number of pending decisions for this task */
	pendingDecisionCount?: number;
	/** Position number to display (1-based, for queue ordering) */
	position?: number;
}

// Priority dot colors
const PRIORITY_COLORS: Record<TaskPriority, string> = {
	[TaskPriority.CRITICAL]: 'var(--red)',
	[TaskPriority.HIGH]: 'var(--orange)',
	[TaskPriority.NORMAL]: 'var(--blue)',
	[TaskPriority.LOW]: 'var(--text-muted)',
	[TaskPriority.UNSPECIFIED]: 'var(--text-muted)',
};

// Category config with labels, colors, and icons
const CATEGORY_CONFIG: Record<TaskCategory, { label: string; color: string; icon: IconName }> = {
	[TaskCategory.FEATURE]: { label: 'Feature', color: 'var(--status-success)', icon: 'sparkles' },
	[TaskCategory.BUG]: { label: 'Bug', color: 'var(--status-error)', icon: 'bug' },
	[TaskCategory.REFACTOR]: { label: 'Refactor', color: 'var(--status-info)', icon: 'recycle' },
	[TaskCategory.CHORE]: { label: 'Chore', color: 'var(--text-muted)', icon: 'tools' },
	[TaskCategory.DOCS]: { label: 'Docs', color: 'var(--status-warning)', icon: 'file-text' },
	[TaskCategory.TEST]: { label: 'Test', color: 'var(--cyan)', icon: 'beaker' },
	[TaskCategory.UNSPECIFIED]: { label: 'Feature', color: 'var(--status-success)', icon: 'sparkles' },
};

/** Get human-readable priority label */
function getPriorityLabel(priority: TaskPriority): string {
	switch (priority) {
		case TaskPriority.CRITICAL: return 'critical';
		case TaskPriority.HIGH: return 'high';
		case TaskPriority.NORMAL: return 'normal';
		case TaskPriority.LOW: return 'low';
		default: return 'normal';
	}
}

/**
 * Build accessible aria-label for task card
 */
function buildAriaLabel(task: Task, position?: number): string {
	const priority = task.priority || TaskPriority.NORMAL;
	const category = task.category || TaskCategory.FEATURE;
	const priorityLabel = getPriorityLabel(priority);
	const categoryConfig = CATEGORY_CONFIG[category];
	const parts = [`${task.id}: ${task.title}`, `${priorityLabel} priority`, categoryConfig.label.toLowerCase()];

	if (position !== undefined) {
		parts.push(`position ${position}`);
	}
	if (task.isBlocked) {
		parts.push('blocked');
	}
	if (task.status === TaskStatus.RUNNING) {
		parts.push('running');
	}

	return parts.join(', ');
}

export function TaskCard({
	task,
	onClick,
	onContextMenu,
	isSelected = false,
	showInitiative = false,
	className = '',
	pendingDecisionCount = 0,
	position,
}: TaskCardProps) {
	const priority = task.priority || TaskPriority.NORMAL;
	const category = task.category || TaskCategory.FEATURE;
	const categoryConfig = CATEGORY_CONFIG[category];
	const priorityColor = PRIORITY_COLORS[priority];

	const isRunning = task.status === TaskStatus.RUNNING;
	const isBlocked = task.isBlocked;
	const hasPendingDecision = pendingDecisionCount > 0;

	// Click handler
	const handleClick = useCallback(() => {
		onClick?.();
	}, [onClick]);

	// Keyboard handler for accessibility
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onClick?.();
			}
		},
		[onClick]
	);

	// Context menu handler
	const handleContextMenu = useCallback(
		(e: React.MouseEvent) => {
			onContextMenu?.(e);
		},
		[onContextMenu]
	);

	// Build class names
	const cardClasses = [
		'task-card',
		isSelected && 'selected',
		isRunning && 'running',
		isBlocked && 'blocked',
		hasPendingDecision && 'has-pending-decision',
		className,
	]
		.filter(Boolean)
		.join(' ');

	return (
		<article
			className={cardClasses}
			data-task-id={task.id}
			onClick={handleClick}
			onContextMenu={handleContextMenu}
			onKeyDown={handleKeyDown}
			tabIndex={0}
			role="button"
			aria-label={buildAriaLabel(task, position)}
		>
			{/* Position number (optional) */}
			{position !== undefined && (
				<span className="task-position task-card-position">{position}</span>
			)}

			{/* Category icon */}
			<div
				className="task-card-category"
				style={{ color: categoryConfig.color }}
				title={categoryConfig.label}
			>
				<Icon name={categoryConfig.icon} size={14} />
			</div>

			{/* Main content */}
			<div className="task-card-content">
				<div className="task-card-header">
					<span className="task-card-id">{task.id}</span>
					{showInitiative && task.initiativeId && (
						<span className="task-card-initiative">{task.initiativeId}</span>
					)}
				</div>
				<Tooltip content={task.title} side="top">
					<h3 className="task-card-title">{task.title}</h3>
				</Tooltip>
			</div>

			{/* Status indicators */}
			<div className="task-card-status">
				{/* Blocked warning icon */}
				{isBlocked && (
					<span className="task-card-blocked" title={`Blocked by ${task.unmetBlockers?.join(', ')}`}>
						<Icon name="alert-triangle" size={12} />
					</span>
				)}

				{/* Running progress indicator */}
				{isRunning && (
					<span className="task-card-running" title={`Running: ${task.currentPhase || 'starting'}`}>
						<span className="task-card-running-dot" />
					</span>
				)}

				{/* Decision count badge */}
				{hasPendingDecision && (
					<span className="task-card-decision-badge" title={`${pendingDecisionCount} pending decision${pendingDecisionCount > 1 ? 's' : ''}`}>
						{pendingDecisionCount}
					</span>
				)}

				{/* Priority dot */}
				<span
					className="task-card-priority"
					style={{ backgroundColor: priorityColor }}
					title={`${priority} priority`}
				/>
			</div>
		</article>
	);
}
