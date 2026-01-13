<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		createProjectTask,
		runProjectTask,
		pauseProjectTask,
		resumeProjectTask,
		deleteProjectTask
	} from '$lib/api';
	import type { Task } from '$lib/types';
	import TaskCard from '$lib/components/TaskCard.svelte';
	import Modal from '$lib/components/overlays/Modal.svelte';
	import { currentProjectId, currentProject } from '$lib/stores/project';
	import { tasks as tasksStore, tasksLoading, tasksError, loadTasks, addTask, removeTask } from '$lib/stores/tasks';
	import { setupTaskListShortcuts, getShortcutManager } from '$lib/shortcuts';
	import { toast } from '$lib/stores/toast.svelte';

	// Get reactive values from stores
	let allTasks = $derived($tasksStore);
	let loading = $derived($tasksLoading);
	let storeError = $derived($tasksError);

	let error = $state<string | null>(null);
	let showNewTask = $state(false);
	let newTaskTitle = $state('');
	let newTaskDescription = $state('');
	let newTaskInputRef: HTMLInputElement;
	let selectedIndex = $state(-1);
	let cleanupShortcuts: (() => void) | null = null;

	// Filters
	let searchQuery = $state('');
	let statusFilter = $state<'all' | 'active' | 'completed' | 'failed'>('all');
	let weightFilter = $state<string>('all');
	let sortBy = $state<'recent' | 'oldest' | 'status'>('recent');

	// Sync store error to local error
	$effect(() => {
		if (storeError) {
			error = storeError;
		}
	});

	// Focus input when modal opens
	$effect(() => {
		if (showNewTask && newTaskInputRef) {
			newTaskInputRef.focus();
		}
	});

	// Get selected task from filtered list
	function getSelectedTask(): Task | null {
		const filtered = filteredTasks;
		if (selectedIndex >= 0 && selectedIndex < filtered.length) {
			return filtered[selectedIndex];
		}
		return null;
	}

	// Listen for new task event from command palette
	onMount(() => {
		function handleNewTask() {
			showNewTask = true;
		}

		// Setup task list keyboard shortcuts
		cleanupShortcuts = setupTaskListShortcuts({
			onNavDown: () => {
				const filtered = filteredTasks;
				if (filtered.length > 0) {
					selectedIndex = Math.min(selectedIndex + 1, filtered.length - 1);
					scrollToSelected();
				}
			},
			onNavUp: () => {
				if (selectedIndex > 0) {
					selectedIndex = selectedIndex - 1;
					scrollToSelected();
				}
			},
			onOpen: () => {
				const task = getSelectedTask();
				if (task) {
					goto(`/tasks/${task.id}`);
				}
			},
			onRun: () => {
				const task = getSelectedTask();
				if (task && task.status !== 'running') {
					handleRunTask(task.id);
					toast.info(`Running task ${task.id}`);
				}
			},
			onPause: () => {
				const task = getSelectedTask();
				if (task && task.status === 'running') {
					handlePauseTask(task.id);
					toast.info(`Paused task ${task.id}`);
				}
			},
			onDelete: () => {
				const task = getSelectedTask();
				if (task) {
					if (confirm(`Delete task ${task.id}?`)) {
						handleDeleteTask(task.id);
						toast.success(`Deleted task ${task.id}`);
					}
				}
			}
		});

		window.addEventListener('orc:new-task', handleNewTask);
		return () => window.removeEventListener('orc:new-task', handleNewTask);
	});

	onDestroy(() => {
		if (cleanupShortcuts) {
			cleanupShortcuts();
		}
	});

	function scrollToSelected() {
		// Scroll the selected task into view
		const taskElements = document.querySelectorAll('.task-card');
		if (taskElements[selectedIndex]) {
			taskElements[selectedIndex].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
		}
	}

	async function handleCreateTask() {
		if (!newTaskTitle.trim()) return;
		if (!$currentProjectId) {
			error = 'Please select a project first';
			return;
		}
		try {
			const description = newTaskDescription.trim() || undefined;
			const newTask = await createProjectTask($currentProjectId, newTaskTitle.trim(), description);
			// Add the new task to the store immediately
			addTask(newTask);
			newTaskTitle = '';
			newTaskDescription = '';
			showNewTask = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create task';
		}
	}

	async function handleRunTask(id: string) {
		if (!$currentProjectId) {
			error = 'Please select a project first';
			return;
		}
		try {
			await runProjectTask($currentProjectId, id);
			// WebSocket will update task status via global event handler
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to run task';
		}
	}

	async function handlePauseTask(id: string) {
		if (!$currentProjectId) {
			error = 'Please select a project first';
			return;
		}
		try {
			await pauseProjectTask($currentProjectId, id);
			// WebSocket will update task status via global event handler
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to pause task';
		}
	}

	async function handleResumeTask(id: string) {
		if (!$currentProjectId) {
			error = 'Please select a project first';
			return;
		}
		try {
			await resumeProjectTask($currentProjectId, id);
			// WebSocket will update task status via global event handler
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to resume task';
		}
	}

	async function handleDeleteTask(id: string) {
		if (!$currentProjectId) {
			error = 'Please select a project first';
			return;
		}
		try {
			await deleteProjectTask($currentProjectId, id);
			// Remove from store immediately
			removeTask(id);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete task';
		}
	}

	// Derived filtered tasks (memoized - only recomputes when dependencies change)
	const filteredTasks = $derived.by(() => {
		let result = [...allTasks];

		// Status filter
		if (statusFilter === 'active') {
			result = result.filter((t) => !['completed', 'failed'].includes(t.status));
		} else if (statusFilter === 'completed') {
			result = result.filter((t) => t.status === 'completed');
		} else if (statusFilter === 'failed') {
			result = result.filter((t) => t.status === 'failed');
		}

		// Weight filter
		if (weightFilter !== 'all') {
			result = result.filter((t) => t.weight === weightFilter);
		}

		// Search filter
		if (searchQuery.trim()) {
			const query = searchQuery.toLowerCase();
			result = result.filter(
				(t) => t.id.toLowerCase().includes(query) || t.title.toLowerCase().includes(query)
			);
		}

		// Sort
		if (sortBy === 'recent') {
			result.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
		} else if (sortBy === 'oldest') {
			result.sort((a, b) => new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime());
		} else if (sortBy === 'status') {
			const statusOrder = ['running', 'paused', 'blocked', 'planned', 'created', 'completed', 'failed'];
			result.sort((a, b) => statusOrder.indexOf(a.status) - statusOrder.indexOf(b.status));
		}

		return result;
	});

	// Status counts for tabs (memoized)
	const statusCounts = $derived.by(() => ({
		all: allTasks.length,
		active: allTasks.filter((t) => !['completed', 'failed'].includes(t.status)).length,
		completed: allTasks.filter((t) => t.status === 'completed').length,
		failed: allTasks.filter((t) => t.status === 'failed').length
	}));

	// Available weights
	const weights = ['trivial', 'small', 'medium', 'large', 'greenfield'];
</script>

<svelte:head>
	<title>orc - Tasks</title>
</svelte:head>

<div class="page">
	<!-- Error Banner -->
	{#if error}
		<div class="error-banner">
			<div class="error-content">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<circle cx="12" cy="12" r="10" />
					<line x1="12" y1="8" x2="12" y2="12" />
					<line x1="12" y1="16" x2="12.01" y2="16" />
				</svg>
				<span>{error}</span>
			</div>
			<button class="error-dismiss" onclick={() => (error = null)} aria-label="Dismiss error">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<line x1="18" y1="6" x2="6" y2="18" />
					<line x1="6" y1="6" x2="18" y2="18" />
				</svg>
			</button>
		</div>
	{/if}

	<!-- Filter Bar -->
	<div class="filter-bar">
		<!-- Status Tabs -->
		<div class="status-tabs">
			<button
				class="status-tab"
				class:active={statusFilter === 'all'}
				onclick={() => (statusFilter = 'all')}
			>
				All
				<span class="tab-count">{statusCounts.all}</span>
			</button>
			<button
				class="status-tab"
				class:active={statusFilter === 'active'}
				onclick={() => (statusFilter = 'active')}
			>
				Active
				<span class="tab-count">{statusCounts.active}</span>
			</button>
			<button
				class="status-tab"
				class:active={statusFilter === 'completed'}
				onclick={() => (statusFilter = 'completed')}
			>
				Completed
				<span class="tab-count">{statusCounts.completed}</span>
			</button>
			<button
				class="status-tab"
				class:active={statusFilter === 'failed'}
				onclick={() => (statusFilter = 'failed')}
			>
				Failed
				<span class="tab-count">{statusCounts.failed}</span>
			</button>
		</div>

		<!-- Filters Row -->
		<div class="filters-row">
			<!-- Search -->
			<div class="search-input">
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<circle cx="11" cy="11" r="8" />
					<path d="m21 21-4.35-4.35" />
				</svg>
				<input type="text" placeholder="Search tasks..." bind:value={searchQuery} />
			</div>

			<!-- Weight Filter -->
			<select class="filter-select" bind:value={weightFilter}>
				<option value="all">All weights</option>
				{#each weights as w}
					<option value={w}>{w}</option>
				{/each}
			</select>

			<!-- Sort -->
			<select class="filter-select" bind:value={sortBy}>
				<option value="recent">Most recent</option>
				<option value="oldest">Oldest first</option>
				<option value="status">By status</option>
			</select>

			<!-- New Task Button -->
			<button class="primary new-task-btn" onclick={() => (showNewTask = true)}>
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<line x1="12" y1="5" x2="12" y2="19" />
					<line x1="5" y1="12" x2="19" y2="12" />
				</svg>
				New Task
			</button>
		</div>
	</div>

	<!-- Keyboard Hints -->
	{#if filteredTasks.length > 0 && selectedIndex >= 0}
		<div class="keyboard-hints">
			<span class="hint"><kbd>j</kbd><kbd>k</kbd> navigate</span>
			<span class="hint"><kbd>Enter</kbd> open</span>
			<span class="hint"><kbd>r</kbd> run</span>
			<span class="hint"><kbd>p</kbd> pause</span>
			<span class="hint"><kbd>d</kbd> delete</span>
			<span class="hint"><kbd>?</kbd> all shortcuts</span>
		</div>
	{/if}

	<!-- Task List -->
	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading tasks...</span>
		</div>
	{:else if filteredTasks.length === 0}
		<div class="empty-state">
			{#if !$currentProjectId}
				<!-- No project selected -->
				<div class="empty-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
					</svg>
				</div>
				<h3>No project selected</h3>
				<p>Select a project to view and manage tasks</p>
				<button class="primary" onclick={() => window.dispatchEvent(new CustomEvent('orc:switch-project'))}>
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
					</svg>
					Select Project
				</button>
			{:else if allTasks.length === 0}
				<div class="empty-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
						<line x1="9" y1="9" x2="15" y2="15" />
						<line x1="15" y1="9" x2="9" y2="15" />
					</svg>
				</div>
				<h3>No tasks yet</h3>
				<p>Create your first task to get started with orc</p>
				<button class="primary" onclick={() => (showNewTask = true)}>
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<line x1="12" y1="5" x2="12" y2="19" />
						<line x1="5" y1="12" x2="19" y2="12" />
					</svg>
					Create Task
				</button>
			{:else}
				<div class="empty-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<circle cx="11" cy="11" r="8" />
						<path d="m21 21-4.35-4.35" />
					</svg>
				</div>
				<h3>No matching tasks</h3>
				<p>Try adjusting your filters or search query</p>
				<button
					onclick={() => {
						searchQuery = '';
						statusFilter = 'all';
						weightFilter = 'all';
					}}
				>
					Clear filters
				</button>
			{/if}
		</div>
	{:else}
		<div class="task-list">
			{#each filteredTasks as task, index (task.id)}
				<div
					class="task-card-wrapper"
					class:selected={index === selectedIndex}
					onclick={() => (selectedIndex = index)}
					onkeydown={(e) => e.key === 'Enter' && goto(`/tasks/${task.id}`)}
					role="button"
					tabindex="0"
				>
					<TaskCard
						{task}
						onRun={() => handleRunTask(task.id)}
						onPause={() => handlePauseTask(task.id)}
						onResume={() => handleResumeTask(task.id)}
						onDelete={() => handleDeleteTask(task.id)}
					/>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- New Task Modal -->
<Modal open={showNewTask} onClose={() => (showNewTask = false)} size="md" title="Create New Task">
	<div class="new-task-form">
		<label class="form-label">
			Task Title
			<input
				bind:this={newTaskInputRef}
				type="text"
				placeholder="What needs to be done?"
				bind:value={newTaskTitle}
				onkeydown={(e) => e.key === 'Enter' && !newTaskDescription && handleCreateTask()}
				class="form-input"
			/>
		</label>
		<label class="form-label">
			Description <span class="optional">(optional)</span>
			<textarea
				placeholder="Provide additional context, acceptance criteria, or implementation details..."
				bind:value={newTaskDescription}
				class="form-textarea"
				rows="4"
			></textarea>
		</label>
		<p class="form-hint">
			Orc will classify the weight and create a plan automatically based on the title and description.
		</p>
		<div class="form-actions">
			<button
				onclick={() => {
					showNewTask = false;
					newTaskTitle = '';
					newTaskDescription = '';
				}}
			>
				Cancel
			</button>
			<button class="primary" onclick={handleCreateTask} disabled={!newTaskTitle.trim()}>
				Create Task
			</button>
		</div>
	</div>
</Modal>

<style>
	.page {
		max-width: 900px;
	}

	/* Error Banner */
	.error-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-lg);
		margin-bottom: var(--space-5);
	}

	.error-content {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--status-danger);
		font-size: var(--text-sm);
	}

	.error-dismiss {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--status-danger);
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.error-dismiss:hover {
		background: rgba(239, 68, 68, 0.2);
	}

	/* Filter Bar */
	.filter-bar {
		margin-bottom: var(--space-5);
	}

	.status-tabs {
		display: flex;
		gap: var(--space-1);
		padding: var(--space-1);
		background: var(--bg-secondary);
		border-radius: var(--radius-lg);
		margin-bottom: var(--space-4);
	}

	.status-tab {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-2-5) var(--space-4);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.status-tab:hover {
		color: var(--text-primary);
		background: var(--bg-tertiary);
	}

	.status-tab.active {
		background: var(--accent-primary);
		color: var(--text-inverse);
	}

	.tab-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		padding: var(--space-0-5) var(--space-1-5);
		background: rgba(0, 0, 0, 0.2);
		border-radius: var(--radius-full);
	}

	.status-tab.active .tab-count {
		background: rgba(255, 255, 255, 0.2);
	}

	/* Filters Row */
	.filters-row {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.search-input {
		flex: 1;
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-muted);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.search-input:focus-within {
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.search-input input {
		flex: 1;
		background: transparent;
		border: none;
		font-size: var(--text-sm);
		color: var(--text-primary);
		outline: none;
	}

	.search-input input::placeholder {
		color: var(--text-muted);
	}

	.filter-select {
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-primary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.filter-select:hover {
		border-color: var(--border-strong);
	}

	.filter-select:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.new-task-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		white-space: nowrap;
	}

	/* Task List */
	.task-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.task-card-wrapper {
		border-radius: var(--radius-lg);
		outline: none;
		transition: box-shadow var(--duration-fast) var(--ease-out);
	}

	.task-card-wrapper:focus-visible,
	.task-card-wrapper.selected {
		box-shadow: 0 0 0 2px var(--accent-primary);
	}

	.task-card-wrapper.selected :global(.task-card) {
		border-color: var(--accent-primary);
	}

	/* Loading State */
	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-4);
		padding: var(--space-16);
		text-align: center;
	}

	.spinner {
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

	.loading-state span {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	/* Empty State */
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-4);
		padding: var(--space-16);
		text-align: center;
	}

	.empty-icon {
		color: var(--text-muted);
		opacity: 0.5;
	}

	.empty-state h3 {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.empty-state p {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin: 0;
	}

	.empty-state button {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-top: var(--space-2);
	}

	/* New Task Form */
	.new-task-form {
		padding: var(--space-5);
	}

	.form-label {
		display: block;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		margin-bottom: var(--space-4);
	}

	.form-label + .form-label {
		margin-top: var(--space-3);
	}

	.form-input {
		width: 100%;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-base);
		color: var(--text-primary);
		margin-top: var(--space-2);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.form-input:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.form-input::placeholder {
		color: var(--text-muted);
	}

	.form-hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin-top: var(--space-2);
		margin-bottom: var(--space-5);
	}

	.form-textarea {
		width: 100%;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-primary);
		margin-top: var(--space-2);
		resize: vertical;
		min-height: 80px;
		font-family: inherit;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.form-textarea:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.form-textarea::placeholder {
		color: var(--text-muted);
	}

	.optional {
		font-weight: var(--font-normal);
		color: var(--text-muted);
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-3);
	}

	/* Keyboard Hints */
	.keyboard-hints {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-4);
		padding: var(--space-2) var(--space-4);
		background: var(--bg-secondary);
		border-radius: var(--radius-md);
		margin-bottom: var(--space-4);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.keyboard-hints .hint {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}

	.keyboard-hints kbd {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 18px;
		height: 18px;
		padding: 0 var(--space-1);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
		color: var(--text-secondary);
	}
</style>
