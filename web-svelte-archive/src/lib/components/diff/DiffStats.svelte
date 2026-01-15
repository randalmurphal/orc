<script lang="ts">
	import type { DiffStats } from '$lib/types';

	interface Props {
		stats: DiffStats;
	}

	let { stats }: Props = $props();

	// Visual bar representing add/delete ratio
	const addPercent = $derived.by(() => {
		const total = stats.additions + stats.deletions;
		if (total === 0) return 50;
		return Math.round((stats.additions / total) * 100);
	});
</script>

<div class="diff-stats">
	<span class="stat-files">{stats.files_changed} file{stats.files_changed !== 1 ? 's' : ''}</span>
	<div class="stat-changes">
		<span class="additions">+{stats.additions}</span>
		<span class="deletions">-{stats.deletions}</span>
	</div>
	{#if stats.additions + stats.deletions > 0}
		<div class="change-bar" title="{addPercent}% additions, {100 - addPercent}% deletions">
			<div class="bar-fill additions-bar" style:width="{addPercent}%"></div>
			<div class="bar-fill deletions-bar" style:width="{100 - addPercent}%"></div>
		</div>
	{/if}
</div>

<style>
	.diff-stats {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		font-size: var(--text-xs);
	}

	.stat-files {
		color: var(--text-muted);
	}

	.stat-changes {
		display: flex;
		gap: var(--space-2);
		font-weight: var(--font-medium);
		font-family: var(--font-mono);
	}

	.additions {
		color: var(--status-success);
	}

	.deletions {
		color: var(--status-danger);
	}

	.change-bar {
		display: flex;
		width: 60px;
		height: 6px;
		border-radius: var(--radius-full);
		overflow: hidden;
		background: var(--bg-tertiary);
	}

	.bar-fill {
		height: 100%;
		transition: width var(--duration-normal) var(--ease-out);
	}

	.additions-bar {
		background: var(--status-success);
	}

	.deletions-bar {
		background: var(--status-danger);
	}
</style>
