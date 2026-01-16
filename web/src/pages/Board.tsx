/**
 * Kanban board page (/board)
 *
 * Features:
 * - Flat and swimlane view modes
 * - Filter by initiative
 * - Task drag-drop for status changes
 * - Live transcript modal for running tasks
 * - Finalize modal for completed tasks
 *
 * URL params:
 * - project: Project filter
 * - initiative: Initiative filter
 * - dependency_status: Dependency status filter
 */

import { useState, useCallback, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
	useTaskStore,
	useInitiatives,
	useCurrentProjectId,
	useCurrentInitiativeId,
	useInitiativeStore,
	UNASSIGNED_INITIATIVE,
} from '@/stores';
import {
	Board as BoardComponent,
	ViewModeDropdown,
	InitiativeDropdown,
	type BoardViewMode,
} from '@/components/board';
import { Icon } from '@/components/ui/Icon';
import {
	runProjectTask,
	pauseProjectTask,
	resumeProjectTask,
	escalateProjectTask,
	updateTask,
	type FinalizeState,
} from '@/lib/api';
import type { Task, DependencyStatus } from '@/lib/types';
import './Board.css';

// LocalStorage key for view mode
const VIEW_MODE_STORAGE_KEY = 'orc-board-view-mode';

export function Board() {
	const [searchParams] = useSearchParams();
	const currentProjectId = useCurrentProjectId();
	const currentInitiativeId = useCurrentInitiativeId();
	const selectInitiative = useInitiativeStore((state) => state.selectInitiative);
	const dependencyStatus = searchParams.get('dependency_status') as DependencyStatus | null;

	// Get tasks and initiatives from stores (data loaded by DataProvider)
	const tasks = useTaskStore((state) => state.tasks);
	const updateTaskInStore = useTaskStore((state) => state.updateTask);
	const initiatives = useInitiatives();
	const loading = useTaskStore((state) => state.loading);
	const error = useTaskStore((state) => state.error);

	// UI state
	const [viewMode, setViewMode] = useState<BoardViewMode>(() => {
		if (typeof window === 'undefined') return 'flat';
		const stored = localStorage.getItem(VIEW_MODE_STORAGE_KEY);
		return stored === 'swimlane' ? 'swimlane' : 'flat';
	});

	// Modal states - setters used, getters for when modals are implemented
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_transcriptModalOpen, setTranscriptModalOpen] = useState(false);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_selectedTask, setSelectedTask] = useState<Task | null>(null);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_finalizeModalOpen, setFinalizeModalOpen] = useState(false);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_finalizeTask, setFinalizeTask] = useState<Task | null>(null);

	// Finalize states map - getter used by Board component via getFinalizeState
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [finalizeStates, _setFinalizeStates] = useState<Map<string, FinalizeState>>(new Map());

	// Swimlane view disabled when initiative filter is active in URL
	// Use URL param as source of truth, not store value (which includes localStorage-persisted state)
	// This ensures clean URL navigation (/board) has dropdown enabled
	const urlInitiativeFilter = searchParams.get('initiative');
	const swimlaneDisabled = urlInitiativeFilter !== null;

	// Filter tasks by initiative and dependency status
	const filteredTasks = useMemo(() => {
		let filtered = tasks;

		// Filter by initiative
		if (currentInitiativeId === UNASSIGNED_INITIATIVE) {
			filtered = filtered.filter((t) => !t.initiative_id);
		} else if (currentInitiativeId) {
			filtered = filtered.filter((t) => t.initiative_id === currentInitiativeId);
		}

		// Filter by dependency status
		if (dependencyStatus) {
			filtered = filtered.filter((t) => t.dependency_status === dependencyStatus);
		}

		return filtered;
	}, [tasks, currentInitiativeId, dependencyStatus]);

	// Handle view mode change
	const handleViewModeChange = useCallback((mode: BoardViewMode) => {
		setViewMode(mode);
		localStorage.setItem(VIEW_MODE_STORAGE_KEY, mode);
	}, []);

	// Handle initiative filter change
	const handleInitiativeChange = useCallback(
		(id: string | null) => {
			selectInitiative(id);
		},
		[selectInitiative]
	);

	// Handle task actions (run/pause/resume)
	const handleAction = useCallback(
		async (taskId: string, action: 'run' | 'pause' | 'resume') => {
			if (!currentProjectId) return;

			try {
				if (action === 'run') {
					await runProjectTask(currentProjectId, taskId);
				} else if (action === 'pause') {
					await pauseProjectTask(currentProjectId, taskId);
				} else if (action === 'resume') {
					await resumeProjectTask(currentProjectId, taskId);
				}
				// WebSocket will update the store
			} catch (err) {
				console.error(`Failed to ${action} task:`, err);
			}
		},
		[currentProjectId]
	);

	// Handle escalate
	const handleEscalate = useCallback(
		async (taskId: string, reason: string) => {
			if (!currentProjectId) return;

			try {
				await escalateProjectTask(currentProjectId, taskId, reason);
				// WebSocket will update the store
			} catch (err) {
				console.error('Failed to escalate task:', err);
			}
		},
		[currentProjectId]
	);

	// Handle task click (for running tasks, show transcript modal)
	const handleTaskClick = useCallback((task: Task) => {
		setSelectedTask(task);
		setTranscriptModalOpen(true);
	}, []);

	// Handle finalize click
	const handleFinalizeClick = useCallback((task: Task) => {
		setFinalizeTask(task);
		setFinalizeModalOpen(true);
	}, []);

	// Handle initiative click (from task card badge)
	const handleInitiativeClick = useCallback(
		(initiativeId: string) => {
			selectInitiative(initiativeId);
		},
		[selectInitiative]
	);

	// Handle initiative change via drag-drop
	const handleInitiativeChangeFromDrag = useCallback(
		async (taskId: string, initiativeId: string | null) => {
			try {
				const updated = await updateTask(taskId, { initiative_id: initiativeId ?? '' });
				updateTaskInStore(taskId, updated);
			} catch (err) {
				console.error('Failed to update task initiative:', err);
			}
		},
		[updateTaskInStore]
	);

	// Get finalize state for a task
	const getFinalizeState = useCallback(
		(taskId: string) => {
			return finalizeStates.get(taskId);
		},
		[finalizeStates]
	);

	// Clear initiative filter
	const clearInitiativeFilter = useCallback(() => {
		selectInitiative(null);
	}, [selectInitiative]);

	// Get initiative title for banner
	const getInitiativeTitle = useCallback(
		(id: string): string => {
			if (id === UNASSIGNED_INITIATIVE) return 'Unassigned';
			const init = initiatives.find((i) => i.id === id);
			return init?.title ?? id;
		},
		[initiatives]
	);

	// Loading state
	if (loading) {
		return (
			<div className="page board-page">
				<div className="loading-state">
					<div className="spinner" />
					<span>Loading board...</span>
				</div>
			</div>
		);
	}

	// Error state
	if (error) {
		return (
			<div className="page board-page">
				<div className="error-state">
					<Icon name="error" size={24} />
					<span>{error}</span>
				</div>
			</div>
		);
	}

	// No project selected
	if (!currentProjectId) {
		return (
			<div className="page board-page">
				<div className="empty-state">
					<Icon name="board" size={32} />
					<h3>No Project Selected</h3>
					<p>Select a project to view the board.</p>
				</div>
			</div>
		);
	}

	return (
		<div className="page board-page">
			<div className="page-header">
				<div className="header-left">
					<h2>Board</h2>
					<span className="task-count">{filteredTasks.length} {filteredTasks.length === 1 ? 'task' : 'tasks'}</span>
				</div>
				<div className="header-filters">
					<div className={swimlaneDisabled ? 'view-mode-disabled' : ''}>
						<ViewModeDropdown
							value={viewMode}
							onChange={handleViewModeChange}
							disabled={swimlaneDisabled}
						/>
					</div>
					<InitiativeDropdown
						currentInitiativeId={currentInitiativeId}
						onSelect={handleInitiativeChange}
						tasks={tasks}
					/>
				</div>
			</div>

			{/* Initiative filter banner */}
			{currentInitiativeId && (
				<div className="initiative-banner">
					<span className="banner-label">
						Filtered by:{' '}
						<strong>{getInitiativeTitle(currentInitiativeId)}</strong>
					</span>
					<button
						type="button"
						className="banner-clear"
						onClick={clearInitiativeFilter}
					>
						<Icon name="x" size={14} />
						Clear filter
					</button>
				</div>
			)}

			<BoardComponent
				tasks={filteredTasks}
				viewMode={swimlaneDisabled ? 'flat' : viewMode}
				initiatives={initiatives}
				onAction={handleAction}
				onEscalate={handleEscalate}
				onTaskClick={handleTaskClick}
				onFinalizeClick={handleFinalizeClick}
				onInitiativeClick={handleInitiativeClick}
				onInitiativeChange={handleInitiativeChangeFromDrag}
				getFinalizeState={getFinalizeState}
			/>

			{/* TODO: Add LiveTranscriptModal and FinalizeModal when those components exist */}
		</div>
	);
}
