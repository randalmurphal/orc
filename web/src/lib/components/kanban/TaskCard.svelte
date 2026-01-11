<script lang="ts">
	import { goto } from '$app/navigation';
	import StatusIndicator from '$lib/components/ui/StatusIndicator.svelte';
	import type { Task } from '$lib/types';

	interface Props {
		task: Task;
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
	}

	let { task, onAction }: Props = $props();

	let actionLoading = $state(false);
	let isDragging = $state(false);

	function handleDragStart(e: DragEvent) {
		if (e.dataTransfer) {
			e.dataTransfer.setData('application/json', JSON.stringify(task));
			e.dataTransfer.effectAllowed = 'move';
		}
		isDragging = true;
	}

	function handleDragEnd() {
		isDragging = false;
	}

	async function handleAction(action: 'run' | 'pause' | 'resume', e: MouseEvent) {
		e.stopPropagation();
		e.preventDefault();
		actionLoading = true;
		try {
			await onAction(task.id, action);
		} finally {
			actionLoading = false;
		}
	}

	function openTask(e: MouseEvent) {
		// Don't navigate if clicking on action buttons
		const target = e.target as HTMLElement;
		if (target.closest('.actions')) {
			return;
		}
		goto(`/tasks/${task.id}`);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			goto(`/tasks/${task.id}`);
		}
	}

	const weightConfig: Record<string, { color: string; bg: string }> = {
		trivial: { color: 'var(--weight-trivial)', bg: 'rgba(107, 114, 128, 0.15)' },
		small: { color: 'var(--weight-small)', bg: 'var(--status-success-bg)' },
		medium: { color: 'var(--weight-medium)', bg: 'var(--status-info-bg)' },
		large: { color: 'var(--weight-large)', bg: 'var(--status-warning-bg)' },
		greenfield: { color: 'var(--weight-greenfield)', bg: 'var(--accent-subtle)' }
	};

	const weight = $derived(weightConfig[task.weight] || weightConfig.small);
	const isRunning = $derived(task.status === 'running');

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
</script>

<div
	class="task-card"
	class:dragging={isDragging}
	class:running={isRunning}
	draggable="true"
	ondragstart={handleDragStart}
	ondragend={handleDragEnd}
	onclick={openTask}
	onkeydown={handleKeydown}
	role="button"
	tabindex="0"
>
	<div class="card-header">
		<span class="task-id">{task.id}</span>
		<StatusIndicator status={task.status} size="sm" />
	</div>

	<h3 class="task-title">{task.title}</h3>

	{#if task.current_phase}
		<div class="task-phase">
			<span class="phase-label">Phase:</span>
			<span class="phase-value">{task.current_phase}</span>
		</div>
	{/if}

	<div class="card-footer">
		<div class="footer-left">
			<span
				class="weight-badge"
				style:color={weight.color}
				style:background={weight.bg}
			>
				{task.weight}
			</span>
			<span class="updated-time">{formatDate(task.updated_at)}</span>
		</div>

		<div class="actions">
			{#if task.status === 'created' || task.status === 'planned'}
				<button
					class="action-btn run"
					onclick={(e) => handleAction('run', e)}
					disabled={actionLoading}
					title="Run task"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<polygon points="5 3 19 12 5 21 5 3" />
					</svg>
				</button>
			{:else if task.status === 'running'}
				<button
					class="action-btn pause"
					onclick={(e) => handleAction('pause', e)}
					disabled={actionLoading}
					title="Pause task"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<rect x="6" y="4" width="4" height="16" rx="1" />
						<rect x="14" y="4" width="4" height="16" rx="1" />
					</svg>
				</button>
			{:else if task.status === 'paused'}
				<button
					class="action-btn resume"
					onclick={(e) => handleAction('resume', e)}
					disabled={actionLoading}
					title="Resume task"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<polygon points="5 3 19 12 5 21 5 3" />
					</svg>
				</button>
			{/if}
		</div>
	</div>
</div>

<style>
	.task-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-3);
		cursor: grab;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			box-shadow var(--duration-fast) var(--ease-out),
			transform var(--duration-fast) var(--ease-out),
			opacity var(--duration-fast) var(--ease-out);
	}

	.task-card:hover {
		border-color: var(--border-strong);
		box-shadow: var(--shadow-md);
	}

	.task-card:focus-visible {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.task-card:active {
		cursor: grabbing;
	}

	.task-card.dragging {
		opacity: 0.5;
		transform: rotate(2deg) scale(1.02);
		box-shadow: var(--shadow-lg);
	}

	.task-card.running {
		border-color: var(--accent-primary);
		animation: card-pulse 2.5s ease-in-out infinite;
	}

	@keyframes card-pulse {
		0%,
		100% {
			box-shadow: 0 0 0 0 var(--accent-glow);
		}
		50% {
			box-shadow: 0 0 0 4px var(--accent-glow);
		}
	}

	.card-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: var(--space-2);
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		letter-spacing: var(--tracking-wide);
	}

	.task-title {
		margin: 0 0 var(--space-2);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		line-height: var(--leading-snug);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		text-transform: none;
		letter-spacing: normal;
	}

	.task-phase {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		margin-bottom: var(--space-3);
		font-size: var(--text-xs);
	}

	.phase-label {
		color: var(--text-muted);
	}

	.phase-value {
		color: var(--accent-primary);
		font-weight: var(--font-medium);
	}

	.card-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.footer-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.weight-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-sm);
	}

	.updated-time {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.actions {
		display: flex;
		gap: var(--space-1);
	}

	.action-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.action-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.action-btn.run {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.action-btn.run:hover:not(:disabled) {
		background: var(--status-success);
		color: white;
	}

	.action-btn.pause {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.action-btn.pause:hover:not(:disabled) {
		background: var(--status-warning);
		color: white;
	}

	.action-btn.resume {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.action-btn.resume:hover:not(:disabled) {
		background: var(--status-info);
		color: white;
	}
</style>
