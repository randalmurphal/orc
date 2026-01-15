/**
 * Board component - main Kanban board with flat and swimlane views
 *
 * Features:
 * - Flat view: columns for each phase
 * - Swimlane view: grouped by initiative
 * - Drag-drop status/initiative changes
 * - Confirmation modals for escalation/initiative change
 */

import { useState, useCallback, useMemo } from 'react';
import { Column, type ColumnConfig } from './Column';
import { QueuedColumn } from './QueuedColumn';
import { Swimlane } from './Swimlane';
import { Button } from '@/components/ui/Button';
import type { Task, Initiative, TaskPriority } from '@/lib/types';
import type { FinalizeState } from '@/lib/api';
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
const TERMINAL_STATUSES = ['finalizing', 'completed', 'finished', 'failed'];

interface BoardProps {
	tasks: Task[];
	viewMode?: BoardViewMode;
	initiatives?: Initiative[];
	onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
	onEscalate?: (taskId: string, reason: string) => Promise<void>;
	onRefresh?: () => Promise<void>;
	onTaskClick?: (task: Task) => void;
	onFinalizeClick?: (task: Task) => void;
	onInitiativeClick?: (initiativeId: string) => void;
	onInitiativeChange?: (taskId: string, initiativeId: string | null) => Promise<void>;
	getFinalizeState?: (taskId: string) => FinalizeState | null | undefined;
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
	onAction,
	onEscalate,
	onTaskClick,
	onFinalizeClick,
	onInitiativeClick,
	onInitiativeChange,
	getFinalizeState,
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
	const [actionLoading, setActionLoading] = useState(false);

	// Modal states
	const [escalateTask, setEscalateTask] = useState<Task | null>(null);
	const [escalateReason, setEscalateReason] = useState('');
	const [initiativeChangeModal, setInitiativeChangeModal] = useState<{
		task: Task;
		targetInitiativeId: string | null;
		columnId: string;
	} | null>(null);

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

	// Handle drop in flat view
	const handleFlatDrop = useCallback(
		async (targetColumnId: string, task: Task) => {
			const currentColumn = getTaskColumn(task);
			if (currentColumn === targetColumnId) return;

			// Determine action based on transition
			// Queued -> Phase = Run
			if (currentColumn === 'queued' && targetColumnId !== 'done') {
				setActionLoading(true);
				try {
					await onAction(task.id, 'run');
				} finally {
					setActionLoading(false);
				}
				return;
			}

			// Paused/Blocked -> Phase = Resume
			if (
				(task.status === 'paused' || task.status === 'blocked') &&
				targetColumnId !== 'queued' &&
				targetColumnId !== 'done'
			) {
				setActionLoading(true);
				try {
					await onAction(task.id, 'resume');
				} finally {
					setActionLoading(false);
				}
				return;
			}

			// Running -> Queued or earlier phase = Escalate
			if (task.status === 'running') {
				setEscalateTask(task);
				setEscalateReason('');
				return;
			}
		},
		[onAction]
	);

	// Handle drop in swimlane view
	const handleSwimlaneDrop = useCallback(
		async (
			columnId: string,
			task: Task,
			targetInitiativeId: string | null
		) => {
			const currentInitiativeId = task.initiative_id ?? null;

			// If initiative changed, show confirmation modal
			if (currentInitiativeId !== targetInitiativeId) {
				setInitiativeChangeModal({
					task,
					targetInitiativeId,
					columnId,
				});
				return;
			}

			// Otherwise handle as normal column drop
			await handleFlatDrop(columnId, task);
		},
		[handleFlatDrop]
	);

	// Confirm initiative change
	const confirmInitiativeChange = useCallback(async () => {
		if (!initiativeChangeModal || !onInitiativeChange) return;

		setActionLoading(true);
		try {
			await onInitiativeChange(
				initiativeChangeModal.task.id,
				initiativeChangeModal.targetInitiativeId
			);
			// Then handle column change if needed
			await handleFlatDrop(
				initiativeChangeModal.columnId,
				initiativeChangeModal.task
			);
		} finally {
			setActionLoading(false);
			setInitiativeChangeModal(null);
		}
	}, [initiativeChangeModal, onInitiativeChange, handleFlatDrop]);

	// Cancel initiative change
	const cancelInitiativeChange = useCallback(() => {
		setInitiativeChangeModal(null);
	}, []);

	// Handle escalate confirm
	const handleEscalateConfirm = useCallback(async () => {
		if (!escalateTask || !onEscalate || !escalateReason.trim()) return;

		setActionLoading(true);
		try {
			await onEscalate(escalateTask.id, escalateReason.trim());
		} finally {
			setActionLoading(false);
			setEscalateTask(null);
			setEscalateReason('');
		}
	}, [escalateTask, onEscalate, escalateReason]);

	// Cancel escalate
	const handleEscalateCancel = useCallback(() => {
		setEscalateTask(null);
		setEscalateReason('');
	}, []);

	// Columns without queued (handled specially)
	const columnsWithoutQueued = BOARD_COLUMNS.filter((c) => c.id !== 'queued');

	// Initiative name for modal
	const getInitiativeName = useCallback(
		(id: string | null): string => {
			if (!id) return 'Unassigned';
			const init = initiatives.find((i) => i.id === id);
			return init?.title ?? id;
		},
		[initiatives]
	);

	return (
		<div className={`board ${viewMode === 'swimlane' ? 'swimlane-view' : 'flat-view'}`}>
			{viewMode === 'flat' ? (
				// Flat view: columns side by side
				<>
					<QueuedColumn
						column={BOARD_COLUMNS[0]}
						activeTasks={queuedTasks.active}
						backlogTasks={queuedTasks.backlog}
						showBacklog={showBacklog}
						onToggleBacklog={handleToggleBacklog}
						onDrop={(task) => handleFlatDrop('queued', task)}
						onAction={onAction}
						onTaskClick={onTaskClick}
						onFinalizeClick={onFinalizeClick}
						onInitiativeClick={onInitiativeClick}
						getFinalizeState={getFinalizeState}
					/>
					{columnsWithoutQueued.map((column) => (
						<Column
							key={column.id}
							column={column}
							tasks={tasksByColumn[column.id] || []}
							onDrop={(task) => handleFlatDrop(column.id, task)}
							onAction={onAction}
							onTaskClick={onTaskClick}
							onFinalizeClick={onFinalizeClick}
							onInitiativeClick={onInitiativeClick}
							getFinalizeState={getFinalizeState}
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
									onDrop={handleSwimlaneDrop}
									onAction={onAction}
									onTaskClick={onTaskClick}
									onFinalizeClick={onFinalizeClick}
									onInitiativeClick={onInitiativeClick}
									getFinalizeState={getFinalizeState}
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
								onDrop={handleSwimlaneDrop}
								onAction={onAction}
								onTaskClick={onTaskClick}
								onFinalizeClick={onFinalizeClick}
								onInitiativeClick={onInitiativeClick}
								getFinalizeState={getFinalizeState}
							/>
						)}
					</div>
				</>
			)}

			{/* Escalate Modal */}
			{escalateTask && (
				<>
					<div className="modal-backdrop" onClick={handleEscalateCancel} />
					<div className="modal escalate-modal">
						<h3>Escalate Task</h3>
						<p>
							Moving a running task back requires an escalation reason.
							This helps understand why the task needs to restart.
						</p>
						<div className="form-group">
							<label htmlFor="escalate-reason">Reason</label>
							<textarea
								id="escalate-reason"
								value={escalateReason}
								onChange={(e) => setEscalateReason(e.target.value)}
								placeholder="Why does this task need to be escalated?"
								rows={3}
							/>
						</div>
						<div className="modal-actions">
							<Button
								variant="secondary"
								onClick={handleEscalateCancel}
							>
								Cancel
							</Button>
							<Button
								variant="primary"
								onClick={handleEscalateConfirm}
								disabled={!escalateReason.trim() || actionLoading}
								loading={actionLoading}
							>
								{actionLoading ? 'Escalating...' : 'Escalate'}
							</Button>
						</div>
					</div>
				</>
			)}

			{/* Initiative Change Modal */}
			{initiativeChangeModal && (
				<>
					<div className="modal-backdrop" onClick={cancelInitiativeChange} />
					<div className="modal initiative-change-modal">
						<h3>Change Initiative</h3>
						<p>
							Move <strong>{initiativeChangeModal.task.title}</strong> to{' '}
							<strong>
								{getInitiativeName(initiativeChangeModal.targetInitiativeId)}
							</strong>
							?
						</p>
						<div className="modal-actions">
							<Button
								variant="secondary"
								onClick={cancelInitiativeChange}
							>
								Cancel
							</Button>
							<Button
								variant="primary"
								onClick={confirmInitiativeChange}
								disabled={actionLoading}
								loading={actionLoading}
							>
								{actionLoading ? 'Moving...' : 'Move'}
							</Button>
						</div>
					</div>
				</>
			)}
		</div>
	);
}
