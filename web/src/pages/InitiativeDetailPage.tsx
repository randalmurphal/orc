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
	type InitiativeNote,
	InitiativeStatus,
} from '@/gen/orc/v1/initiative_pb';
import { type Task, TaskStatus, type DependencyGraph as DependencyGraphData } from '@/gen/orc/v1/task_pb';
import { initiativeClient, taskClient } from '@/lib/client';
import { timestampToDate } from '@/lib/time';
import { useInitiativeStore, useCurrentProjectId } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import {
	AddDecisionModal,
	ArchiveInitiativeModal,
	EditInitiativeModal,
	getInitiativeEmoji,
	InitiativeDecisionsSection,
	InitiativeDependencyGraphSection,
	InitiativeHeaderSection,
	InitiativeKnowledgeSection,
	InitiativeProgressSection,
	InitiativeStatsSection,
	InitiativeTasksSection,
	LinkTaskModal,
	stripLeadingEmoji,
	type TaskFilter,
} from '@/components/initiatives';
import './InitiativeDetailPage.css';

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

	// Knowledge (notes) state - collapsible and lazy loaded
	const [knowledgeExpanded, setKnowledgeExpanded] = useState(true);
	const [notes, setNotes] = useState<InitiativeNote[]>([]);
	const [notesLoading, setNotesLoading] = useState(false);
	const [notesLoaded, setNotesLoaded] = useState(false);

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

	// Group notes by type for display
	const notesByType = useMemo(() => {
		const grouped: Record<string, InitiativeNote[]> = {};
		for (const note of notes) {
			const noteType = note.noteType || 'other';
			if (!grouped[noteType]) {
				grouped[noteType] = [];
			}
			grouped[noteType].push(note);
		}
		return grouped;
	}, [notes]);

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

	const loadNotes = useCallback(async () => {
		if (!initiative || !projectId || notesLoaded) return;
		setNotesLoading(true);
		try {
			const response = await initiativeClient.listInitiativeNotes({
				projectId,
				initiativeId: initiative.id,
			});
			setNotes(response.notes || []);
			setNotesLoaded(true);
		} catch (e) {
			console.error('Failed to load notes:', e);
		} finally {
			setNotesLoading(false);
		}
	}, [initiative, projectId, notesLoaded]);

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

	// Load notes when initiative is loaded
	useEffect(() => {
		if (initiative && !notesLoaded && !notesLoading) {
			loadNotes();
		}
	}, [initiative, notesLoaded, notesLoading, loadNotes]);

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

	const emoji = getInitiativeEmoji(`${initiative.title} ${initiative.vision || ''}`, initiative.status);
	const titleWithoutEmoji = stripLeadingEmoji(initiative.title);

	return (
		<div className="page initiative-detail-page">
			<div className="initiative-detail">
				<Link to="/initiatives" className="back-link">
					<Icon name="arrow-left" size={16} />
					<span>Back to Initiatives</span>
				</Link>
				<InitiativeHeaderSection
					initiative={initiative}
					emoji={emoji}
					titleWithoutEmoji={titleWithoutEmoji}
					statusActionLoading={statusActionLoading}
					onActivate={handleActivate}
					onComplete={handleComplete}
					onEdit={openEditModal}
					onArchive={() => setConfirmArchiveOpen(true)}
				/>
				<InitiativeProgressSection progress={progress} />
				<InitiativeStatsSection progress={progress} totalCost={totalCost} formatCost={formatCost} />
				<InitiativeDecisionsSection
					initiative={initiative}
					formatDate={formatDate}
					onAddDecision={openAddDecisionModal}
				/>
				<InitiativeKnowledgeSection
					notes={notes}
					notesByType={notesByType}
					notesLoading={notesLoading}
					knowledgeExpanded={knowledgeExpanded}
					formatDate={formatDate}
					onToggle={() => setKnowledgeExpanded((prev) => !prev)}
				/>
				<InitiativeTasksSection
					tasks={filteredTasks}
					allTaskCount={initiative.tasks?.length ?? 0}
					taskFilter={taskFilter}
					onTaskFilterChange={setTaskFilter}
					onLinkTask={openLinkTaskModal}
					onUnlinkTask={unlinkTask}
				/>
				<InitiativeDependencyGraphSection
					graphExpanded={graphExpanded}
					graphLoading={graphLoading}
					graphError={graphError}
					graphData={graphData}
					onToggle={toggleGraph}
					onRetry={() => {
						setGraphData(null);
						loadGraphData();
					}}
				/>
			</div>

			<EditInitiativeModal
				open={editModalOpen}
				title={editTitle}
				vision={editVision}
				status={editStatus}
				branchBase={editBranchBase}
				branchPrefix={editBranchPrefix}
				onClose={() => setEditModalOpen(false)}
				onSave={saveEdit}
				setTitle={setEditTitle}
				setVision={setEditVision}
				setStatus={setEditStatus}
				setBranchBase={setEditBranchBase}
				setBranchPrefix={setEditBranchPrefix}
			/>
			<LinkTaskModal
				open={linkTaskModalOpen}
				search={linkTaskSearch}
				loading={linkTaskLoading}
				tasks={filteredAvailableTasks}
				onClose={() => setLinkTaskModalOpen(false)}
				onSearchChange={setLinkTaskSearch}
				onLinkTask={linkTask}
			/>
			<AddDecisionModal
				open={addDecisionModalOpen}
				decisionText={decisionText}
				decisionRationale={decisionRationale}
				decisionBy={decisionBy}
				addingDecision={addingDecision}
				onClose={() => setAddDecisionModalOpen(false)}
				onAddDecision={addDecision}
				setDecisionText={setDecisionText}
				setDecisionRationale={setDecisionRationale}
				setDecisionBy={setDecisionBy}
			/>
			<ArchiveInitiativeModal
				open={confirmArchiveOpen}
				title={initiative.title}
				loading={statusActionLoading}
				onClose={() => setConfirmArchiveOpen(false)}
				onArchive={handleArchive}
			/>
			{error && initiative && (
				<div className="error-toast">
					<span>{error}</span>
					<Button variant="ghost" iconOnly size="sm" onClick={() => setError(null)} aria-label="Dismiss">×</Button>
				</div>
			)}
		</div>
	);
}
