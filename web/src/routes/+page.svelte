<script lang="ts">
	import { onMount } from 'svelte';
	import { listTasks, createTask, runTask, pauseTask } from '$lib/api';
	import type { Task } from '$lib/types';
	import TaskCard from '$lib/components/TaskCard.svelte';

	let tasks = $state<Task[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showNewTask = $state(false);
	let newTaskTitle = $state('');

	onMount(async () => {
		await loadTasks();
	});

	async function loadTasks() {
		loading = true;
		error = null;
		try {
			tasks = await listTasks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
		} finally {
			loading = false;
		}
	}

	async function handleCreateTask() {
		if (!newTaskTitle.trim()) return;
		try {
			await createTask(newTaskTitle.trim());
			newTaskTitle = '';
			showNewTask = false;
			await loadTasks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create task';
		}
	}

	async function handleRunTask(id: string) {
		try {
			await runTask(id);
			await loadTasks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to run task';
		}
	}

	async function handlePauseTask(id: string) {
		try {
			await pauseTask(id);
			await loadTasks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to pause task';
		}
	}

	const activeTasks = $derived(tasks.filter(t => !['completed', 'failed'].includes(t.status)));
	const completedTasks = $derived(tasks.filter(t => ['completed', 'failed'].includes(t.status)));
</script>

<svelte:head>
	<title>orc - Tasks</title>
</svelte:head>

<div class="page">
	<header class="page-header">
		<h1>Tasks</h1>
		<button class="primary" onclick={() => showNewTask = true}>New Task</button>
	</header>

	{#if error}
		<div class="error-banner">
			{error}
			<button onclick={() => error = null}>Dismiss</button>
		</div>
	{/if}

	{#if showNewTask}
		<div class="new-task-form">
			<input
				type="text"
				placeholder="Task title..."
				bind:value={newTaskTitle}
				onkeydown={(e) => e.key === 'Enter' && handleCreateTask()}
			/>
			<div class="form-actions">
				<button onclick={() => { showNewTask = false; newTaskTitle = ''; }}>Cancel</button>
				<button class="primary" onclick={handleCreateTask}>Create</button>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="loading">Loading tasks...</div>
	{:else if tasks.length === 0}
		<div class="empty-state">
			<p>No tasks yet</p>
			<button class="primary" onclick={() => showNewTask = true}>Create your first task</button>
		</div>
	{:else}
		{#if activeTasks.length > 0}
			<section>
				<h2>Active ({activeTasks.length})</h2>
				<div class="task-grid">
					{#each activeTasks as task (task.id)}
						<TaskCard
							{task}
							onRun={() => handleRunTask(task.id)}
							onPause={() => handlePauseTask(task.id)}
						/>
					{/each}
				</div>
			</section>
		{/if}

		{#if completedTasks.length > 0}
			<section>
				<h2>Completed ({completedTasks.length})</h2>
				<div class="task-grid">
					{#each completedTasks as task (task.id)}
						<TaskCard {task} />
					{/each}
				</div>
			</section>
		{/if}
	{/if}
</div>

<style>
	.page {
		max-width: 900px;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 2rem;
	}

	h1 {
		font-size: 1.5rem;
		font-weight: 600;
	}

	h2 {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-secondary);
		margin-bottom: 1rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	section {
		margin-bottom: 2rem;
	}

	.task-grid {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.new-task-form {
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
		border-radius: 8px;
		padding: 1rem;
		margin-bottom: 2rem;
	}

	.new-task-form input {
		width: 100%;
		background: var(--bg-tertiary);
		border: 1px solid var(--border-color);
		border-radius: 6px;
		padding: 0.75rem;
		color: var(--text-primary);
		font-size: 0.875rem;
		margin-bottom: 1rem;
	}

	.new-task-form input:focus {
		outline: none;
		border-color: var(--accent-primary);
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
	}

	.error-banner {
		background: rgba(248, 81, 73, 0.1);
		border: 1px solid var(--accent-danger);
		border-radius: 6px;
		padding: 0.75rem 1rem;
		margin-bottom: 1rem;
		display: flex;
		justify-content: space-between;
		align-items: center;
		color: var(--accent-danger);
	}

	.error-banner button {
		background: transparent;
		border: none;
		color: var(--accent-danger);
		padding: 0.25rem 0.5rem;
	}

	.loading, .empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	.empty-state p {
		margin-bottom: 1rem;
	}
</style>
