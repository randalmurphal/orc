<script lang="ts">
	import { get } from 'svelte/store';
	import Board from '$lib/components/kanban/Board.svelte';
	import LiveTranscriptModal from '$lib/components/overlays/LiveTranscriptModal.svelte';
	import { runProjectTask, pauseProjectTask, resumeProjectTask, escalateProjectTask } from '$lib/api';
	import { currentProjectId } from '$lib/stores/project';
	import { tasks as tasksStore, tasksLoading, tasksError, loadTasks } from '$lib/stores/tasks';
	import { currentInitiativeId, currentInitiative, selectInitiative, UNASSIGNED_INITIATIVE } from '$lib/stores/initiative';
	import InitiativeDropdown from '$lib/components/filters/InitiativeDropdown.svelte';
	import type { Task } from '$lib/types';

	// Reactive binding to global task store
	let allTasks = $derived($tasksStore);
	let loading = $derived($tasksLoading);
	let error = $derived($tasksError);

	// Filter tasks by initiative if one is selected
	let tasks = $derived.by(() => {
		const initiativeId = $currentInitiativeId;
		if (!initiativeId) return allTasks;

		// Handle unassigned filter - show only tasks with no initiative
		if (initiativeId === UNASSIGNED_INITIATIVE) {
			return allTasks.filter(task => !task.initiative_id);
		}

		// Get task IDs from the initiative
		const initiative = $currentInitiative;
		if (!initiative) return allTasks; // Initiative not found/loaded yet

		// If initiative exists but has no tasks, return empty array (not all tasks)
		const initTasks = initiative.tasks || [];
		const initiativeTaskIds = new Set(initTasks.map(t => t.id));
		return allTasks.filter(task => initiativeTaskIds.has(task.id));
	});

	// Transcript modal state
	let transcriptModalOpen = $state(false);
	let selectedTask = $state<Task | null>(null);

	function handleTaskClick(task: Task) {
		selectedTask = task;
		transcriptModalOpen = true;
	}

	function closeTranscriptModal() {
		transcriptModalOpen = false;
		selectedTask = null;
	}

	async function handleAction(taskId: string, action: 'run' | 'pause' | 'resume') {
		const projectId = get(currentProjectId);
		if (!projectId) return;
		try {
			if (action === 'run') await runProjectTask(projectId, taskId);
			else if (action === 'pause') await pauseProjectTask(projectId, taskId);
			else if (action === 'resume') await resumeProjectTask(projectId, taskId);
			// No need to reload - WebSocket will update the global store
		} catch (e) {
			console.error('Action failed:', e);
		}
	}

	async function handleEscalate(taskId: string, reason: string) {
		const projectId = get(currentProjectId);
		if (!projectId) return;
		try {
			await escalateProjectTask(projectId, taskId, reason);
			// No need to reload - WebSocket will update the global store
		} catch (e) {
			console.error('Escalation failed:', e);
		}
	}
</script>

<svelte:head>
	<title>Board - orc</title>
</svelte:head>

<div class="board-page full-width">
	<header class="page-header">
		<div class="header-left">
			<h1>Task Board</h1>
			<span class="task-count">{tasks.length} tasks</span>
		</div>
		<div class="header-filters">
			<!-- Initiative Filter -->
			<InitiativeDropdown />

			<!-- New Task Button -->
			<button class="new-task-btn" onclick={() => window.dispatchEvent(new CustomEvent('orc:new-task'))}>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<line x1="12" y1="5" x2="12" y2="19" />
					<line x1="5" y1="12" x2="19" y2="12" />
				</svg>
				New Task
			</button>
		</div>
	</header>

	<!-- Initiative Filter Banner -->
	{#if $currentInitiative || $currentInitiativeId === UNASSIGNED_INITIATIVE}
		<div class="initiative-banner">
			<span class="banner-text">
				{#if $currentInitiativeId === UNASSIGNED_INITIATIVE}
					Showing: <strong>Unassigned tasks</strong>
				{:else}
					Filtered by: <strong>{$currentInitiative?.title}</strong>
				{/if}
			</span>
			<button class="banner-clear" onclick={() => selectInitiative(null)}>
				Clear filter
			</button>
		</div>
	{/if}

	{#if loading}
		<div class="loading-state">
			<div class="loading-spinner"></div>
			<span>Loading tasks...</span>
		</div>
	{:else if error}
		<div class="error-state">
			<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
				<circle cx="12" cy="12" r="10" />
				<line x1="12" y1="8" x2="12" y2="12" />
				<line x1="12" y1="16" x2="12.01" y2="16" />
			</svg>
			<span>{error}</span>
			<button onclick={loadTasks}>Try Again</button>
		</div>
	{:else}
		<Board {tasks} onAction={handleAction} onEscalate={handleEscalate} onTaskClick={handleTaskClick} />
	{/if}
</div>

<!-- Live Transcript Modal -->
{#if selectedTask}
	<LiveTranscriptModal
		open={transcriptModalOpen}
		task={selectedTask}
		onClose={closeTranscriptModal}
	/>
{/if}

<style>
	.board-page {
		display: flex;
		flex-direction: column;
		height: calc(100vh - var(--header-height) - var(--space-12));
		min-height: 0;
		overflow-x: auto;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: var(--space-4);
		flex-shrink: 0;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.page-header h1 {
		margin: 0;
		font-size: var(--text-xl);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
	}

	.task-count {
		font-size: var(--text-sm);
		color: var(--text-muted);
		background: var(--bg-tertiary);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-md);
	}

	.header-filters {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.new-task-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		background: var(--accent-primary);
		color: var(--text-inverse);
		border: none;
		border-radius: var(--radius-md);
		text-decoration: none;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.new-task-btn:hover {
		background: var(--accent-hover);
		color: var(--text-inverse);
	}

	/* Initiative Banner */
	.initiative-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		background: var(--accent-subtle);
		border: 1px solid var(--accent-primary);
		border-radius: var(--radius-md);
		margin-bottom: var(--space-3);
		flex-shrink: 0;
	}

	.banner-text {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.banner-text strong {
		color: var(--accent-primary);
	}

	.banner-clear {
		padding: var(--space-1) var(--space-2);
		background: transparent;
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-xs);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.banner-clear:hover {
		background: var(--bg-tertiary);
		border-color: var(--border-strong);
	}

	.loading-state,
	.error-state {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		color: var(--text-muted);
	}

	.loading-spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.error-state {
		color: var(--status-danger);
	}

	.error-state svg {
		opacity: 0.7;
	}

	.error-state button {
		margin-top: var(--space-2);
		padding: var(--space-2) var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		cursor: pointer;
	}

	.error-state button:hover {
		background: var(--bg-tertiary);
	}
</style>
