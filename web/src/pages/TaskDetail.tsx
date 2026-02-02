import { useEffect, useState, useCallback, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { WorkflowProgress } from '@/components/task-detail/WorkflowProgress';
import { TaskFooter } from '@/components/task-detail/TaskFooter';
import { SplitPane } from '@/components/core/SplitPane';
import { TranscriptTab } from '@/components/task-detail/TranscriptTab';
import { ChangesTab } from '@/components/task-detail/ChangesTab';
import { FeedbackPanel } from '@/components/feedback/FeedbackPanel';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { taskClient } from '@/lib/client';
import { useTaskSubscription, useDocumentTitle } from '@/hooks';
import { useTask as useStoreTask } from '@/stores/taskStore';
import { useCurrentProjectId } from '@/stores';
import type { Task, TaskPlan } from '@/gen/orc/v1/task_pb';
import { GetTaskRequestSchema, GetTaskPlanRequestSchema, TaskStatus } from '@/gen/orc/v1/task_pb';
import './TaskDetail.css';

/**
 * Format elapsed time since a timestamp
 */
function formatElapsedTime(startedAt: { seconds: bigint } | undefined): string {
	if (!startedAt) return '—';
	const startMs = Number(startedAt.seconds) * 1000;
	const elapsed = Date.now() - startMs;
	const totalSeconds = Math.floor(elapsed / 1000);
	const minutes = Math.floor(totalSeconds / 60);
	const seconds = totalSeconds % 60;
	return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

/**
 * Task detail page (/tasks/:id)
 *
 * New "deep work" layout with:
 * - Header with back link, task info, workflow, branch, elapsed time
 * - Workflow progress visualization
 * - Split pane (Live Output + Changes)
 * - Footer with metrics and action buttons
 */
export function TaskDetail() {
	const { id } = useParams<{ id: string }>();
	const projectId = useCurrentProjectId();

	// State
	const [task, setTask] = useState<Task | null>(null);
	const [plan, setPlan] = useState<TaskPlan | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [elapsedTime, setElapsedTime] = useState('—');

	// Set document title based on task
	useDocumentTitle(task ? `${task.id}: ${task.title}` : id);

	// Subscribe to real-time updates
	const { state: taskState, transcript: streamingTranscript } = useTaskSubscription(id);

	// Get task from store (updated by WebSocket events)
	const storeTask = useStoreTask(id ?? '');

	// Sync local task state with store task when WebSocket updates arrive
	useEffect(() => {
		if (storeTask) {
			setTask((prev) => {
				if (prev && (prev.status !== storeTask.status || prev.currentPhase !== storeTask.currentPhase)) {
					return { ...prev, status: storeTask.status, currentPhase: storeTask.currentPhase };
				}
				return prev;
			});
		}
	}, [storeTask]);

	// Load task data
	const loadTask = useCallback(async () => {
		if (!id || !projectId) return;

		setLoading(true);
		setError(null);

		try {
			const [taskResponse, planResponse] = await Promise.all([
				taskClient.getTask(create(GetTaskRequestSchema, { projectId, taskId: id })),
				taskClient.getTaskPlan(create(GetTaskPlanRequestSchema, { projectId, taskId: id })).catch(() => null),
			]);

			if (taskResponse.task) {
				setTask(taskResponse.task);
			}
			if (planResponse?.plan) {
				setPlan(planResponse.plan);
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load task');
		} finally {
			setLoading(false);
		}
	}, [id, projectId]);

	// Initial load
	useEffect(() => {
		loadTask();
	}, [loadTask]);

	// Update elapsed time every second when task is running
	useEffect(() => {
		if (!task?.startedAt) return;

		// Initial update
		setElapsedTime(formatElapsedTime(task.startedAt));

		// Update every second
		const interval = setInterval(() => {
			setElapsedTime(formatElapsedTime(task.startedAt));
		}, 1000);

		return () => clearInterval(interval);
	}, [task?.startedAt]);

	// Handle task update (from footer actions)
	const handleTaskUpdate = useCallback((updatedTask: Task) => {
		setTask(updatedTask);
	}, []);

	// Build metrics from task state
	const metrics = useMemo(() => {
		if (!taskState) return null;
		// Extract metrics from taskState if available
		// The actual structure depends on the WebSocket event format
		return {
			tokens: 0,
			cost: 0,
		};
	}, [taskState]);

	// Loading state
	if (loading) {
		return (
			<div className="task-detail-page">
				<div className="task-detail-loading">
					<div className="loading-spinner" />
					<span>Loading task...</span>
				</div>
			</div>
		);
	}

	// Error state
	if (error || !task) {
		return (
			<div className="task-detail-page">
				<div className="task-detail-error">
					<Icon name="alert-circle" size={32} />
					<h2>Failed to load task</h2>
					<p>{error || 'Task not found'}</p>
					<Button variant="secondary" onClick={loadTask}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="task-detail-page">
			{/* Header */}
			<header className="task-detail-header">
				<div className="task-detail-header__top">
					<Link to="/board" className="task-detail-header__back">
						<Icon name="arrow-left" size={16} />
						Back to Board
					</Link>
					<div className="task-detail-header__info">
						<span className="task-detail-header__id">{task.id}</span>
						<h1 className="task-detail-header__title">{task.title}</h1>
					</div>
					<div className="task-detail-header__meta">
						{task.workflowId && (
							<span className="task-detail-header__workflow">
								<Icon name="workflow" size={14} />
								{task.workflowId}
							</span>
						)}
						{task.branch && (
							<span className="task-detail-header__branch">
								<Icon name="branch" size={14} />
								<code>{task.branch}</code>
							</span>
						)}
						<span className="task-detail-header__elapsed">
							<Icon name="clock" size={14} />
							{elapsedTime}
						</span>
					</div>
				</div>

				{/* Workflow Progress */}
				<WorkflowProgress task={task} plan={plan} />
			</header>

			{/* Main content: Split pane */}
			<div className="task-detail-content">
				<SplitPane
					left={
						<div className="task-detail-panel">
							<h2 className="task-detail-panel__title">Live Output</h2>
							<div className="task-detail-panel__content">
								<TranscriptTab taskId={task.id} streamingLines={streamingTranscript} />
							</div>
						</div>
					}
					right={
						<div className="task-detail-panel">
							<h2 className="task-detail-panel__title">Changes</h2>
							<div className="task-detail-panel__content">
								<ChangesTab taskId={task.id} />
							</div>
						</div>
					}
					persistKey="task-detail"
					initialRatio={60}
					minLeftWidth={200}
					minRightWidth={200}
					leftEmptyMessage="No output yet"
					rightEmptyMessage="No changes yet"
				/>

				{/* Feedback Panel - shown for running tasks or when feedback is available */}
				{(task.status === TaskStatus.RUNNING || task.status === TaskStatus.PAUSED) && projectId && (
					<div className="task-detail-feedback">
						<FeedbackPanel
							taskId={task.id}
							projectId={projectId}
							isTaskRunning={task.status === TaskStatus.RUNNING}
							onFeedbackAdded={(feedback) => {
								console.log('Feedback added:', feedback);
							}}
							onTaskPaused={() => {
								console.log('Task paused for feedback');
								// The task status will be updated via WebSocket subscription
							}}
							onError={(error) => {
								setError(error);
							}}
						/>
					</div>
				)}
			</div>

			{/* Footer */}
			<TaskFooter
				task={task}
				plan={plan}
				taskState={taskState && task.currentPhase
					? {
						error: taskState.phases[task.currentPhase]?.error,
						phase: task.currentPhase,
					}
					: null
				}
				metrics={metrics}
				onTaskUpdate={handleTaskUpdate}
			/>
		</div>
	);
}
