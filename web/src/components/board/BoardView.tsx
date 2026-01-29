/**
 * BoardView main container component
 *
 * Assembles the board layout with two-column grid (Queue + Running).
 * The right panel (Blocked, Decisions, Config, Files, Completed) is rendered
 * by BoardCommandPanel in AppShell — BoardView has no side effects.
 *
 * Layout:
 * - Queue column (flex: 1, min-width: 280px): Initiative swimlanes
 * - Running column (420px fixed): Active tasks with Pipeline visualization
 *
 * Data Flow:
 * - Reads from stores: taskStore, initiativeStore
 * - Groups queued tasks by initiative for swimlanes
 * - Filters tasks by status for different columns
 */

import { useMemo, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { QueueColumn } from './QueueColumn';
import { RunningColumn } from './RunningColumn';
import { useTaskStore } from '@/stores/taskStore';
import { useInitiatives } from '@/stores/initiativeStore';
import { usePendingDecisions } from '@/stores/uiStore';
import { type Task, TaskStatus, type ExecutionState } from '@/gen/orc/v1/task_pb';
import './BoardView.css';

export interface BoardViewProps {
	className?: string;
}

/**
 * BoardView displays the main task board with queue and running columns.
 * No side effects — panel content is handled by BoardCommandPanel in AppShell.
 */
export function BoardView({ className }: BoardViewProps): React.ReactElement {
	const navigate = useNavigate();

	// Store hooks
	const tasks = useTaskStore((state) => state.tasks);
	const taskStates = useTaskStore((state) => state.taskStates);
	const loading = useTaskStore((state) => state.loading);
	const initiatives = useInitiatives();

	// Pending decisions for task card glow indicators
	const pendingDecisions = usePendingDecisions();

	// Local state
	const [collapsedSwimlanes, setCollapsedSwimlanes] = useState<Set<string>>(
		new Set()
	);

	// Deduplicate tasks by ID to prevent React duplicate key warnings.
	// Store-level dedup handles setTasks/addTask, but concurrent WebSocket
	// events and API fetches can race, so we also dedup at render time.
	const uniqueTasks = useMemo(() => {
		const seen = new Set<string>();
		return tasks.filter((task: Task) => {
			if (seen.has(task.id)) return false;
			seen.add(task.id);
			return true;
		});
	}, [tasks]);

	// Derived state: filter tasks by status
	const queuedTasks = useMemo(
		() =>
			uniqueTasks.filter(
				(task: Task) =>
					task.status === TaskStatus.PLANNED ||
					task.status === TaskStatus.CREATED ||
					task.status === TaskStatus.CLASSIFYING
			),
		[uniqueTasks]
	);

	const runningTasks = useMemo(
		() => uniqueTasks.filter((task: Task) => task.status === TaskStatus.RUNNING),
		[uniqueTasks]
	);

	// Map of task ID to pending decision count
	const taskDecisionCounts = useMemo(() => {
		const counts = new Map<string, number>();
		for (const decision of pendingDecisions) {
			const currentCount = counts.get(decision.taskId) || 0;
			counts.set(decision.taskId, currentCount + 1);
		}
		return counts;
	}, [pendingDecisions]);

	// Convert Map to Record for RunningColumn
	const taskStatesRecord = useMemo(() => {
		const record: Record<string, ExecutionState> = {};
		for (const [id, state] of taskStates) {
			record[id] = state;
		}
		return record;
	}, [taskStates]);

	// Task outputs placeholder - real implementation tracks WebSocket transcript events
	const taskOutputs = useMemo((): Record<string, string[]> => {
		return {};
	}, []);

	// Handlers
	const handleToggleSwimlane = useCallback((id: string) => {
		setCollapsedSwimlanes((prev) => {
			const next = new Set(prev);
			if (next.has(id)) {
				next.delete(id);
			} else {
				next.add(id);
			}
			return next;
		});
	}, []);

	const handleTaskClick = useCallback(
		(task: Task) => {
			navigate(`/tasks/${task.id}`);
		},
		[navigate]
	);

	const handleContextMenu = useCallback(
		(_task: Task, _e: React.MouseEvent) => {
			// Context menu handling would go here (future feature)
		},
		[]
	);

	// Loading state
	if (loading) {
		return (
			<div className={`board-view board-view--loading ${className || ''}`}>
				<div className="board-view__skeleton board-view__skeleton--queue">
					<div className="board-view__skeleton-header" />
					<div className="board-view__skeleton-content">
						<div className="board-view__skeleton-card" />
						<div className="board-view__skeleton-card" />
						<div className="board-view__skeleton-card" />
					</div>
				</div>
				<div className="board-view__skeleton board-view__skeleton--running">
					<div className="board-view__skeleton-header" />
					<div className="board-view__skeleton-content">
						<div className="board-view__skeleton-card board-view__skeleton-card--large" />
					</div>
				</div>
			</div>
		);
	}

	return (
		<div
			className={`board-view ${className || ''}`}
			role="region"
			aria-label="Task board"
		>
			<div className="board-view__queue">
				<QueueColumn
					tasks={queuedTasks}
					initiatives={initiatives}
					collapsedSwimlanes={collapsedSwimlanes}
					onToggleSwimlane={handleToggleSwimlane}
					onTaskClick={handleTaskClick}
					onContextMenu={handleContextMenu}
					taskDecisionCounts={taskDecisionCounts}
				/>
			</div>
			<div className="board-view__running">
				<RunningColumn
					tasks={runningTasks}
					taskStates={taskStatesRecord}
					taskOutputs={taskOutputs}
					onTaskClick={handleTaskClick}
					taskDecisionCounts={taskDecisionCounts}
				/>
			</div>
		</div>
	);
}
