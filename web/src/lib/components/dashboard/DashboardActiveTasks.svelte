<script lang="ts">
	import type { Task } from '$lib/types';
	import TaskCard from '../TaskCard.svelte';

	interface Props {
		tasks: Task[];
	}

	let { tasks }: Props = $props();
</script>

{#if tasks.length > 0}
	<section class="tasks-section">
		<div class="section-header">
			<h2 class="section-title">Active Tasks</h2>
			<span class="section-count">{tasks.length}</span>
		</div>
		<div class="task-list">
			{#each tasks as task (task.id)}
				<TaskCard {task} compact />
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

	.section-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		padding: var(--space-0-5) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		color: var(--text-muted);
	}

	.task-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}
</style>
