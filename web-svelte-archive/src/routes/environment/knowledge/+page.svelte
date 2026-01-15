<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import {
		listKnowledge,
		getKnowledgeStatus,
		listStaleKnowledge,
		approveKnowledge,
		approveAllKnowledge,
		rejectKnowledge,
		validateKnowledge,
		deleteKnowledge,
		type KnowledgeEntry,
		type KnowledgeStatusResponse,
		type KnowledgeStatus
	} from '$lib/api';

	// State
	let loading = $state(true);
	let error = $state<string | null>(null);
	let entries = $state<KnowledgeEntry[]>([]);
	let status = $state<KnowledgeStatusResponse | null>(null);
	let staleEntries = $state<KnowledgeEntry[]>([]);

	// Filters
	let filterStatus = $state<KnowledgeStatus | 'all'>('all');
	let filterType = $state<'pattern' | 'gotcha' | 'decision' | 'all'>('all');

	// Action states
	let actionInProgress = $state<string | null>(null);

	async function loadData() {
		loading = true;
		error = null;
		try {
			const [entriesRes, statusRes, staleRes] = await Promise.all([
				listKnowledge(),
				getKnowledgeStatus(),
				listStaleKnowledge(90)
			]);
			entries = entriesRes;
			status = statusRes;
			staleEntries = staleRes;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load knowledge data';
		} finally {
			loading = false;
		}
	}

	onMount(loadData);

	// Filtered entries
	const filteredEntries = $derived(
		entries.filter((e) => {
			if (filterStatus !== 'all' && e.status !== filterStatus) return false;
			if (filterType !== 'all' && e.type !== filterType) return false;
			return true;
		})
	);

	// Group by type
	const patterns = $derived(filteredEntries.filter((e) => e.type === 'pattern'));
	const gotchas = $derived(filteredEntries.filter((e) => e.type === 'gotcha'));
	const decisions = $derived(filteredEntries.filter((e) => e.type === 'decision'));

	// Actions
	async function handleApprove(id: string) {
		actionInProgress = id;
		try {
			await approveKnowledge(id);
			await loadData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to approve';
		} finally {
			actionInProgress = null;
		}
	}

	async function handleApproveAll() {
		actionInProgress = 'all';
		try {
			await approveAllKnowledge();
			await loadData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to approve all';
		} finally {
			actionInProgress = null;
		}
	}

	async function handleReject(id: string) {
		const reason = prompt('Reason for rejection:');
		if (reason === null) return;

		actionInProgress = id;
		try {
			await rejectKnowledge(id, reason);
			await loadData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to reject';
		} finally {
			actionInProgress = null;
		}
	}

	async function handleValidate(id: string) {
		actionInProgress = id;
		try {
			await validateKnowledge(id);
			await loadData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to validate';
		} finally {
			actionInProgress = null;
		}
	}

	async function handleDelete(id: string) {
		if (!confirm('Delete this knowledge entry?')) return;

		actionInProgress = id;
		try {
			await deleteKnowledge(id);
			await loadData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete';
		} finally {
			actionInProgress = null;
		}
	}

	function isStale(entry: KnowledgeEntry): boolean {
		return staleEntries.some((s) => s.id === entry.id);
	}

	function getTypeIcon(type: string): string {
		switch (type) {
			case 'pattern':
				return 'code';
			case 'gotcha':
				return 'warning';
			case 'decision':
				return 'settings';
			default:
				return 'info';
		}
	}

	function getStatusClass(status: string): string {
		switch (status) {
			case 'pending':
				return 'status-pending';
			case 'approved':
				return 'status-approved';
			case 'rejected':
				return 'status-rejected';
			default:
				return '';
		}
	}
</script>

<div class="knowledge-page">
	<header class="page-header">
		<div class="header-content">
			<h1>Knowledge Queue</h1>
			<p class="subtitle">Patterns, gotchas, and decisions learned during development</p>
		</div>
		<div class="header-actions">
			{#if status && status.pending_count > 0}
				<button
					class="btn btn-primary"
					onclick={handleApproveAll}
					disabled={actionInProgress === 'all'}
				>
					{#if actionInProgress === 'all'}
						Approving...
					{:else}
						Approve All ({status.pending_count})
					{/if}
				</button>
			{/if}
			<button class="btn btn-secondary" onclick={loadData} disabled={loading}>
				<Icon name="refresh" size={16} />
				Refresh
			</button>
		</div>
	</header>

	{#if error}
		<div class="error-banner">
			<Icon name="warning" size={16} />
			{error}
			<button onclick={() => (error = null)}>Dismiss</button>
		</div>
	{/if}

	{#if loading}
		<div class="loading">Loading knowledge data...</div>
	{:else}
		<!-- Status cards -->
		{#if status}
			<div class="status-cards">
				<div class="status-card" class:highlight={status.pending_count > 0}>
					<div class="card-icon pending">
						<Icon name="clock" size={24} />
					</div>
					<div class="card-content">
						<span class="card-value">{status.pending_count}</span>
						<span class="card-label">Pending</span>
					</div>
				</div>
				<div class="status-card" class:highlight={status.stale_count > 0}>
					<div class="card-icon stale">
						<Icon name="warning" size={24} />
					</div>
					<div class="card-content">
						<span class="card-value">{status.stale_count}</span>
						<span class="card-label">Stale</span>
					</div>
				</div>
				<div class="status-card">
					<div class="card-icon approved">
						<Icon name="check" size={24} />
					</div>
					<div class="card-content">
						<span class="card-value">{status.approved_count}</span>
						<span class="card-label">Approved</span>
					</div>
				</div>
			</div>
		{/if}

		<!-- Filters -->
		<div class="filters">
			<div class="filter-group">
				<label for="filter-status">Status:</label>
				<select id="filter-status" bind:value={filterStatus}>
					<option value="all">All</option>
					<option value="pending">Pending</option>
					<option value="approved">Approved</option>
					<option value="rejected">Rejected</option>
				</select>
			</div>
			<div class="filter-group">
				<label for="filter-type">Type:</label>
				<select id="filter-type" bind:value={filterType}>
					<option value="all">All</option>
					<option value="pattern">Patterns</option>
					<option value="gotcha">Gotchas</option>
					<option value="decision">Decisions</option>
				</select>
			</div>
		</div>

		<!-- Entries list -->
		{#if filteredEntries.length === 0}
			<div class="empty-state">
				<Icon name="database" size={48} />
				<p>No knowledge entries found</p>
				<p class="empty-hint">
					Knowledge is captured during task execution via the docs phase.
				</p>
			</div>
		{:else}
			<div class="entries-table">
				<table>
					<thead>
						<tr>
							<th>Type</th>
							<th>Name</th>
							<th>Description</th>
							<th>Source</th>
							<th>Status</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each filteredEntries as entry (entry.id)}
							<tr class:stale={isStale(entry)}>
								<td class="type-cell">
									<span class="type-badge type-{entry.type}">
										<Icon name={getTypeIcon(entry.type)} size={14} />
										{entry.type}
									</span>
								</td>
								<td class="name-cell">
									<span class="entry-name">{entry.name}</span>
									{#if isStale(entry)}
										<span class="stale-badge" title="Needs validation">Stale</span>
									{/if}
								</td>
								<td class="desc-cell">
									<span class="entry-desc">{entry.description}</span>
								</td>
								<td class="source-cell">
									{entry.source_task || '-'}
								</td>
								<td class="status-cell">
									<span class="status-badge {getStatusClass(entry.status)}">
										{entry.status}
									</span>
								</td>
								<td class="actions-cell">
									{#if entry.status === 'pending'}
										<button
											class="action-btn approve"
											onclick={() => handleApprove(entry.id)}
											disabled={actionInProgress === entry.id}
											title="Approve"
										>
											<Icon name="check" size={14} />
										</button>
										<button
											class="action-btn reject"
											onclick={() => handleReject(entry.id)}
											disabled={actionInProgress === entry.id}
											title="Reject"
										>
											<Icon name="x" size={14} />
										</button>
									{:else if entry.status === 'approved'}
										<button
											class="action-btn validate"
											onclick={() => handleValidate(entry.id)}
											disabled={actionInProgress === entry.id}
											title="Validate (mark as still relevant)"
										>
											<Icon name="refresh" size={14} />
										</button>
									{/if}
									<button
										class="action-btn delete"
										onclick={() => handleDelete(entry.id)}
										disabled={actionInProgress === entry.id}
										title="Delete"
									>
										<Icon name="trash" size={14} />
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</div>

<style>
	.knowledge-page {
		padding: 1.5rem;
		max-width: 1400px;
		margin: 0 auto;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 1.5rem;
	}

	.header-content h1 {
		margin: 0;
		font-size: 1.5rem;
		font-weight: 600;
	}

	.subtitle {
		margin: 0.25rem 0 0;
		color: var(--text-secondary, #666);
		font-size: 0.875rem;
	}

	.header-actions {
		display: flex;
		gap: 0.5rem;
	}

	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.5rem 1rem;
		border: none;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.15s, opacity 0.15s;
	}

	.btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--accent, #3b82f6);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--accent-hover, #2563eb);
	}

	.btn-secondary {
		background: var(--bg-secondary, #f3f4f6);
		color: var(--text-primary, #111);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--bg-tertiary, #e5e7eb);
	}

	.error-banner {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		background: #fef2f2;
		border: 1px solid #fecaca;
		border-radius: 6px;
		color: #b91c1c;
		margin-bottom: 1rem;
	}

	.error-banner button {
		margin-left: auto;
		background: none;
		border: none;
		color: inherit;
		cursor: pointer;
		text-decoration: underline;
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary, #666);
	}

	.status-cards {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.status-card {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 1rem;
		background: var(--bg-secondary, #f9fafb);
		border: 1px solid var(--border, #e5e7eb);
		border-radius: 8px;
	}

	.status-card.highlight {
		border-color: var(--accent, #3b82f6);
		background: #eff6ff;
	}

	.card-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 48px;
		height: 48px;
		border-radius: 8px;
	}

	.card-icon.pending {
		background: #fef3c7;
		color: #d97706;
	}

	.card-icon.stale {
		background: #fee2e2;
		color: #dc2626;
	}

	.card-icon.approved {
		background: #dcfce7;
		color: #16a34a;
	}

	.card-content {
		display: flex;
		flex-direction: column;
	}

	.card-value {
		font-size: 1.5rem;
		font-weight: 600;
	}

	.card-label {
		font-size: 0.75rem;
		color: var(--text-secondary, #666);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.filters {
		display: flex;
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.filter-group {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.filter-group label {
		font-size: 0.875rem;
		color: var(--text-secondary, #666);
	}

	.filter-group select {
		padding: 0.375rem 0.75rem;
		border: 1px solid var(--border, #e5e7eb);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary, white);
	}

	.empty-state {
		text-align: center;
		padding: 4rem 2rem;
		color: var(--text-secondary, #666);
	}

	.empty-state p {
		margin: 0.5rem 0 0;
	}

	.empty-hint {
		font-size: 0.875rem;
		opacity: 0.8;
	}

	.entries-table {
		border: 1px solid var(--border, #e5e7eb);
		border-radius: 8px;
		overflow: hidden;
	}

	.entries-table table {
		width: 100%;
		border-collapse: collapse;
	}

	.entries-table th {
		text-align: left;
		padding: 0.75rem 1rem;
		background: var(--bg-secondary, #f9fafb);
		border-bottom: 1px solid var(--border, #e5e7eb);
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-secondary, #666);
	}

	.entries-table td {
		padding: 0.75rem 1rem;
		border-bottom: 1px solid var(--border, #e5e7eb);
		font-size: 0.875rem;
	}

	.entries-table tr:last-child td {
		border-bottom: none;
	}

	.entries-table tr.stale {
		background: #fffbeb;
	}

	.type-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.75rem;
		font-weight: 500;
		text-transform: capitalize;
	}

	.type-badge.type-pattern {
		background: #dbeafe;
		color: #1d4ed8;
	}

	.type-badge.type-gotcha {
		background: #fef3c7;
		color: #b45309;
	}

	.type-badge.type-decision {
		background: #f3e8ff;
		color: #7c3aed;
	}

	.name-cell {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.entry-name {
		font-weight: 500;
	}

	.stale-badge {
		padding: 0.125rem 0.375rem;
		background: #fee2e2;
		color: #dc2626;
		font-size: 0.625rem;
		font-weight: 600;
		text-transform: uppercase;
		border-radius: 4px;
	}

	.desc-cell {
		max-width: 300px;
	}

	.entry-desc {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		color: var(--text-secondary, #666);
	}

	.source-cell {
		font-family: monospace;
		font-size: 0.75rem;
		color: var(--text-secondary, #666);
	}

	.status-badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.75rem;
		font-weight: 500;
		text-transform: capitalize;
	}

	.status-badge.status-pending {
		background: #fef3c7;
		color: #b45309;
	}

	.status-badge.status-approved {
		background: #dcfce7;
		color: #16a34a;
	}

	.status-badge.status-rejected {
		background: #fee2e2;
		color: #dc2626;
	}

	.actions-cell {
		display: flex;
		gap: 0.25rem;
	}

	.action-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		transition: background-color 0.15s, opacity 0.15s;
	}

	.action-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.action-btn.approve {
		background: #dcfce7;
		color: #16a34a;
	}

	.action-btn.approve:hover:not(:disabled) {
		background: #bbf7d0;
	}

	.action-btn.reject {
		background: #fee2e2;
		color: #dc2626;
	}

	.action-btn.reject:hover:not(:disabled) {
		background: #fecaca;
	}

	.action-btn.validate {
		background: #dbeafe;
		color: #1d4ed8;
	}

	.action-btn.validate:hover:not(:disabled) {
		background: #bfdbfe;
	}

	.action-btn.delete {
		background: var(--bg-secondary, #f3f4f6);
		color: var(--text-secondary, #666);
	}

	.action-btn.delete:hover:not(:disabled) {
		background: #fee2e2;
		color: #dc2626;
	}
</style>
