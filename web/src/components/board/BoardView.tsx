/**
 * BoardView main container component
 *
 * Assembles the new board layout with two-column grid (Queue + Running)
 * and sets right panel content via AppShell context.
 *
 * Layout:
 * - Queue column (flex: 1, min-width: 280px): Initiative swimlanes
 * - Running column (420px fixed): Active tasks with Pipeline visualization
 * - Right panel: Blocked, Decisions, Config, Files, Completed sections
 *
 * Data Flow:
 * - Reads from stores: taskStore, initiativeStore, sessionStore
 * - Groups queued tasks by initiative for swimlanes
 * - Filters tasks by status for different columns/panels
 * - Sets right panel content on mount, clears on unmount
 */

import { useMemo, useState, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { QueueColumn } from './QueueColumn';
import { RunningColumn } from './RunningColumn';
import { BlockedPanel } from './BlockedPanel';
import { DecisionsPanel } from './DecisionsPanel';
import { ConfigPanel, type ConfigStats } from './ConfigPanel';
import { FilesPanel, type ChangedFile } from './FilesPanel';
import { CompletedPanel } from './CompletedPanel';
import { useAppShell } from '@/components/layout/AppShellContext';
import { useTaskStore } from '@/stores/taskStore';
import { useInitiatives } from '@/stores/initiativeStore';
import { useSessionStore } from '@/stores/sessionStore';
import type { Task, TaskState, PendingDecision } from '@/lib/types';
import './BoardView.css';

export interface BoardViewProps {
	className?: string;
}

/**
 * Check if a task was completed today
 */
function isCompletedToday(task: Task): boolean {
	if (task.status !== 'completed' || !task.completed_at) {
		return false;
	}

	const completedDate = new Date(task.completed_at);
	const today = new Date();

	return (
		completedDate.getFullYear() === today.getFullYear() &&
		completedDate.getMonth() === today.getMonth() &&
		completedDate.getDate() === today.getDate()
	);
}

/**
 * BoardView displays the main task board with queue and running columns.
 */
export function BoardView({ className }: BoardViewProps): React.ReactElement {
	const navigate = useNavigate();
	const { setRightPanelContent } = useAppShell();

	// Store hooks with explicit state types
	const tasks = useTaskStore((state) => state.tasks);
	const taskStates = useTaskStore((state) => state.taskStates);
	const loading = useTaskStore((state) => state.loading);
	const initiatives = useInitiatives();
	const totalTokens = useSessionStore((state) => state.totalTokens);
	const totalCost = useSessionStore((state) => state.totalCost);

	// Local state
	const [collapsedSwimlanes, setCollapsedSwimlanes] = useState<Set<string>>(
		new Set()
	);
	const [pendingDecisions] = useState<PendingDecision[]>([]);
	const [configStats] = useState<ConfigStats | undefined>(undefined);
	const [changedFiles] = useState<ChangedFile[]>([]);

	// Derived state: filter tasks by status
	const queuedTasks = useMemo(
		() =>
			tasks.filter(
				(task: Task) =>
					task.status === 'planned' ||
					task.status === 'created' ||
					task.status === 'classifying'
			),
		[tasks]
	);

	const runningTasks = useMemo(
		() => tasks.filter((task: Task) => task.status === 'running'),
		[tasks]
	);

	const blockedTasks = useMemo(
		() => tasks.filter((task: Task) => task.status === 'blocked' || task.is_blocked),
		[tasks]
	);

	const completedToday = useMemo(
		() => tasks.filter((task: Task) => isCompletedToday(task)),
		[tasks]
	);

	// Convert Map to Record for RunningColumn
	const taskStatesRecord = useMemo(() => {
		const record: Record<string, TaskState> = {};
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

	const handleSkipBlock = useCallback((taskId: string) => {
		// TODO: Call API to skip block for task
		console.log('Skip block for:', taskId);
	}, []);

	const handleForceBlock = useCallback((taskId: string) => {
		// TODO: Call API to force run blocked task
		console.log('Force run:', taskId);
	}, []);

	const handleDecide = useCallback(
		async (decisionId: string, optionId: string) => {
			// TODO: Call API to submit decision
			console.log('Decision:', decisionId, 'Option:', optionId);
		},
		[]
	);

	const handleFileClick = useCallback(
		(file: ChangedFile) => {
			// Navigate to file in task detail view
			if (file.taskId) {
				navigate(`/tasks/${file.taskId}?file=${encodeURIComponent(file.path)}`);
			}
		},
		[navigate]
	);

	// Set right panel content on mount
	useEffect(() => {
		const panelContent = (
			<>
				<BlockedPanel
					tasks={blockedTasks}
					onSkip={handleSkipBlock}
					onForce={handleForceBlock}
				/>
				<DecisionsPanel decisions={pendingDecisions} onDecide={handleDecide} />
				<ConfigPanel config={configStats} />
				<FilesPanel files={changedFiles} onFileClick={handleFileClick} />
				<CompletedPanel
					completedCount={completedToday.length}
					todayTokens={totalTokens}
					todayCost={totalCost}
					recentTasks={completedToday}
				/>
			</>
		);

		setRightPanelContent(panelContent);

		// Cleanup: clear right panel content on unmount
		return () => {
			setRightPanelContent(null);
		};
	}, [
		blockedTasks,
		pendingDecisions,
		configStats,
		changedFiles,
		completedToday,
		totalTokens,
		totalCost,
		handleSkipBlock,
		handleForceBlock,
		handleDecide,
		handleFileClick,
		setRightPanelContent,
	]);

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
				/>
			</div>
			<div className="board-view__running">
				<RunningColumn
					tasks={runningTasks}
					taskStates={taskStatesRecord}
					taskOutputs={taskOutputs}
					onTaskClick={handleTaskClick}
				/>
			</div>
		</div>
	);
}
