/**
 * TaskFooter component
 *
 * Footer bar displaying session metrics and compact status/actions.
 */

import { useState, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { taskClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import { PauseTaskRequestSchema, ResumeTaskRequestSchema } from '@/gen/orc/v1/task_pb';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import './TaskFooter.css';

interface TaskMetrics {
	tokens: number;
	cost: number;
	inputTokens?: number;
	outputTokens?: number;
}

interface TaskFooterProps {
	task: Task;
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

export function TaskFooter({ task, metrics, onTaskUpdate }: TaskFooterProps) {
	const projectId = useCurrentProjectId();
	const [isLoading, setIsLoading] = useState(false);

	const isRunning = task.status === TaskStatus.RUNNING;
	const isPaused = task.status === TaskStatus.PAUSED;
	const isFailed = task.status === TaskStatus.FAILED;
	const isCompleted = task.status === TaskStatus.COMPLETED;

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
	 * Handle cancel task
	 */
	const handleCancel = useCallback(async () => {
		// For now, canceling is essentially pausing
		// A more complete implementation would call a dedicated cancel endpoint
		await handlePause();
	}, [handlePause]);

	if (isCompleted || isFailed) {
		const statusIcon = isCompleted ? 'check-circle' : 'alert-circle';
		const statusText = isCompleted ? 'Completed' : 'Failed';

		return (
			<footer
				className={`task-footer ${isCompleted ? 'task-footer--completed' : 'task-footer--failed'}`}
			>
				<div className="task-footer__status">
					<Icon name={statusIcon} size={16} className="task-footer__status-icon" />
					<span>{statusText}</span>
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
