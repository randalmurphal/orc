<script lang="ts">
	import { goto } from '$app/navigation';
	import type { Initiative } from '$lib/types';

	interface Props {
		initiatives: Initiative[];
	}

	let { initiatives }: Props = $props();

	// Calculate progress for an initiative
	function getProgress(initiative: Initiative): { completed: number; total: number; percent: number } {
		const tasks = initiative.tasks || [];
		const total = tasks.length;
		if (total === 0) return { completed: 0, total: 0, percent: 0 };

		const completed = tasks.filter((t) => t.status === 'completed' || t.status === 'finished').length;
		const percent = Math.round((completed / total) * 100);
		return { completed, total, percent };
	}

	// Get color class based on progress percentage
	function getProgressColor(percent: number): string {
		if (percent >= 75) return 'progress-high';
		if (percent >= 25) return 'progress-medium';
		return 'progress-low';
	}

	// Navigate to board filtered by initiative
	function handleInitiativeClick(initiativeId: string) {
		goto(`/board?initiative=${initiativeId}`);
	}

	function handleViewAll() {
		// Navigate to board with no filter (shows all) or future initiatives page
		goto('/board');
	}

	// Truncate title if too long
	function truncateTitle(title: string, maxLength: number = 30): string {
		if (title.length <= maxLength) return title;
		return title.slice(0, maxLength - 1) + '…';
	}

	// Sort by progress (most active) or most recently updated
	let sortedInitiatives = $derived(
		[...initiatives].sort((a, b) => {
			// Sort by updated_at descending (most recent first)
			return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
		}).slice(0, 5)
	);

	let hasMore = $derived(initiatives.length > 5);
</script>

{#if initiatives.length > 0}
	<section class="initiatives-section">
		<div class="section-header">
			<h2 class="section-title">Active Initiatives</h2>
			<span class="section-count">{initiatives.length}</span>
		</div>

		<div class="initiatives-list">
			{#each sortedInitiatives as initiative (initiative.id)}
				{@const progress = getProgress(initiative)}
				<button
					class="initiative-row"
					onclick={() => handleInitiativeClick(initiative.id)}
					title={initiative.vision ? `${initiative.title}\n\n${initiative.vision}` : initiative.title}
				>
					<span class="initiative-title">{truncateTitle(initiative.title)}</span>
					{#if initiative.status !== 'active'}
						<span class="initiative-status status-{initiative.status}">{initiative.status}</span>
					{:else}
						<div class="progress-container">
							<div class="progress-bar">
								<div
									class="progress-fill {getProgressColor(progress.percent)}"
									style="width: {progress.percent}%"
								></div>
							</div>
							<span class="progress-count">{progress.completed}/{progress.total}</span>
						</div>
					{/if}
				</button>
			{/each}
		</div>

		{#if hasMore}
			<button class="view-all-link" onclick={handleViewAll}>
				View All →
			</button>
		{/if}
	</section>
{/if}

<style>
	.initiatives-section {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.section-title {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
	}

	.section-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		padding: var(--space-0-5) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		color: var(--text-muted);
	}

	.initiatives-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		padding: var(--space-2);
	}

	.initiative-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		padding: var(--space-2-5) var(--space-3);
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		text-align: left;
		width: 100%;
	}

	.initiative-row:hover {
		border-color: var(--accent-primary);
		background: var(--bg-secondary);
	}

	.initiative-title {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		flex-shrink: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.progress-container {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex-shrink: 0;
	}

	.progress-bar {
		width: 80px;
		height: 8px;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		border-radius: var(--radius-full);
		transition: width var(--duration-normal) var(--ease-out);
	}

	.progress-fill.progress-high {
		background: var(--status-success);
	}

	.progress-fill.progress-medium {
		background: var(--status-warning);
	}

	.progress-fill.progress-low {
		background: var(--text-muted);
	}

	.progress-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
		min-width: 36px;
		text-align: right;
	}

	.view-all-link {
		align-self: flex-end;
		font-size: var(--text-sm);
		color: var(--accent-primary);
		background: none;
		border: none;
		padding: var(--space-1) var(--space-2);
		cursor: pointer;
		transition: color var(--duration-fast) var(--ease-out);
	}

	.view-all-link:hover {
		color: var(--accent-hover);
		text-decoration: underline;
	}

	.initiative-status {
		font-size: var(--text-xs);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-sm);
		text-transform: capitalize;
		flex-shrink: 0;
	}

	.initiative-status.status-draft {
		background: var(--bg-tertiary);
		color: var(--text-muted);
	}

	.initiative-status.status-completed {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.initiative-status.status-archived {
		background: var(--bg-tertiary);
		color: var(--text-muted);
		opacity: 0.7;
	}
</style>
