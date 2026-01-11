<script lang="ts">
	import { onMount } from 'svelte';
	import Board from '$lib/components/kanban/Board.svelte';
	import { listTasks, runTask, pauseTask, resumeTask } from '$lib/api';
	import type { Task } from '$lib/types';

	let tasks = $state<Task[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		await loadTasks();
	});

	async function loadTasks() {
		loading = true;
		error = null;
		try {
			const result = await listTasks();
			// Handle both array and paginated response
			if (Array.isArray(result)) {
				tasks = result;
			} else {
				tasks = result.tasks || [];
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
		} finally {
			loading = false;
		}
	}

	async function handleAction(taskId: string, action: 'run' | 'pause' | 'resume') {
		try {
			if (action === 'run') await runTask(taskId);
			else if (action === 'pause') await pauseTask(taskId);
			else if (action === 'resume') await resumeTask(taskId);
			await loadTasks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Action failed';
		}
	}
</script>

<svelte:head>
	<title>Board - orc</title>
</svelte:head>

<div class="board-page">
	<header class="page-header">
		<div class="header-left">
			<h1>Task Board</h1>
			<span class="task-count">{tasks.length} tasks</span>
		</div>
		<div class="header-actions">
			<button class="refresh-btn" onclick={loadTasks} disabled={loading}>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="23 4 23 10 17 10" />
					<polyline points="1 20 1 14 7 14" />
					<path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
				</svg>
				Refresh
			</button>
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
		<Board {tasks} onAction={handleAction} onRefresh={loadTasks} />
	{/if}
</div>

<style>
	.board-page {
		display: flex;
		flex-direction: column;
		height: calc(100vh - var(--header-height) - var(--space-12));
		min-height: 0;
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

	.header-actions {
		display: flex;
		gap: var(--space-2);
	}

	.refresh-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		font-size: var(--text-sm);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.refresh-btn:hover:not(:disabled) {
		background: var(--bg-tertiary);
		border-color: var(--border-strong);
		color: var(--text-primary);
	}

	.refresh-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
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
