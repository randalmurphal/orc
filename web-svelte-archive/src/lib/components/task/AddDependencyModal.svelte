<script lang="ts">
	import { listTasks, type PaginatedTasks } from '$lib/api';
	import type { Task } from '$lib/types';
	import Modal from '$lib/components/overlays/Modal.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
		onSelect: (taskId: string) => void;
		type: 'blocker' | 'related';
		currentTaskId: string;
		existingBlockers: string[];
		existingRelated: string[];
	}

	let { open, onClose, onSelect, type, currentTaskId, existingBlockers, existingRelated }: Props =
		$props();

	let tasks = $state<Task[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let searchQuery = $state('');

	// Load tasks when modal opens
	$effect(() => {
		if (open) {
			loadTasks();
			searchQuery = '';
		}
	});

	async function loadTasks() {
		loading = true;
		error = null;

		try {
			const result = await listTasks();
			// Handle both array and paginated response
			tasks = Array.isArray(result) ? result : (result as PaginatedTasks).tasks;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
		} finally {
			loading = false;
		}
	}

	// Filter tasks to exclude current task and already selected
	const filteredTasks = $derived.by(() => {
		const excludeIds = new Set([currentTaskId]);

		// Exclude already selected depending on type
		if (type === 'blocker') {
			existingBlockers.forEach((id) => excludeIds.add(id));
		} else {
			existingRelated.forEach((id) => excludeIds.add(id));
		}

		return tasks
			.filter((t) => !excludeIds.has(t.id))
			.filter((t) => {
				if (!searchQuery) return true;
				const query = searchQuery.toLowerCase();
				return t.id.toLowerCase().includes(query) || t.title.toLowerCase().includes(query);
			});
	});

	const modalTitle = $derived(type === 'blocker' ? 'Add Blocking Task' : 'Add Related Task');
	const helpText = $derived(
		type === 'blocker'
			? 'Select a task that must be completed before this one'
			: 'Select a task that is related to this one'
	);

	function handleSelect(taskId: string) {
		onSelect(taskId);
	}

	function getStatusIcon(status: string): { icon: string; class: string } {
		if (status === 'completed' || status === 'finished') {
			return { icon: '✓', class: 'status-completed' };
		}
		if (status === 'running') {
			return { icon: '●', class: 'status-running' };
		}
		return { icon: '○', class: 'status-pending' };
	}

	function getStatusLabel(status: string): string {
		switch (status) {
			case 'completed':
			case 'finished':
				return 'Completed';
			case 'running':
				return 'Running';
			case 'paused':
				return 'Paused';
			case 'blocked':
				return 'Blocked';
			case 'failed':
				return 'Failed';
			default:
				return 'Pending';
		}
	}
</script>

<Modal {open} {onClose} title={modalTitle} size="md">
	<div class="add-dependency-modal">
		<p class="help-text">{helpText}</p>

		<div class="search-box">
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="16"
				height="16"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				<circle cx="11" cy="11" r="8" />
				<line x1="21" y1="21" x2="16.65" y2="16.65" />
			</svg>
			<input
				type="text"
				placeholder="Search by ID or title..."
				bind:value={searchQuery}
				disabled={loading}
			/>
		</div>

		{#if loading}
			<div class="loading-state" role="status" aria-live="polite">
				<div class="spinner" aria-hidden="true"></div>
				<span>Loading tasks...</span>
			</div>
		{:else if error}
			<div class="error-state" role="alert">
				<span class="error-icon" aria-hidden="true">!</span>
				<span>{error}</span>
				<button type="button" class="btn-retry" onclick={loadTasks}>Retry</button>
			</div>
		{:else if filteredTasks.length === 0}
			<div class="empty-state">
				{#if searchQuery}
					<p>No tasks matching "{searchQuery}"</p>
				{:else}
					<p>No available tasks to add</p>
				{/if}
			</div>
		{:else}
			<ul class="task-list">
				{#each filteredTasks as task}
					{@const statusInfo = getStatusIcon(task.status)}
					<li>
						<button
							type="button"
							class="task-item"
							onclick={() => handleSelect(task.id)}
						>
							<div class="task-main">
								<span class="status-icon {statusInfo.class}" title={getStatusLabel(task.status)}>
									{statusInfo.icon}
								</span>
								<span class="task-id">{task.id}</span>
								<span class="task-title">{task.title}</span>
							</div>
							<div class="task-meta">
								<span class="task-status">{getStatusLabel(task.status)}</span>
							</div>
						</button>
					</li>
				{/each}
			</ul>
		{/if}

		<div class="modal-actions">
			<button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
		</div>
	</div>
</Modal>

<style>
	.add-dependency-modal {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.help-text {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin: 0;
	}

	/* Search box */
	.search-box {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}

	.search-box:focus-within {
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.search-box svg {
		flex-shrink: 0;
		color: var(--text-muted);
	}

	.search-box input {
		flex: 1;
		border: none;
		background: transparent;
		font-size: var(--text-sm);
		color: var(--text-primary);
		outline: none;
	}

	.search-box input::placeholder {
		color: var(--text-muted);
	}

	/* Loading state */
	.loading-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-8);
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Error state */
	.error-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-4);
		font-size: var(--text-sm);
		color: var(--status-danger);
	}

	.error-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		background: var(--status-danger);
		color: white;
		border-radius: 50%;
		font-size: 10px;
		font-weight: bold;
	}

	.btn-retry {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		background: transparent;
		border: 1px solid currentColor;
		border-radius: var(--radius-sm);
		color: inherit;
		cursor: pointer;
	}

	/* Empty state */
	.empty-state {
		padding: var(--space-8);
		text-align: center;
		color: var(--text-muted);
		font-size: var(--text-sm);
	}

	.empty-state p {
		margin: 0;
	}

	/* Task list */
	.task-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		max-height: 400px;
		overflow-y: auto;
	}

	.task-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid transparent;
		border-radius: var(--radius-md);
		cursor: pointer;
		text-align: left;
		transition: background 0.15s ease, border-color 0.15s ease;
	}

	.task-item:hover {
		background: var(--bg-primary);
		border-color: var(--accent-primary);
	}

	.task-main {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex: 1;
		min-width: 0;
	}

	.status-icon {
		flex-shrink: 0;
		font-size: var(--text-xs);
	}

	.status-icon.status-completed {
		color: var(--status-success);
	}

	.status-icon.status-running {
		color: var(--accent-primary);
	}

	.status-icon.status-pending {
		color: var(--text-muted);
	}

	.task-id {
		flex-shrink: 0;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
	}

	.task-title {
		flex: 1;
		min-width: 0;
		font-size: var(--text-sm);
		color: var(--text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.task-meta {
		flex-shrink: 0;
		margin-left: var(--space-2);
	}

	.task-status {
		font-size: var(--text-xs);
		color: var(--text-muted);
		padding: var(--space-0-5) var(--space-2);
		background: var(--bg-secondary);
		border-radius: var(--radius-sm);
	}

	/* Actions */
	.modal-actions {
		display: flex;
		justify-content: flex-end;
		padding-top: var(--space-2);
		border-top: 1px solid var(--border-subtle);
	}

	.btn-cancel {
		padding: var(--space-2) var(--space-4);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease;
	}

	.btn-cancel:hover {
		background: var(--bg-primary);
		border-color: var(--border-hover);
	}
</style>
