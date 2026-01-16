/**
 * Initiative detail page (/initiatives/:id)
 *
 * Features:
 * - Header with title, status, progress bar
 * - Status management buttons (activate, complete, archive)
 * - Tab navigation (Tasks, Decisions, Graph)
 * - Task linking and unlinking
 * - Decision recording
 * - Dependency graph visualization
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
	getInitiative,
	updateInitiative,
	addInitiativeTask,
	removeInitiativeTask,
	addInitiativeDecision,
	listTasks,
	getInitiativeDependencyGraph,
	type DependencyGraphData,
	type AddInitiativeTaskRequest,
	type AddInitiativeDecisionRequest,
} from '@/lib/api';
import type { Initiative, InitiativeStatus, Task } from '@/lib/types';
import { useInitiativeStore } from '@/stores';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { DependencyGraph } from '@/components/initiative/DependencyGraph';
import './InitiativeDetail.css';

type Tab = 'tasks' | 'graph' | 'decisions';

export function InitiativeDetail() {
	const { id } = useParams<{ id: string }>();
	const navigate = useNavigate();
	const updateInitiativeInStore = useInitiativeStore((state) => state.updateInitiative);

	const [initiative, setInitiative] = useState<Initiative | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Active tab state
	const [activeTab, setActiveTab] = useState<Tab>('tasks');

	// Graph data
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
	const [editStatus, setEditStatus] = useState<InitiativeStatus>('draft');
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
		const completed = initiative.tasks.filter(
			(t) => t.status === 'completed' || t.status === 'finished'
		).length;
		const total = initiative.tasks.length;
		return { completed, total, percentage: Math.round((completed / total) * 100) };
	}, [initiative?.tasks]);

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

	// Task dependencies within initiative
	const taskDependencies = useMemo(() => {
		if (!initiative?.tasks) return [];
		const deps: { taskId: string; dependsOn: string[] }[] = [];
		for (const task of initiative.tasks) {
			if (task.depends_on && task.depends_on.length > 0) {
				deps.push({ taskId: task.id, dependsOn: task.depends_on });
			}
		}
		return deps;
	}, [initiative?.tasks]);

	const loadInitiative = useCallback(async () => {
		if (!id) return;
		setLoading(true);
		setError(null);
		try {
			const data = await getInitiative(id);
			setInitiative(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load initiative');
		} finally {
			setLoading(false);
		}
	}, [id]);

	const loadGraphData = useCallback(async () => {
		if (!initiative || graphData) return; // Don't reload if already loaded
		setGraphLoading(true);
		setGraphError(null);
		try {
			const data = await getInitiativeDependencyGraph(initiative.id);
			setGraphData(data);
		} catch (e) {
			setGraphError(e instanceof Error ? e.message : 'Failed to load dependency graph');
		} finally {
			setGraphLoading(false);
		}
	}, [initiative, graphData]);

	useEffect(() => {
		loadInitiative();
	}, [loadInitiative]);

	const handleTabChange = useCallback(
		(tab: Tab) => {
			setActiveTab(tab);
			if (tab === 'graph') {
				loadGraphData();
			}
		},
		[loadGraphData]
	);

	const openEditModal = useCallback(() => {
		if (initiative) {
			setEditTitle(initiative.title);
			setEditVision(initiative.vision || '');
			setEditStatus(initiative.status);
			setEditBranchBase(initiative.branch_base || '');
			setEditBranchPrefix(initiative.branch_prefix || '');
		}
		setEditModalOpen(true);
	}, [initiative]);

	const saveEdit = useCallback(async () => {
		if (!initiative) return;
		try {
			const updated = await updateInitiative(initiative.id, {
				title: editTitle,
				vision: editVision,
				status: editStatus,
				branch_base: editBranchBase.trim() || undefined,
				branch_prefix: editBranchPrefix.trim() || undefined,
			});
			setInitiative(updated);
			updateInitiativeInStore(updated.id, updated);
			setEditModalOpen(false);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to update initiative');
		}
	}, [initiative, editTitle, editVision, editStatus, editBranchBase, editBranchPrefix, updateInitiativeInStore]);

	const handleStatusChange = useCallback(
		async (newStatus: InitiativeStatus) => {
			if (!initiative) return;
			setStatusActionLoading(true);
			try {
				const updated = await updateInitiative(initiative.id, { status: newStatus });
				setInitiative(updated);
				updateInitiativeInStore(updated.id, updated);
			} catch (e) {
				setError(e instanceof Error ? e.message : `Failed to ${newStatus} initiative`);
			} finally {
				setStatusActionLoading(false);
			}
		},
		[initiative, updateInitiativeInStore]
	);

	const handleActivate = useCallback(() => handleStatusChange('active'), [handleStatusChange]);
	const handleComplete = useCallback(
		() => handleStatusChange('completed'),
		[handleStatusChange]
	);
	const handleArchive = useCallback(() => {
		setConfirmArchiveOpen(false);
		handleStatusChange('archived');
	}, [handleStatusChange]);

	const openLinkTaskModal = useCallback(async () => {
		setLinkTaskLoading(true);
		setLinkTaskSearch('');
		setLinkTaskModalOpen(true);
		try {
			const result = await listTasks();
			setAvailableTasks(Array.isArray(result) ? result : result.tasks);
		} catch (e) {
			console.error('Failed to load tasks:', e);
			setAvailableTasks([]);
		} finally {
			setLinkTaskLoading(false);
		}
	}, []);

	const linkTask = useCallback(
		async (taskId: string) => {
			if (!initiative) return;
			try {
				const req: AddInitiativeTaskRequest = { task_id: taskId };
				await addInitiativeTask(initiative.id, req);
				await loadInitiative();
				setLinkTaskModalOpen(false);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to link task');
			}
		},
		[initiative, loadInitiative]
	);

	const unlinkTask = useCallback(
		async (taskId: string) => {
			if (!initiative || !confirm(`Remove task ${taskId} from this initiative?`)) return;
			try {
				await removeInitiativeTask(initiative.id, taskId);
				await loadInitiative();
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to remove task');
			}
		},
		[initiative, loadInitiative]
	);

	const openAddDecisionModal = useCallback(() => {
		setDecisionText('');
		setDecisionRationale('');
		setDecisionBy('');
		setAddDecisionModalOpen(true);
	}, []);

	const addDecision = useCallback(async () => {
		if (!initiative || !decisionText.trim()) return;
		setAddingDecision(true);
		try {
			const req: AddInitiativeDecisionRequest = {
				decision: decisionText.trim(),
				rationale: decisionRationale.trim() || undefined,
				by: decisionBy.trim() || undefined,
			};
			await addInitiativeDecision(initiative.id, req);
			await loadInitiative();
			setAddDecisionModalOpen(false);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to add decision');
		} finally {
			setAddingDecision(false);
		}
	}, [initiative, decisionText, decisionRationale, decisionBy, loadInitiative]);

	const getStatusIcon = useCallback((status: string) => {
		switch (status) {
			case 'completed':
			case 'finished':
				return 'check-circle';
			case 'running':
				return 'play-circle';
			case 'failed':
				return 'x-circle';
			case 'paused':
				return 'pause-circle';
			case 'blocked':
				return 'alert-circle';
			default:
				return 'circle';
		}
	}, []);

	const getStatusClass = useCallback((status: string) => {
		switch (status) {
			case 'completed':
			case 'finished':
				return 'status-success';
			case 'running':
				return 'status-running';
			case 'failed':
				return 'status-danger';
			case 'blocked':
			case 'paused':
				return 'status-warning';
			default:
				return 'status-pending';
		}
	}, []);

	const formatDate = useCallback((dateStr: string) => {
		const date = new Date(dateStr);
		return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
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
				<button className="btn btn-primary" onClick={loadInitiative}>
					Retry
				</button>
			</div>
		);
	}

	if (!initiative) {
		return (
			<div className="error-state">
				<div className="error-icon">!</div>
				<p>Initiative not found</p>
				<Link to="/board" className="btn btn-primary">
					Back to Board
				</Link>
			</div>
		);
	}

	return (
		<div className="page initiative-detail-page">
			<div className="initiative-detail">
				{/* Back Link - navigates to board filtered by this initiative */}
				<Link to={`/board?initiative=${initiative.id}`} className="back-link">
					<Icon name="arrow-left" size={16} />
					<span>Back to Tasks</span>
				</Link>

				{/* Header Section */}
				<header className="initiative-header">
					<div className="header-top">
						<h1 className="initiative-title">{initiative.title}</h1>
						<div className="header-actions">
							{/* Status transition buttons based on current status */}
							{initiative.status === 'draft' && (
								<button
									className="btn btn-primary"
									onClick={handleActivate}
									disabled={statusActionLoading}
								>
									<Icon name="play" size={16} />
									{statusActionLoading ? 'Activating...' : 'Activate'}
								</button>
							)}
							{initiative.status === 'active' && (
								<button
									className="btn btn-success"
									onClick={handleComplete}
									disabled={statusActionLoading}
								>
									<Icon name="check" size={16} />
									{statusActionLoading ? 'Completing...' : 'Complete'}
								</button>
							)}
							{initiative.status === 'completed' && (
								<button
									className="btn btn-secondary"
									onClick={handleActivate}
									disabled={statusActionLoading}
								>
									<Icon name="rotate-ccw" size={16} />
									{statusActionLoading ? 'Reopening...' : 'Reopen'}
								</button>
							)}

							<button className="btn btn-secondary" onClick={openEditModal}>
								<Icon name="edit" size={16} />
								Edit
							</button>

							{initiative.status !== 'archived' && (
								<button
									className="btn btn-ghost btn-danger-hover"
									onClick={() => setConfirmArchiveOpen(true)}
								>
									<Icon name="archive" size={16} />
									Archive
								</button>
							)}
						</div>
					</div>

					{initiative.vision && (
						<p className="initiative-vision">{initiative.vision}</p>
					)}

					<div className="initiative-meta">
						{/* Progress Bar */}
						<div className="progress-section">
							<div className="progress-label">
								<span>Progress</span>
								<span className="progress-count">
									{progress.completed}/{progress.total} tasks ({progress.percentage}
									%)
								</span>
							</div>
							<div className="progress-bar">
								<div
									className="progress-fill"
									style={{ width: `${progress.percentage}%` }}
								></div>
							</div>
						</div>

						<div className="meta-grid">
							{initiative.owner?.initials && (
								<div className="meta-item">
									<span className="meta-label">Owner</span>
									<span className="meta-value">
										{initiative.owner.display_name || initiative.owner.initials}
									</span>
								</div>
							)}
							<div className="meta-item">
								<span className="meta-label">Status</span>
								<span className={`status-badge status-${initiative.status}`}>
									{initiative.status}
								</span>
							</div>
							<div className="meta-item">
								<span className="meta-label">Created</span>
								<span className="meta-value">
									{formatDate(initiative.created_at)}
								</span>
							</div>
						</div>
					</div>
				</header>

				{/* Tab Navigation */}
				<div className="tabs-nav" role="tablist" aria-label="Initiative sections">
					<button
						className={`tab-btn ${activeTab === 'tasks' ? 'active' : ''}`}
						onClick={() => handleTabChange('tasks')}
						role="tab"
						aria-selected={activeTab === 'tasks'}
					>
						<Icon name="list" size={16} />
						Tasks
						{initiative.tasks && initiative.tasks.length > 0 && (
							<span className="tab-count">{initiative.tasks.length}</span>
						)}
					</button>
					<button
						className={`tab-btn ${activeTab === 'graph' ? 'active' : ''}`}
						onClick={() => handleTabChange('graph')}
						role="tab"
						aria-selected={activeTab === 'graph'}
					>
						<Icon name="git-branch" size={16} />
						Graph
					</button>
					<button
						className={`tab-btn ${activeTab === 'decisions' ? 'active' : ''}`}
						onClick={() => handleTabChange('decisions')}
						role="tab"
						aria-selected={activeTab === 'decisions'}
					>
						<Icon name="message-circle" size={16} />
						Decisions
						{initiative.decisions && initiative.decisions.length > 0 && (
							<span className="tab-count">{initiative.decisions.length}</span>
						)}
					</button>
				</div>

				{/* Tab Content */}
				<div className="tab-content">
					{/* Tasks Tab */}
					{activeTab === 'tasks' && (
						<section className="section tasks-section">
							<div className="section-header">
								<h2>Tasks</h2>
								<div className="section-actions">
									<button
										className="btn btn-primary btn-sm"
										onClick={() => {
											navigate(`/?initiative=${initiative?.id}`);
											window.dispatchEvent(
												new CustomEvent('orc:new-task')
											);
										}}
									>
										<Icon name="plus" size={14} />
										Add Task
									</button>
									<button
										className="btn btn-secondary btn-sm"
										onClick={openLinkTaskModal}
									>
										<Icon name="link" size={14} />
										Link Existing
									</button>
								</div>
							</div>

							{initiative.tasks && initiative.tasks.length > 0 ? (
								<>
									<div className="task-list">
										{initiative.tasks.map((task) => (
											<div key={task.id} className="task-item">
												<Link
													to={`/tasks/${task.id}`}
													className="task-link"
												>
													<span
														className={`task-status ${getStatusClass(task.status)}`}
													>
														<Icon
															name={getStatusIcon(task.status) as any}
															size={16}
														/>
													</span>
													<span className="task-id">{task.id}</span>
													<span className="task-title">
														{task.title}
													</span>
													<span className="task-status-text">
														{task.status}
													</span>
												</Link>
												<button
													className="btn-icon btn-remove"
													onClick={() => unlinkTask(task.id)}
													title="Remove from initiative"
												>
													<Icon name="x" size={14} />
												</button>
											</div>
										))}
									</div>

									{/* Dependencies Section */}
									{taskDependencies.length > 0 && (
										<div className="dependencies-section">
											<h3>Dependencies</h3>
											<ul className="dependency-list">
												{taskDependencies.map((dep) => (
													<li key={dep.taskId}>
														<span className="dep-task">
															{dep.taskId}
														</span>
														<span className="dep-arrow">
															depends on
														</span>
														<span className="dep-targets">
															{dep.dependsOn.join(', ')}
														</span>
													</li>
												))}
											</ul>
										</div>
									)}
								</>
							) : (
								<div className="empty-state">
									<Icon name="clipboard" size={32} />
									<p>No tasks in this initiative yet</p>
									<button
										className="btn btn-primary"
										onClick={openLinkTaskModal}
									>
										Link a Task
									</button>
								</div>
							)}
						</section>
					)}

					{/* Graph Tab */}
					{activeTab === 'graph' && (
						<section className="section graph-section">
							<div className="section-header">
								<h2>Dependency Graph</h2>
							</div>

							{graphLoading ? (
								<div className="graph-loading">
									<div className="spinner"></div>
									<span>Loading graph...</span>
								</div>
							) : graphError ? (
								<div className="graph-error">
									<p>{graphError}</p>
									<button
										className="btn btn-secondary"
										onClick={() => {
											setGraphData(null);
											loadGraphData();
										}}
									>
										Retry
									</button>
								</div>
							) : graphData && graphData.nodes.length > 0 ? (
								<div className="graph-container-wrapper">
									<DependencyGraph
										nodes={graphData.nodes}
										edges={graphData.edges}
									/>
								</div>
							) : (
								<div className="empty-state">
									<Icon name="git-branch" size={32} />
									<p>No tasks with dependencies to visualize</p>
									<p className="empty-hint">
										Add tasks with dependencies to see the dependency graph
									</p>
								</div>
							)}
						</section>
					)}

					{/* Decisions Tab */}
					{activeTab === 'decisions' && (
						<section className="section decisions-section">
							<div className="section-header">
								<h2>Decisions</h2>
								<button
									className="btn btn-secondary btn-sm"
									onClick={openAddDecisionModal}
								>
									<Icon name="plus" size={14} />
									Add Decision
								</button>
							</div>

							{initiative.decisions && initiative.decisions.length > 0 ? (
								<div className="decision-list">
									{initiative.decisions.map((decision) => (
										<div key={decision.id} className="decision-item">
											<div className="decision-header">
												<span className="decision-id">{decision.id}</span>
												<span className="decision-date">
													({formatDate(decision.date)})
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
													<strong>Rationale:</strong>{' '}
													{decision.rationale}
												</p>
											)}
										</div>
									))}
								</div>
							) : (
								<div className="empty-state">
									<Icon name="message-circle" size={32} />
									<p>No decisions recorded yet</p>
									<button
										className="btn btn-secondary"
										onClick={openAddDecisionModal}
									>
										Record a Decision
									</button>
								</div>
							)}
						</section>
					)}
				</div>
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
							onChange={(e) => setEditStatus(e.target.value as InitiativeStatus)}
						>
							<option value="draft">Draft</option>
							<option value="active">Active</option>
							<option value="completed">Completed</option>
							<option value="archived">Archived</option>
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
						<button
							type="button"
							className="btn btn-secondary"
							onClick={() => setEditModalOpen(false)}
						>
							Cancel
						</button>
						<button type="submit" className="btn btn-primary">
							Save Changes
						</button>
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
								<button
									key={task.id}
									className="available-task-item"
									onClick={() => linkTask(task.id)}
								>
									<span className="task-id">{task.id}</span>
									<span className="task-title">{task.title}</span>
									<span className={`task-status-badge status-${task.status}`}>
										{task.status}
									</span>
								</button>
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
						<button
							type="button"
							className="btn btn-secondary"
							onClick={() => setAddDecisionModalOpen(false)}
						>
							Cancel
						</button>
						<button
							type="submit"
							className="btn btn-primary"
							disabled={addingDecision || !decisionText.trim()}
						>
							{addingDecision ? 'Adding...' : 'Add Decision'}
						</button>
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
						<button
							type="button"
							className="btn btn-secondary"
							onClick={() => setConfirmArchiveOpen(false)}
						>
							Cancel
						</button>
						<button
							type="button"
							className="btn btn-danger"
							onClick={handleArchive}
							disabled={statusActionLoading}
						>
							{statusActionLoading ? 'Archiving...' : 'Archive Initiative'}
						</button>
					</div>
				</div>
			</Modal>

			{/* Error notification */}
			{error && initiative && (
				<div className="error-toast">
					<span>{error}</span>
					<button onClick={() => setError(null)}>×</button>
				</div>
			)}
		</div>
	);
}
