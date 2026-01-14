<script lang="ts">
	import Column from './Column.svelte';
	import type { Task, Initiative } from '$lib/types';
	import { initiativeProgress } from '$lib/stores/initiative';

	interface SwimlaneColumn {
		id: string;
		title: string;
		phases: string[];
	}

	interface Props {
		initiative: Initiative | null; // null = unassigned
		tasks: Task[];
		columns: SwimlaneColumn[];
		tasksByColumn: Record<string, Task[]>;
		collapsed: boolean;
		onToggleCollapse: () => void;
		onDrop: (columnId: string, task: Task, targetInitiativeId: string | null) => void;
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onTaskClick?: (task: Task) => void;
	}

	let {
		initiative,
		tasks,
		columns,
		tasksByColumn,
		collapsed,
		onToggleCollapse,
		onDrop,
		onAction,
		onTaskClick
	}: Props = $props();

	let progress = $derived($initiativeProgress);

	// Get progress for this initiative
	let initiativeStats = $derived.by(() => {
		if (!initiative) {
			// For unassigned, count completed vs total from tasks
			const completed = tasks.filter(t =>
				t.status === 'completed' || t.status === 'finished'
			).length;
			return { completed, total: tasks.length };
		}
		const p = progress.get(initiative.id);
		return p ?? { completed: 0, total: tasks.length };
	});

	// Calculate progress percentage
	let progressPercent = $derived(
		initiativeStats.total > 0
			? Math.round((initiativeStats.completed / initiativeStats.total) * 100)
			: 0
	);

	// Title for display
	let displayTitle = $derived(initiative?.title ?? 'Unassigned');

	// Target initiative ID for drag-drop (null for unassigned)
	let targetInitiativeId = $derived(initiative?.id ?? null);

	function handleDrop(columnId: string, task: Task) {
		onDrop(columnId, task, targetInitiativeId);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			onToggleCollapse();
		}
	}
</script>

<div class="swimlane" class:collapsed>
	<button
		class="swimlane-header"
		onclick={onToggleCollapse}
		onkeydown={handleKeydown}
		aria-expanded={!collapsed}
		aria-controls="swimlane-content-{initiative?.id ?? 'unassigned'}"
	>
		<div class="header-left">
			<span class="collapse-icon" class:collapsed>
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="6 9 12 15 18 9" />
				</svg>
			</span>
			<h3 class="swimlane-title">{displayTitle}</h3>
			<span class="task-count">
				{initiativeStats.completed}/{initiativeStats.total}
			</span>
		</div>
		<div class="header-right">
			{#if initiativeStats.total > 0}
				<div class="progress-bar">
					<div class="progress-fill" style:width="{progressPercent}%"></div>
				</div>
				<span class="progress-text">{progressPercent}%</span>
			{/if}
		</div>
	</button>

	{#if !collapsed}
		<div
			class="swimlane-content"
			id="swimlane-content-{initiative?.id ?? 'unassigned'}"
		>
			{#each columns as column (column.id)}
				<Column
					{column}
					tasks={tasksByColumn[column.id] || []}
					onDrop={(task) => handleDrop(column.id, task)}
					{onAction}
					{onTaskClick}
				/>
			{/each}
		</div>
	{/if}
</div>

<style>
	.swimlane {
		display: flex;
		flex-direction: column;
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.swimlane:hover {
		border-color: var(--border-default);
	}

	.swimlane.collapsed {
		background: var(--bg-tertiary);
	}

	.swimlane-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-3) var(--space-4);
		background: var(--bg-secondary);
		border: none;
		border-bottom: 1px solid var(--border-subtle);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		width: 100%;
		text-align: left;
	}

	.swimlane-header:hover {
		background: var(--bg-tertiary);
	}

	.swimlane.collapsed .swimlane-header {
		border-bottom: none;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.collapse-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
		transition: transform var(--duration-fast) var(--ease-out);
	}

	.collapse-icon.collapsed {
		transform: rotate(-90deg);
	}

	.swimlane-title {
		margin: 0;
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
	}

	.task-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
		background: var(--bg-tertiary);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-full);
	}

	.header-right {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.progress-bar {
		width: 80px;
		height: 4px;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background: var(--status-success);
		border-radius: var(--radius-full);
		transition: width var(--duration-normal) var(--ease-out);
	}

	.progress-text {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
		min-width: 32px;
		text-align: right;
	}

	.swimlane-content {
		display: flex;
		gap: var(--space-2);
		padding: var(--space-3);
		overflow-x: auto;
		min-height: 150px;
	}

	/* Ensure columns in swimlane are sized appropriately */
	.swimlane-content :global(.column) {
		min-height: 120px;
	}

	.swimlane-content :global(.column-content) {
		min-height: 80px;
	}
</style>
