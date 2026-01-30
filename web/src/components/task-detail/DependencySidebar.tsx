import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import { taskClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import type { Task, DependencyGraph } from '@/gen/orc/v1/task_pb';
import {
	TaskStatus,
	ListTasksRequestSchema,
	GetDependenciesRequestSchema,
	AddBlockerRequestSchema,
	RemoveBlockerRequestSchema,
	AddRelatedRequestSchema,
	RemoveRelatedRequestSchema,
} from '@/gen/orc/v1/task_pb';
import './DependencySidebar.css';

// Transformed dependency data for display
interface DependencyInfo {
	id: string;
	title: string;
	status: TaskStatus;
}

interface DependencyData {
	blockedBy: DependencyInfo[];
	blocks: DependencyInfo[];
	relatedTo: DependencyInfo[];
	referencedBy: DependencyInfo[];
}

// Transform graph response to display format
function transformGraphToData(graph: DependencyGraph | undefined, taskId: string): DependencyData {
	if (!graph) {
		return { blockedBy: [], blocks: [], relatedTo: [], referencedBy: [] };
	}

	const nodeMap = new Map(graph.nodes.map(n => [n.id, n]));
	const blockedBy: DependencyInfo[] = [];
	const blocks: DependencyInfo[] = [];
	const relatedTo: DependencyInfo[] = [];
	const referencedBy: DependencyInfo[] = [];

	for (const edge of graph.edges) {
		if (edge.type === 'blocks') {
			if (edge.to === taskId) {
				// This task is blocked by edge.from
				const node = nodeMap.get(edge.from);
				if (node) {
					blockedBy.push({ id: node.id, title: node.title, status: node.status });
				}
			} else if (edge.from === taskId) {
				// This task blocks edge.to
				const node = nodeMap.get(edge.to);
				if (node) {
					blocks.push({ id: node.id, title: node.title, status: node.status });
				}
			}
		} else if (edge.type === 'related') {
			if (edge.from === taskId) {
				const node = nodeMap.get(edge.to);
				if (node) {
					relatedTo.push({ id: node.id, title: node.title, status: node.status });
				}
			} else if (edge.to === taskId) {
				const node = nodeMap.get(edge.from);
				if (node) {
					referencedBy.push({ id: node.id, title: node.title, status: node.status });
				}
			}
		}
	}

	return { blockedBy, blocks, relatedTo, referencedBy };
}

interface DependencySidebarProps {
	task: Task;
	collapsed: boolean;
	onToggle: () => void;
}

export function DependencySidebar({ task, collapsed, onToggle }: DependencySidebarProps) {
	const projectId = useCurrentProjectId();
	const [deps, setDeps] = useState<DependencyData | null>(null);
	const [loading, setLoading] = useState(true);
	const [showAddBlocker, setShowAddBlocker] = useState(false);
	const [showAddRelated, setShowAddRelated] = useState(false);
	const [availableTasks, setAvailableTasks] = useState<Task[]>([]);
	const [addingDep, setAddingDep] = useState(false);

	// Load dependencies using Connect RPC
	const loadDependencies = useCallback(async () => {
		if (!projectId) return;
		setLoading(true);
		try {
			const response = await taskClient.getDependencies(
				create(GetDependenciesRequestSchema, { projectId, taskId: task.id, transitive: true })
			);
			const data = transformGraphToData(response.graph, task.id);
			setDeps(data);
		} catch (e) {
			console.error('Failed to load dependencies:', e);
		} finally {
			setLoading(false);
		}
	}, [projectId, task.id]);

	useEffect(() => {
		loadDependencies();
	}, [loadDependencies]);

	// Load available tasks for adding dependencies (using Connect)
	const loadAvailableTasks = useCallback(async () => {
		if (!projectId) return;
		try {
			const response = await taskClient.listTasks(
				create(ListTasksRequestSchema, { projectId })
			);
			// Filter out current task
			setAvailableTasks(response.tasks.filter((t) => t.id !== task.id));
		} catch (e) {
			console.error('Failed to load tasks:', e);
		}
	}, [projectId, task.id]);

	// Handle add blocker
	const handleAddBlocker = useCallback(async (blockerId: string) => {
		if (!projectId) return;
		setAddingDep(true);
		try {
			await taskClient.addBlocker(
				create(AddBlockerRequestSchema, { projectId, taskId: task.id, blockerId })
			);
			await loadDependencies();
			setShowAddBlocker(false);
			toast.success('Blocker added');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to add blocker');
		} finally {
			setAddingDep(false);
		}
	}, [projectId, task.id, loadDependencies]);

	// Handle remove blocker
	const handleRemoveBlocker = useCallback(async (blockerId: string) => {
		if (!projectId) return;
		try {
			await taskClient.removeBlocker(
				create(RemoveBlockerRequestSchema, { projectId, taskId: task.id, blockerId })
			);
			await loadDependencies();
			toast.success('Blocker removed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to remove blocker');
		}
	}, [projectId, task.id, loadDependencies]);

	// Handle add related
	const handleAddRelated = useCallback(async (relatedId: string) => {
		if (!projectId) return;
		setAddingDep(true);
		try {
			await taskClient.addRelated(
				create(AddRelatedRequestSchema, { projectId, taskId: task.id, relatedId })
			);
			await loadDependencies();
			setShowAddRelated(false);
			toast.success('Related task added');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to add related task');
		} finally {
			setAddingDep(false);
		}
	}, [projectId, task.id, loadDependencies]);

	// Handle remove related
	const handleRemoveRelated = useCallback(async (relatedId: string) => {
		if (!projectId) return;
		try {
			await taskClient.removeRelated(
				create(RemoveRelatedRequestSchema, { projectId, taskId: task.id, relatedId })
			);
			await loadDependencies();
			toast.success('Related task removed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to remove related task');
		}
	}, [projectId, task.id, loadDependencies]);

	// Open add modal and load tasks
	const openAddBlocker = useCallback(() => {
		loadAvailableTasks();
		setShowAddBlocker(true);
	}, [loadAvailableTasks]);

	const openAddRelated = useCallback(() => {
		loadAvailableTasks();
		setShowAddRelated(true);
	}, [loadAvailableTasks]);

	// Filter out already-linked tasks
	const getFilteredTasks = useCallback((exclude: string[]) => {
		return availableTasks.filter((t) => !exclude.includes(t.id));
	}, [availableTasks]);

	if (collapsed) {
		return (
			<aside className="dependency-sidebar collapsed">
				<Button variant="ghost" iconOnly onClick={onToggle} title="Show dependencies" aria-label="Show dependencies" className="toggle-btn">
					<Icon name="panel-left-open" size={18} />
				</Button>
			</aside>
		);
	}

	const blockedByIds = deps?.blockedBy?.map((d) => d.id) ?? [];
	const relatedIds = deps?.relatedTo?.map((d) => d.id) ?? [];

	return (
		<aside className="dependency-sidebar">
			<div className="sidebar-header">
				<h3>
					<Icon name="link" size={16} />
					Dependencies
				</h3>
				<Button variant="ghost" iconOnly onClick={onToggle} title="Hide dependencies" aria-label="Hide dependencies" className="toggle-btn">
					<Icon name="panel-left-close" size={18} />
				</Button>
			</div>

			{loading ? (
				<div className="sidebar-loading">
					<div className="loading-spinner" />
				</div>
			) : (
				<div className="sidebar-content">
					{/* Blocked By Section */}
					<DependencySection
						title="Blocked By"
						items={deps?.blockedBy ?? []}
						emptyText="No blockers"
						canRemove
						onRemove={handleRemoveBlocker}
						onAdd={openAddBlocker}
					/>

					{/* Blocks Section (computed, read-only) */}
					<DependencySection
						title="Blocks"
						items={deps?.blocks ?? []}
						emptyText="Doesn't block any tasks"
						readonly
					/>

					{/* Related To Section */}
					<DependencySection
						title="Related To"
						items={deps?.relatedTo ?? []}
						emptyText="No related tasks"
						canRemove
						onRemove={handleRemoveRelated}
						onAdd={openAddRelated}
					/>

					{/* Referenced By Section (computed, read-only) */}
					<DependencySection
						title="Referenced By"
						items={deps?.referencedBy ?? []}
						emptyText="Not referenced"
						readonly
					/>
				</div>
			)}

			{/* Add Blocker Modal */}
			{showAddBlocker && (
				<AddDependencyModal
					title="Add Blocker"
					tasks={getFilteredTasks([...blockedByIds, task.id])}
					onSelect={handleAddBlocker}
					onClose={() => setShowAddBlocker(false)}
					loading={addingDep}
				/>
			)}

			{/* Add Related Modal */}
			{showAddRelated && (
				<AddDependencyModal
					title="Add Related Task"
					tasks={getFilteredTasks([...relatedIds, task.id])}
					onSelect={handleAddRelated}
					onClose={() => setShowAddRelated(false)}
					loading={addingDep}
				/>
			)}
		</aside>
	);
}

// Dependency Section Component
interface DependencySectionProps {
	title: string;
	items: DependencyInfo[];
	emptyText: string;
	readonly?: boolean;
	canRemove?: boolean;
	onRemove?: (id: string) => void;
	onAdd?: () => void;
}

function DependencySection({
	title,
	items,
	emptyText,
	readonly,
	canRemove,
	onRemove,
	onAdd,
}: DependencySectionProps) {
	return (
		<div className="dep-section">
			<div className="dep-section-header">
				<span className="dep-section-title">{title}</span>
				{!readonly && onAdd && (
					<Button variant="ghost" iconOnly size="sm" onClick={onAdd} title={`Add ${title.toLowerCase()}`} aria-label={`Add ${title.toLowerCase()}`} className="add-dep-btn">
						<Icon name="plus" size={14} />
					</Button>
				)}
			</div>
			{items.length === 0 ? (
				<div className="dep-empty">{emptyText}</div>
			) : (
				<ul className="dep-list">
					{items.map((item) => (
						<li key={item.id} className="dep-item">
							<Link to={`/tasks/${item.id}`} className="dep-link">
								<StatusIndicator status={item.status} size="sm" />
								<span className="dep-id">{item.id}</span>
								<span className="dep-title">{item.title}</span>
							</Link>
							{canRemove && onRemove && (
								<Button
									variant="ghost"
									iconOnly
									size="sm"
									className="remove-dep-btn"
									onClick={() => onRemove(item.id)}
									title="Remove"
									aria-label="Remove dependency"
								>
									<Icon name="x" size={14} />
								</Button>
							)}
						</li>
					))}
				</ul>
			)}
		</div>
	);
}

// Add Dependency Modal
interface AddDependencyModalProps {
	title: string;
	tasks: Task[];
	onSelect: (id: string) => void;
	onClose: () => void;
	loading: boolean;
}

function AddDependencyModal({ title, tasks, onSelect, onClose, loading }: AddDependencyModalProps) {
	const [search, setSearch] = useState('');

	const filteredTasks = tasks.filter(
		(t) =>
			t.id.toLowerCase().includes(search.toLowerCase()) ||
			t.title.toLowerCase().includes(search.toLowerCase())
	);

	return (
		<div className="modal-overlay" onClick={onClose}>
			<div className="add-dep-modal" onClick={(e) => e.stopPropagation()}>
				<div className="modal-header">
					<h4>{title}</h4>
					<Button variant="ghost" iconOnly onClick={onClose} aria-label="Close" className="close-btn">
						<Icon name="x" size={18} />
					</Button>
				</div>
				<div className="modal-search">
					<Icon name="search" size={16} />
					<input
						type="text"
						placeholder="Search tasks..."
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						autoFocus
					/>
				</div>
				<div className="modal-list">
					{filteredTasks.length === 0 ? (
						<div className="modal-empty">No matching tasks</div>
					) : (
						filteredTasks.map((t) => (
							<Button
								key={t.id}
								variant="ghost"
								className="task-option"
								onClick={() => onSelect(t.id)}
								disabled={loading}
							>
								<StatusIndicator status={t.status} size="sm" />
								<span className="task-id">{t.id}</span>
								<span className="task-title">{t.title}</span>
							</Button>
						))
					)}
				</div>
			</div>
		</div>
	);
}
