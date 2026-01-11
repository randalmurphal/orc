<script lang="ts">
	import type { DashboardStats } from '$lib/api';

	interface Props {
		stats: DashboardStats;
	}

	let { stats }: Props = $props();
</script>

<section class="summary-section">
	<div class="summary-stats">
		<div class="summary-item">
			<span class="summary-label">Total Tasks</span>
			<span class="summary-value">{stats.total}</span>
		</div>
		<div class="summary-item">
			<span class="summary-label">Completed</span>
			<span class="summary-value success">{stats.completed}</span>
		</div>
		<div class="summary-item">
			<span class="summary-label">Failed</span>
			<span class="summary-value danger">{stats.failed}</span>
		</div>
	</div>
</section>

<style>
	.summary-section {
		padding-top: var(--space-4);
		border-top: 1px solid var(--border-subtle);
	}

	.summary-stats {
		display: flex;
		gap: var(--space-8);
	}

	.summary-item {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.summary-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.summary-value {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		font-family: var(--font-mono);
		color: var(--text-primary);
	}

	.summary-value.success {
		color: var(--status-success);
	}

	.summary-value.danger {
		color: var(--status-danger);
	}
</style>
