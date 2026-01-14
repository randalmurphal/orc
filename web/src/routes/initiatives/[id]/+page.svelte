<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		getInitiative,
		updateInitiative,
		addInitiativeTask,
		removeInitiativeTask,
		addInitiativeDecision,
		listTasks,
		type AddInitiativeTaskRequest,
		type AddInitiativeDecisionRequest
	} from '$lib/api';
	import type { Initiative, InitiativeStatus, InitiativeTaskRef, InitiativeDecision, Task } from '$lib/types';
	import { updateInitiativeInStore } from '$lib/stores/initiative';
	import Modal from '$lib/components/overlays/Modal.svelte';
	import Icon from '$lib/components/ui/Icon.svelte';

	let initiative = $state<Initiative | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Modal states
	let editModalOpen = $state(false);
	let linkTaskModalOpen = $state(false);
	let addDecisionModalOpen = $state(false);

	// Edit form state
	let editTitle = $state('');
	let editVision = $state('');
	let editStatus = $state<InitiativeStatus>('draft');

	// Link task state
	let availableTasks = $state<Task[]>([]);
	let linkTaskSearch = $state('');
	let linkTaskLoading = $state(false);

	// Add decision state
	let decisionText = $state('');
	let decisionRationale = $state('');
	let decisionBy = $state('');
	let addingDecision = $state(false);

	const initiativeId = $derived($page.params.id ?? '');

	// Compute progress
	const progress = $derived.by(() => {
		if (!initiative?.tasks || initiative.tasks.length === 0) {
			return { completed: 0, total: 0, percentage: 0 };
		}
		const completed = initiative.tasks.filter(t => t.status === 'completed' || t.status === 'finished').length;
		const total = initiative.tasks.length;
		return { completed, total, percentage: Math.round((completed / total) * 100) };
	});

	// Filter tasks for linking (not already in initiative)
	const filteredAvailableTasks = $derived.by(() => {
		const existingIds = new Set(initiative?.tasks?.map(t => t.id) || []);
		let filtered = availableTasks.filter(t => !existingIds.has(t.id));
		if (linkTaskSearch) {
			const search = linkTaskSearch.toLowerCase();
			filtered = filtered.filter(t =>
				t.id.toLowerCase().includes(search) ||
				t.title.toLowerCase().includes(search)
			);
		}
		return filtered;
	});

	// Task dependencies within initiative
	const taskDependencies = $derived.by(() => {
		if (!initiative?.tasks) return [];
		const deps: { taskId: string; dependsOn: string[] }[] = [];
		for (const task of initiative.tasks) {
			if (task.depends_on && task.depends_on.length > 0) {
				deps.push({ taskId: task.id, dependsOn: task.depends_on });
			}
		}
		return deps;
	});

	onMount(async () => {
		await loadInitiative();
	});

	async function loadInitiative() {
		loading = true;
		error = null;
		try {
			initiative = await getInitiative(initiativeId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load initiative';
		} finally {
			loading = false;
		}
	}

	function openEditModal() {
		if (initiative) {
			editTitle = initiative.title;
			editVision = initiative.vision || '';
			editStatus = initiative.status;
		}
		editModalOpen = true;
	}

	async function saveEdit() {
		if (!initiative) return;
		try {
			const updated = await updateInitiative(initiative.id, {
				title: editTitle,
				vision: editVision,
				status: editStatus
			});
			initiative = updated;
			updateInitiativeInStore(updated.id, updated);
			editModalOpen = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update initiative';
		}
	}

	async function handleArchive() {
		if (!initiative || !confirm(`Archive initiative "${initiative.title}"?`)) return;
		try {
			const updated = await updateInitiative(initiative.id, { status: 'archived' });
			initiative = updated;
			updateInitiativeInStore(updated.id, updated);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to archive initiative';
		}
	}

	async function openLinkTaskModal() {
		linkTaskLoading = true;
		linkTaskSearch = '';
		linkTaskModalOpen = true;
		try {
			const result = await listTasks();
			availableTasks = Array.isArray(result) ? result : result.tasks;
		} catch (e) {
			console.error('Failed to load tasks:', e);
			availableTasks = [];
		} finally {
			linkTaskLoading = false;
		}
	}

	async function linkTask(taskId: string) {
		if (!initiative) return;
		try {
			const req: AddInitiativeTaskRequest = { task_id: taskId };
			await addInitiativeTask(initiative.id, req);
			await loadInitiative();
			linkTaskModalOpen = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to link task';
		}
	}

	async function unlinkTask(taskId: string) {
		if (!initiative || !confirm(`Remove task ${taskId} from this initiative?`)) return;
		try {
			await removeInitiativeTask(initiative.id, taskId);
			await loadInitiative();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove task';
		}
	}

	function openAddDecisionModal() {
		decisionText = '';
		decisionRationale = '';
		decisionBy = '';
		addDecisionModalOpen = true;
	}

	async function addDecision() {
		if (!initiative || !decisionText.trim()) return;
		addingDecision = true;
		try {
			const req: AddInitiativeDecisionRequest = {
				decision: decisionText.trim(),
				rationale: decisionRationale.trim() || undefined,
				by: decisionBy.trim() || undefined
			};
			await addInitiativeDecision(initiative.id, req);
			await loadInitiative();
			addDecisionModalOpen = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add decision';
		} finally {
			addingDecision = false;
		}
	}

	function getStatusIcon(status: string): string {
		switch (status) {
			case 'completed':
			case 'finished':
				return 'check-circle';
			case 'running':
				return 'play-circle';
			case 'failed':
				return 'x-circle';
			case 'paused':
				return 'pause-circle';
			case 'blocked':
				return 'alert-circle';
			default:
				return 'circle';
		}
	}

	function getStatusClass(status: string): string {
		switch (status) {
			case 'completed':
			case 'finished':
				return 'status-success';
			case 'running':
				return 'status-running';
			case 'failed':
				return 'status-danger';
			case 'blocked':
			case 'paused':
				return 'status-warning';
			default:
				return 'status-pending';
		}
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}
</script>

<svelte:head>
	<title>{initiative?.title || 'Initiative'} - orc</title>
</svelte:head>

{#if loading}
	<div class="loading-state">
		<div class="spinner"></div>
		<span>Loading initiative...</span>
	</div>
{:else if error}
	<div class="error-state">
		<div class="error-icon">!</div>
		<p>{error}</p>
		<button onclick={loadInitiative}>Retry</button>
	</div>
{:else if initiative}
	<div class="initiative-detail">
		<!-- Back Link -->
		<a href="/" class="back-link" onclick={(e) => { e.preventDefault(); goto('/'); }}>
			<Icon name="arrow-left" size={16} />
			<span>Back to Tasks</span>
		</a>

		<!-- Header Section -->
		<header class="initiative-header">
			<div class="header-top">
				<h1 class="initiative-title">{initiative.title}</h1>
				<div class="header-actions">
					<button class="btn btn-secondary" onclick={openEditModal}>
						<Icon name="edit" size={16} />
						Edit
					</button>
					{#if initiative.status !== 'archived'}
						<button class="btn btn-ghost" onclick={handleArchive}>
							<Icon name="archive" size={16} />
							Archive
						</button>
					{/if}
				</div>
			</div>

			{#if initiative.vision}
				<p class="initiative-vision">{initiative.vision}</p>
			{/if}

			<div class="initiative-meta">
				<!-- Progress Bar -->
				<div class="progress-section">
					<div class="progress-label">
						<span>Progress</span>
						<span class="progress-count">{progress.completed}/{progress.total} tasks ({progress.percentage}%)</span>
					</div>
					<div class="progress-bar">
						<div class="progress-fill" style="width: {progress.percentage}%"></div>
					</div>
				</div>

				<div class="meta-grid">
					{#if initiative.owner?.initials}
						<div class="meta-item">
							<span class="meta-label">Owner</span>
							<span class="meta-value">{initiative.owner.display_name || initiative.owner.initials}</span>
						</div>
					{/if}
					<div class="meta-item">
						<span class="meta-label">Status</span>
						<span class="status-badge status-{initiative.status}">{initiative.status}</span>
					</div>
					<div class="meta-item">
						<span class="meta-label">Created</span>
						<span class="meta-value">{formatDate(initiative.created_at)}</span>
					</div>
				</div>
			</div>
		</header>

		<!-- Tasks Section -->
		<section class="section tasks-section">
			<div class="section-header">
				<h2>Tasks</h2>
				<div class="section-actions">
					<button class="btn btn-primary btn-sm" onclick={() => { goto(`/?initiative=${initiative?.id}`); window.dispatchEvent(new CustomEvent('orc:new-task')); }}>
						<Icon name="plus" size={14} />
						Add Task
					</button>
					<button class="btn btn-secondary btn-sm" onclick={openLinkTaskModal}>
						<Icon name="link" size={14} />
						Link Existing
					</button>
				</div>
			</div>

			{#if initiative.tasks && initiative.tasks.length > 0}
				<div class="task-list">
					{#each initiative.tasks as task (task.id)}
						<div class="task-item">
							<a href="/tasks/{task.id}" class="task-link">
								<span class="task-status {getStatusClass(task.status)}">
									<Icon name={getStatusIcon(task.status)} size={16} />
								</span>
								<span class="task-id">{task.id}</span>
								<span class="task-title">{task.title}</span>
								<span class="task-status-text">{task.status}</span>
							</a>
							<button
								class="btn-icon btn-remove"
								onclick={() => unlinkTask(task.id)}
								title="Remove from initiative"
							>
								<Icon name="x" size={14} />
							</button>
						</div>
					{/each}
				</div>

				<!-- Dependencies Section -->
				{#if taskDependencies.length > 0}
					<div class="dependencies-section">
						<h3>Dependencies</h3>
						<ul class="dependency-list">
							{#each taskDependencies as dep}
								<li>
									<span class="dep-task">{dep.taskId}</span>
									<span class="dep-arrow">depends on</span>
									<span class="dep-targets">{dep.dependsOn.join(', ')}</span>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			{:else}
				<div class="empty-state">
					<Icon name="clipboard" size={32} />
					<p>No tasks in this initiative yet</p>
					<button class="btn btn-primary" onclick={openLinkTaskModal}>
						Link a Task
					</button>
				</div>
			{/if}
		</section>

		<!-- Decisions Section -->
		<section class="section decisions-section">
			<div class="section-header">
				<h2>Decisions</h2>
				<button class="btn btn-secondary btn-sm" onclick={openAddDecisionModal}>
					<Icon name="plus" size={14} />
					Add Decision
				</button>
			</div>

			{#if initiative.decisions && initiative.decisions.length > 0}
				<div class="decision-list">
					{#each initiative.decisions as decision (decision.id)}
						<div class="decision-item">
							<div class="decision-header">
								<span class="decision-id">{decision.id}</span>
								<span class="decision-date">({formatDate(decision.date)})</span>
								{#if decision.by}
									<span class="decision-by">by {decision.by}</span>
								{/if}
							</div>
							<p class="decision-text">{decision.decision}</p>
							{#if decision.rationale}
								<p class="decision-rationale">
									<strong>Rationale:</strong> {decision.rationale}
								</p>
							{/if}
						</div>
					{/each}
				</div>
			{:else}
				<div class="empty-state">
					<Icon name="message-circle" size={32} />
					<p>No decisions recorded yet</p>
					<button class="btn btn-secondary" onclick={openAddDecisionModal}>
						Record a Decision
					</button>
				</div>
			{/if}
		</section>
	</div>

	<!-- Edit Initiative Modal -->
	<Modal open={editModalOpen} onClose={() => (editModalOpen = false)} title="Edit Initiative">
		<form onsubmit={(e) => { e.preventDefault(); saveEdit(); }}>
			<div class="form-group">
				<label for="edit-title">Title</label>
				<input
					id="edit-title"
					type="text"
					bind:value={editTitle}
					required
				/>
			</div>

			<div class="form-group">
				<label for="edit-vision">Vision</label>
				<textarea
					id="edit-vision"
					bind:value={editVision}
					rows={3}
					placeholder="What is the goal of this initiative?"
				></textarea>
			</div>

			<div class="form-group">
				<label for="edit-status">Status</label>
				<select id="edit-status" bind:value={editStatus}>
					<option value="draft">Draft</option>
					<option value="active">Active</option>
					<option value="completed">Completed</option>
					<option value="archived">Archived</option>
				</select>
			</div>

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (editModalOpen = false)}>
					Cancel
				</button>
				<button type="submit" class="btn btn-primary">
					Save Changes
				</button>
			</div>
		</form>
	</Modal>

	<!-- Link Task Modal -->
	<Modal open={linkTaskModalOpen} onClose={() => (linkTaskModalOpen = false)} title="Link Existing Task">
		<div class="link-task-content">
			<div class="form-group">
				<label for="task-search">Search Tasks</label>
				<input
					id="task-search"
					type="text"
					bind:value={linkTaskSearch}
					placeholder="Search by ID or title..."
				/>
			</div>

			{#if linkTaskLoading}
				<div class="loading-inline">
					<div class="spinner-sm"></div>
					<span>Loading tasks...</span>
				</div>
			{:else if filteredAvailableTasks.length > 0}
				<div class="available-tasks">
					{#each filteredAvailableTasks as task (task.id)}
						<button class="available-task-item" onclick={() => linkTask(task.id)}>
							<span class="task-id">{task.id}</span>
							<span class="task-title">{task.title}</span>
							<span class="task-status-badge status-{task.status}">{task.status}</span>
						</button>
					{/each}
				</div>
			{:else}
				<p class="no-tasks-message">No available tasks to link</p>
			{/if}
		</div>
	</Modal>

	<!-- Add Decision Modal -->
	<Modal open={addDecisionModalOpen} onClose={() => (addDecisionModalOpen = false)} title="Add Decision">
		<form onsubmit={(e) => { e.preventDefault(); addDecision(); }}>
			<div class="form-group">
				<label for="decision-text">Decision</label>
				<textarea
					id="decision-text"
					bind:value={decisionText}
					rows={2}
					required
					placeholder="What was decided?"
				></textarea>
			</div>

			<div class="form-group">
				<label for="decision-rationale">Rationale (optional)</label>
				<textarea
					id="decision-rationale"
					bind:value={decisionRationale}
					rows={2}
					placeholder="Why was this decision made?"
				></textarea>
			</div>

			<div class="form-group">
				<label for="decision-by">Decided By (optional)</label>
				<input
					id="decision-by"
					type="text"
					bind:value={decisionBy}
					placeholder="Name or initials"
				/>
			</div>

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (addDecisionModalOpen = false)}>
					Cancel
				</button>
				<button type="submit" class="btn btn-primary" disabled={addingDecision || !decisionText.trim()}>
					{addingDecision ? 'Adding...' : 'Add Decision'}
				</button>
			</div>
		</form>
	</Modal>
{/if}

<style>
	.initiative-detail {
		max-width: 1000px;
		display: flex;
		flex-direction: column;
		gap: var(--space-6);
	}

	/* Back Link */
	.back-link {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-sm);
		color: var(--text-muted);
		text-decoration: none;
		transition: color var(--duration-fast);
	}

	.back-link:hover {
		color: var(--text-primary);
	}

	/* Header */
	.initiative-header {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		padding-bottom: var(--space-6);
		border-bottom: 1px solid var(--border-subtle);
	}

	.header-top {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--space-4);
	}

	.initiative-title {
		font-size: var(--text-2xl);
		font-weight: var(--font-bold);
		color: var(--text-primary);
		margin: 0;
	}

	.header-actions {
		display: flex;
		gap: var(--space-2);
	}

	.initiative-vision {
		font-size: var(--text-base);
		color: var(--text-secondary);
		margin: 0;
		line-height: var(--leading-relaxed);
	}

	.initiative-meta {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* Progress */
	.progress-section {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.progress-label {
		display: flex;
		justify-content: space-between;
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.progress-count {
		font-weight: var(--font-medium);
		color: var(--text-secondary);
	}

	.progress-bar {
		height: 8px;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background: var(--accent-primary);
		border-radius: var(--radius-full);
		transition: width var(--duration-normal);
	}

	/* Meta Grid */
	.meta-grid {
		display: flex;
		gap: var(--space-8);
	}

	.meta-item {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.meta-label {
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
	}

	.meta-value {
		font-size: var(--text-sm);
		color: var(--text-primary);
	}

	/* Status Badge */
	.status-badge {
		display: inline-flex;
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		text-transform: capitalize;
		border-radius: var(--radius-md);
	}

	.status-draft {
		background: var(--bg-tertiary);
		color: var(--text-muted);
	}

	.status-active {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.status-completed {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.status-archived {
		background: var(--bg-secondary);
		color: var(--text-muted);
	}

	/* Sections */
	.section {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-6);
	}

	.section-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: var(--space-4);
	}

	.section-header h2 {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.section-actions {
		display: flex;
		gap: var(--space-2);
	}

	/* Task List */
	.task-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.task-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		transition: border-color var(--duration-fast);
	}

	.task-item:hover {
		border-color: var(--border-default);
	}

	.task-link {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		flex: 1;
		text-decoration: none;
		color: inherit;
	}

	.task-status {
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.task-status.status-success {
		color: var(--status-success);
	}

	.task-status.status-running {
		color: var(--status-info);
	}

	.task-status.status-danger {
		color: var(--status-danger);
	}

	.task-status.status-warning {
		color: var(--status-warning);
	}

	.task-status.status-pending {
		color: var(--text-muted);
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.task-title {
		flex: 1;
		font-size: var(--text-sm);
		color: var(--text-primary);
	}

	.task-status-text {
		font-size: var(--text-xs);
		text-transform: capitalize;
		color: var(--text-muted);
	}

	.btn-remove {
		opacity: 0;
		transition: opacity var(--duration-fast);
	}

	.task-item:hover .btn-remove {
		opacity: 1;
	}

	/* Dependencies */
	.dependencies-section {
		margin-top: var(--space-4);
		padding-top: var(--space-4);
		border-top: 1px solid var(--border-subtle);
	}

	.dependencies-section h3 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		margin: 0 0 var(--space-2) 0;
	}

	.dependency-list {
		list-style: none;
		margin: 0;
		padding: 0;
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.dependency-list li {
		padding: var(--space-1) 0;
	}

	.dep-task {
		font-family: var(--font-mono);
		color: var(--text-primary);
	}

	.dep-arrow {
		color: var(--text-muted);
		padding: 0 var(--space-2);
	}

	.dep-targets {
		font-family: var(--font-mono);
		color: var(--accent-primary);
	}

	/* Decision List */
	.decision-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.decision-item {
		padding: var(--space-4);
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
	}

	.decision-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-bottom: var(--space-2);
	}

	.decision-id {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--accent-primary);
	}

	.decision-date {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.decision-by {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.decision-text {
		margin: 0 0 var(--space-2) 0;
		font-size: var(--text-sm);
		color: var(--text-primary);
		line-height: var(--leading-relaxed);
	}

	.decision-rationale {
		margin: 0;
		font-size: var(--text-sm);
		color: var(--text-secondary);
		font-style: italic;
	}

	/* Empty State */
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-8);
		color: var(--text-muted);
		text-align: center;
	}

	.empty-state p {
		margin: 0;
	}

	/* Loading / Error States */
	.loading-state,
	.error-state {
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

	.spinner-sm {
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

	.loading-state span {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.error-icon {
		width: 48px;
		height: 48px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--status-danger-bg);
		border-radius: 50%;
		font-size: var(--text-xl);
		font-weight: var(--font-bold);
		color: var(--status-danger);
	}

	.error-state p {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	/* Buttons */
	.btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-2) var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		border: 1px solid transparent;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast);
	}

	.btn-sm {
		padding: var(--space-1-5) var(--space-2-5);
		font-size: var(--text-xs);
	}

	.btn-primary {
		background: var(--accent-primary);
		color: white;
	}

	.btn-primary:hover {
		background: var(--accent-primary-hover);
	}

	.btn-secondary {
		background: var(--bg-secondary);
		border-color: var(--border-default);
		color: var(--text-primary);
	}

	.btn-secondary:hover {
		background: var(--bg-tertiary);
	}

	.btn-ghost {
		background: transparent;
		color: var(--text-muted);
	}

	.btn-ghost:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.btn-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast);
	}

	.btn-icon:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* Forms */
	.form-group {
		margin-bottom: var(--space-4);
	}

	.form-group label {
		display: block;
		margin-bottom: var(--space-1);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
	}

	.form-group input,
	.form-group textarea,
	.form-group select {
		width: 100%;
		padding: var(--space-2) var(--space-3);
		font-size: var(--text-sm);
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
	}

	.form-group input:focus,
	.form-group textarea:focus,
	.form-group select:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 2px var(--accent-primary-transparent);
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		margin-top: var(--space-6);
	}

	/* Link Task Modal */
	.link-task-content {
		min-height: 200px;
	}

	.loading-inline {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-8);
		color: var(--text-muted);
	}

	.available-tasks {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		max-height: 300px;
		overflow-y: auto;
	}

	.available-task-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		cursor: pointer;
		text-align: left;
		transition: all var(--duration-fast);
	}

	.available-task-item:hover {
		border-color: var(--accent-primary);
		background: var(--bg-secondary);
	}

	.available-task-item .task-id {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.available-task-item .task-title {
		flex: 1;
		font-size: var(--text-sm);
		color: var(--text-primary);
	}

	.task-status-badge {
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-2xs);
		text-transform: capitalize;
		border-radius: var(--radius-sm);
	}

	.task-status-badge.status-completed,
	.task-status-badge.status-finished {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.task-status-badge.status-running {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.task-status-badge.status-failed {
		background: var(--status-danger-bg);
		color: var(--status-danger);
	}

	.task-status-badge.status-paused,
	.task-status-badge.status-blocked {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.no-tasks-message {
		text-align: center;
		padding: var(--space-8);
		color: var(--text-muted);
	}
</style>
