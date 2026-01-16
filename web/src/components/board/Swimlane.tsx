/**
 * Swimlane component for Kanban board
 *
 * Groups tasks by initiative in horizontal rows with:
 * - Collapsible header with progress bar
 * - Columns for each phase within the swimlane
 */

import { useCallback } from 'react';
import { TaskCard } from './TaskCard';
import { Button } from '@/components/ui/Button';
import type { Task, Initiative } from '@/lib/types';
import type { FinalizeState } from '@/lib/api';
import type { ColumnConfig } from './Column';
import './Swimlane.css';

interface SwimlaneProps {
	initiative: Initiative | null; // null = unassigned tasks
	tasks: Task[];
	columns: ColumnConfig[];
	tasksByColumn: Record<string, Task[]>;
	collapsed: boolean;
	onToggleCollapse: () => void;
	onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
	onTaskClick?: (task: Task) => void;
	onFinalizeClick?: (task: Task) => void;
	onInitiativeClick?: (initiativeId: string) => void;
	getFinalizeState?: (taskId: string) => FinalizeState | null | undefined;
}

export function Swimlane({
	initiative,
	tasks,
	columns,
	tasksByColumn,
	collapsed,
	onToggleCollapse,
	onAction,
	onTaskClick,
	onFinalizeClick,
	onInitiativeClick,
	getFinalizeState,
}: SwimlaneProps) {
	// Calculate progress
	const completedCount = tasks.filter(
		(t) => t.status === 'completed' || t.status === 'finished'
	).length;
	const totalCount = tasks.length;
	const progress = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;

	const swimlaneId = initiative?.id ?? 'unassigned';
	const swimlaneTitle = initiative?.title ?? 'Unassigned';

	// Handle keyboard for toggle
	const handleKeydown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onToggleCollapse();
			}
		},
		[onToggleCollapse]
	);

	const swimlaneClasses = ['swimlane', collapsed && 'collapsed'].filter(Boolean).join(' ');

	return (
		<div className={swimlaneClasses}>
			<Button
				variant="ghost"
				size="sm"
				className="swimlane-header"
				onClick={onToggleCollapse}
				onKeyDown={handleKeydown}
				aria-expanded={!collapsed}
				aria-controls={`swimlane-content-${swimlaneId ?? 'unassigned'}`}
				leftIcon={
					<svg
						className={`collapse-icon ${collapsed ? 'collapsed' : ''}`}
						xmlns="http://www.w3.org/2000/svg"
						width="14"
						height="14"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<polyline points="6 9 12 15 18 9" />
					</svg>
				}
			>
				<span className="swimlane-title">{swimlaneTitle}</span>
				<span className="task-count">
					{completedCount}/{totalCount}
				</span>
				<div className="progress-bar">
					<div className="progress-fill" style={{ width: `${progress}%` }} />
				</div>
				<span className="progress-percent">{progress}%</span>
			</Button>

			{!collapsed && (
				<div
					id={`swimlane-content-${swimlaneId}`}
					className="swimlane-content"
				>
					{columns.map((column) => {
						const columnTasks = tasksByColumn[column.id] || [];
						return (
							<div
								key={column.id}
								className="swimlane-column"
							>
								{columnTasks.length === 0 ? (
									<div className="empty-column" />
								) : (
									columnTasks.map((task) => (
										<TaskCard
											key={task.id}
											task={task}
											onAction={onAction}
											onTaskClick={onTaskClick}
											onFinalizeClick={onFinalizeClick}
											onInitiativeClick={onInitiativeClick}
											finalizeState={getFinalizeState?.(task.id)}
										/>
									))
								)}
							</div>
						);
					})}
				</div>
			)}
		</div>
	);
}
