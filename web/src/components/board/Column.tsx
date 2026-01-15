/**
 * Column component for Kanban board
 *
 * Represents a single column in the board (e.g., Spec, Implement, Test)
 * Handles drag-and-drop with visual feedback.
 */

import { useState, useCallback } from 'react';
import { TaskCard } from './TaskCard';
import type { Task } from '@/lib/types';
import type { FinalizeState } from '@/lib/api';
import './Column.css';

export interface ColumnConfig {
	id: string;
	title: string;
	phases: string[];
}

// Column-specific styling
const COLUMN_STYLES: Record<
	string,
	{ accentColor: string; bgColor: string }
> = {
	queued: {
		accentColor: 'var(--text-muted)',
		bgColor: 'rgba(148, 163, 184, 0.05)',
	},
	spec: {
		accentColor: 'rgb(59, 130, 246)',
		bgColor: 'rgba(59, 130, 246, 0.05)',
	},
	implement: {
		accentColor: 'var(--accent-primary)',
		bgColor: 'rgba(139, 92, 246, 0.05)',
	},
	test: {
		accentColor: 'rgb(6, 182, 212)',
		bgColor: 'rgba(6, 182, 212, 0.05)',
	},
	review: {
		accentColor: 'var(--status-warning)',
		bgColor: 'rgba(245, 158, 11, 0.05)',
	},
	done: {
		accentColor: 'var(--status-success)',
		bgColor: 'rgba(16, 185, 129, 0.05)',
	},
};

interface ColumnProps {
	column: ColumnConfig;
	tasks: Task[];
	onDrop: (task: Task) => void;
	onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
	onTaskClick?: (task: Task) => void;
	onFinalizeClick?: (task: Task) => void;
	onInitiativeClick?: (initiativeId: string) => void;
	getFinalizeState?: (taskId: string) => FinalizeState | null | undefined;
}

export function Column({
	column,
	tasks,
	onDrop,
	onAction,
	onTaskClick,
	onFinalizeClick,
	onInitiativeClick,
	getFinalizeState,
}: ColumnProps) {
	const [dragOver, setDragOver] = useState(false);
	// Counter used via updater function, not read directly
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_dragCounter, setDragCounter] = useState(0);

	const style = COLUMN_STYLES[column.id] || COLUMN_STYLES.queued;

	// Drag handlers with counter to handle nested elements
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

	const columnClasses = ['column', dragOver && 'drag-over'].filter(Boolean).join(' ');

	return (
		<div
			className={columnClasses}
			role="region"
			aria-label={`${column.title} column`}
			style={
				{
					'--column-accent': style.accentColor,
					'--column-bg': style.bgColor,
				} as React.CSSProperties
			}
			onDragEnter={handleDragEnter}
			onDragLeave={handleDragLeave}
			onDragOver={handleDragOver}
			onDrop={handleDrop}
		>
			<div className="column-header">
				<h2>{column.title}</h2>
				<span className="count">{tasks.length}</span>
			</div>

			<div className="column-content">
				{tasks.length === 0 ? (
					<div className="empty">No tasks</div>
				) : (
					tasks.map((task) => (
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
		</div>
	);
}
