<script lang="ts">
	import type { Task } from '$lib/types';

	interface Props {
		task: Task;
		onRun?: () => void;
		onPause?: () => void;
		onResume?: () => void;
		onDelete?: () => void;
	}

	let { task, onRun, onPause, onResume, onDelete }: Props = $props();

	const statusColors: Record<string, string> = {
		created: 'var(--text-secondary)',
		classifying: 'var(--accent-warning)',
		planned: 'var(--text-secondary)',
		running: 'var(--accent-primary)',
		paused: 'var(--accent-warning)',
		blocked: 'var(--accent-danger)',
		completed: 'var(--accent-success)',
		failed: 'var(--accent-danger)'
	};

	const weightColors: Record<string, string> = {
		trivial: '#6e7681',
		small: '#3fb950',
		medium: '#58a6ff',
		large: '#d29922',
		greenfield: '#a371f7'
	};

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMins / 60);
		const diffDays = Math.floor(diffHours / 24);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return date.toLocaleDateString();
	}

	function handlePause(e: Event) {
		e.stopPropagation();
		e.preventDefault();
		onPause?.();
	}

	function handleRun(e: Event) {
		e.stopPropagation();
		e.preventDefault();
		onRun?.();
	}

	function handleDelete(e: Event) {
		e.stopPropagation();
		e.preventDefault();
		if (confirm(`Delete task ${task.id}?`)) {
			onDelete?.();
		}
	}

	function handleResume(e: Event) {
		e.stopPropagation();
		e.preventDefault();
		onResume?.();
	}
</script>

<a href="/tasks/{task.id}" class="task-card">
	<div class="task-main">
		<div class="task-header">
			<span class="task-id">{task.id}</span>
			<span class="task-weight" style="background: {weightColors[task.weight]}">
				{task.weight}
			</span>
		</div>
		<h3 class="task-title">{task.title}</h3>
		{#if task.current_phase}
			<div class="task-phase">Phase: {task.current_phase}</div>
		{/if}
	</div>

	<div class="task-meta">
		<span class="task-status" style="color: {statusColors[task.status]}">
			{task.status}
		</span>
		<span class="task-time">{formatDate(task.updated_at)}</span>
	</div>

	<div class="task-actions">
		{#if task.status === 'running' && onPause}
			<button class="control-btn" onclick={handlePause}>Pause</button>
		{:else if task.status === 'paused' && onResume}
			<button class="control-btn primary" onclick={handleResume}>Resume</button>
		{:else if ['created', 'planned'].includes(task.status) && onRun}
			<button class="control-btn primary" onclick={handleRun}>Run</button>
		{/if}
		{#if task.status !== 'running' && onDelete}
			<button class="control-btn delete" onclick={handleDelete} title="Delete task">
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="3 6 5 6 21 6"></polyline>
					<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
				</svg>
			</button>
		{/if}
	</div>
</a>

<style>
	.task-card {
		display: flex;
		align-items: center;
		gap: 1rem;
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
		border-radius: 8px;
		padding: 1rem;
		text-decoration: none;
		color: inherit;
		transition: border-color 0.2s;
	}

	.task-card:hover {
		border-color: var(--accent-primary);
		text-decoration: none;
	}

	.task-main {
		flex: 1;
		min-width: 0;
	}

	.task-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.25rem;
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.task-weight {
		font-size: 0.625rem;
		font-weight: 600;
		text-transform: uppercase;
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		color: #fff;
	}

	.task-title {
		font-size: 0.9375rem;
		font-weight: 500;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.task-phase {
		font-size: 0.75rem;
		color: var(--text-secondary);
		margin-top: 0.25rem;
	}

	.task-meta {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.25rem;
		font-size: 0.75rem;
	}

	.task-status {
		font-weight: 500;
		text-transform: uppercase;
	}

	.task-time {
		color: var(--text-muted);
	}

	.task-actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}

	.control-btn {
		padding: 0.375rem 0.75rem;
		font-size: 0.75rem;
	}

	.control-btn.delete {
		padding: 0.375rem;
		background: transparent;
		border-color: var(--border-color);
		color: var(--text-secondary);
	}

	.control-btn.delete:hover {
		background: var(--accent-danger);
		border-color: var(--accent-danger);
		color: white;
	}
</style>
