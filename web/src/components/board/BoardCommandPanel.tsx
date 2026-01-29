/**
 * BoardCommandPanel - Self-contained right panel for the board view.
 *
 * Reads directly from Zustand stores instead of receiving data through
 * context or props. This eliminates the render cascade that occurred when
 * BoardView pushed JSX into AppShellContext via useEffect.
 *
 * Sections:
 * - Blocked: Tasks blocked by dependencies (skip/force actions)
 * - Decisions: Pending decisions from running tasks
 * - Config: Claude Code configuration stats
 * - Files: Changed files across running tasks
 * - Completed: Today's completed task summary with token/cost stats
 */

import { useMemo, useState, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { BlockedPanel } from './BlockedPanel';
import { DecisionsPanel } from './DecisionsPanel';
import { ConfigPanel } from './ConfigPanel';
import { FilesPanel, type ChangedFile } from './FilesPanel';
import { CompletedPanel } from './CompletedPanel';
import { useTaskStore } from '@/stores/taskStore';
import { useSessionStore } from '@/stores/sessionStore';
import { usePendingDecisions, useUIStore } from '@/stores/uiStore';
import { decisionClient, taskClient, configClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import { ResolveDecisionRequestSchema } from '@/gen/orc/v1/decision_pb';
import { type Task, TaskStatus, SkipBlockRequestSchema, RunTaskRequestSchema } from '@/gen/orc/v1/task_pb';
import { GetConfigStatsRequestSchema } from '@/gen/orc/v1/config_pb';
import { timestampToDate } from '@/lib/time';
import { type ConfigStats } from './ConfigPanel';

/**
 * Check if a task was completed today.
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
 * Self-contained command panel for the board's right panel.
 * Reads all data from stores — no props, no context injection.
 */
export function BoardCommandPanel(): React.ReactElement {
	const navigate = useNavigate();

	// Store data
	const tasks = useTaskStore((state) => state.tasks);
	const totalTokens = useSessionStore((state) => state.totalTokens);
	const totalCost = useSessionStore((state) => state.totalCost);
	const pendingDecisions = usePendingDecisions();
	const removePendingDecision = useUIStore((state) => state.removePendingDecision);
	const updateTask = useTaskStore((state) => state.updateTask);

	// Local state
	const [configStats, setConfigStats] = useState<ConfigStats | undefined>(undefined);
	// TODO: changedFiles need to be populated from event handlers
	const [changedFiles] = useState<ChangedFile[]>([]);

	// Deduplicate tasks by ID
	const uniqueTasks = useMemo(() => {
		const seen = new Set<string>();
		return tasks.filter((task: Task) => {
			if (seen.has(task.id)) return false;
			seen.add(task.id);
			return true;
		});
	}, [tasks]);

	// Derived state
	const blockedTasks = useMemo(
		() => uniqueTasks.filter((task: Task) => task.status === TaskStatus.BLOCKED || task.isBlocked),
		[uniqueTasks]
	);

	const completedToday = useMemo(
		() => uniqueTasks.filter((task: Task) => isCompletedToday(task)),
		[uniqueTasks]
	);

	// Handlers
	const handleSkipBlock = useCallback(
		async (taskId: string) => {
			try {
				const result = await taskClient.skipBlock(create(SkipBlockRequestSchema, { id: taskId }));
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
			} catch (error) {
				console.error('Failed to skip block:', error);
			}
		},
		[updateTask]
	);

	const handleForceBlock = useCallback(
		async (taskId: string) => {
			try {
				const result = await taskClient.runTask(create(RunTaskRequestSchema, { id: taskId }));
				if (result.task) {
					updateTask(taskId, result.task);
				} else {
					updateTask(taskId, { status: TaskStatus.RUNNING });
				}
			} catch (error) {
				console.error('Failed to force run task:', error);
			}
		},
		[updateTask]
	);

	const handleDecide = useCallback(
		async (decisionId: string, optionId: string) => {
			try {
				const decision = pendingDecisions.find((d) => d.id === decisionId);
				if (!decision) return;

				const option = decision.options.find((o) => o.id === optionId);
				if (!option) return;

				await decisionClient.resolveDecision(create(ResolveDecisionRequestSchema, {
					id: decisionId,
					approved: true,
					reason: option.label,
				}));

				removePendingDecision(decisionId);
			} catch (error) {
				console.error('Failed to submit decision:', error);
			}
		},
		[pendingDecisions, removePendingDecision]
	);

	const handleFileClick = useCallback(
		(file: ChangedFile) => {
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
					const stats: ConfigStats = {
						slashCommandsCount: response.stats.slashCommandsCount,
						claudeMdSize: Number(response.stats.claudeMdSize),
						mcpServersCount: response.stats.mcpServersCount,
						permissionsProfile: response.stats.permissionsProfile,
					};
					setConfigStats(stats);
				}
			})
			.catch((error) => {
				console.error('Failed to fetch config stats:', error);
			});

		return () => {
			mounted = false;
		};
	}, []);

	return (
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
}
