import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Tooltip } from '@/components/ui/Tooltip';
import { TaskEditModal } from '@/components/task-detail/TaskEditModal';
import { ExportDropdown } from '@/components/task-detail/ExportDropdown';
import { taskClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { getInitiativeBadgeTitle, useCurrentProjectId } from '@/stores';
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
	const projectId = useCurrentProjectId();
	const [showEditModal, setShowEditModal] = useState(false);
	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
	const [isDeleting, setIsDeleting] = useState(false);
	const [actionLoading, setActionLoading] = useState(false);

	// Handle task actions
	const handleRun = useCallback(async () => {
		if (!projectId) return;
		setActionLoading(true);
		try {
			const result = await taskClient.runTask(
				create(RunTaskRequestSchema, { projectId, taskId: task.id })
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
	}, [projectId, task.id, onTaskUpdate]);

	const handlePause = useCallback(async () => {
		if (!projectId) return;
		setActionLoading(true);
		try {
			await taskClient.pauseTask(
				create(PauseTaskRequestSchema, { projectId, taskId: task.id })
			);
			toast.success('Task paused');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to pause task');
		} finally {
			setActionLoading(false);
		}
	}, [projectId, task.id]);

	const handleResume = useCallback(async () => {
		if (!projectId) return;
		setActionLoading(true);
		try {
			await taskClient.resumeTask(
				create(ResumeTaskRequestSchema, { projectId, taskId: task.id })
			);
			toast.success('Task resumed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to resume task');
		} finally {
			setActionLoading(false);
		}
	}, [projectId, task.id]);

	const handleDelete = useCallback(async () => {
		if (!projectId) return;
		setIsDeleting(true);
		try {
			await taskClient.deleteTask(
				create(DeleteTaskRequestSchema, { projectId, taskId: task.id })
			);
			toast.success('Task deleted');
			onTaskDelete();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete task');
			setShowDeleteConfirm(false);
		} finally {
			setIsDeleting(false);
		}
	}, [projectId, task.id, onTaskDelete]);

	// Determine which action button to show
	const getActionButton = () => {
		switch (task.status) {
			case TaskStatus.CREATED:
			case TaskStatus.PLANNED:
			case TaskStatus.FAILED:
				return (
					<Button
						variant="primary"
						onClick={handleRun}
						title="Run task"
						loading={actionLoading}
						leftIcon={<Icon name="play" size={16} />}
						className="action-btn run"
					>
						Run
					</Button>
				);
			case TaskStatus.RUNNING:
				return (
					<Button
						variant="secondary"
						onClick={handlePause}
						title="Pause task"
						loading={actionLoading}
						leftIcon={<Icon name="pause" size={16} />}
						className="action-btn pause"
					>
						Pause
					</Button>
				);
			case TaskStatus.PAUSED:
				return (
					<Button
						variant="primary"
						onClick={handleResume}
						title="Resume task"
						loading={actionLoading}
						leftIcon={<Icon name="play" size={16} />}
						className="action-btn resume"
					>
						Resume
					</Button>
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
				<Button
					variant="ghost"
					iconOnly
					onClick={() => navigate(-1)}
					title="Go back"
					aria-label="Go back"
					className="back-btn"
				>
					<Icon name="arrow-left" size={20} />
				</Button>

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
							<Button
								variant="ghost"
								size="sm"
								className="initiative-badge"
								onClick={() => navigate(`/initiatives/${task.initiativeId}`)}
							>
								<Icon name="layers" size={12} />
								{initiativeBadge.display}
							</Button>
						</Tooltip>
					)}
				</div>

				<div className="task-actions">
					{getActionButton()}
					<ExportDropdown taskId={task.id} />
					<Button
						variant="ghost"
						iconOnly
						onClick={() => setShowEditModal(true)}
						title="Edit task"
						aria-label="Edit task"
						className="icon-btn"
					>
						<Icon name="edit" size={18} />
					</Button>
					<Button
						variant="danger"
						iconOnly
						onClick={() => setShowDeleteConfirm(true)}
						title="Delete task"
						aria-label="Delete task"
						className="icon-btn"
					>
						<Icon name="trash" size={18} />
					</Button>
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
							<Button
								variant="secondary"
								onClick={() => setShowDeleteConfirm(false)}
								disabled={isDeleting}
							>
								Cancel
							</Button>
							<Button
								variant="danger"
								onClick={handleDelete}
								loading={isDeleting}
							>
								Delete
							</Button>
						</div>
					</div>
				</div>
			)}
		</header>
	);
}
