<script lang="ts">
	import TaskCard from './TaskCard.svelte';
	import type { Task } from '$lib/types';

	interface Props {
		column: { id: string; title: string; phases: string[] };
		activeTasks: Task[];
		backlogTasks: Task[];
		showBacklog: boolean;
		onToggleBacklog: () => void;
		onDrop: (task: Task) => void;
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onTaskClick?: (task: Task) => void;
	}

	let { column, activeTasks, backlogTasks, showBacklog, onToggleBacklog, onDrop, onAction, onTaskClick }: Props = $props();

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

	const activeCount = $derived(activeTasks.length);
	const backlogCount = $derived(backlogTasks.length);
	const totalCount = $derived(activeCount + backlogCount);
</script>

<div
	class="column"
	class:drag-over={dragOver}
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
		<span class="count">{totalCount}</span>
	</div>

	<div class="column-content">
		<!-- Active section -->
		<div class="section active-section">
			{#if activeCount > 0}
				{#each activeTasks as task (task.id)}
					<TaskCard {task} {onAction} {onTaskClick} />
				{/each}
			{:else}
				<div class="empty-section">
					<span class="empty-text">No active tasks</span>
				</div>
			{/if}
		</div>

		<!-- Backlog section (collapsible) -->
		{#if backlogCount > 0}
			<div class="backlog-divider">
				<button
					class="backlog-toggle"
					onclick={onToggleBacklog}
					aria-expanded={showBacklog}
					aria-controls="backlog-section"
				>
					<svg
						class="toggle-icon"
						class:expanded={showBacklog}
						xmlns="http://www.w3.org/2000/svg"
						width="12"
						height="12"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<polyline points="9 18 15 12 9 6" />
					</svg>
					<span class="backlog-label">Backlog</span>
					<span class="backlog-count">{backlogCount}</span>
				</button>
			</div>

			{#if showBacklog}
				<div class="section backlog-section" id="backlog-section">
					{#each backlogTasks as task (task.id)}
						<TaskCard {task} {onAction} {onTaskClick} />
					{/each}
				</div>
			{/if}
		{/if}

		{#if totalCount === 0}
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
		background: rgba(148, 163, 184, 0.05);
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
		background: var(--text-muted);
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

	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.backlog-divider {
		padding: var(--space-2) 0;
		margin-top: var(--space-2);
	}

	.backlog-toggle {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px dashed var(--border-default);
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: var(--text-xs);
		color: var(--text-secondary);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.backlog-toggle:hover {
		background: var(--bg-secondary);
		border-color: var(--border-strong);
		color: var(--text-primary);
	}

	.toggle-icon {
		transition: transform var(--duration-fast) var(--ease-out);
	}

	.toggle-icon.expanded {
		transform: rotate(90deg);
	}

	.backlog-label {
		font-weight: var(--font-medium);
	}

	.backlog-count {
		margin-left: auto;
		background: var(--bg-primary);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-full);
		font-size: var(--text-2xs);
		color: var(--text-muted);
	}

	.backlog-section {
		padding-top: var(--space-2);
		opacity: 0.85;
	}

	.backlog-section :global(.task-card) {
		border-style: dashed;
	}

	.empty-section {
		padding: var(--space-4);
		text-align: center;
	}

	.empty-section .empty-text {
		font-size: var(--text-xs);
		color: var(--text-muted);
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
