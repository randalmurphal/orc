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
import type { Task, TaskPriority, TaskCategory } from '@/lib/types';
import { CATEGORY_CONFIG } from '@/lib/types';
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
}

// Priority dot colors
const PRIORITY_COLORS: Record<TaskPriority, string> = {
	critical: 'var(--red)',
	high: 'var(--orange)',
	normal: 'var(--blue)',
	low: 'var(--text-muted)',
};

// Category icon mapping (to IconName)
const CATEGORY_ICONS: Record<TaskCategory, IconName> = {
	feature: 'sparkles',
	bug: 'bug',
	refactor: 'recycle',
	chore: 'tools',
	docs: 'file-text',
	test: 'beaker',
};

/**
 * Build accessible aria-label for task card
 */
function buildAriaLabel(task: Task): string {
	const priority = task.priority || 'normal';
	const category = task.category || 'feature';
	const parts = [`${task.id}: ${task.title}`, `${priority} priority`, category];

	if (task.is_blocked) {
		parts.push('blocked');
	}
	if (task.status === 'running') {
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
}: TaskCardProps) {
	const priority = (task.priority || 'normal') as TaskPriority;
	const category = (task.category || 'feature') as TaskCategory;
	const categoryConfig = CATEGORY_CONFIG[category];
	const categoryIcon = CATEGORY_ICONS[category];
	const priorityColor = PRIORITY_COLORS[priority];

	const isRunning = task.status === 'running';
	const isBlocked = task.is_blocked;
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
			onClick={handleClick}
			onContextMenu={handleContextMenu}
			onKeyDown={handleKeyDown}
			tabIndex={0}
			role="button"
			aria-label={buildAriaLabel(task)}
		>
			{/* Category icon */}
			<div
				className="task-card-category"
				style={{ color: categoryConfig.color }}
				title={categoryConfig.label}
			>
				<Icon name={categoryIcon} size={14} />
			</div>

			{/* Main content */}
			<div className="task-card-content">
				<div className="task-card-header">
					<span className="task-card-id">{task.id}</span>
					{showInitiative && task.initiative_id && (
						<span className="task-card-initiative">{task.initiative_id}</span>
					)}
				</div>
				<h3 className="task-card-title">{task.title}</h3>
			</div>

			{/* Status indicators */}
			<div className="task-card-status">
				{/* Blocked warning icon */}
				{isBlocked && (
					<span className="task-card-blocked" title={`Blocked by ${task.unmet_blockers?.join(', ')}`}>
						<Icon name="alert-triangle" size={12} />
					</span>
				)}

				{/* Running progress indicator */}
				{isRunning && (
					<span className="task-card-running" title={`Running: ${task.current_phase || 'starting'}`}>
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
