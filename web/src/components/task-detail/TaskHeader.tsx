import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { TaskEditModal } from '@/components/task-detail/TaskEditModal';
import { ExportDropdown } from '@/components/task-detail/ExportDropdown';
import { deleteTask, runTask, pauseTask, resumeTask } from '@/lib/api';
import { toast } from '@/stores/uiStore';
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
		switch (task.status) {
			case 'created':
			case 'planned':
			case 'failed':
				return (
					<Button
						variant="success"
						size="md"
						onClick={handleRun}
						loading={actionLoading}
						leftIcon={<Icon name="play" size={16} />}
						title="Run task"
					>
						Run
					</Button>
				);
			case 'running':
				return (
					<Button
						variant="secondary"
						size="md"
						onClick={handlePause}
						loading={actionLoading}
						leftIcon={<Icon name="pause" size={16} />}
						title="Pause task"
					>
						Pause
					</Button>
				);
			case 'paused':
				return (
					<Button
						variant="primary"
						size="md"
						onClick={handleResume}
						loading={actionLoading}
						leftIcon={<Icon name="play" size={16} />}
						title="Resume task"
					>
						Resume
					</Button>
				);
			default:
				return null;
		}
	};

	const categoryConfig = task.category ? CATEGORY_CONFIG[task.category] : null;
	const priorityConfig = task.priority ? PRIORITY_CONFIG[task.priority] : null;

	return (
		<header className="task-header">
			<div className="task-header-top">
				<Button
					variant="ghost"
					size="sm"
					iconOnly
					onClick={() => navigate(-1)}
					title="Go back"
					aria-label="Go back"
				>
					<Icon name="arrow-left" size={20} />
				</Button>

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
					{task.priority && task.priority !== 'normal' && priorityConfig && (
						<span
							className="priority-badge"
							style={{ '--priority-color': priorityConfig.color } as React.CSSProperties}
						>
							{priorityConfig.label}
						</span>
					)}
				</div>

				<div className="task-actions">
					{getActionButton()}
					<ExportDropdown taskId={task.id} />
					<Button
						variant="ghost"
						size="sm"
						iconOnly
						onClick={() => setShowEditModal(true)}
						title="Edit task"
						aria-label="Edit task"
					>
						<Icon name="edit" size={18} />
					</Button>
					<Button
						variant="danger"
						size="sm"
						iconOnly
						onClick={() => setShowDeleteConfirm(true)}
						title="Delete task"
						aria-label="Delete task"
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
								size="md"
								onClick={() => setShowDeleteConfirm(false)}
								disabled={isDeleting}
							>
								Cancel
							</Button>
							<Button
								variant="danger"
								size="md"
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
