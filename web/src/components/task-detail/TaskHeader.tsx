import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Icon } from '@/components/ui/Icon';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Tooltip } from '@/components/ui/Tooltip';
import { TaskEditModal } from '@/components/task-detail/TaskEditModal';
import { ExportDropdown } from '@/components/task-detail/ExportDropdown';
import { deleteTask, runTask, pauseTask, resumeTask } from '@/lib/api';
import { toast } from '@/stores/uiStore';
import { getInitiativeBadgeTitle } from '@/stores';
import type { Task } from '@/lib/types';
import { CATEGORY_CONFIG, PRIORITY_CONFIG } from '@/lib/types';
import './TaskHeader.css';

interface TaskHeaderProps {
	task: Task;
	onTaskUpdate: (task: Task) => void;
	onTaskDelete: () => void;
}

export function TaskHeader({ task, onTaskUpdate, onTaskDelete }: TaskHeaderProps) {
	const navigate = useNavigate();
	const [showEditModal, setShowEditModal] = useState(false);
	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
	const [isDeleting, setIsDeleting] = useState(false);
	const [actionLoading, setActionLoading] = useState(false);

	// Handle task actions
	const handleRun = useCallback(async () => {
		setActionLoading(true);
		try {
			const result = await runTask(task.id);
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
			await pauseTask(task.id);
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
			await resumeTask(task.id);
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
			await deleteTask(task.id);
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
			case 'created':
			case 'planned':
			case 'failed':
				return (
					<button className="action-btn run" onClick={handleRun} title="Run task">
						<Icon name="play" size={16} />
						<span>Run</span>
					</button>
				);
			case 'running':
				return (
					<button className="action-btn pause" onClick={handlePause} title="Pause task">
						<Icon name="pause" size={16} />
						<span>Pause</span>
					</button>
				);
			case 'paused':
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

	const categoryConfig = task.category ? CATEGORY_CONFIG[task.category] : null;
	const priority = task.priority || 'normal';
	const priorityConfig = PRIORITY_CONFIG[priority];
	const initiativeBadge = task.initiative_id ? getInitiativeBadgeTitle(task.initiative_id) : null;

	return (
		<header className="task-header">
			<div className="task-header-top">
				<button className="back-btn" onClick={() => navigate(-1)} title="Go back">
					<Icon name="arrow-left" size={20} />
				</button>

				<div className="task-identity">
					<span className="task-id">{task.id}</span>
					<StatusIndicator status={task.status} size="sm" />
					{task.weight && (
						<span className="weight-badge">{task.weight}</span>
					)}
					{categoryConfig && (
						<span
							className="category-badge"
							style={{ '--category-color': categoryConfig.color } as React.CSSProperties}
						>
							<span className="category-icon">{categoryConfig.icon}</span>
							{categoryConfig.label}
						</span>
					)}
					<Tooltip content={`${priorityConfig.label} priority`}>
						<span
							className={`priority-badge priority-${priority}`}
							style={{ '--priority-color': priorityConfig.color } as React.CSSProperties}
						>
							{priorityConfig.label}
						</span>
					</Tooltip>
					{initiativeBadge && (
						<Tooltip content={initiativeBadge.full}>
							<button
								className="initiative-badge"
								onClick={() => navigate(`/initiatives/${task.initiative_id}`)}
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
