/**
 * TaskFooter component
 *
 * Footer bar displaying session metrics and action buttons.
 * Handles pause/resume, cancel, and retry with feedback.
 */

import { useState, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { taskClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import {
	PauseTaskRequestSchema,
	ResumeTaskRequestSchema,
	RetryTaskRequestSchema,
} from '@/gen/orc/v1/task_pb';
import type { Task, TaskPlan } from '@/gen/orc/v1/task_pb';
import { TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import './TaskFooter.css';

interface TaskState {
	error?: string;
	phase?: string;
}

interface TaskMetrics {
	tokens: number;
	cost: number;
	inputTokens?: number;
	outputTokens?: number;
}

interface TaskFooterProps {
	task: Task;
	plan?: TaskPlan | null;
	taskState?: TaskState | null;
	metrics: TaskMetrics | null;
	onTaskUpdate?: (task: Task) => void;
}

/**
 * Format token count with K/M suffix
 */
function formatTokens(tokens: number): string {
	if (tokens >= 1_000_000) {
		const value = tokens / 1_000_000;
		return value % 1 === 0 ? `${value}M` : `${value.toFixed(1)}M`;
	}
	if (tokens >= 1_000) {
		const value = tokens / 1_000;
		return value % 1 === 0 ? `${value}K` : `${value.toFixed(1)}K`;
	}
	return String(tokens);
}

/**
 * Format cost as currency
 */
function formatCost(cost: number): string {
	return `$${cost.toFixed(2)}`;
}

export function TaskFooter({
	task,
	plan,
	taskState,
	metrics,
	onTaskUpdate,
}: TaskFooterProps) {
	const projectId = useCurrentProjectId();
	const [isLoading, setIsLoading] = useState(false);
	const [feedback, setFeedback] = useState('');

	const isRunning = task.status === TaskStatus.RUNNING;
	const isPaused = task.status === TaskStatus.PAUSED;
	const isFailed = task.status === TaskStatus.FAILED;
	const isCompleted = task.status === TaskStatus.COMPLETED;

	// Get completed phases for "retry from" options
	const completedPhases =
		plan?.phases.filter((p) => p.status === PhaseStatus.COMPLETED) ?? [];

	const findMostRecentRetryFromPhase = (failedPhase: string | undefined) => {
		for (let i = completedPhases.length - 1; i >= 0; i -= 1) {
			const phase = completedPhases[i];
			if (phase.name !== failedPhase) {
				return phase;
			}
		}
		return null;
	};

	/**
	 * Handle pause task
	 */
	const handlePause = useCallback(async () => {
		if (!projectId) return;
		setIsLoading(true);
		try {
			const result = await taskClient.pauseTask(
				create(PauseTaskRequestSchema, { projectId, taskId: task.id })
			);
			if (result.task) {
				onTaskUpdate?.(result.task);
			}
			toast.success('Task paused');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to pause task');
		} finally {
			setIsLoading(false);
		}
	}, [projectId, task.id, onTaskUpdate]);

	/**
	 * Handle resume task
	 */
	const handleResume = useCallback(async () => {
		if (!projectId) return;
		setIsLoading(true);
		try {
			const result = await taskClient.resumeTask(
				create(ResumeTaskRequestSchema, { projectId, taskId: task.id })
			);
			if (result.task) {
				onTaskUpdate?.(result.task);
			}
			toast.success('Task resumed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to resume task');
		} finally {
			setIsLoading(false);
		}
	}, [projectId, task.id, onTaskUpdate]);

	/**
	 * Handle retry task from a specific phase
	 */
	const handleRetry = useCallback(
		async (fromPhase: string) => {
			if (!projectId) return;
			setIsLoading(true);
			try {
				const result = await taskClient.retryTask(
					create(RetryTaskRequestSchema, {
						projectId,
						taskId: task.id,
						fromPhase,
						instructions: feedback || undefined,
					})
				);
				if (result.task) {
					onTaskUpdate?.(result.task);
				}
				toast.success('Task retry started');
				setFeedback('');
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Failed to retry task');
			} finally {
				setIsLoading(false);
			}
		},
		[projectId, task.id, feedback, onTaskUpdate]
	);

	/**
	 * Handle cancel task
	 */
	const handleCancel = useCallback(async () => {
		// For now, canceling is essentially pausing
		// A more complete implementation would call a dedicated cancel endpoint
		await handlePause();
	}, [handlePause]);

	// Completed state
	if (isCompleted) {
		return (
			<footer className="task-footer task-footer--completed">
				<div className="task-footer__status">
					<Icon name="check-circle" size={16} className="task-footer__status-icon" />
					<span>Completed</span>
				</div>
				{metrics && (
					<div className="task-footer__metrics">
						<span className="task-footer__metric">
							<Icon name="code" size={14} />
							{formatTokens(metrics.tokens)}
						</span>
						<span className="task-footer__metric">
							<Icon name="dollar" size={14} />
							{formatCost(metrics.cost)}
						</span>
					</div>
				)}
			</footer>
		);
	}

	// Failed state with error display and retry options
	if (isFailed) {
		const failedPhase = taskState?.phase || task.currentPhase;
		const errorMessage = taskState?.error || 'Task failed';
		const retryFromPhase = findMostRecentRetryFromPhase(failedPhase);

		return (
			<footer className="task-footer task-footer--failed">
				{/* Error summary */}
				<div className="task-footer__error">
					<div className="task-footer__error-header">
						<Icon name="alert-circle" size={16} className="task-footer__error-icon" />
						<span>
							Error at <strong>{failedPhase}</strong>
						</span>
					</div>
					<div
						className="task-footer__error-details"
						style={{ overflow: 'auto', maxHeight: '80px' }}
					>
						{errorMessage}
					</div>
				</div>

				{/* Guidance textarea */}
				<div className="task-footer__guidance">
					<textarea
						className="task-footer__feedback"
						placeholder="Add guidance or feedback for retry..."
						value={feedback}
						onChange={(e) => setFeedback(e.target.value)}
						rows={2}
					/>
				</div>

				{/* Retry actions */}
				<div className="task-footer__actions">
					{/* Retry from current phase */}
					<Button
						variant="primary"
						onClick={() => handleRetry(failedPhase || '')}
						loading={isLoading}
						leftIcon={<Icon name="refresh" size={14} />}
					>
						Retry {failedPhase}
					</Button>

					{/* Retry from most recent completed phase before failure */}
					{retryFromPhase && (
						<Button
							variant="secondary"
							onClick={() => handleRetry(retryFromPhase.name)}
							loading={isLoading}
							aria-label={`Retry from ${retryFromPhase.name}`}
						>
							Retry from {retryFromPhase.name}
						</Button>
					)}
				</div>
			</footer>
		);
	}

	// Running or paused state
	return (
		<footer className="task-footer">
			{/* Metrics */}
			<div className="task-footer__metrics">
				{metrics ? (
					<>
						<span className="task-footer__metric">
							<Icon name="code" size={14} />
							{formatTokens(metrics.tokens)}
						</span>
						<span className="task-footer__metric">
							<Icon name="dollar" size={14} />
							{formatCost(metrics.cost)}
						</span>
					</>
				) : (
					<span className="task-footer__metric">—</span>
				)}
			</div>

			{/* Actions */}
			<div className="task-footer__actions">
				{isRunning && (
					<>
						<Button
							variant="secondary"
							onClick={handlePause}
							loading={isLoading}
							leftIcon={<Icon name="pause" size={14} />}
						>
							Pause
						</Button>
						<Button
							variant="ghost"
							onClick={handleCancel}
							loading={isLoading}
							leftIcon={<Icon name="x" size={14} />}
						>
							Cancel
						</Button>
					</>
				)}
				{isPaused && (
					<Button
						variant="primary"
						onClick={handleResume}
						loading={isLoading}
						leftIcon={<Icon name="play" size={14} />}
					>
						Resume
					</Button>
				)}
			</div>
		</footer>
	);
}
