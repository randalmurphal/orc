/**
 * Board component - main Kanban board with flat and swimlane views
 *
 * Features:
 * - Flat view: columns for each phase
 * - Swimlane view: grouped by initiative
 * - Clickable task cards for navigation
 */

import { useState, useCallback, useMemo } from 'react';
import { Column, type ColumnConfig } from './Column';
import { QueuedColumn } from './QueuedColumn';
import { Swimlane } from './Swimlane';
import type { Task, Initiative, TaskPriority } from '@/lib/types';
import { PRIORITY_ORDER } from '@/lib/types';
import './Board.css';

export type BoardViewMode = 'flat' | 'swimlane';

// Column definitions for the board
export const BOARD_COLUMNS: ColumnConfig[] = [
	{ id: 'queued', title: 'Queued', phases: [] },
	{ id: 'spec', title: 'Spec', phases: ['research', 'spec', 'design'] },
	{ id: 'implement', title: 'Implement', phases: ['implement'] },
	{ id: 'test', title: 'Test', phases: ['test'] },
	{ id: 'review', title: 'Review', phases: ['docs', 'validate', 'review'] },
	{ id: 'done', title: 'Done', phases: [] },
];

// Terminal statuses that go to Done column
const TERMINAL_STATUSES = ['finalizing', 'completed', 'failed'];

interface BoardProps {
	tasks: Task[];
	viewMode?: BoardViewMode;
	initiatives?: Initiative[];
	onTaskClick?: (task: Task) => void;
	onContextMenu?: (task: Task, e: React.MouseEvent) => void;
}

// Categorize task to column based on status and phase
function getTaskColumn(task: Task): string {
	// Terminal statuses go to Done
	if (TERMINAL_STATUSES.includes(task.status)) {
		return 'done';
	}

	// No phase + running = Implement (transitional state)
	if (!task.current_phase && task.status === 'running') {
		return 'implement';
	}

	// No phase = Queued
	if (!task.current_phase) {
		return 'queued';
	}

	// Find column by matching phase
	const phase = task.current_phase;
	for (const column of BOARD_COLUMNS) {
		if (column.phases.includes(phase)) {
			return column.id;
		}
	}

	// Default to implement
	return 'implement';
}

// Sort tasks: running first, then by priority
function sortTasks(tasks: Task[]): Task[] {
	return [...tasks].sort((a, b) => {
		// Running tasks first
		if (a.status === 'running' && b.status !== 'running') return -1;
		if (b.status === 'running' && a.status !== 'running') return 1;

		// Then by priority
		const priorityA = PRIORITY_ORDER[(a.priority || 'normal') as TaskPriority] ?? 2;
		const priorityB = PRIORITY_ORDER[(b.priority || 'normal') as TaskPriority] ?? 2;
		return priorityA - priorityB;
	});
}

export function Board({
	tasks,
	viewMode = 'flat',
	initiatives = [],
	onTaskClick,
	onContextMenu,
}: BoardProps) {
	// UI state
	const [showBacklog, setShowBacklog] = useState(() => {
		if (typeof window === 'undefined') return false;
		return localStorage.getItem('orc-show-backlog') === 'true';
	});
	const [collapsedSwimlanes, setCollapsedSwimlanes] = useState<Set<string>>(() => {
		if (typeof window === 'undefined') return new Set();
		try {
			const stored = localStorage.getItem('orc-collapsed-swimlanes');
			return stored ? new Set(JSON.parse(stored)) : new Set();
		} catch {
			return new Set();
		}
	});

	// Group tasks by column
	const tasksByColumn = useMemo(() => {
		const grouped: Record<string, Task[]> = {};
		for (const column of BOARD_COLUMNS) {
			grouped[column.id] = [];
		}

		for (const task of tasks) {
			const columnId = getTaskColumn(task);
			if (grouped[columnId]) {
				grouped[columnId].push(task);
			}
		}

		// Sort each column
		for (const columnId of Object.keys(grouped)) {
			grouped[columnId] = sortTasks(grouped[columnId]);
		}

		return grouped;
	}, [tasks]);

	// Split queued tasks by queue
	const queuedTasks = useMemo(() => {
		const queued = tasksByColumn['queued'] || [];
		return {
			active: sortTasks(queued.filter((t) => t.queue !== 'backlog')),
			backlog: sortTasks(queued.filter((t) => t.queue === 'backlog')),
		};
	}, [tasksByColumn]);

	// Group tasks by initiative for swimlane view
	const tasksByInitiative = useMemo(() => {
		const grouped: Record<string, Task[]> = { unassigned: [] };
		for (const initiative of initiatives) {
			grouped[initiative.id] = [];
		}

		for (const task of tasks) {
			const initId = task.initiative_id;
			if (initId && grouped[initId]) {
				grouped[initId].push(task);
			} else {
				grouped['unassigned'].push(task);
			}
		}

		return grouped;
	}, [tasks, initiatives]);

	// Get tasks by column within an initiative
	const getTasksByColumnForInitiative = useCallback(
		(initiativeTasks: Task[]) => {
			const grouped: Record<string, Task[]> = {};
			for (const column of BOARD_COLUMNS) {
				grouped[column.id] = [];
			}

			for (const task of initiativeTasks) {
				const columnId = getTaskColumn(task);
				if (grouped[columnId]) {
					grouped[columnId].push(task);
				}
			}

			// Sort each column
			for (const columnId of Object.keys(grouped)) {
				grouped[columnId] = sortTasks(grouped[columnId]);
			}

			return grouped;
		},
		[]
	);

	// Toggle backlog visibility
	const handleToggleBacklog = useCallback(() => {
		setShowBacklog((prev) => {
			const next = !prev;
			localStorage.setItem('orc-show-backlog', String(next));
			return next;
		});
	}, []);

	// Toggle swimlane collapse
	const toggleSwimlane = useCallback((id: string) => {
		setCollapsedSwimlanes((prev) => {
			const next = new Set(prev);
			if (next.has(id)) {
				next.delete(id);
			} else {
				next.add(id);
			}
			localStorage.setItem('orc-collapsed-swimlanes', JSON.stringify([...next]));
			return next;
		});
	}, []);

	// Columns without queued (handled specially)
	const columnsWithoutQueued = BOARD_COLUMNS.filter((c) => c.id !== 'queued');

	return (
		<div
			className={`board ${viewMode === 'swimlane' ? 'swimlane-view' : 'flat-view'}`}
			tabIndex={0}
			role="region"
			aria-label="Task board"
		>
			{viewMode === 'flat' ? (
				// Flat view: columns side by side
				<>
					<QueuedColumn
						column={BOARD_COLUMNS[0]}
						activeTasks={queuedTasks.active}
						backlogTasks={queuedTasks.backlog}
						showBacklog={showBacklog}
						onToggleBacklog={handleToggleBacklog}
						onTaskClick={onTaskClick}
						onContextMenu={onContextMenu}
					/>
					{columnsWithoutQueued.map((column) => (
						<Column
							key={column.id}
							column={column}
							tasks={tasksByColumn[column.id] || []}
							onTaskClick={onTaskClick}
							onContextMenu={onContextMenu}
						/>
					))}
				</>
			) : (
				// Swimlane view: rows per initiative
				<>
					{/* Column headers */}
					<div className="swimlane-headers">
						<div className="header-spacer" />
						{BOARD_COLUMNS.map((column) => (
							<div key={column.id} className="swimlane-column-header">
								{column.title}
							</div>
						))}
					</div>

					{/* Swimlanes */}
					<div className="swimlanes">
						{/* Initiative swimlanes */}
						{initiatives.map((initiative) => {
							const initTasks = tasksByInitiative[initiative.id] || [];
							if (initTasks.length === 0) return null;

							return (
								<Swimlane
									key={initiative.id}
									initiative={initiative}
									tasks={initTasks}
									columns={BOARD_COLUMNS}
									tasksByColumn={getTasksByColumnForInitiative(initTasks)}
									collapsed={collapsedSwimlanes.has(initiative.id)}
									onToggleCollapse={() => toggleSwimlane(initiative.id)}
									onTaskClick={onTaskClick}
									onContextMenu={onContextMenu}
								/>
							);
						})}

						{/* Unassigned swimlane */}
						{(tasksByInitiative['unassigned'] || []).length > 0 && (
							<Swimlane
								initiative={null}
								tasks={tasksByInitiative['unassigned']}
								columns={BOARD_COLUMNS}
								tasksByColumn={getTasksByColumnForInitiative(
									tasksByInitiative['unassigned']
								)}
								collapsed={collapsedSwimlanes.has('unassigned')}
								onToggleCollapse={() => toggleSwimlane('unassigned')}
								onTaskClick={onTaskClick}
								onContextMenu={onContextMenu}
							/>
						)}
					</div>
				</>
			)}
		</div>
	);
}
