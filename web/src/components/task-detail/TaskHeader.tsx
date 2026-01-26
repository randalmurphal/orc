import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { Icon } from '@/components/ui/Icon';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Tooltip } from '@/components/ui/Tooltip';
import { TaskEditModal } from '@/components/task-detail/TaskEditModal';
import { ExportDropdown } from '@/components/task-detail/ExportDropdown';
import { taskClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { getInitiativeBadgeTitle } from '@/stores';
import {
	DeleteTaskRequestSchema,
	RunTaskRequestSchema,
	PauseTaskRequestSchema,
	ResumeTaskRequestSchema,
} from '@/gen/orc/v1/task_pb';
import type { Task, TaskPlan } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskWeight, TaskCategory, TaskPriority } from '@/gen/orc/v1/task_pb';
import type { IconName } from '@/components/ui/Icon';
import './TaskHeader.css';

// Config for category display with proto enum keys
const CATEGORY_CONFIG: Record<TaskCategory, { label: string; color: string; icon: IconName }> = {
	[TaskCategory.FEATURE]: { label: 'Feature', color: 'var(--status-success)', icon: 'sparkles' },
	[TaskCategory.BUG]: { label: 'Bug', color: 'var(--status-error)', icon: 'bug' },
	[TaskCategory.REFACTOR]: { label: 'Refactor', color: 'var(--status-info)', icon: 'recycle' },
	[TaskCategory.CHORE]: { label: 'Chore', color: 'var(--text-muted)', icon: 'tools' },
	[TaskCategory.DOCS]: { label: 'Docs', color: 'var(--status-warning)', icon: 'file-text' },
	[TaskCategory.TEST]: { label: 'Test', color: 'var(--cyan)', icon: 'beaker' },
	[TaskCategory.UNSPECIFIED]: { label: '', color: '', icon: 'sparkles' },
};

// Config for priority display with proto enum keys
const PRIORITY_CONFIG: Record<TaskPriority, { label: string; color: string }> = {
	[TaskPriority.CRITICAL]: { label: 'Critical', color: 'var(--status-error)' },
	[TaskPriority.HIGH]: { label: 'High', color: 'var(--status-warning)' },
	[TaskPriority.NORMAL]: { label: 'Normal', color: 'var(--text-muted)' },
	[TaskPriority.LOW]: { label: 'Low', color: 'var(--text-muted)' },
	[TaskPriority.UNSPECIFIED]: { label: 'Normal', color: 'var(--text-muted)' },
};

// Weight labels for display
const WEIGHT_LABELS: Record<TaskWeight, string> = {
	[TaskWeight.TRIVIAL]: 'trivial',
	[TaskWeight.SMALL]: 'small',
	[TaskWeight.MEDIUM]: 'medium',
	[TaskWeight.LARGE]: 'large',
	[TaskWeight.UNSPECIFIED]: '',
};

// Priority keys for CSS class names
const PRIORITY_KEYS: Record<TaskPriority, string> = {
	[TaskPriority.CRITICAL]: 'critical',
	[TaskPriority.HIGH]: 'high',
	[TaskPriority.NORMAL]: 'normal',
	[TaskPriority.LOW]: 'low',
	[TaskPriority.UNSPECIFIED]: 'normal',
};

interface TaskHeaderProps {
	task: Task;
	plan?: TaskPlan;
	onTaskUpdate: (task: Task) => void;
	onTaskDelete: () => void;
}

export function TaskHeader({ task, plan, onTaskUpdate, onTaskDelete }: TaskHeaderProps) {
	const navigate = useNavigate();
	const [showEditModal, setShowEditModal] = useState(false);
	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
	const [isDeleting, setIsDeleting] = useState(false);
	const [actionLoading, setActionLoading] = useState(false);

	// Handle task actions
	const handleRun = useCallback(async () => {
		setActionLoading(true);
		try {
			const result = await taskClient.runTask(
				create(RunTaskRequestSchema, { id: task.id })
			);
			if (result.task) {
				onTaskUpdate(result.task);
			}
			toast.success('Task started');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to start task');
		} finally {
			setActionLoading(false);
		}
	}, [task.id, onTaskUpdate]);

	const handlePause = useCallback(async () => {
		setActionLoading(true);
		try {
			await taskClient.pauseTask(
				create(PauseTaskRequestSchema, { id: task.id })
			);
			toast.success('Task paused');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to pause task');
		} finally {
			setActionLoading(false);
		}
	}, [task.id]);

	const handleResume = useCallback(async () => {
		setActionLoading(true);
		try {
			await taskClient.resumeTask(
				create(ResumeTaskRequestSchema, { id: task.id })
			);
			toast.success('Task resumed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to resume task');
		} finally {
			setActionLoading(false);
		}
	}, [task.id]);

	const handleDelete = useCallback(async () => {
		setIsDeleting(true);
		try {
			await taskClient.deleteTask(
				create(DeleteTaskRequestSchema, { id: task.id })
			);
			toast.success('Task deleted');
			onTaskDelete();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete task');
			setShowDeleteConfirm(false);
		} finally {
			setIsDeleting(false);
		}
	}, [task.id, onTaskDelete]);

	// Determine which action button to show
	const getActionButton = () => {
		if (actionLoading) {
			return (
				<button className="action-btn" disabled>
					<div className="btn-spinner" />
				</button>
			);
		}

		switch (task.status) {
			case TaskStatus.CREATED:
			case TaskStatus.PLANNED:
			case TaskStatus.FAILED:
				return (
					<button className="action-btn run" onClick={handleRun} title="Run task">
						<Icon name="play" size={16} />
						<span>Run</span>
					</button>
				);
			case TaskStatus.RUNNING:
				return (
					<button className="action-btn pause" onClick={handlePause} title="Pause task">
						<Icon name="pause" size={16} />
						<span>Pause</span>
					</button>
				);
			case TaskStatus.PAUSED:
				return (
					<button className="action-btn resume" onClick={handleResume} title="Resume task">
						<Icon name="play" size={16} />
						<span>Resume</span>
					</button>
				);
			default:
				return null;
		}
	};

	const categoryConfig = task.category !== TaskCategory.UNSPECIFIED
		? CATEGORY_CONFIG[task.category]
		: null;
	const priority = task.priority || TaskPriority.NORMAL;
	const priorityConfig = PRIORITY_CONFIG[priority];
	const initiativeBadge = task.initiativeId ? getInitiativeBadgeTitle(task.initiativeId) : null;

	// Calculate phase progress for running tasks
	const isRunning = task.status === TaskStatus.RUNNING;
	const phaseProgress = (() => {
		if (!isRunning || !plan || !task.currentPhase) return null;
		const currentIndex = plan.phases.findIndex(p => p.name === task.currentPhase);
		if (currentIndex === -1) return null;
		return { current: currentIndex + 1, total: plan.phases.length };
	})();

	return (
		<header className="task-header">
			<div className="task-header-top">
				<button className="back-btn" onClick={() => navigate(-1)} title="Go back">
					<Icon name="arrow-left" size={20} />
				</button>

				<div className="task-identity">
					<span className="task-id">{task.id}</span>
					<StatusIndicator status={task.status} size="sm" />
					{isRunning && task.currentPhase && (
						<span className="running-status-badge pulse">
							Running: {task.currentPhase}
							{phaseProgress && (
								<span className="phase-progress">
									({phaseProgress.current} of {phaseProgress.total})
								</span>
							)}
						</span>
					)}
					{task.weight !== TaskWeight.UNSPECIFIED && (
						<span className="weight-badge">{WEIGHT_LABELS[task.weight]}</span>
					)}
					{categoryConfig && (
						<span
							className="category-badge"
							style={{ '--category-color': categoryConfig.color } as React.CSSProperties}
						>
							<Icon name={categoryConfig.icon} size={12} className="category-icon" />
							{categoryConfig.label}
						</span>
					)}
					<Tooltip content={`${priorityConfig.label} priority`}>
						<span
							className={`priority-badge priority-${PRIORITY_KEYS[priority]}`}
							style={{ '--priority-color': priorityConfig.color } as React.CSSProperties}
						>
							{priorityConfig.label}
						</span>
					</Tooltip>
					{initiativeBadge && (
						<Tooltip content={initiativeBadge.full}>
							<button
								className="initiative-badge"
								onClick={() => navigate(`/initiatives/${task.initiativeId}`)}
							>
								<Icon name="layers" size={12} />
								{initiativeBadge.display}
							</button>
						</Tooltip>
					)}
				</div>

				<div className="task-actions">
					{getActionButton()}
					<ExportDropdown taskId={task.id} />
					<button
						className="icon-btn"
						onClick={() => setShowEditModal(true)}
						title="Edit task"
					>
						<Icon name="edit" size={18} />
					</button>
					<button
						className="icon-btn danger"
						onClick={() => setShowDeleteConfirm(true)}
						title="Delete task"
					>
						<Icon name="trash" size={18} />
					</button>
				</div>
			</div>

			<h1 className="task-title">{task.title}</h1>
			{task.description && (
				<p className="task-description">{task.description}</p>
			)}

			{/* Branch info */}
			{task.branch && (
				<div className="task-branch">
					<Icon name="branch" size={14} />
					<code>{task.branch}</code>
				</div>
			)}

			{/* Edit Modal */}
			<TaskEditModal
				open={showEditModal}
				task={task}
				onClose={() => setShowEditModal(false)}
				onUpdate={onTaskUpdate}
			/>

			{/* Delete Confirmation */}
			{showDeleteConfirm && (
				<div className="modal-overlay" onClick={() => setShowDeleteConfirm(false)}>
					<div className="delete-confirm" onClick={(e) => e.stopPropagation()}>
						<h3>Delete Task?</h3>
						<p>
							Are you sure you want to delete <strong>{task.id}</strong>? This action cannot
							be undone.
						</p>
						<div className="confirm-actions">
							<button
								className="cancel-btn"
								onClick={() => setShowDeleteConfirm(false)}
								disabled={isDeleting}
							>
								Cancel
							</button>
							<button
								className="delete-btn"
								onClick={handleDelete}
								disabled={isDeleting}
							>
								{isDeleting ? 'Deleting...' : 'Delete'}
							</button>
						</div>
					</div>
				</div>
			)}
		</header>
	);
}
