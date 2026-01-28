/**
 * RunningColumn component for Kanban board
 *
 * Displays currently executing tasks with pipeline visualization.
 * Features:
 * - Fixed width column (420px) for running tasks
 * - Contains RunningCard components with expanded task view
 * - Maximum 2 visible cards with scroll for overflow
 * - Real-time updates via WebSocket (parent handles state)
 * - Pulsing indicator to show active execution
 * - Empty state with suggestion to start a task
 */

import { useRef, useCallback, useEffect, useState } from 'react';
import { RunningCard } from './RunningCard';
import { type Task, type ExecutionState } from '@/gen/orc/v1/task_pb';
import './RunningColumn.css';

export interface RunningColumnProps {
	/** Tasks filtered to running status */
	tasks: Task[];
	/** Task states keyed by task ID (from WebSocket updates) */
	taskStates?: Record<string, ExecutionState>;
	/** Output lines per task (from WebSocket transcript events) */
	taskOutputs?: Record<string, string[]>;
	/** Callback when a task card is clicked */
	onTaskClick?: (task: Task) => void;
	/** Map of task ID to pending decision count */
	taskDecisionCounts?: Map<string, number>;
}

/**
 * RunningColumn displays active tasks with live execution progress.
 */
export function RunningColumn({
	tasks,
	taskStates = {},
	taskOutputs = {},
	onTaskClick: _onTaskClick,
	taskDecisionCounts,
}: RunningColumnProps) {
	// Note: onTaskClick is reserved for future task detail navigation (e.g., double-click)
	// Currently, single-click on RunningCard toggles expanded state
	void _onTaskClick;
	// Track which cards are expanded (only one at a time)
	const [expandedTaskId, setExpandedTaskId] = useState<string | null>(null);

	// Track tasks that are completing (for exit animation)
	const [completingTasks, setCompletingTasks] = useState<Set<string>>(new Set());

	// Scroll container ref for preserving scroll position
	const scrollRef = useRef<HTMLDivElement>(null);
	const scrollPositionRef = useRef<number>(0);

	// Preserve scroll position on updates
	useEffect(() => {
		const container = scrollRef.current;
		if (!container) return;

		// Save current scroll position before updates
		const handleScroll = () => {
			scrollPositionRef.current = container.scrollTop;
		};

		container.addEventListener('scroll', handleScroll);
		return () => container.removeEventListener('scroll', handleScroll);
	}, []);

	// Restore scroll position after task list changes
	useEffect(() => {
		const container = scrollRef.current;
		if (container && scrollPositionRef.current > 0) {
			// Use requestAnimationFrame to ensure DOM has updated
			requestAnimationFrame(() => {
				container.scrollTop = scrollPositionRef.current;
			});
		}
	}, [tasks.length]);

	// Detect task completion for animation
	const prevTaskIdsRef = useRef<Set<string>>(new Set());
	useEffect(() => {
		const currentTaskIds = new Set(tasks.map((t) => t.id));
		const prevTaskIds = prevTaskIdsRef.current;

		// Find tasks that were removed (completed)
		for (const prevId of prevTaskIds) {
			if (!currentTaskIds.has(prevId)) {
				// Task completed - trigger exit animation
				setCompletingTasks((prev) => new Set(prev).add(prevId));

				// Remove from completing set after animation
				setTimeout(() => {
					setCompletingTasks((prev) => {
						const next = new Set(prev);
						next.delete(prevId);
						return next;
					});
				}, 300); // Match animation duration
			}
		}

		prevTaskIdsRef.current = currentTaskIds;
	}, [tasks]);

	// Toggle card expansion
	const handleToggleExpand = useCallback((taskId: string) => {
		setExpandedTaskId((prev) => (prev === taskId ? null : taskId));
	}, []);

	// Build default task state for tasks without WebSocket state
	const getTaskState = useCallback(
		(task: Task): ExecutionState | undefined => {
			const wsState = taskStates[task.id];
			if (wsState) return wsState;

			// Return task's embedded execution state, or undefined
			return task.execution;
		},
		[taskStates]
	);

	const taskCount = tasks.length;

	return (
		<div
			className="running-column column running"
			role="region"
			aria-label="Running tasks column"
			aria-live="polite"
		>
			{/* Column Header */}
			<div className="running-column-header column-header">
				<div className="running-column-title column-title">
					<span className="running-indicator column-indicator" aria-hidden="true" />
					<span>Running</span>
				</div>
				<span className="running-count" aria-label={`${taskCount} running tasks`}>
					{taskCount}
				</span>
			</div>

			{/* Column Content */}
			<div className="running-column-content" ref={scrollRef}>
				{tasks.length === 0 ? (
					<div className="running-empty">
						<div className="running-empty-text">No running tasks</div>
						<div className="running-empty-hint">
							Run a task with <code>orc run</code> or click Start on a queued task
						</div>
					</div>
				) : (
					tasks.map((task) => (
						<div
							key={task.id}
							className={`running-card-wrapper ${completingTasks.has(task.id) ? 'completing' : ''}`}
						>
							<RunningCard
								task={task}
								state={getTaskState(task)}
								expanded={expandedTaskId === task.id}
								onToggleExpand={() => handleToggleExpand(task.id)}
								outputLines={taskOutputs[task.id]}
								pendingDecisionCount={taskDecisionCounts?.get(task.id) ?? 0}
							/>
						</div>
					))
				)}
			</div>
		</div>
	);
}
