/**
 * Initiative detail page (/initiatives/:id)
 *
 * Layout:
 * - Header with back link, title (with emoji), status badge, edit button
 * - Progress bar
 * - Stats row (3 cards: Total Tasks, Completed, Total Cost)
 * - Side-by-side: Stats summary | Decisions list
 * - Filterable task list
 * - Collapsible dependency graph
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import {
	type Initiative,
	InitiativeStatus,
} from '@/gen/orc/v1/initiative_pb';
import { type Task, TaskStatus, type DependencyGraph as DependencyGraphData } from '@/gen/orc/v1/task_pb';
import { initiativeClient, taskClient } from '@/lib/client';
import { timestampToDate } from '@/lib/time';
import { useInitiativeStore, useCurrentProjectId } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { Icon, type IconName } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/overlays/Modal';
import { DependencyGraph } from '@/components/initiative/DependencyGraph';
import './InitiativeDetailPage.css';

type TaskFilter = 'all' | 'completed' | 'running' | 'planned';

// Map TaskStatus enum to display string for task cards
function getTaskStatusDisplay(status: TaskStatus): string {
	switch (status) {
		case TaskStatus.COMPLETED:
			return 'completed';
		case TaskStatus.RUNNING:
			return 'running';
		case TaskStatus.BLOCKED:
			return 'blocked';
		case TaskStatus.PAUSED:
			return 'paused';
		case TaskStatus.FAILED:
			return 'failed';
		case TaskStatus.PLANNED:
			return 'planned';
		case TaskStatus.CREATED:
			return 'created';
		case TaskStatus.CLASSIFYING:
			return 'classifying';
		case TaskStatus.FINALIZING:
			return 'finalizing';
		case TaskStatus.RESOLVED:
			return 'resolved';
		default:
			return 'pending';
	}
}

// Map InitiativeStatus enum to display string
function getInitiativeStatusDisplay(status: InitiativeStatus): string {
	switch (status) {
		case InitiativeStatus.DRAFT:
			return 'draft';
		case InitiativeStatus.ACTIVE:
			return 'active';
		case InitiativeStatus.COMPLETED:
			return 'completed';
		case InitiativeStatus.ARCHIVED:
			return 'archived';
		default:
			return 'unknown';
	}
}

/**
 * Extract first emoji from text or return default based on status
 */
function extractEmoji(text: string, status?: InitiativeStatus): string {
	const emojiMatch = text.match(/^(\p{Emoji})/u);
	if (emojiMatch) return emojiMatch[1];

	// Default emojis by status
	switch (status) {
		case InitiativeStatus.ACTIVE:
			return '🚀';
		case InitiativeStatus.COMPLETED:
			return '✅';
		case InitiativeStatus.ARCHIVED:
			return '📦';
		default:
			return '📋';
	}
}

export function InitiativeDetailPage() {
	const { id } = useParams<{ id: string }>();
	const projectId = useCurrentProjectId();
	const updateInitiativeInStore = useInitiativeStore((state) => state.updateInitiative);

	const [initiative, setInitiative] = useState<Initiative | null>(null);

	// Set document title based on initiative
	useDocumentTitle(initiative?.title ?? id);

	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Task filter state
	const [taskFilter, setTaskFilter] = useState<TaskFilter>('all');

	// Graph state - collapsible and lazy loaded
	const [graphExpanded, setGraphExpanded] = useState(false);
	const [graphData, setGraphData] = useState<DependencyGraphData | null>(null);
	const [graphLoading, setGraphLoading] = useState(false);
	const [graphError, setGraphError] = useState<string | null>(null);

	// Modal states
	const [editModalOpen, setEditModalOpen] = useState(false);
	const [linkTaskModalOpen, setLinkTaskModalOpen] = useState(false);
	const [addDecisionModalOpen, setAddDecisionModalOpen] = useState(false);
	const [confirmArchiveOpen, setConfirmArchiveOpen] = useState(false);

	// Edit form state
	const [editTitle, setEditTitle] = useState('');
	const [editVision, setEditVision] = useState('');
	const [editStatus, setEditStatus] = useState<InitiativeStatus>(InitiativeStatus.DRAFT);
	const [editBranchBase, setEditBranchBase] = useState('');
	const [editBranchPrefix, setEditBranchPrefix] = useState('');

	// Link task state
	const [availableTasks, setAvailableTasks] = useState<Task[]>([]);
	const [linkTaskSearch, setLinkTaskSearch] = useState('');
	const [linkTaskLoading, setLinkTaskLoading] = useState(false);

	// Add decision state
	const [decisionText, setDecisionText] = useState('');
	const [decisionRationale, setDecisionRationale] = useState('');
	const [decisionBy, setDecisionBy] = useState('');
	const [addingDecision, setAddingDecision] = useState(false);

	// Status action states
	const [statusActionLoading, setStatusActionLoading] = useState(false);

	// Compute progress
	const progress = useMemo(() => {
		if (!initiative?.tasks || initiative.tasks.length === 0) {
			return { completed: 0, total: 0, percentage: 0 };
		}
		const completed = initiative.tasks.filter((t) => t.status === TaskStatus.COMPLETED).length;
		const total = initiative.tasks.length;
		return { completed, total, percentage: Math.round((completed / total) * 100) };
	}, [initiative?.tasks]);

	// Filter tasks by status
	const filteredTasks = useMemo(() => {
		if (!initiative?.tasks) return [];
		if (taskFilter === 'all') return initiative.tasks;

		return initiative.tasks.filter((task) => {
			switch (taskFilter) {
				case 'completed':
					return task.status === TaskStatus.COMPLETED;
				case 'running':
					return task.status === TaskStatus.RUNNING;
				case 'planned':
					return ![TaskStatus.COMPLETED, TaskStatus.RUNNING, TaskStatus.FAILED].includes(task.status);
				default:
					return true;
			}
		});
	}, [initiative?.tasks, taskFilter]);

	// Filter tasks for linking (not already in initiative)
	const filteredAvailableTasks = useMemo(() => {
		const existingIds = new Set(initiative?.tasks?.map((t) => t.id) || []);
		let filtered = availableTasks.filter((t) => !existingIds.has(t.id));
		if (linkTaskSearch) {
			const search = linkTaskSearch.toLowerCase();
			filtered = filtered.filter(
				(t) =>
					t.id.toLowerCase().includes(search) || t.title.toLowerCase().includes(search)
			);
		}
		return filtered;
	}, [availableTasks, initiative?.tasks, linkTaskSearch]);

	// Calculate total cost - placeholder value since InitiativeTaskRef doesn't have cost data
	// In a real implementation, this would come from the initiative or aggregated task data
	const totalCost = useMemo(() => {
		// Return 0 as placeholder - cost tracking not yet implemented in API
		return 0;
	}, []);

	const loadInitiative = useCallback(async () => {
		if (!id || !projectId) return;
		setLoading(true);
		setError(null);
		try {
			const response = await initiativeClient.getInitiative({ projectId, initiativeId: id });
			if (response.initiative) {
				setInitiative(response.initiative);
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load initiative');
		} finally {
			setLoading(false);
		}
	}, [projectId, id]);

	const loadGraphData = useCallback(async () => {
		if (!initiative || !projectId || graphData) return; // Don't reload if already loaded
		setGraphLoading(true);
		setGraphError(null);
		try {
			const response = await initiativeClient.getDependencyGraph({ projectId, initiativeId: initiative.id });
			if (response.graph) {
				// Store proto graph directly - DependencyGraph now uses proto types
				setGraphData(response.graph);
			}
		} catch (e) {
			setGraphError(e instanceof Error ? e.message : 'Failed to load dependency graph');
		} finally {
			setGraphLoading(false);
		}
	}, [initiative, projectId, graphData]);

	useEffect(() => {
		loadInitiative();
	}, [loadInitiative]);

	// Toggle graph expansion and load data on first expand
	const toggleGraph = useCallback(() => {
		setGraphExpanded((prev) => {
			const newExpanded = !prev;
			if (newExpanded && !graphData && !graphLoading) {
				loadGraphData();
			}
			return newExpanded;
		});
	}, [graphData, graphLoading, loadGraphData]);

	const openEditModal = useCallback(() => {
		if (initiative) {
			setEditTitle(initiative.title);
			setEditVision(initiative.vision || '');
			setEditStatus(initiative.status);
			setEditBranchBase(initiative.branchBase || '');
			setEditBranchPrefix(initiative.branchPrefix || '');
		}
		setEditModalOpen(true);
	}, [initiative]);

	const saveEdit = useCallback(async () => {
		if (!initiative || !projectId) return;
		try {
			const response = await initiativeClient.updateInitiative({
				projectId,
				initiativeId: initiative.id,
				title: editTitle,
				vision: editVision,
				status: editStatus,
				branchBase: editBranchBase.trim() || undefined,
				branchPrefix: editBranchPrefix.trim() || undefined,
			});
			if (response.initiative) {
				setInitiative(response.initiative);
				updateInitiativeInStore(response.initiative.id, response.initiative);
			}
			setEditModalOpen(false);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to update initiative');
		}
	}, [projectId, initiative, editTitle, editVision, editStatus, editBranchBase, editBranchPrefix, updateInitiativeInStore]);

	const handleStatusChange = useCallback(
		async (newStatus: InitiativeStatus) => {
			if (!initiative || !projectId) return;
			setStatusActionLoading(true);
			try {
				const response = await initiativeClient.updateInitiative({
					projectId,
					initiativeId: initiative.id,
					status: newStatus,
				});
				if (response.initiative) {
					setInitiative(response.initiative);
					updateInitiativeInStore(response.initiative.id, response.initiative);
				}
			} catch (e) {
				setError(e instanceof Error ? e.message : `Failed to update initiative status`);
			} finally {
				setStatusActionLoading(false);
			}
		},
		[projectId, initiative, updateInitiativeInStore]
	);

	const handleActivate = useCallback(() => handleStatusChange(InitiativeStatus.ACTIVE), [handleStatusChange]);
	const handleComplete = useCallback(
		() => handleStatusChange(InitiativeStatus.COMPLETED),
		[handleStatusChange]
	);
	const handleArchive = useCallback(() => {
		setConfirmArchiveOpen(false);
		handleStatusChange(InitiativeStatus.ARCHIVED);
	}, [handleStatusChange]);

	const openLinkTaskModal = useCallback(async () => {
		if (!projectId) return;
		setLinkTaskLoading(true);
		setLinkTaskSearch('');
		setLinkTaskModalOpen(true);
		try {
			const response = await taskClient.listTasks({ projectId });
			setAvailableTasks(response.tasks);
		} catch (e) {
			console.error('Failed to load tasks:', e);
			setAvailableTasks([]);
		} finally {
			setLinkTaskLoading(false);
		}
	}, [projectId]);

	const linkTask = useCallback(
		async (taskId: string) => {
			if (!initiative || !projectId) return;
			try {
				await initiativeClient.linkTasks({
					projectId,
					initiativeId: initiative.id,
					taskIds: [taskId],
				});
				await loadInitiative();
				setLinkTaskModalOpen(false);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to link task');
			}
		},
		[initiative, projectId, loadInitiative]
	);

	const unlinkTask = useCallback(
		async (taskId: string) => {
			if (!initiative || !projectId || !confirm(`Remove task ${taskId} from this initiative?`)) return;
			try {
				await initiativeClient.unlinkTask({
					projectId,
					initiativeId: initiative.id,
					taskId,
				});
				await loadInitiative();
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to remove task');
			}
		},
		[initiative, projectId, loadInitiative]
	);

	const openAddDecisionModal = useCallback(() => {
		setDecisionText('');
		setDecisionRationale('');
		setDecisionBy('');
		setAddDecisionModalOpen(true);
	}, []);

	const addDecision = useCallback(async () => {
		if (!initiative || !projectId || !decisionText.trim()) return;
		setAddingDecision(true);
		try {
			await initiativeClient.addDecision({
				projectId,
				initiativeId: initiative.id,
				decision: decisionText.trim(),
				rationale: decisionRationale.trim() || undefined,
				by: decisionBy.trim() || undefined,
			});
			await loadInitiative();
			setAddDecisionModalOpen(false);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to add decision');
		} finally {
			setAddingDecision(false);
		}
	}, [initiative, projectId, decisionText, decisionRationale, decisionBy, loadInitiative]);

	const getStatusIcon = useCallback((status: TaskStatus) => {
		switch (status) {
			case TaskStatus.COMPLETED:
				return 'check-circle';
			case TaskStatus.RUNNING:
				return 'play-circle';
			case TaskStatus.FAILED:
				return 'x-circle';
			case TaskStatus.PAUSED:
				return 'pause-circle';
			case TaskStatus.BLOCKED:
				return 'alert-circle';
			default:
				return 'circle';
		}
	}, []);

	const getStatusClass = useCallback((status: TaskStatus) => {
		switch (status) {
			case TaskStatus.COMPLETED:
				return 'status-success';
			case TaskStatus.RUNNING:
				return 'status-running';
			case TaskStatus.FAILED:
				return 'status-danger';
			case TaskStatus.BLOCKED:
			case TaskStatus.PAUSED:
				return 'status-warning';
			default:
				return 'status-pending';
		}
	}, []);

	const formatDate = useCallback((timestamp?: Timestamp) => {
		const date = timestampToDate(timestamp);
		if (!date) return 'Unknown date';
		return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}, []);

	const formatCost = useCallback((cost: number) => {
		return `$${cost.toFixed(2)}`;
	}, []);

	if (loading) {
		return (
			<div className="loading-state">
				<div className="spinner"></div>
				<span>Loading initiative...</span>
			</div>
		);
	}

	if (error && !initiative) {
		return (
			<div className="error-state">
				<div className="error-icon">!</div>
				<p>{error}</p>
				<Button variant="primary" onClick={loadInitiative}>
					Retry
				</Button>
			</div>
		);
	}

	if (!initiative) {
		return (
			<div className="error-state">
				<div className="error-icon">!</div>
				<p>Initiative not found</p>
				<Link to="/initiatives" className="btn btn-primary">
					Back to Initiatives
				</Link>
			</div>
		);
	}

	const emoji = extractEmoji(initiative.title + ' ' + (initiative.vision || ''), initiative.status);
	const titleWithoutEmoji = initiative.title.replace(/^(\p{Emoji})\s*/u, '');

	return (
		<div className="page initiative-detail-page">
			<div className="initiative-detail">
				{/* Back Link - navigates to initiatives list */}
				<Link to="/initiatives" className="back-link">
					<Icon name="arrow-left" size={16} />
					<span>Back to Initiatives</span>
				</Link>

				{/* Header Section */}
				<header className="initiative-header">
					<div className="header-top">
						<div className="title-row">
							<span className="initiative-emoji">{emoji}</span>
							<h1 className="initiative-title">{titleWithoutEmoji}</h1>
						</div>
						<div className="header-actions">
							<span className={`status-badge status-${getInitiativeStatusDisplay(initiative.status)}`}>
								{getInitiativeStatusDisplay(initiative.status)}
							</span>
							{/* Status transition buttons based on current status */}
							{initiative.status === InitiativeStatus.DRAFT && (
								<Button
									variant="primary"
									onClick={handleActivate}
									loading={statusActionLoading}
									leftIcon={<Icon name="play" size={16} />}
								>
									Activate
								</Button>
							)}
							{initiative.status === InitiativeStatus.ACTIVE && (
								<Button
									variant="success"
									onClick={handleComplete}
									loading={statusActionLoading}
									leftIcon={<Icon name="check" size={16} />}
								>
									Complete
								</Button>
							)}
							{initiative.status === InitiativeStatus.COMPLETED && (
								<Button
									variant="secondary"
									onClick={handleActivate}
									loading={statusActionLoading}
									leftIcon={<Icon name="rotate-ccw" size={16} />}
								>
									Reopen
								</Button>
							)}

							<Button variant="secondary" onClick={openEditModal} leftIcon={<Icon name="edit" size={16} />}>
								Edit
							</Button>

							{initiative.status !== InitiativeStatus.ARCHIVED && (
								<Button
									variant="ghost"
									className="btn-danger-hover"
									onClick={() => setConfirmArchiveOpen(true)}
									leftIcon={<Icon name="archive" size={16} />}
								>
									Archive
								</Button>
							)}
						</div>
					</div>

					{initiative.vision && (
						<p className="initiative-vision">{initiative.vision}</p>
					)}
				</header>

				{/* Progress Section */}
				<div className="progress-section">
					<div className="progress-label">
						<span>Progress</span>
						<span className="progress-count">
							{progress.completed}/{progress.total} tasks ({progress.percentage}%)
						</span>
					</div>
					<div className="progress-bar">
						<div
							className="progress-fill"
							style={{ width: `${progress.percentage}%` }}
						></div>
					</div>
				</div>

				{/* Stats Row */}
				<div className="stats-row">
					<div className="stat-card">
						<span className="stat-label">Total Tasks</span>
						<span className="stat-value">{progress.total}</span>
					</div>
					<div className="stat-card">
						<span className="stat-label">Completed</span>
						<span className="stat-value stat-success">{progress.completed}</span>
					</div>
					<div className="stat-card">
						<span className="stat-label">Total Cost</span>
						<span className="stat-value stat-primary">{formatCost(totalCost)}</span>
					</div>
				</div>

				{/* Side-by-Side: Decisions Section */}
				<section className="decisions-section">
					<div className="section-header">
						<h2>Decisions</h2>
						<Button
							variant="secondary"
							size="sm"
							onClick={openAddDecisionModal}
							leftIcon={<Icon name="plus" size={14} />}
						>
							Add Decision
						</Button>
					</div>

					{initiative.decisions && initiative.decisions.length > 0 ? (
						<div className="decision-list">
							{initiative.decisions.map((decision) => (
								<div key={decision.id} className="decision-item">
									<div className="decision-header">
										<span className="decision-date">
											{formatDate(decision.date)}
										</span>
										{decision.by && (
											<span className="decision-by">
												by {decision.by}
											</span>
										)}
									</div>
									<p className="decision-text">{decision.decision}</p>
									{decision.rationale && (
										<p className="decision-rationale">
											{decision.rationale}
										</p>
									)}
								</div>
							))}
						</div>
					) : (
						<div className="empty-state-inline">
							<span>No decisions recorded yet</span>
						</div>
					)}
				</section>

				{/* Tasks Section */}
				<section className="tasks-section">
					<div className="section-header">
						<h2>Tasks</h2>
						<div className="section-actions">
							<select
								className="filter-select"
								value={taskFilter}
								onChange={(e) => setTaskFilter(e.target.value as TaskFilter)}
								aria-label="Filter tasks"
							>
								<option value="all">All</option>
								<option value="completed">Completed</option>
								<option value="running">In Progress</option>
								<option value="planned">Planned</option>
							</select>
							<Button
								variant="secondary"
								size="sm"
								onClick={openLinkTaskModal}
								leftIcon={<Icon name="link" size={14} />}
							>
								Link Existing
							</Button>
						</div>
					</div>

					{filteredTasks.length > 0 ? (
						<div className="task-list">
							{filteredTasks.map((task) => (
								<div key={task.id} className="task-item">
									<Link
										to={`/tasks/${task.id}`}
										className="task-link"
									>
										<span
											className={`task-status ${getStatusClass(task.status)}`}
										>
											<Icon
												name={getStatusIcon(task.status) as IconName}
												size={16}
											/>
										</span>
										<span className="task-id">{task.id}</span>
										<span className="task-title">
											{task.title}
										</span>
										<span className="task-status-text">
											{getTaskStatusDisplay(task.status)}
										</span>
									</Link>
									<Button
										variant="ghost"
										iconOnly
										size="sm"
										className="btn-icon btn-remove"
										onClick={() => unlinkTask(task.id)}
										title="Remove from initiative"
										aria-label="Remove from initiative"
									>
										<Icon name="x" size={14} />
									</Button>
								</div>
							))}
						</div>
					) : initiative.tasks && initiative.tasks.length > 0 ? (
						<div className="empty-state-inline">
							<span>No tasks match the current filter</span>
						</div>
					) : (
						<div className="empty-state">
							<Icon name="clipboard" size={32} />
							<p>No tasks in this initiative yet</p>
							<Button variant="primary" onClick={openLinkTaskModal}>
								Link a Task
							</Button>
						</div>
					)}
				</section>

				{/* Dependency Graph Section - Collapsible */}
				<section className="graph-section">
					<div className="section-header section-header-collapsible">
						<h2>Dependency Graph</h2>
						<Button
							variant="ghost"
							size="sm"
							onClick={toggleGraph}
							aria-expanded={graphExpanded}
							leftIcon={<Icon name={graphExpanded ? 'chevron-up' : 'chevron-down'} size={16} />}
						>
							{graphExpanded ? 'Collapse' : 'Expand'}
						</Button>
					</div>

					{graphExpanded && (
						<div className="graph-content">
							{graphLoading ? (
								<div className="graph-loading">
									<div className="spinner"></div>
									<span>Loading graph...</span>
								</div>
							) : graphError ? (
								<div className="graph-error">
									<p>{graphError}</p>
									<Button
										variant="secondary"
										onClick={() => {
											setGraphData(null);
											loadGraphData();
										}}
									>
										Retry
									</Button>
								</div>
							) : graphData && graphData.nodes.length > 0 ? (
								<div className="graph-container-wrapper">
									<DependencyGraph
										nodes={graphData.nodes}
										edges={graphData.edges}
									/>
								</div>
							) : (
								<div className="empty-state-inline">
									<Icon name="git-branch" size={24} />
									<span>No tasks with dependencies to visualize</span>
								</div>
							)}
						</div>
					)}
				</section>
			</div>

			{/* Edit Initiative Modal */}
			<Modal
				open={editModalOpen}
				onClose={() => setEditModalOpen(false)}
				title="Edit Initiative"
			>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						saveEdit();
					}}
				>
					<div className="form-group">
						<label htmlFor="edit-title">Title</label>
						<input
							id="edit-title"
							type="text"
							value={editTitle}
							onChange={(e) => setEditTitle(e.target.value)}
							required
						/>
					</div>

					<div className="form-group">
						<label htmlFor="edit-vision">Vision</label>
						<textarea
							id="edit-vision"
							value={editVision}
							onChange={(e) => setEditVision(e.target.value)}
							rows={3}
							placeholder="What is the goal of this initiative?"
						></textarea>
					</div>

					<div className="form-group">
						<label htmlFor="edit-status">Status</label>
						<select
							id="edit-status"
							value={editStatus}
							onChange={(e) => setEditStatus(Number(e.target.value) as InitiativeStatus)}
						>
							<option value={InitiativeStatus.DRAFT}>Draft</option>
							<option value={InitiativeStatus.ACTIVE}>Active</option>
							<option value={InitiativeStatus.COMPLETED}>Completed</option>
							<option value={InitiativeStatus.ARCHIVED}>Archived</option>
						</select>
					</div>

					<div className="form-section-divider">
						<span className="divider-label">Branch Configuration</span>
					</div>

					<div className="form-group">
						<label htmlFor="edit-branch-base">Target Branch</label>
						<input
							id="edit-branch-base"
							type="text"
							value={editBranchBase}
							onChange={(e) => setEditBranchBase(e.target.value)}
							placeholder="e.g., feature/user-auth"
						/>
						<span className="form-hint">
							Tasks in this initiative will target this branch instead of main
						</span>
					</div>

					<div className="form-group">
						<label htmlFor="edit-branch-prefix">Task Branch Prefix</label>
						<input
							id="edit-branch-prefix"
							type="text"
							value={editBranchPrefix}
							onChange={(e) => setEditBranchPrefix(e.target.value)}
							placeholder="e.g., feature/auth-"
						/>
						<span className="form-hint">
							Task branches will be named: {editBranchPrefix || 'feature/auth-'}TASK-XXX
						</span>
					</div>

					<div className="modal-actions">
						<Button variant="secondary" onClick={() => setEditModalOpen(false)}>
							Cancel
						</Button>
						<Button variant="primary" type="submit">
							Save Changes
						</Button>
					</div>
				</form>
			</Modal>

			{/* Link Task Modal */}
			<Modal
				open={linkTaskModalOpen}
				onClose={() => setLinkTaskModalOpen(false)}
				title="Link Existing Task"
			>
				<div className="link-task-content">
					<div className="form-group">
						<label htmlFor="task-search">Search Tasks</label>
						<input
							id="task-search"
							type="text"
							value={linkTaskSearch}
							onChange={(e) => setLinkTaskSearch(e.target.value)}
							placeholder="Search by ID or title..."
						/>
					</div>

					{linkTaskLoading ? (
						<div className="loading-inline">
							<div className="spinner-sm"></div>
							<span>Loading tasks...</span>
						</div>
					) : filteredAvailableTasks.length > 0 ? (
						<div className="available-tasks">
							{filteredAvailableTasks.map((task) => (
								<Button
									key={task.id}
									variant="ghost"
									className="available-task-item"
									onClick={() => linkTask(task.id)}
								>
									<span className="task-id">{task.id}</span>
									<span className="task-title">{task.title}</span>
									<span className={`task-status-badge status-${getTaskStatusDisplay(task.status)}`}>
										{getTaskStatusDisplay(task.status)}
									</span>
								</Button>
							))}
						</div>
					) : (
						<p className="no-tasks-message">No available tasks to link</p>
					)}
				</div>
			</Modal>

			{/* Add Decision Modal */}
			<Modal
				open={addDecisionModalOpen}
				onClose={() => setAddDecisionModalOpen(false)}
				title="Add Decision"
			>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						addDecision();
					}}
				>
					<div className="form-group">
						<label htmlFor="decision-text">Decision</label>
						<textarea
							id="decision-text"
							value={decisionText}
							onChange={(e) => setDecisionText(e.target.value)}
							rows={2}
							required
							placeholder="What was decided?"
						></textarea>
					</div>

					<div className="form-group">
						<label htmlFor="decision-rationale">Rationale (optional)</label>
						<textarea
							id="decision-rationale"
							value={decisionRationale}
							onChange={(e) => setDecisionRationale(e.target.value)}
							rows={2}
							placeholder="Why was this decision made?"
						></textarea>
					</div>

					<div className="form-group">
						<label htmlFor="decision-by">Decided By (optional)</label>
						<input
							id="decision-by"
							type="text"
							value={decisionBy}
							onChange={(e) => setDecisionBy(e.target.value)}
							placeholder="Name or initials"
						/>
					</div>

					<div className="modal-actions">
						<Button variant="secondary" onClick={() => setAddDecisionModalOpen(false)}>
							Cancel
						</Button>
						<Button
							variant="primary"
							type="submit"
							disabled={!decisionText.trim()}
							loading={addingDecision}
						>
							Add Decision
						</Button>
					</div>
				</form>
			</Modal>

			{/* Archive Confirmation Modal */}
			<Modal
				open={confirmArchiveOpen}
				onClose={() => setConfirmArchiveOpen(false)}
				title="Archive Initiative"
			>
				<div className="confirm-dialog">
					<p className="confirm-message">
						Are you sure you want to archive <strong>"{initiative.title}"</strong>?
					</p>
					<p className="confirm-hint">
						Archived initiatives are hidden from most views but can be restored later.
					</p>
					<div className="modal-actions">
						<Button variant="secondary" onClick={() => setConfirmArchiveOpen(false)}>
							Cancel
						</Button>
						<Button
							variant="danger"
							onClick={handleArchive}
							loading={statusActionLoading}
						>
							Archive Initiative
						</Button>
					</div>
				</div>
			</Modal>

			{/* Error notification */}
			{error && initiative && (
				<div className="error-toast">
					<span>{error}</span>
					<Button variant="ghost" iconOnly size="sm" onClick={() => setError(null)} aria-label="Dismiss">×</Button>
				</div>
			)}
		</div>
	);
}
