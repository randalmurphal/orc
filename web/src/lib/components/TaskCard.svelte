<script lang="ts">
	import type { Task } from '$lib/types';
	import StatusIndicator from './ui/StatusIndicator.svelte';
	import { getInitiativeBadgeTitle } from '$lib/stores/initiatives';

	interface Props {
		task: Task;
		compact?: boolean;
		onRun?: () => void;
		onPause?: () => void;
		onResume?: () => void;
		onDelete?: () => void;
		onTaskClick?: (task: Task) => void;
		onInitiativeClick?: (initiativeId: string) => void;
	}

	let { task, compact = false, onRun, onPause, onResume, onDelete, onTaskClick, onInitiativeClick }: Props = $props();

	const weightConfig: Record<string, { color: string; bg: string }> = {
		trivial: { color: 'var(--weight-trivial)', bg: 'rgba(107, 114, 128, 0.15)' },
		small: { color: 'var(--weight-small)', bg: 'var(--status-success-bg)' },
		medium: { color: 'var(--weight-medium)', bg: 'var(--status-info-bg)' },
		large: { color: 'var(--weight-large)', bg: 'var(--status-warning-bg)' },
		greenfield: { color: 'var(--weight-greenfield)', bg: 'var(--accent-subtle)' }
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

	const isRunning = $derived(task.status === 'running');
	const weight = $derived(weightConfig[task.weight] || weightConfig.small);
	const initiativeBadge = $derived(task.initiative_id ? getInitiativeBadgeTitle(task.initiative_id) : null);

	function handleInitiativeClick(e: MouseEvent) {
		e.stopPropagation();
		e.preventDefault();
		if (task.initiative_id && onInitiativeClick) {
			onInitiativeClick(task.initiative_id);
		}
	}

	function handleCardClick(e: MouseEvent) {
		// For running tasks, show transcript modal if callback provided
		if (task.status === 'running' && onTaskClick) {
			e.preventDefault();
			onTaskClick(task);
		}
	}
</script>

<a href="/tasks/{task.id}" class="task-card" class:running={isRunning} onclick={handleCardClick}>
	<!-- Left: Status Orb -->
	<div class="status-col">
		<StatusIndicator status={task.status} size="lg" />
	</div>

	<!-- Middle: Task Info -->
	<div class="task-info">
		<div class="task-header">
			<span class="task-id">{task.id}</span>
			{#if task.weight}
				<span
					class="weight-badge"
					style:color={weight.color}
					style:background={weight.bg}
				>
					{task.weight}
				</span>
			{/if}
			{#if initiativeBadge}
				<button
					class="initiative-badge"
					onclick={handleInitiativeClick}
					title={initiativeBadge.full}
					type="button"
				>
					{initiativeBadge.display}
				</button>
			{/if}
		</div>
		<h3 class="task-title">{task.title}</h3>
		{#if task.current_phase}
			<div class="task-phase">
				<span class="phase-label">Phase:</span>
				<span class="phase-value">{task.current_phase}</span>
			</div>
		{/if}
	</div>

	<!-- Right: Meta & Actions -->
	<div class="task-right">
		<div class="task-meta">
			<span class="task-time">{formatDate(task.updated_at)}</span>
		</div>

		<div class="task-actions">
			{#if task.status === 'running' && onPause}
				<button class="action-btn" onclick={handlePause} title="Pause task">
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<rect x="6" y="4" width="4" height="16" rx="1" />
						<rect x="14" y="4" width="4" height="16" rx="1" />
					</svg>
					Pause
				</button>
			{:else if task.status === 'paused' && onResume}
				<button class="action-btn primary" onclick={handleResume} title="Resume task">
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<polygon points="5 3 19 12 5 21 5 3" />
					</svg>
					Resume
				</button>
			{:else if ['created', 'planned'].includes(task.status) && onRun}
				<button class="action-btn primary" onclick={handleRun} title="Run task">
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<polygon points="5 3 19 12 5 21 5 3" />
					</svg>
					Run
				</button>
			{/if}
			{#if task.status !== 'running' && onDelete}
				<button class="action-btn delete" onclick={handleDelete} title="Delete task">
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="3 6 5 6 21 6" />
						<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
					</svg>
				</button>
			{/if}
		</div>
	</div>
</a>

<style>
	.task-card {
		display: flex;
		align-items: center;
		gap: var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
		text-decoration: none;
		color: inherit;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			box-shadow var(--duration-fast) var(--ease-out),
			transform var(--duration-fast) var(--ease-out);
	}

	.task-card:hover {
		border-color: var(--border-strong);
		transform: translateY(-1px);
		box-shadow: var(--shadow-md);
	}

	.task-card:focus-visible {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	/* Running state animation */
	.task-card.running {
		animation: card-running-glow 2.5s ease-in-out infinite;
	}

	/* Status Column */
	.status-col {
		flex-shrink: 0;
		width: 24px;
		display: flex;
		justify-content: center;
	}

	/* Task Info */
	.task-info {
		flex: 1;
		min-width: 0;
	}

	.task-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-bottom: var(--space-1);
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		letter-spacing: var(--tracking-wide);
	}

	.weight-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-sm);
	}

	.initiative-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		letter-spacing: var(--tracking-wide);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-sm);
		background: var(--bg-tertiary);
		color: var(--text-secondary);
		border: 1px solid var(--border-subtle);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.initiative-badge:hover {
		background: var(--bg-surface);
		border-color: var(--border-default);
		color: var(--text-primary);
	}

	.task-title {
		font-size: var(--text-base);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		margin: 0;
		letter-spacing: normal;
		text-transform: none;
	}

	.task-phase {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		margin-top: var(--space-1);
		font-size: var(--text-xs);
	}

	.phase-label {
		color: var(--text-muted);
	}

	.phase-value {
		color: var(--accent-primary);
		font-weight: var(--font-medium);
	}

	/* Right Section */
	.task-right {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: var(--space-2);
		flex-shrink: 0;
	}

	.task-meta {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.task-actions {
		display: flex;
		gap: var(--space-2);
		align-items: center;
	}

	/* Action Buttons */
	.action-btn {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-1-5) var(--space-3);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.action-btn:hover {
		background: var(--bg-surface);
		color: var(--text-primary);
		border-color: var(--border-strong);
	}

	.action-btn.primary {
		background: var(--accent-primary);
		border-color: var(--accent-primary);
		color: var(--text-inverse);
	}

	.action-btn.primary:hover {
		background: var(--accent-hover);
		border-color: var(--accent-hover);
	}

	.action-btn.delete {
		padding: var(--space-1-5);
		background: transparent;
		border-color: transparent;
		color: var(--text-muted);
	}

	.action-btn.delete:hover {
		background: var(--status-danger-bg);
		border-color: var(--status-danger);
		color: var(--status-danger);
	}
</style>
