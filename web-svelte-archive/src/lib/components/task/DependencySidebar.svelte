<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		getTaskDependencies,
		addBlocker,
		removeBlocker,
		addRelated,
		removeRelated,
		type DependencyGraph,
		type DependencyInfo
	} from '$lib/api';
	import type { Task } from '$lib/types';
	import AddDependencyModal from './AddDependencyModal.svelte';

	interface Props {
		task: Task;
		onTaskUpdated?: (task: Task) => void;
	}

	let { task, onTaskUpdated }: Props = $props();

	let dependencies = $state<DependencyGraph | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let expanded = $state(true);

	// Modal state
	let addModalOpen = $state(false);
	let addModalType = $state<'blocker' | 'related'>('blocker');

	// Load dependencies when task changes
	$effect(() => {
		if (task?.id) {
			loadDependencies();
		}
	});

	async function loadDependencies() {
		loading = true;
		error = null;

		try {
			dependencies = await getTaskDependencies(task.id);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load dependencies';
		} finally {
			loading = false;
		}
	}

	function navigateToTask(taskId: string) {
		goto(`/tasks/${taskId}`);
	}

	function openAddModal(type: 'blocker' | 'related') {
		addModalType = type;
		addModalOpen = true;
	}

	async function handleAddDependency(selectedTaskId: string) {
		try {
			let updatedTask: Task;
			if (addModalType === 'blocker') {
				updatedTask = await addBlocker(task.id, selectedTaskId);
			} else {
				updatedTask = await addRelated(task.id, selectedTaskId);
			}
			onTaskUpdated?.(updatedTask);
			await loadDependencies();
			addModalOpen = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add dependency';
		}
	}

	async function handleRemoveBlocker(blockerId: string) {
		try {
			const updatedTask = await removeBlocker(task.id, blockerId);
			onTaskUpdated?.(updatedTask);
			await loadDependencies();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove blocker';
		}
	}

	async function handleRemoveRelated(relatedId: string) {
		try {
			const updatedTask = await removeRelated(task.id, relatedId);
			onTaskUpdated?.(updatedTask);
			await loadDependencies();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove relation';
		}
	}

	function getStatusIcon(status: string, isMet?: boolean): { icon: string; class: string } {
		if (status === 'completed' || status === 'finished' || isMet) {
			return { icon: '✓', class: 'status-completed' };
		}
		if (status === 'running') {
			return { icon: '●', class: 'status-running' };
		}
		return { icon: '○', class: 'status-pending' };
	}

	// Check if task is blocked
	const isBlocked = $derived(
		dependencies && dependencies.unmet_dependencies && dependencies.unmet_dependencies.length > 0
	);

	// Check if task blocks others
	const blocksOthersCount = $derived(dependencies?.blocks?.length ?? 0);

	// Count for display
	const blockedByCount = $derived(dependencies?.blocked_by?.length ?? 0);
	const relatedCount = $derived(dependencies?.related_to?.length ?? 0);
	const referencedByCount = $derived(dependencies?.referenced_by?.length ?? 0);
</script>

<div class="dependency-sidebar">
	<button
		type="button"
		class="sidebar-header"
		onclick={() => (expanded = !expanded)}
		aria-expanded={expanded}
		aria-controls="dependency-content"
	>
		<div class="header-content">
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
				<circle cx="18" cy="5" r="3" />
				<circle cx="6" cy="12" r="3" />
				<circle cx="18" cy="19" r="3" />
				<line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
				<line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
			</svg>
			<span>Dependencies</span>
		</div>
		<svg
			class="chevron"
			class:expanded
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
			<polyline points="6 9 12 15 18 9" />
		</svg>
	</button>

	{#if expanded}
		<div id="dependency-content" class="sidebar-content" role="region" aria-label="Task dependencies">
			{#if loading}
				<div class="loading-state" role="status" aria-live="polite">
					<div class="spinner" aria-hidden="true"></div>
					<span>Loading dependencies...</span>
				</div>
			{:else if error}
				<div class="error-state" role="alert">
					<span class="error-icon" aria-hidden="true">!</span>
					<span>{error}</span>
					<button type="button" class="btn-retry" onclick={loadDependencies}>Retry</button>
				</div>
			{:else if dependencies}
				<!-- Blocked banner -->
				{#if isBlocked}
					<div class="blocked-banner" role="alert">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"
						>
							<circle cx="12" cy="12" r="10" />
							<line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
						</svg>
						<span>Blocked by {dependencies.unmet_dependencies?.length} incomplete task{dependencies.unmet_dependencies?.length === 1 ? '' : 's'}</span>
					</div>
				{/if}

				<!-- Blocking others info -->
				{#if blocksOthersCount > 0}
					<div class="blocking-info">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"
						>
							<circle cx="12" cy="12" r="10" />
							<line x1="12" y1="8" x2="12" y2="12" />
							<line x1="12" y1="16" x2="12.01" y2="16" />
						</svg>
						<span>Blocking {blocksOthersCount} task{blocksOthersCount === 1 ? '' : 's'}</span>
					</div>
				{/if}

				<!-- Blocked By section -->
				<div class="section">
					<div class="section-header">
						<span class="section-title">
							Blocked by
							{#if blockedByCount > 0}
								<span class="count">({blockedByCount})</span>
							{/if}
						</span>
						<button
							type="button"
							class="btn-add"
							onclick={() => openAddModal('blocker')}
							title="Add blocker"
							aria-label="Add blocking task"
						>
							+ Add
						</button>
					</div>
					{#if blockedByCount === 0}
						<div class="empty-section">
							<span class="empty-text">No blocking dependencies</span>
						</div>
					{:else}
						<ul class="dependency-list">
							{#each dependencies.blocked_by as dep}
								{@const statusInfo = getStatusIcon(dep.status, dep.is_met)}
								<li class="dependency-item">
									<button
										type="button"
										class="dep-link"
										onclick={() => navigateToTask(dep.id)}
										title="View {dep.id}"
									>
										<span class="status-icon {statusInfo.class}">{statusInfo.icon}</span>
										<span class="dep-id">{dep.id}</span>
										<span class="dep-title">{dep.title}</span>
									</button>
									<button
										type="button"
										class="btn-remove"
										onclick={() => handleRemoveBlocker(dep.id)}
										title="Remove blocker"
										aria-label="Remove {dep.id} as blocker"
									>
										<svg
											xmlns="http://www.w3.org/2000/svg"
											width="12"
											height="12"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
											stroke-linecap="round"
											stroke-linejoin="round"
											aria-hidden="true"
										>
											<line x1="18" y1="6" x2="6" y2="18" />
											<line x1="6" y1="6" x2="18" y2="18" />
										</svg>
									</button>
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<!-- Blocks section -->
				{#if blocksOthersCount > 0}
					<div class="section">
						<div class="section-header">
							<span class="section-title">
								Blocks
								<span class="count">({blocksOthersCount})</span>
							</span>
						</div>
						<ul class="dependency-list">
							{#each dependencies.blocks as dep}
								{@const statusInfo = getStatusIcon(dep.status)}
								<li class="dependency-item readonly">
									<button
										type="button"
										class="dep-link"
										onclick={() => navigateToTask(dep.id)}
										title="View {dep.id}"
									>
										<span class="status-icon {statusInfo.class}">{statusInfo.icon}</span>
										<span class="dep-id">{dep.id}</span>
										<span class="dep-title">{dep.title}</span>
									</button>
								</li>
							{/each}
						</ul>
					</div>
				{/if}

				<!-- Related section -->
				<div class="section">
					<div class="section-header">
						<span class="section-title">
							Related
							{#if relatedCount > 0}
								<span class="count">({relatedCount})</span>
							{/if}
						</span>
						<button
							type="button"
							class="btn-add"
							onclick={() => openAddModal('related')}
							title="Add related task"
							aria-label="Add related task"
						>
							+ Add
						</button>
					</div>
					{#if relatedCount === 0}
						<div class="empty-section">
							<span class="empty-text">No related tasks</span>
						</div>
					{:else}
						<ul class="dependency-list">
							{#each dependencies.related_to as dep}
								{@const statusInfo = getStatusIcon(dep.status)}
								<li class="dependency-item">
									<button
										type="button"
										class="dep-link"
										onclick={() => navigateToTask(dep.id)}
										title="View {dep.id}"
									>
										<span class="status-icon {statusInfo.class}">{statusInfo.icon}</span>
										<span class="dep-id">{dep.id}</span>
										<span class="dep-title">{dep.title}</span>
									</button>
									<button
										type="button"
										class="btn-remove"
										onclick={() => handleRemoveRelated(dep.id)}
										title="Remove relation"
										aria-label="Remove relation to {dep.id}"
									>
										<svg
											xmlns="http://www.w3.org/2000/svg"
											width="12"
											height="12"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
											stroke-linecap="round"
											stroke-linejoin="round"
											aria-hidden="true"
										>
											<line x1="18" y1="6" x2="6" y2="18" />
											<line x1="6" y1="6" x2="18" y2="18" />
										</svg>
									</button>
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<!-- Referenced in section -->
				{#if referencedByCount > 0}
					<div class="section">
						<div class="section-header">
							<span class="section-title">
								Referenced in
								<span class="count">({referencedByCount})</span>
							</span>
						</div>
						<ul class="dependency-list">
							{#each dependencies.referenced_by as dep}
								{@const statusInfo = getStatusIcon(dep.status)}
								<li class="dependency-item readonly">
									<button
										type="button"
										class="dep-link"
										onclick={() => navigateToTask(dep.id)}
										title="View {dep.id}"
									>
										<span class="status-icon {statusInfo.class}">{statusInfo.icon}</span>
										<span class="dep-id">{dep.id}</span>
										<span class="dep-title">{dep.title}</span>
									</button>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>

<!-- Add Dependency Modal -->
<AddDependencyModal
	open={addModalOpen}
	onClose={() => (addModalOpen = false)}
	onSelect={handleAddDependency}
	type={addModalType}
	currentTaskId={task.id}
	existingBlockers={dependencies?.blocked_by?.map((d) => d.id) ?? []}
	existingRelated={dependencies?.related_to?.map((d) => d.id) ?? []}
/>

<style>
	.dependency-sidebar {
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		background: var(--bg-secondary);
		overflow: hidden;
	}

	.sidebar-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: var(--space-3) var(--space-4);
		background: transparent;
		border: none;
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.sidebar-header:hover {
		background: var(--bg-tertiary);
	}

	.header-content {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.chevron {
		color: var(--text-muted);
		transition: transform 0.2s ease;
	}

	.chevron.expanded {
		transform: rotate(180deg);
	}

	.sidebar-content {
		padding: var(--space-4);
		padding-top: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* Blocked banner */
	.blocked-banner {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--status-danger);
	}

	/* Blocking info */
	.blocking-info {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--status-info-bg);
		border: 1px solid var(--status-info);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--status-info);
	}

	/* Loading state */
	.loading-state {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		color: var(--text-muted);
		padding: var(--space-2) 0;
	}

	.spinner {
		width: 14px;
		height: 14px;
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
		gap: var(--space-2);
		font-size: var(--text-sm);
		color: var(--status-danger);
		padding: var(--space-2) 0;
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

	/* Section */
	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.section-title {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
	}

	.count {
		font-weight: var(--font-normal);
		color: var(--text-disabled);
	}

	.btn-add {
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		background: transparent;
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		color: var(--text-secondary);
		cursor: pointer;
		transition:
			border-color 0.15s ease,
			color 0.15s ease;
	}

	.btn-add:hover {
		border-color: var(--accent-primary);
		color: var(--accent-primary);
	}

	/* Empty section */
	.empty-section {
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		text-align: center;
	}

	.empty-text {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	/* Dependency list */
	.dependency-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.dependency-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1-5) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		transition: background 0.15s ease;
	}

	.dependency-item:hover {
		background: var(--bg-primary);
	}

	.dependency-item.readonly {
		padding-right: var(--space-2);
	}

	.dep-link {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex: 1;
		min-width: 0;
		background: none;
		border: none;
		cursor: pointer;
		text-align: left;
		padding: 0;
	}

	.dep-link:hover .dep-id {
		color: var(--accent-primary);
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
		animation: pulse 2s ease-in-out infinite;
	}

	.status-icon.status-pending {
		color: var(--text-muted);
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.5;
		}
	}

	.dep-id {
		flex-shrink: 0;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		transition: color 0.15s ease;
	}

	.dep-title {
		flex: 1;
		min-width: 0;
		font-size: var(--text-sm);
		color: var(--text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.btn-remove {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		background: transparent;
		border: none;
		border-radius: var(--radius-sm);
		color: var(--text-muted);
		cursor: pointer;
		opacity: 0;
		transition:
			opacity 0.15s ease,
			color 0.15s ease,
			background 0.15s ease;
	}

	.dependency-item:hover .btn-remove {
		opacity: 1;
	}

	.btn-remove:hover {
		color: var(--status-danger);
		background: var(--status-danger-bg);
	}
</style>
