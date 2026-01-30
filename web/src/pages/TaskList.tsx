/**
 * Task list page (/)
 *
 * Features:
 * - Filter by status (All, Active, Completed, Failed)
 * - Filter by initiative
 * - Filter by dependency status
 * - Filter by weight
 * - Search by ID/title with debounce
 * - Sort by recent/oldest/status
 * - Keyboard navigation (j/k/Enter/r/p/d)
 *
 * URL params:
 * - project: Project filter
 * - initiative: Initiative filter
 * - dependency_status: Dependency status filter
 */

import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
	useTaskStore,
	useCurrentProjectId,
	useCurrentInitiativeId,
	useCurrentInitiative,
	useInitiativeStore,
	useCurrentDependencyStatus,
	toast,
	UNASSIGNED_INITIATIVE,
} from '@/stores';
import { useTaskListShortcuts, useDocumentTitle } from '@/hooks';
import { TaskCard } from '@/components/board/TaskCard';
import { InitiativeDropdown } from '@/components/board/InitiativeDropdown';
import { DependencyDropdown } from '@/components/filters';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { taskClient } from '@/lib/client';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskWeight, DependencyStatus } from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import './TaskList.css';

// Status filter options
type StatusFilter = 'all' | 'active' | 'completed' | 'failed';

// Sort options
type SortBy = 'recent' | 'oldest' | 'status';

// Terminal statuses (not active anymore)
const TERMINAL_STATUSES = [TaskStatus.FINALIZING, TaskStatus.COMPLETED, TaskStatus.FAILED];
const DONE_STATUSES = [TaskStatus.COMPLETED];

// Status order for sorting
const STATUS_ORDER = [
	TaskStatus.RUNNING,
	TaskStatus.PAUSED,
	TaskStatus.BLOCKED,
	TaskStatus.PLANNED,
	TaskStatus.CREATED,
	TaskStatus.FINALIZING,
	TaskStatus.COMPLETED,
	TaskStatus.FAILED,
];

// Available weights
const WEIGHTS: TaskWeight[] = [TaskWeight.TRIVIAL, TaskWeight.SMALL, TaskWeight.MEDIUM, TaskWeight.LARGE];

// Weight display labels
const WEIGHT_LABELS: Record<TaskWeight, string> = {
	[TaskWeight.UNSPECIFIED]: 'Unspecified',
	[TaskWeight.TRIVIAL]: 'trivial',
	[TaskWeight.SMALL]: 'small',
	[TaskWeight.MEDIUM]: 'medium',
	[TaskWeight.LARGE]: 'large',
};

// Debounce hook
function useDebounce<T>(value: T, delay: number): T {
	const [debouncedValue, setDebouncedValue] = useState(value);

	useEffect(() => {
		const handler = setTimeout(() => {
			setDebouncedValue(value);
		}, delay);

		return () => {
			clearTimeout(handler);
		};
	}, [value, delay]);

	return debouncedValue;
}

export function TaskList() {
	// Set page title
	useDocumentTitle('Tasks');

	const navigate = useNavigate();
	const currentProjectId = useCurrentProjectId();
	const currentInitiativeId = useCurrentInitiativeId();
	const currentInitiative = useCurrentInitiative();
	const selectInitiative = useInitiativeStore((state) => state.selectInitiative);
	const currentDependencyStatus = useCurrentDependencyStatus();

	// Get tasks from store
	const tasks = useTaskStore((state) => state.tasks);
	const removeTask = useTaskStore((state) => state.removeTask);
	const loading = useTaskStore((state) => state.loading);
	// Note: error state available via useTaskStore((state) => state.error) if needed

	// Local filter state
	const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
	const [weightFilter, setWeightFilter] = useState<string>('all');
	const [sortBy, setSortBy] = useState<SortBy>('recent');
	const [searchQuery, setSearchQuery] = useState('');
	const [selectedIndex, setSelectedIndex] = useState(-1);

	// Debounced search query (300ms)
	const debouncedSearchQuery = useDebounce(searchQuery, 300);

	// Search input ref for keyboard shortcut
	const searchInputRef = useRef<HTMLInputElement>(null);

	// Filter tasks by initiative
	const initiativeFilteredTasks = useMemo(() => {
		if (!currentInitiativeId) return tasks;

		// Handle unassigned filter - show only tasks with no initiative
		if (currentInitiativeId === UNASSIGNED_INITIATIVE) {
			return tasks.filter((task) => !task.initiativeId);
		}

		// Get task IDs from the initiative
		if (!currentInitiative) return tasks;

		// If initiative exists but has no tasks, return empty array (not all tasks)
		const initiativeTasks = currentInitiative.tasks || [];
		const initiativeTaskIds = new Set(initiativeTasks.map((t) => t.id));
		return tasks.filter((task) => initiativeTaskIds.has(task.id));
	}, [tasks, currentInitiativeId, currentInitiative]);

	// Apply all filters
	const filteredTasks = useMemo(() => {
		let result = [...initiativeFilteredTasks];

		// Status filter
		if (statusFilter === 'active') {
			result = result.filter((t) => !TERMINAL_STATUSES.includes(t.status));
		} else if (statusFilter === 'completed') {
			result = result.filter((t) => DONE_STATUSES.includes(t.status));
		} else if (statusFilter === 'failed') {
			result = result.filter((t) => t.status === TaskStatus.FAILED);
		}

		// Dependency status filter
		if (currentDependencyStatus !== 'all') {
			// Map string filter to DependencyStatus enum
			const depStatusMap: Record<string, DependencyStatus> = {
				blocked: DependencyStatus.BLOCKED,
				ready: DependencyStatus.READY,
				none: DependencyStatus.NONE,
			};
			const targetDepStatus = depStatusMap[currentDependencyStatus];
			if (targetDepStatus !== undefined) {
				result = result.filter((t) => t.dependencyStatus === targetDepStatus);
			}
		}

		// Weight filter
		if (weightFilter !== 'all') {
			// Map string filter to TaskWeight enum
			const weightMap: Record<string, TaskWeight> = {
				trivial: TaskWeight.TRIVIAL,
				small: TaskWeight.SMALL,
				medium: TaskWeight.MEDIUM,
				large: TaskWeight.LARGE,
			};
			const targetWeight = weightMap[weightFilter];
			if (targetWeight !== undefined) {
				result = result.filter((t) => t.weight === targetWeight);
			}
		}

		// Search filter (debounced)
		if (debouncedSearchQuery.trim()) {
			const query = debouncedSearchQuery.toLowerCase();
			result = result.filter(
				(t) =>
					t.id.toLowerCase().includes(query) || t.title.toLowerCase().includes(query)
			);
		}

		// Sort
		if (sortBy === 'recent') {
			result.sort((a, b) => {
				const aTime = timestampToDate(a.updatedAt)?.getTime() ?? 0;
				const bTime = timestampToDate(b.updatedAt)?.getTime() ?? 0;
				return bTime - aTime;
			});
		} else if (sortBy === 'oldest') {
			result.sort((a, b) => {
				const aTime = timestampToDate(a.updatedAt)?.getTime() ?? 0;
				const bTime = timestampToDate(b.updatedAt)?.getTime() ?? 0;
				return aTime - bTime;
			});
		} else if (sortBy === 'status') {
			result.sort(
				(a, b) => STATUS_ORDER.indexOf(a.status) - STATUS_ORDER.indexOf(b.status)
			);
		}

		return result;
	}, [
		initiativeFilteredTasks,
		statusFilter,
		currentDependencyStatus,
		weightFilter,
		debouncedSearchQuery,
		sortBy,
	]);

	// Status counts for tabs
	const statusCounts = useMemo(
		() => ({
			all: initiativeFilteredTasks.length,
			active: initiativeFilteredTasks.filter((t) => !TERMINAL_STATUSES.includes(t.status))
				.length,
			completed: initiativeFilteredTasks.filter((t) => DONE_STATUSES.includes(t.status))
				.length,
			failed: initiativeFilteredTasks.filter((t) => t.status === TaskStatus.FAILED).length,
		}),
		[initiativeFilteredTasks]
	);

	// Reset selection when filtered list changes
	useEffect(() => {
		if (selectedIndex >= filteredTasks.length) {
			setSelectedIndex(Math.max(0, filteredTasks.length - 1));
		}
	}, [filteredTasks.length, selectedIndex]);

	// Get currently selected task
	const getSelectedTask = useCallback((): Task | null => {
		if (selectedIndex >= 0 && selectedIndex < filteredTasks.length) {
			return filteredTasks[selectedIndex];
		}
		return null;
	}, [selectedIndex, filteredTasks]);

	// Scroll selected task into view
	const scrollToSelected = useCallback(() => {
		const taskElements = document.querySelectorAll('.task-card-wrapper');
		if (taskElements[selectedIndex]) {
			taskElements[selectedIndex].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
		}
	}, [selectedIndex]);

	// Task actions
	const handleRunTask = useCallback(
		async (taskId: string) => {
			if (!currentProjectId) {
				toast.error('Please select a project first');
				return;
			}
			try {
				await taskClient.runTask({ projectId: currentProjectId, taskId });
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Failed to run task');
			}
		},
		[currentProjectId]
	);

	const handlePauseTask = useCallback(
		async (taskId: string) => {
			if (!currentProjectId) {
				toast.error('Please select a project first');
				return;
			}
			try {
				await taskClient.pauseTask({ projectId: currentProjectId, taskId });
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Failed to pause task');
			}
		},
		[currentProjectId]
	);

	const handleDeleteTask = useCallback(
		async (taskId: string) => {
			if (!currentProjectId) {
				toast.error('Please select a project first');
				return;
			}
			try {
				await taskClient.deleteTask({ projectId: currentProjectId, taskId });
				removeTask(taskId);
				toast.success(`Deleted task ${taskId}`);
			} catch (e) {
				toast.error(e instanceof Error ? e.message : 'Failed to delete task');
			}
		},
		[currentProjectId, removeTask]
	);

	// Keyboard shortcuts
	useTaskListShortcuts({
		onNavDown: () => {
			if (filteredTasks.length > 0) {
				setSelectedIndex((prev) => {
					const newIndex = Math.min(prev + 1, filteredTasks.length - 1);
					return newIndex;
				});
				// Scroll after state update
				setTimeout(scrollToSelected, 0);
			}
		},
		onNavUp: () => {
			if (selectedIndex > 0) {
				setSelectedIndex((prev) => prev - 1);
				setTimeout(scrollToSelected, 0);
			}
		},
		onOpen: () => {
			const task = getSelectedTask();
			if (task) {
				navigate(`/tasks/${task.id}`);
			}
		},
		onRun: () => {
			const task = getSelectedTask();
			if (task && task.status !== TaskStatus.RUNNING) {
				handleRunTask(task.id);
				toast.info(`Running task ${task.id}`);
			}
		},
		onPause: () => {
			const task = getSelectedTask();
			if (task && task.status === TaskStatus.RUNNING) {
				handlePauseTask(task.id);
				toast.info(`Paused task ${task.id}`);
			}
		},
		onDelete: () => {
			const task = getSelectedTask();
			if (task) {
				if (confirm(`Delete task ${task.id}?`)) {
					handleDeleteTask(task.id);
				}
			}
		},
	});

	// Focus search handler (for / keyboard shortcut)
	useEffect(() => {
		const handleFocusSearch = () => {
			if (searchInputRef.current) {
				searchInputRef.current.focus();
			}
		};

		window.addEventListener('orc:focus-search', handleFocusSearch);
		return () => {
			window.removeEventListener('orc:focus-search', handleFocusSearch);
		};
	}, []);

	// Clear filters
	const clearFilters = useCallback(() => {
		setSearchQuery('');
		setStatusFilter('all');
		setWeightFilter('all');
	}, []);

	// Handle initiative change
	const handleInitiativeChange = useCallback(
		(id: string | null) => {
			selectInitiative(id);
		},
		[selectInitiative]
	);

	// Initiative banner title
	const initiativeBannerTitle = useMemo(() => {
		if (currentInitiativeId === UNASSIGNED_INITIATIVE) {
			return 'Unassigned tasks';
		}
		return currentInitiative?.title ?? currentInitiativeId;
	}, [currentInitiativeId, currentInitiative]);

	return (
		<div className="page task-list-page">
			{/* Initiative Filter Banner */}
			{(currentInitiative || currentInitiativeId === UNASSIGNED_INITIATIVE) && (
				<div className="initiative-banner">
					<span className="banner-icon">
						<Icon name="folder" size={16} />
					</span>
					<span className="banner-text">
						{currentInitiativeId === UNASSIGNED_INITIATIVE ? (
							<>
								Showing: <strong>Unassigned tasks</strong>
							</>
						) : (
							<>
								Filtered by initiative: <strong>{initiativeBannerTitle}</strong>
							</>
						)}
					</span>
					<Button
						variant="ghost"
						size="sm"
						className="banner-clear"
						onClick={() => selectInitiative(null)}
					>
						Clear filter
					</Button>
				</div>
			)}

			{/* Filter Bar */}
			<div className="filter-bar">
				{/* Status Tabs */}
				<div className="status-tabs">
					<Button
						variant={statusFilter === 'all' ? 'primary' : 'ghost'}
						size="sm"
						className={`status-tab ${statusFilter === 'all' ? 'active' : ''}`}
						onClick={() => setStatusFilter('all')}
					>
						All
						<span className="tab-count">{statusCounts.all}</span>
					</Button>
					<Button
						variant={statusFilter === 'active' ? 'primary' : 'ghost'}
						size="sm"
						className={`status-tab ${statusFilter === 'active' ? 'active' : ''}`}
						onClick={() => setStatusFilter('active')}
					>
						Active
						<span className="tab-count">{statusCounts.active}</span>
					</Button>
					<Button
						variant={statusFilter === 'completed' ? 'primary' : 'ghost'}
						size="sm"
						className={`status-tab ${statusFilter === 'completed' ? 'active' : ''}`}
						onClick={() => setStatusFilter('completed')}
					>
						Completed
						<span className="tab-count">{statusCounts.completed}</span>
					</Button>
					<Button
						variant={statusFilter === 'failed' ? 'primary' : 'ghost'}
						size="sm"
						className={`status-tab ${statusFilter === 'failed' ? 'active' : ''}`}
						onClick={() => setStatusFilter('failed')}
					>
						Failed
						<span className="tab-count">{statusCounts.failed}</span>
					</Button>
				</div>

				{/* Filters Row */}
				<div className="filters-row">
					{/* Search */}
					<div className="search-input">
						<Icon name="search" size={14} />
						<input
							type="text"
							placeholder="Search tasks..."
							value={searchQuery}
							onChange={(e) => setSearchQuery(e.target.value)}
							ref={searchInputRef}
						/>
					</div>

					{/* Initiative Filter */}
					<InitiativeDropdown
						currentInitiativeId={currentInitiativeId}
						onSelect={handleInitiativeChange}
						tasks={tasks}
					/>

					{/* Dependency Filter */}
					<DependencyDropdown tasks={initiativeFilteredTasks} />

					{/* Weight Filter */}
					<select
						className="filter-select"
						value={weightFilter}
						onChange={(e) => setWeightFilter(e.target.value)}
						aria-label="Filter by weight"
					>
						<option value="all">All weights</option>
						{WEIGHTS.map((w) => (
							<option key={w} value={WEIGHT_LABELS[w]}>
								{WEIGHT_LABELS[w]}
							</option>
						))}
					</select>

					{/* Sort */}
					<select
						className="filter-select"
						value={sortBy}
						onChange={(e) => setSortBy(e.target.value as SortBy)}
						aria-label="Sort tasks by"
					>
						<option value="recent">Most recent</option>
						<option value="oldest">Oldest first</option>
						<option value="status">By status</option>
					</select>
				</div>
			</div>

			{/* Keyboard Hints */}
			{filteredTasks.length > 0 && selectedIndex >= 0 && (
				<div className="keyboard-hints">
					<span className="hint">
						<kbd>j</kbd>
						<kbd>k</kbd> navigate
					</span>
					<span className="hint">
						<kbd>Enter</kbd> open
					</span>
					<span className="hint">
						<kbd>r</kbd> run
					</span>
					<span className="hint">
						<kbd>p</kbd> pause
					</span>
					<span className="hint">
						<kbd>d</kbd> delete
					</span>
					<span className="hint">
						<kbd>?</kbd> all shortcuts
					</span>
				</div>
			)}

			{/* Task List */}
			{loading ? (
				<div className="loading-state">
					<div className="spinner" />
					<span>Loading tasks...</span>
				</div>
			) : filteredTasks.length === 0 ? (
				<div className="empty-state">
					{!currentProjectId ? (
						<>
							<div className="empty-icon">
								<Icon name="folder" size={48} />
							</div>
							<h3>No project selected</h3>
							<p>Select a project to view and manage tasks</p>
							<Button
								variant="primary"
								onClick={() =>
									window.dispatchEvent(new CustomEvent('orc:switch-project'))
								}
								leftIcon={<Icon name="folder" size={14} />}
							>
								Select Project
							</Button>
						</>
					) : initiativeFilteredTasks.length === 0 ? (
						<>
							<div className="empty-icon">
								<Icon name="tasks" size={48} />
							</div>
							<h3>No tasks yet</h3>
							<p>Create your first task to get started with orc</p>
							<Button
								variant="primary"
								onClick={() => window.dispatchEvent(new CustomEvent('orc:new-task'))}
								leftIcon={<Icon name="plus" size={14} />}
							>
								Create Task
							</Button>
						</>
					) : (
						<>
							<div className="empty-icon">
								<Icon name="search" size={48} />
							</div>
							<h3>No matching tasks</h3>
							<p>Try adjusting your filters or search query</p>
							<Button variant="secondary" onClick={clearFilters}>
								Clear filters
							</Button>
						</>
					)}
				</div>
			) : (
				<div className="task-list" role="list" aria-label="Task list">
					{filteredTasks.map((task, index) => (
						<div
							key={task.id}
							className={`task-card-wrapper ${index === selectedIndex ? 'selected' : ''}`}
							onClick={() => setSelectedIndex(index)}
							role="listitem"
							aria-current={index === selectedIndex ? 'true' : undefined}
						>
							<TaskCard
								task={task}
								onClick={() => navigate(`/tasks/${task.id}`)}
								isSelected={index === selectedIndex}
							/>
						</div>
					))}
				</div>
			)}
		</div>
	);
}
