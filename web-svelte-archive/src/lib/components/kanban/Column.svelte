<script lang="ts">
	import TaskCard from './TaskCard.svelte';
	import type { Task } from '$lib/types';

	interface Props {
		column: { id: string; title: string; phases: string[] };
		tasks: Task[];
		onDrop: (task: Task) => void;
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onTaskClick?: (task: Task) => void;
		onFinalizeClick?: (task: Task) => void;
	}

	let { column, tasks, onDrop, onAction, onTaskClick, onFinalizeClick }: Props = $props();

	let dragOver = $state(false);
	let dragCounter = $state(0);

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		if (e.dataTransfer) {
			e.dataTransfer.dropEffect = 'move';
		}
	}

	function handleDragEnter(e: DragEvent) {
		e.preventDefault();
		dragCounter++;
		dragOver = true;
	}

	function handleDragLeave(e: DragEvent) {
		e.preventDefault();
		dragCounter--;
		if (dragCounter === 0) {
			dragOver = false;
		}
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		dragOver = false;
		dragCounter = 0;

		const taskData = e.dataTransfer?.getData('application/json');
		if (taskData) {
			try {
				const task = JSON.parse(taskData) as Task;
				onDrop(task);
			} catch (e) {
				console.warn('Invalid drop data:', e);
			}
		}
	}

	// Column-specific styling - matches column IDs from Board.svelte
	const columnStyles: Record<string, { accentColor: string; bgColor: string }> = {
		queued: { accentColor: 'var(--text-muted)', bgColor: 'rgba(148, 163, 184, 0.05)' },
		spec: { accentColor: 'rgb(59, 130, 246)', bgColor: 'rgba(59, 130, 246, 0.05)' },
		implement: { accentColor: 'var(--accent-primary)', bgColor: 'rgba(139, 92, 246, 0.05)' },
		test: { accentColor: 'rgb(6, 182, 212)', bgColor: 'rgba(6, 182, 212, 0.05)' },
		review: { accentColor: 'var(--status-warning)', bgColor: 'rgba(245, 158, 11, 0.05)' },
		done: { accentColor: 'var(--status-success)', bgColor: 'rgba(16, 185, 129, 0.05)' }
	};

	const style = $derived(columnStyles[column.id] || columnStyles.queued);
</script>

<div
	class="column"
	class:drag-over={dragOver}
	style:--column-accent={style.accentColor}
	style:--column-bg={style.bgColor}
	ondragover={handleDragOver}
	ondragenter={handleDragEnter}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
	role="region"
	aria-label="{column.title} column"
>
	<div class="column-header">
		<div class="header-left">
			<span class="header-indicator"></span>
			<h2>{column.title}</h2>
		</div>
		<span class="count">{tasks.length}</span>
	</div>

	<div class="column-content">
		{#each tasks as task (task.id)}
			<TaskCard {task} {onAction} {onTaskClick} {onFinalizeClick} />
		{/each}

		{#if tasks.length === 0}
			<div class="empty">
				<span class="empty-text">No tasks</span>
				<span class="empty-hint">Drag tasks here</span>
			</div>
		{/if}
	</div>
</div>

<style>
	.column {
		display: flex;
		flex-direction: column;
		min-width: 180px;
		flex: 1 0 180px;
		max-width: 300px;
		background: var(--column-bg);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.column.drag-over {
		background: var(--accent-subtle);
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 2px var(--accent-glow);
	}

	.column-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-3) var(--space-4);
		background: var(--bg-secondary);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.header-indicator {
		width: 8px;
		height: 8px;
		border-radius: var(--radius-full);
		background: var(--column-accent);
	}

	.column-header h2 {
		margin: 0;
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		text-transform: none;
		letter-spacing: normal;
	}

	.count {
		background: var(--bg-tertiary);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-full);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		min-width: 24px;
		text-align: center;
	}

	.column-content {
		flex: 1;
		padding: var(--space-3);
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		min-height: 200px;
	}

	.empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-6);
		text-align: center;
		min-height: 150px;
	}

	.empty-text {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin-bottom: var(--space-1);
	}

	.empty-hint {
		font-size: var(--text-xs);
		color: var(--text-disabled);
	}
</style>
