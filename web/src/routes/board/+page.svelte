<script lang="ts">
	import { get } from 'svelte/store';
	import Board from '$lib/components/kanban/Board.svelte';
	import LiveTranscriptModal from '$lib/components/overlays/LiveTranscriptModal.svelte';
	import { runProjectTask, pauseProjectTask, resumeProjectTask, escalateProjectTask } from '$lib/api';
	import { currentProjectId } from '$lib/stores/project';
	import { tasks as tasksStore, tasksLoading, tasksError, loadTasks } from '$lib/stores/tasks';
	import { currentInitiativeId, currentInitiative } from '$lib/stores/initiative';
	import type { Task } from '$lib/types';

	// Reactive binding to global task store
	let allTasks = $derived($tasksStore);
	let loading = $derived($tasksLoading);
	let error = $derived($tasksError);

	// Filter tasks by initiative if one is selected
	let tasks = $derived.by(() => {
		const initiativeId = $currentInitiativeId;
		if (!initiativeId) return allTasks;

		// Get task IDs from the initiative
		const initiative = $currentInitiative;
		if (!initiative?.tasks) return allTasks;

		const initiativeTaskIds = new Set(initiative.tasks.map(t => t.id));
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
			{#if $currentInitiative}
				<span class="initiative-filter">
					<span class="filter-label">Filtered by</span>
					<span class="filter-value">{$currentInitiative.title}</span>
				</span>
			{/if}
			<span class="task-count">{tasks.length} tasks</span>
		</div>
		<div class="header-actions">
			<a href="/tasks/new" class="new-task-btn">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<line x1="12" y1="5" x2="12" y2="19" />
					<line x1="5" y1="12" x2="19" y2="12" />
				</svg>
				New Task
			</a>
		</div>
	</header>

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

	.initiative-filter {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-sm);
		background: var(--accent-subtle);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-md);
		border: 1px solid var(--accent-primary);
	}

	.filter-label {
		color: var(--text-muted);
	}

	.filter-value {
		color: var(--accent-primary);
		font-weight: var(--font-medium);
	}

	.header-actions {
		display: flex;
		gap: var(--space-2);
	}

	.new-task-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		background: var(--accent-primary);
		color: var(--text-inverse);
		border-radius: var(--radius-md);
		text-decoration: none;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.new-task-btn:hover {
		background: var(--accent-hover);
		color: var(--text-inverse);
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
