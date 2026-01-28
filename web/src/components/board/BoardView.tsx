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
import { ConfigPanel } from './ConfigPanel';
import { FilesPanel, type ChangedFile } from './FilesPanel';
import { CompletedPanel } from './CompletedPanel';
import { useTaskStore } from '@/stores/taskStore';
import { useInitiatives } from '@/stores/initiativeStore';
import { useSessionStore } from '@/stores/sessionStore';
import { usePendingDecisions, useUIStore } from '@/stores/uiStore';
import { useAppShell } from '@/components/layout/AppShellContext';
import { decisionClient, taskClient, configClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import { ResolveDecisionRequestSchema } from '@/gen/orc/v1/decision_pb';
import { type Task, TaskStatus, type ExecutionState, SkipBlockRequestSchema, RunTaskRequestSchema } from '@/gen/orc/v1/task_pb';
import { GetConfigStatsRequestSchema } from '@/gen/orc/v1/config_pb';
import { timestampToDate } from '@/lib/time';
import { type ConfigStats } from './ConfigPanel';
import './BoardView.css';

export interface BoardViewProps {
	className?: string;
}

/**
 * Check if a task was completed today
 */
function isCompletedToday(task: Task): boolean {
	if (task.status !== TaskStatus.COMPLETED || !task.completedAt) {
		return false;
	}

	const completedDate = timestampToDate(task.completedAt);
	if (!completedDate) return false;
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
	const { setRightPanelContent, isRightPanelOpen, toggleRightPanel } = useAppShell();

	// Store hooks with explicit state types
	const tasks = useTaskStore((state) => state.tasks);
	const taskStates = useTaskStore((state) => state.taskStates);
	const loading = useTaskStore((state) => state.loading);
	const initiatives = useInitiatives();
	const totalTokens = useSessionStore((state) => state.totalTokens);
	const totalCost = useSessionStore((state) => state.totalCost);

	// Pending decisions from uiStore (populated by event handlers)
	const pendingDecisions = usePendingDecisions();
	const removePendingDecision = useUIStore((state) => state.removePendingDecision);

	// Local state
	const [collapsedSwimlanes, setCollapsedSwimlanes] = useState<Set<string>>(
		new Set()
	);
	const [configStats, setConfigStats] = useState<ConfigStats | undefined>(undefined);
	const [, setConfigLoading] = useState(true);
	// TODO: changedFiles need to be populated from event handlers
	const [changedFiles, _setChangedFiles] = useState<ChangedFile[]>([]);

	// Derived state: filter tasks by status
	const queuedTasks = useMemo(
		() =>
			tasks.filter(
				(task: Task) =>
					task.status === TaskStatus.PLANNED ||
					task.status === TaskStatus.CREATED ||
					task.status === TaskStatus.CLASSIFYING
			),
		[tasks]
	);

	const runningTasks = useMemo(
		() => tasks.filter((task: Task) => task.status === TaskStatus.RUNNING),
		[tasks]
	);

	const blockedTasks = useMemo(
		() => tasks.filter((task: Task) => task.status === TaskStatus.BLOCKED || task.isBlocked),
		[tasks]
	);

	const completedToday = useMemo(
		() => tasks.filter((task: Task) => isCompletedToday(task)),
		[tasks]
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

	const updateTask = useTaskStore((state) => state.updateTask);

	const handleSkipBlock = useCallback(
		async (taskId: string) => {
			try {
				const result = await taskClient.skipBlock(create(SkipBlockRequestSchema, { id: taskId }));
				// Update task store: use returned task or clear blockers manually
				if (result.task) {
					updateTask(taskId, result.task);
				} else {
					updateTask(taskId, {
						blockedBy: [],
						isBlocked: false,
						unmetBlockers: [],
						status: TaskStatus.PLANNED,
					});
				}
				console.log('Skip block successful for task:', taskId);
			} catch (error) {
				console.error('Failed to skip block:', error);
			}
		},
		[updateTask]
	);

	const handleForceBlock = useCallback(
		async (taskId: string) => {
			try {
				// Note: RunTask doesn't have a force option in proto yet
				// This will start the task if it can be started
				const result = await taskClient.runTask(create(RunTaskRequestSchema, { id: taskId }));
				if (result.task) {
					updateTask(taskId, result.task);
				} else {
					updateTask(taskId, { status: TaskStatus.RUNNING });
				}
				console.log('Force run started for task:', taskId);
			} catch (error) {
				console.error('Failed to force run task:', error);
			}
		},
		[updateTask]
	);

	const handleDecide = useCallback(
		async (decisionId: string, optionId: string) => {
			try {
				// Find the decision to get the option details
				const decision = pendingDecisions.find((d) => d.id === decisionId);
				if (!decision) return;

				// Find the selected option
				const option = decision.options.find((o) => o.id === optionId);
				if (!option) return;

				// Submit the decision (approve with selected option)
				await decisionClient.resolveDecision(create(ResolveDecisionRequestSchema, {
					id: decisionId,
					approved: true,
					reason: option.label,
				}));

				// Remove the decision from pending list immediately (don't wait for events)
				removePendingDecision(decisionId);
			} catch (error) {
				console.error('Failed to submit decision:', error);
			}
		},
		[pendingDecisions, removePendingDecision]
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

	// Fetch config stats on mount
	useEffect(() => {
		let mounted = true;

		configClient.getConfigStats(create(GetConfigStatsRequestSchema, {}))
			.then((response) => {
				if (mounted && response.stats) {
					// Convert proto ConfigStats to ConfigPanel ConfigStats
					// (bigint claudeMdSize -> number)
					const stats: ConfigStats = {
						slashCommandsCount: response.stats.slashCommandsCount,
						claudeMdSize: Number(response.stats.claudeMdSize),
						mcpServersCount: response.stats.mcpServersCount,
						permissionsProfile: response.stats.permissionsProfile,
					};
					setConfigStats(stats);
					setConfigLoading(false);
				}
			})
			.catch((error) => {
				console.error('Failed to fetch config stats:', error);
				if (mounted) {
					setConfigLoading(false);
				}
			});

		return () => {
			mounted = false;
		};
	}, []);

	// Set right panel content via AppShell context and ensure panel is open
	useEffect(() => {
		// Ensure panel is open when board is mounted
		if (!isRightPanelOpen) {
			toggleRightPanel();
		}

		setRightPanelContent(
			<div className="board-view__panel command-panel">
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
			</div>
		);

		return () => {
			setRightPanelContent(null);
		};
	// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [blockedTasks, pendingDecisions, configStats, changedFiles, completedToday, totalTokens, totalCost]);

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
