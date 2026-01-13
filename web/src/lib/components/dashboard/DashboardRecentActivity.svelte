<script lang="ts">
	import type { Task } from '$lib/types';
	import StatusIndicator from '../ui/StatusIndicator.svelte';

	interface Props {
		tasks: Task[];
	}

	let { tasks }: Props = $props();

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return date.toLocaleDateString();
	}
</script>

{#if tasks.length > 0}
	<section class="tasks-section">
		<div class="section-header">
			<h2 class="section-title">Recent Activity</h2>
		</div>
		<div class="activity-list">
			{#each tasks as task (task.id)}
				<a href="/tasks/{task.id}" class="activity-item">
					<div class="activity-status">
						<StatusIndicator status={task.status} size="md" />
					</div>
					<div class="activity-content">
						<span class="activity-id">{task.id}</span>
						<span class="activity-title">{task.title}</span>
					</div>
					<span class="activity-time">{formatRelativeTime(task.updated_at)}</span>
				</a>
			{/each}
		</div>
	</section>
{/if}

<style>
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
		margin-bottom: var(--space-4);
	}

	.activity-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.activity-item {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		text-decoration: none;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.activity-item:hover {
		border-color: var(--accent-primary);
		background: var(--bg-tertiary);
	}

	.activity-status {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
	}

	.activity-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
		overflow: hidden;
	}

	.activity-id {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
	}

	.activity-title {
		font-size: var(--text-sm);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.activity-time {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}
</style>
