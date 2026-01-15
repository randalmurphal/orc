/**
 * TaskCard component for Kanban board
 *
 * Displays task information with:
 * - Status indicator, ID, and priority badge
 * - Title and description preview
 * - Phase indicator when running
 * - Weight badge, blocked indicator, initiative badge
 * - Action buttons (run/pause/resume/finalize)
 * - Quick menu for queue/priority changes
 * - Drag-and-drop support
 */

import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { Button } from '@/components/ui/Button';
import type { Task, TaskPriority, TaskQueue } from '@/lib/types';
import { PRIORITY_CONFIG } from '@/lib/types';
import { updateTask, triggerFinalize, type FinalizeState } from '@/lib/api';
import { useTaskStore, getInitiativeBadgeTitle } from '@/stores';
import './TaskCard.css';

interface TaskCardProps {
	task: Task;
	onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
	onTaskClick?: (task: Task) => void;
	onInitiativeClick?: (initiativeId: string) => void;
	onFinalizeClick?: (task: Task) => void;
	finalizeState?: FinalizeState | null;
}

// Weight badge color config
const WEIGHT_CONFIG: Record<string, { color: string; bg: string }> = {
	trivial: { color: 'var(--weight-trivial)', bg: 'rgba(107, 114, 128, 0.15)' },
	small: { color: 'var(--weight-small)', bg: 'var(--status-success-bg)' },
	medium: { color: 'var(--weight-medium)', bg: 'var(--status-info-bg)' },
	large: { color: 'var(--weight-large)', bg: 'var(--status-warning-bg)' },
	greenfield: { color: 'var(--weight-greenfield)', bg: 'var(--accent-subtle)' },
};

function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMins / 60);
	const diffDays = Math.floor(diffHours / 24);

	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	if (diffHours < 24) return `${diffHours}h ago`;
	if (diffDays < 7) return `${diffDays}d ago`;
	return date.toLocaleDateString();
}

export function TaskCard({
	task,
	onAction,
	onTaskClick,
	onInitiativeClick,
	onFinalizeClick,
	finalizeState,
}: TaskCardProps) {
	const navigate = useNavigate();
	const updateTaskInStore = useTaskStore((state) => state.updateTask);

	const [actionLoading, setActionLoading] = useState(false);
	const [isDragging, setIsDragging] = useState(false);
	const [showQuickMenu, setShowQuickMenu] = useState(false);
	const [quickMenuLoading, setQuickMenuLoading] = useState(false);
	const [finalizeLoading, setFinalizeLoading] = useState(false);

	// Derived values
	const priority = (task.priority || 'normal') as TaskPriority;
	const priorityConfig = PRIORITY_CONFIG[priority];
	const showPriority = priority !== 'normal';
	const queue = (task.queue || 'active') as TaskQueue;
	const weight = WEIGHT_CONFIG[task.weight] || WEIGHT_CONFIG.small;

	const isRunning = task.status === 'running';
	const isFinalizing = task.status === 'finalizing';
	const isFinished = task.status === 'finished';
	const isCompleted = task.status === 'completed';

	// Initiative badge
	const initiativeBadge = task.initiative_id ? getInitiativeBadgeTitle(task.initiative_id) : null;

	// Finalize progress
	const finalizeProgress =
		finalizeState && finalizeState.status !== 'not_started'
			? {
					step: finalizeState.step || 'Processing',
					progress: finalizeState.progress || '',
					percent: finalizeState.step_percent || 0,
				}
			: null;

	// Drag handlers
	const handleDragStart = useCallback(
		(e: React.DragEvent) => {
			e.dataTransfer.setData('application/json', JSON.stringify(task));
			e.dataTransfer.effectAllowed = 'move';
			setIsDragging(true);
		},
		[task]
	);

	const handleDragEnd = useCallback(() => {
		setIsDragging(false);
	}, []);

	// Action handler
	const handleAction = useCallback(
		async (action: 'run' | 'pause' | 'resume', e: React.MouseEvent) => {
			e.stopPropagation();
			e.preventDefault();
			setActionLoading(true);
			try {
				await onAction(task.id, action);
			} finally {
				setActionLoading(false);
			}
		},
		[task.id, onAction]
	);

	// Card click handler
	const openTask = useCallback(
		(e: React.MouseEvent) => {
			const target = e.target as HTMLElement;
			if (target.closest('.actions') || target.closest('.quick-menu')) {
				return;
			}
			// For running tasks, show transcript modal if callback provided
			if (task.status === 'running' && onTaskClick) {
				onTaskClick(task);
				return;
			}
			navigate(`/tasks/${task.id}`);
		},
		[task, onTaskClick, navigate]
	);

	// Keyboard handler
	const handleKeydown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				navigate(`/tasks/${task.id}`);
			}
			if (e.key === 'Escape' && showQuickMenu) {
				setShowQuickMenu(false);
			}
		},
		[task.id, showQuickMenu, navigate]
	);

	// Quick menu handlers
	const toggleQuickMenu = useCallback((e: React.MouseEvent) => {
		e.stopPropagation();
		e.preventDefault();
		setShowQuickMenu((prev) => !prev);
	}, []);

	const closeQuickMenu = useCallback(() => {
		setShowQuickMenu(false);
	}, []);

	const setQueueValue = useCallback(
		async (newQueue: TaskQueue) => {
			if (newQueue === queue) {
				setShowQuickMenu(false);
				return;
			}
			setQuickMenuLoading(true);
			try {
				const updated = await updateTask(task.id, { queue: newQueue });
				updateTaskInStore(task.id, updated);
			} catch (err) {
				console.error('Failed to update queue:', err);
			} finally {
				setQuickMenuLoading(false);
				setShowQuickMenu(false);
			}
		},
		[task.id, queue, updateTaskInStore]
	);

	const setPriorityValue = useCallback(
		async (newPriority: TaskPriority) => {
			if (newPriority === priority) {
				setShowQuickMenu(false);
				return;
			}
			setQuickMenuLoading(true);
			try {
				const updated = await updateTask(task.id, { priority: newPriority });
				updateTaskInStore(task.id, updated);
			} catch (err) {
				console.error('Failed to update priority:', err);
			} finally {
				setQuickMenuLoading(false);
				setShowQuickMenu(false);
			}
		},
		[task.id, priority, updateTaskInStore]
	);

	// Finalize handler
	const handleFinalize = useCallback(
		async (e: React.MouseEvent) => {
			e.stopPropagation();
			e.preventDefault();

			if (onFinalizeClick) {
				onFinalizeClick(task);
				return;
			}

			setFinalizeLoading(true);
			try {
				await triggerFinalize(task.id);
			} catch (err) {
				console.error('Failed to trigger finalize:', err);
			} finally {
				setFinalizeLoading(false);
			}
		},
		[task, onFinalizeClick]
	);

	// Initiative click handler
	const handleInitiativeClick = useCallback(
		(e: React.MouseEvent) => {
			e.stopPropagation();
			e.preventDefault();
			if (task.initiative_id && onInitiativeClick) {
				onInitiativeClick(task.initiative_id);
			}
		},
		[task.initiative_id, onInitiativeClick]
	);

	// Build class names
	const cardClasses = [
		'task-card',
		isDragging && 'dragging',
		isRunning && 'running',
		isFinalizing && 'finalizing',
		isFinished && 'finished',
		isCompleted && 'completed',
	]
		.filter(Boolean)
		.join(' ');

	return (
		<article
			className={cardClasses}
			draggable="true"
			onDragStart={handleDragStart}
			onDragEnd={handleDragEnd}
			onClick={openTask}
			onKeyDown={handleKeydown}
			tabIndex={0}
			aria-label={`Task ${task.id}: ${task.title}`}
		>
			<div className="card-header">
				<div className="header-left">
					<span className="task-id">{task.id}</span>
					{showPriority && (
						<span
							className={`priority-badge ${priority}`}
							style={{ color: priorityConfig.color }}
							title={`${priorityConfig.label} priority`}
						>
							{priority === 'critical' && (
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="10"
									height="10"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									strokeWidth="2.5"
									strokeLinecap="round"
									strokeLinejoin="round"
								>
									<circle cx="12" cy="12" r="10" />
									<line x1="12" y1="8" x2="12" y2="12" />
									<line x1="12" y1="16" x2="12.01" y2="16" />
								</svg>
							)}
							{priority === 'high' && (
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="10"
									height="10"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									strokeWidth="2.5"
									strokeLinecap="round"
									strokeLinejoin="round"
								>
									<polyline points="18 15 12 9 6 15" />
								</svg>
							)}
							{priority === 'low' && (
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="10"
									height="10"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									strokeWidth="2.5"
									strokeLinecap="round"
									strokeLinejoin="round"
								>
									<polyline points="6 9 12 15 18 9" />
								</svg>
							)}
						</span>
					)}
				</div>
				<StatusIndicator status={task.status} size="sm" />
			</div>

			<h3 className="task-title">{task.title}</h3>

			{task.description && <p className="task-description">{task.description}</p>}

			{task.current_phase && (
				<div className="task-phase">
					<span className="phase-label">Phase:</span>
					<span className="phase-value">{task.current_phase}</span>
				</div>
			)}

			{/* Finalize progress indicator */}
			{isFinalizing && finalizeProgress && (
				<div className="finalize-progress">
					<div className="finalize-step">{finalizeProgress.step}</div>
					<div className="progress-bar">
						<div
							className="progress-fill"
							style={{ width: `${finalizeProgress.percent}%` }}
						/>
					</div>
				</div>
			)}

			{/* Finished commit info */}
			{isFinished && finalizeState?.result?.commit_sha && (
				<div className="finished-info">
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="12"
						height="12"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<circle cx="12" cy="12" r="4" />
						<line x1="1.05" y1="12" x2="7" y2="12" />
						<line x1="17.01" y1="12" x2="22.96" y2="12" />
					</svg>
					<span className="commit-sha">
						{finalizeState.result.commit_sha.slice(0, 7)}
					</span>
					<span className="merge-target">
						merged to {finalizeState.result.target_branch}
					</span>
				</div>
			)}

			<div className="card-footer">
				<div className="footer-left">
					<span
						className="weight-badge"
						style={{ color: weight.color, background: weight.bg }}
					>
						{task.weight}
					</span>
					{task.is_blocked && (
						<span
							className="blocked-badge"
							title={`Blocked by ${task.unmet_blockers?.join(', ')}`}
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="10"
								height="10"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2.5"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<circle cx="12" cy="12" r="10" />
								<line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
							</svg>
							Blocked
						</span>
					)}
					{initiativeBadge && (
						<Button
							variant="ghost"
							size="sm"
							className="initiative-badge"
							onClick={handleInitiativeClick}
							title={initiativeBadge.full}
						>
							{initiativeBadge.display}
						</Button>
					)}
					<span className="updated-time">{formatDate(task.updated_at)}</span>
				</div>

				<div className="actions">
					{(task.status === 'created' || task.status === 'planned') && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							className="action-btn run"
							onClick={(e) => handleAction('run', e)}
							disabled={actionLoading}
							title="Run task"
							aria-label="Run task"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="12"
								height="12"
								viewBox="0 0 24 24"
								fill="currentColor"
								stroke="none"
							>
								<polygon points="5 3 19 12 5 21 5 3" />
							</svg>
						</Button>
					)}
					{task.status === 'running' && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							className="action-btn pause"
							onClick={(e) => handleAction('pause', e)}
							disabled={actionLoading}
							title="Pause task"
							aria-label="Pause task"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="12"
								height="12"
								viewBox="0 0 24 24"
								fill="currentColor"
								stroke="none"
							>
								<rect x="6" y="4" width="4" height="16" rx="1" />
								<rect x="14" y="4" width="4" height="16" rx="1" />
							</svg>
						</Button>
					)}
					{task.status === 'paused' && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							className="action-btn resume"
							onClick={(e) => handleAction('resume', e)}
							disabled={actionLoading}
							title="Resume task"
							aria-label="Resume task"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="12"
								height="12"
								viewBox="0 0 24 24"
								fill="currentColor"
								stroke="none"
							>
								<polygon points="5 3 19 12 5 21 5 3" />
							</svg>
						</Button>
					)}
					{task.status === 'completed' && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							className="action-btn finalize"
							onClick={handleFinalize}
							disabled={finalizeLoading}
							loading={finalizeLoading}
							title="Finalize and merge"
							aria-label="Finalize and merge"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="12"
								height="12"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<circle cx="18" cy="18" r="3" />
								<circle cx="6" cy="6" r="3" />
								<path d="M6 21V9a9 9 0 0 0 9 9" />
							</svg>
						</Button>
					)}

					{/* Quick menu for queue/priority */}
					<div className="quick-menu">
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							className="action-btn more"
							onClick={toggleQuickMenu}
							title="Quick actions"
							aria-label="Quick actions"
							aria-expanded={showQuickMenu}
							aria-haspopup="true"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="12"
								height="12"
								viewBox="0 0 24 24"
								fill="currentColor"
								stroke="none"
							>
								<circle cx="12" cy="5" r="2" />
								<circle cx="12" cy="12" r="2" />
								<circle cx="12" cy="19" r="2" />
							</svg>
						</Button>

						{showQuickMenu && (
							<>
								<div
									className="quick-menu-backdrop"
									onClick={closeQuickMenu}
									onKeyDown={(e) => e.key === 'Escape' && closeQuickMenu()}
								/>
								<div className="quick-menu-dropdown" role="menu">
									{quickMenuLoading ? (
										<div className="menu-loading">
											<div className="spinner" />
										</div>
									) : (
										<>
											{/* Queue section */}
											<div className="menu-section">
												<div className="menu-label">Queue</div>
												<Button
													variant="ghost"
													size="sm"
													className={`menu-item ${queue === 'active' ? 'selected' : ''}`}
													onClick={() => setQueueValue('active')}
													role="menuitem"
													leftIcon={
														<span className="menu-icon active-icon" />
													}
												>
													Active
												</Button>
												<Button
													variant="ghost"
													size="sm"
													className={`menu-item ${queue === 'backlog' ? 'selected' : ''}`}
													onClick={() => setQueueValue('backlog')}
													role="menuitem"
													leftIcon={
														<span className="menu-icon backlog-icon" />
													}
												>
													Backlog
												</Button>
											</div>

											<div className="menu-divider" />

											{/* Priority section */}
											<div className="menu-section">
												<div className="menu-label">Priority</div>
												<Button
													variant="ghost"
													size="sm"
													className={`menu-item ${priority === 'critical' ? 'selected' : ''}`}
													onClick={() => setPriorityValue('critical')}
													role="menuitem"
													leftIcon={
														<span
															className="menu-icon priority-icon"
															style={{
																background: 'var(--status-error)',
															}}
														/>
													}
												>
													Critical
												</Button>
												<Button
													variant="ghost"
													size="sm"
													className={`menu-item ${priority === 'high' ? 'selected' : ''}`}
													onClick={() => setPriorityValue('high')}
													role="menuitem"
													leftIcon={
														<span
															className="menu-icon priority-icon"
															style={{
																background: 'var(--status-warning)',
															}}
														/>
													}
												>
													High
												</Button>
												<Button
													variant="ghost"
													size="sm"
													className={`menu-item ${priority === 'normal' ? 'selected' : ''}`}
													onClick={() => setPriorityValue('normal')}
													role="menuitem"
													leftIcon={
														<span
															className="menu-icon priority-icon"
															style={{
																background: 'var(--text-muted)',
															}}
														/>
													}
												>
													Normal
												</Button>
												<Button
													variant="ghost"
													size="sm"
													className={`menu-item ${priority === 'low' ? 'selected' : ''}`}
													onClick={() => setPriorityValue('low')}
													role="menuitem"
													leftIcon={
														<span
															className="menu-icon priority-icon"
															style={{
																background: 'var(--text-disabled)',
															}}
														/>
													}
												>
													Low
												</Button>
											</div>
										</>
									)}
								</div>
							</>
						)}
					</div>
				</div>
			</div>
		</article>
	);
}
