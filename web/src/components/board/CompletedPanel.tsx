/**
 * CompletedPanel component for right panel showing completed tasks summary
 *
 * Displays a green-themed compact summary section with:
 * - Checkmark icon
 * - 'X tasks today' count
 * - Stats: 'Y tokens, Z cost' (formatted)
 * - Expandable list of completed task IDs
 *
 * Reference: example_ui/board.html (.completed-summary class)
 */

import { useState, useCallback, useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import { formatNumber, formatCost } from '@/lib/format';
import type { Task } from '@/gen/orc/v1/task_pb';
import './CompletedPanel.css';

export interface CompletedPanelProps {
	/** Number of tasks completed today */
	completedCount: number;
	/** Total tokens used today */
	todayTokens: number;
	/** Total cost today in dollars */
	todayCost: number;
	/** Recent completed tasks (for expandable list) */
	recentTasks?: Task[];
}

/**
 * CompletedPanel displays completed tasks summary with token/cost stats.
 */
export function CompletedPanel({
	completedCount,
	todayTokens,
	todayCost,
	recentTasks = [],
}: CompletedPanelProps) {
	const [expanded, setExpanded] = useState(false);

	const handleToggle = useCallback(() => {
		setExpanded((prev) => !prev);
	}, []);

	const handleKeyDown = useCallback(
		(event: React.KeyboardEvent) => {
			if (event.key === 'Enter' || event.key === ' ') {
				event.preventDefault();
				handleToggle();
			}
		},
		[handleToggle]
	);

	// Format stats string
	const statsText = useMemo(() => {
		const tokenStr = formatNumber(todayTokens);
		const costStr = formatCost(todayCost);
		return `${tokenStr} tokens · ${costStr}`;
	}, [todayTokens, todayCost]);

	// Determine if we have tasks to show in expanded view
	const hasExpandableContent = recentTasks.length > 0;

	// Empty state message
	if (completedCount === 0) {
		return (
			<div className="completed-panel panel-section">
				<div className="panel-header completed-header-compact">
					<div className="panel-title">
						<div className="panel-title-icon green">
							<Icon name="check" size={12} />
						</div>
						<span>Completed</span>
					</div>
					<span className="completed-empty-text">No tasks completed today</span>
				</div>
			</div>
		);
	}

	return (
		<div className={`completed-panel panel-section ${expanded ? 'expanded' : ''}`}>
			<button
				className="panel-header completed-header-compact"
				onClick={handleToggle}
				onKeyDown={handleKeyDown}
				aria-expanded={hasExpandableContent ? expanded : undefined}
				aria-controls={hasExpandableContent ? 'completed-panel-body' : undefined}
				aria-label={`Completed: ${completedCount} tasks today, ${statsText}`}
				disabled={!hasExpandableContent}
			>
				<div className="panel-title">
					<div className="panel-title-icon green">
						<Icon name="check" size={12} />
					</div>
					<span>Completed</span>
				</div>
				<div className="completed-summary">
					<span className="panel-badge green" aria-label={`${completedCount} tasks completed`}>
						{completedCount}
					</span>
				</div>
				{hasExpandableContent && (
					<Icon
						name={expanded ? 'chevron-down' : 'chevron-right'}
						size={12}
						className="panel-chevron"
					/>
				)}
			</button>

			{hasExpandableContent && expanded && (
				<div id="completed-panel-body" className="panel-body" role="region">
					<div className="completed-stats-detail">
						<span className="completed-stats-tokens">
							{formatNumber(todayTokens)} tokens
						</span>
						<span className="completed-stats-separator">·</span>
						<span className="completed-stats-cost">{formatCost(todayCost)}</span>
					</div>
					<ul className="completed-task-list">
						{recentTasks.map((task) => (
							<li key={task.id} className="completed-task-item">
								<span className="completed-task-id">{task.id}</span>
								<span className="completed-task-title" title={task.title}>
									{task.title}
								</span>
							</li>
						))}
					</ul>
				</div>
			)}
		</div>
	);
}
