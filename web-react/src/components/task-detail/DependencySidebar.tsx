import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { Icon } from '@/components/ui/Icon';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import {
	getTaskDependencies,
	addBlocker,
	removeBlocker,
	addRelated,
	removeRelated,
	listTasks,
} from '@/lib/api';
import { toast } from '@/stores/uiStore';
import type { Task } from '@/lib/types';
import type { DependencyGraph, DependencyInfo } from '@/lib/api';
import './DependencySidebar.css';

interface DependencySidebarProps {
	task: Task;
	collapsed: boolean;
	onToggle: () => void;
}

export function DependencySidebar({ task, collapsed, onToggle }: DependencySidebarProps) {
	const [deps, setDeps] = useState<DependencyGraph | null>(null);
	const [loading, setLoading] = useState(true);
	const [showAddBlocker, setShowAddBlocker] = useState(false);
	const [showAddRelated, setShowAddRelated] = useState(false);
	const [availableTasks, setAvailableTasks] = useState<Task[]>([]);
	const [addingDep, setAddingDep] = useState(false);

	// Load dependencies
	const loadDependencies = useCallback(async () => {
		setLoading(true);
		try {
			const data = await getTaskDependencies(task.id);
			setDeps(data);
		} catch (e) {
			console.error('Failed to load dependencies:', e);
		} finally {
			setLoading(false);
		}
	}, [task.id]);

	useEffect(() => {
		loadDependencies();
	}, [loadDependencies]);

	// Load available tasks for adding dependencies
	const loadAvailableTasks = useCallback(async () => {
		try {
			const result = await listTasks();
			const tasks = Array.isArray(result) ? result : result.tasks;
			// Filter out current task
			setAvailableTasks(tasks.filter((t) => t.id !== task.id));
		} catch (e) {
			console.error('Failed to load tasks:', e);
		}
	}, [task.id]);

	// Handle add blocker
	const handleAddBlocker = useCallback(async (blockerId: string) => {
		setAddingDep(true);
		try {
			await addBlocker(task.id, blockerId);
			await loadDependencies();
			setShowAddBlocker(false);
			toast.success('Blocker added');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to add blocker');
		} finally {
			setAddingDep(false);
		}
	}, [task.id, loadDependencies]);

	// Handle remove blocker
	const handleRemoveBlocker = useCallback(async (blockerId: string) => {
		try {
			await removeBlocker(task.id, blockerId);
			await loadDependencies();
			toast.success('Blocker removed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to remove blocker');
		}
	}, [task.id, loadDependencies]);

	// Handle add related
	const handleAddRelated = useCallback(async (relatedId: string) => {
		setAddingDep(true);
		try {
			await addRelated(task.id, relatedId);
			await loadDependencies();
			setShowAddRelated(false);
			toast.success('Related task added');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to add related task');
		} finally {
			setAddingDep(false);
		}
	}, [task.id, loadDependencies]);

	// Handle remove related
	const handleRemoveRelated = useCallback(async (relatedId: string) => {
		try {
			await removeRelated(task.id, relatedId);
			await loadDependencies();
			toast.success('Related task removed');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to remove related task');
		}
	}, [task.id, loadDependencies]);

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
				<button className="toggle-btn" onClick={onToggle} title="Show dependencies">
					<Icon name="panel-left-open" size={18} />
				</button>
			</aside>
		);
	}

	const blockedByIds = deps?.blocked_by.map((d) => d.id) ?? [];
	const relatedIds = deps?.related_to.map((d) => d.id) ?? [];

	return (
		<aside className="dependency-sidebar">
			<div className="sidebar-header">
				<h3>
					<Icon name="link" size={16} />
					Dependencies
				</h3>
				<button className="toggle-btn" onClick={onToggle} title="Hide dependencies">
					<Icon name="panel-left-close" size={18} />
				</button>
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
						items={deps?.blocked_by ?? []}
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
						items={deps?.related_to ?? []}
						emptyText="No related tasks"
						canRemove
						onRemove={handleRemoveRelated}
						onAdd={openAddRelated}
					/>

					{/* Referenced By Section (computed, read-only) */}
					<DependencySection
						title="Referenced By"
						items={deps?.referenced_by ?? []}
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
					<button className="add-dep-btn" onClick={onAdd} title={`Add ${title.toLowerCase()}`}>
						<Icon name="plus" size={14} />
					</button>
				)}
			</div>
			{items.length === 0 ? (
				<div className="dep-empty">{emptyText}</div>
			) : (
				<ul className="dep-list">
					{items.map((item) => (
						<li key={item.id} className="dep-item">
							<Link to={`/tasks/${item.id}`} className="dep-link">
								<StatusIndicator status={item.status as any} size="sm" />
								<span className="dep-id">{item.id}</span>
								<span className="dep-title">{item.title}</span>
							</Link>
							{canRemove && onRemove && (
								<button
									className="remove-dep-btn"
									onClick={() => onRemove(item.id)}
									title="Remove"
								>
									<Icon name="x" size={14} />
								</button>
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
					<button className="close-btn" onClick={onClose}>
						<Icon name="x" size={18} />
					</button>
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
							<button
								key={t.id}
								className="task-option"
								onClick={() => onSelect(t.id)}
								disabled={loading}
							>
								<StatusIndicator status={t.status} size="sm" />
								<span className="task-id">{t.id}</span>
								<span className="task-title">{t.title}</span>
							</button>
						))
					)}
				</div>
			</div>
		</div>
	);
}
