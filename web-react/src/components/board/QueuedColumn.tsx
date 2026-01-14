/**
 * QueuedColumn component for Kanban board
 *
 * Special column for "Queued" status with:
 * - Active section (always visible)
 * - Backlog section (collapsible)
 * Backlog tasks have dashed border styling.
 */

import { useState, useCallback } from 'react';
import { TaskCard } from './TaskCard';
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
	onDrop: (task: Task) => void;
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
	onDrop,
	onAction,
	onTaskClick,
	onFinalizeClick,
	onInitiativeClick,
	getFinalizeState,
}: QueuedColumnProps) {
	const [dragOver, setDragOver] = useState(false);
	// Counter used via updater function, not read directly
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_dragCounter, setDragCounter] = useState(0);

	// Drag handlers
	const handleDragEnter = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setDragCounter((prev) => {
			const next = prev + 1;
			if (next > 0) setDragOver(true);
			return next;
		});
	}, []);

	const handleDragLeave = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setDragCounter((prev) => {
			const next = prev - 1;
			if (next === 0) setDragOver(false);
			return next;
		});
	}, []);

	const handleDragOver = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		e.dataTransfer.dropEffect = 'move';
	}, []);

	const handleDrop = useCallback(
		(e: React.DragEvent) => {
			e.preventDefault();
			setDragOver(false);
			setDragCounter(0);

			try {
				const taskData = e.dataTransfer.getData('application/json');
				if (taskData) {
					const task = JSON.parse(taskData) as Task;
					onDrop(task);
				}
			} catch (err) {
				console.error('Failed to parse dropped task:', err);
			}
		},
		[onDrop]
	);

	const handleToggleKeydown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onToggleBacklog();
			}
		},
		[onToggleBacklog]
	);

	const columnClasses = ['queued-column', dragOver && 'drag-over'].filter(Boolean).join(' ');

	const totalCount = activeTasks.length + backlogTasks.length;

	return (
		<div
			className={columnClasses}
			onDragEnter={handleDragEnter}
			onDragLeave={handleDragLeave}
			onDragOver={handleDragOver}
			onDrop={handleDrop}
		>
			<div className="column-header">
				<h3 className="column-title">{column.title}</h3>
				<span className="column-count">{totalCount}</span>
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
							<button
								type="button"
								className="backlog-toggle"
								onClick={onToggleBacklog}
								onKeyDown={handleToggleKeydown}
								aria-expanded={showBacklog}
								aria-controls="backlog-section"
							>
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
								<span className="backlog-label">Backlog</span>
								<span className="backlog-count">{backlogTasks.length}</span>
							</button>
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
