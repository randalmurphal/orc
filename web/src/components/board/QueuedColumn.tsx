/**
 * QueuedColumn component for Kanban board
 *
 * Special column for "Queued" status with:
 * - Active section (always visible)
 * - Backlog section (collapsible)
 * Backlog tasks have dashed border styling.
 */

import { useCallback } from 'react';
import { TaskCard } from './TaskCard';
import { Button } from '@/components/ui/Button';
import type { Task } from '@/lib/types';
import type { FinalizeState } from '@/lib/api';
import type { ColumnConfig } from './Column';
import './QueuedColumn.css';

interface QueuedColumnProps {
	column: ColumnConfig;
	activeTasks: Task[];
	backlogTasks: Task[];
	showBacklog: boolean;
	onToggleBacklog: () => void;
	onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
	onTaskClick?: (task: Task) => void;
	onFinalizeClick?: (task: Task) => void;
	onInitiativeClick?: (initiativeId: string) => void;
	getFinalizeState?: (taskId: string) => FinalizeState | null | undefined;
}

export function QueuedColumn({
	column,
	activeTasks,
	backlogTasks,
	showBacklog,
	onToggleBacklog,
	onAction,
	onTaskClick,
	onFinalizeClick,
	onInitiativeClick,
	getFinalizeState,
}: QueuedColumnProps) {
	const handleToggleKeydown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onToggleBacklog();
			}
		},
		[onToggleBacklog]
	);

	const totalCount = activeTasks.length + backlogTasks.length;

	return (
		<div
			className="queued-column"
			role="region"
			aria-label={`${column.title} column`}
		>
			<div className="column-header">
				<h2>{column.title}</h2>
				<span className="count">{totalCount}</span>
			</div>

			<div className="column-content">
				{/* Active section */}
				<div className="active-section">
					{activeTasks.length === 0 ? (
						<div className="empty">No active tasks</div>
					) : (
						activeTasks.map((task) => (
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

				{/* Backlog divider and toggle */}
				{backlogTasks.length > 0 && (
					<>
						<div className="backlog-divider">
							<Button
								variant="ghost"
								size="sm"
								className="backlog-toggle"
								onClick={onToggleBacklog}
								onKeyDown={handleToggleKeydown}
								aria-expanded={showBacklog}
								aria-controls="backlog-section"
								leftIcon={
									<svg
										className={`toggle-icon ${showBacklog ? 'expanded' : ''}`}
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
										<polyline points="9 18 15 12 9 6" />
									</svg>
								}
							>
								<span className="backlog-label">Backlog</span>
								<span className="backlog-count">{backlogTasks.length}</span>
							</Button>
						</div>

						{/* Backlog section */}
						<div
							id="backlog-section"
							className={`backlog-section ${showBacklog ? 'expanded' : ''}`}
							aria-hidden={!showBacklog}
						>
							{backlogTasks.map((task) => (
								<TaskCard
									key={task.id}
									task={task}
									onAction={onAction}
									onTaskClick={onTaskClick}
									onFinalizeClick={onFinalizeClick}
									onInitiativeClick={onInitiativeClick}
									finalizeState={getFinalizeState?.(task.id)}
								/>
							))}
						</div>
					</>
				)}
			</div>
		</div>
	);
}
