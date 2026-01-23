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
import { useAppShell } from '@/components/layout/AppShellContext';
import { useTaskStore } from '@/stores/taskStore';
import { useInitiatives } from '@/stores/initiativeStore';
import { useSessionStore } from '@/stores/sessionStore';
import { useWebSocket } from '@/hooks/useWebSocket';
import { submitDecision, getConfigStats, type ConfigStats } from '@/lib/api';
import type {
	Task,
	TaskState,
	PendingDecision,
	DecisionRequiredData,
	DecisionResolvedData,
	FilesChangedData,
	FileChangedInfo,
	DecisionOption,
} from '@/lib/types';
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

	// WebSocket hook - also get status to re-subscribe when connection is ready
	const { on, status: wsStatus } = useWebSocket();

	// Local state
	const [collapsedSwimlanes, setCollapsedSwimlanes] = useState<Set<string>>(
		new Set()
	);
	const [pendingDecisions, setPendingDecisions] = useState<PendingDecision[]>([]);
	const [configStats, setConfigStats] = useState<ConfigStats | undefined>(undefined);
	const [, setConfigLoading] = useState(true);
	const [changedFiles, setChangedFiles] = useState<ChangedFile[]>([]);

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

	// Map of task ID to pending decision count
	const taskDecisionCounts = useMemo(() => {
		const counts = new Map<string, number>();
		for (const decision of pendingDecisions) {
			const currentCount = counts.get(decision.task_id) || 0;
			counts.set(decision.task_id, currentCount + 1);
		}
		return counts;
	}, [pendingDecisions]);

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

	const handleSkipBlock = useCallback((_taskId: string) => {
		// TODO: Call API to skip block for task
	}, []);

	const handleForceBlock = useCallback((_taskId: string) => {
		// TODO: Call API to force run blocked task
	}, []);

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
				await submitDecision(decisionId, {
					approved: true,
					reason: option.label,
				});

				// Remove the decision from pending list immediately (don't wait for WebSocket)
				setPendingDecisions((prev) => prev.filter((d) => d.id !== decisionId));
			} catch (error) {
				console.error('Failed to submit decision:', error);
			}
		},
		[pendingDecisions]
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

	// Subscribe to WebSocket events for decisions and files
	// Depend on wsStatus to re-subscribe when connection becomes ready
	useEffect(() => {
		// Wait for WebSocket to be connected before subscribing
		if (wsStatus !== 'connected') {
			return;
		}

		// Subscribe to decision_required events
		const unsubDecisionRequired = on('decision_required', (event) => {
			if ('event' in event && event.event === 'decision_required') {
				const data = event.data as DecisionRequiredData;

				// Convert decision_required data to PendingDecision format
				const decision: PendingDecision = {
					id: data.decision_id,
					task_id: data.task_id,
					question: data.question,
					// Parse context into options if it contains newline-separated criteria
					// For now, create a simple approve/reject option set
					options: [
						{
							id: 'approve',
							label: 'Approve',
							description: 'Approve the gate and continue execution',
							recommended: true,
						},
						{
							id: 'reject',
							label: 'Reject',
							description: 'Reject the gate and fail the task',
						},
					] as DecisionOption[],
					created_at: data.requested_at,
				};

				setPendingDecisions((prev) => {
					// Check if decision already exists (avoid duplicates)
					if (prev.some((d) => d.id === decision.id)) {
						return prev;
					}
					return [...prev, decision];
				});
			}
		});

		// Subscribe to decision_resolved events
		const unsubDecisionResolved = on('decision_resolved', (event) => {
			if ('event' in event && event.event === 'decision_resolved') {
				const data = event.data as DecisionResolvedData;

				// Remove the decision from pending list
				setPendingDecisions((prev) => prev.filter((d) => d.id !== data.decision_id));
			}
		});

		// Subscribe to files_changed events
		const unsubFilesChanged = on('files_changed', (event) => {
			if ('event' in event && event.event === 'files_changed') {
				const data = event.data as FilesChangedData;
				const taskId = event.task_id;

				// Convert FilesChangedData to ChangedFile[] format
				const files: ChangedFile[] = data.files.map((file: FileChangedInfo) => ({
					path: file.path,
					status: file.status,
					taskId,
				}));

				// Replace changed files (it's a snapshot, not accumulative)
				setChangedFiles(files);
			}
		});

		// Subscribe to complete event to clear state
		const unsubComplete = on('complete', (event) => {
			if ('event' in event && event.event === 'complete') {
				const taskId = event.task_id;

				// Clear decisions for this task
				setPendingDecisions((prev) => prev.filter((d) => d.task_id !== taskId));

				// Clear files for this task
				setChangedFiles((prev) => prev.filter((f) => f.taskId !== taskId));
			}
		});

		return () => {
			unsubDecisionRequired();
			unsubDecisionResolved();
			unsubFilesChanged();
			unsubComplete();
		};
	}, [on, wsStatus]);

	// Fetch config stats on mount
	useEffect(() => {
		let mounted = true;

		getConfigStats()
			.then((stats) => {
				if (mounted) {
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
