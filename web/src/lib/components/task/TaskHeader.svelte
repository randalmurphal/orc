<script lang="ts">
	import type { Task, TaskState, Plan } from '$lib/types';
	import StatusIndicator from '$lib/components/ui/StatusIndicator.svelte';

	interface Props {
		task: Task;
		taskState?: TaskState | null;
		plan?: Plan | null;
		onRun: () => void;
		onPause: () => void;
		onResume: () => void;
		onCancel: () => void;
		onDelete: () => void;
		onRetry?: () => void;
	}

	let { task, taskState, plan, onRun, onPause, onResume, onCancel, onDelete, onRetry }: Props =
		$props();

	const weightConfig: Record<string, { color: string; bg: string }> = {
		trivial: { color: 'var(--weight-trivial)', bg: 'rgba(107, 114, 128, 0.15)' },
		small: { color: 'var(--weight-small)', bg: 'var(--status-success-bg)' },
		medium: { color: 'var(--weight-medium)', bg: 'var(--status-info-bg)' },
		large: { color: 'var(--weight-large)', bg: 'var(--status-warning-bg)' },
		greenfield: { color: 'var(--weight-greenfield)', bg: 'var(--accent-subtle)' }
	};

	const weight = $derived(weightConfig[task.weight] || weightConfig.small);

	// Calculate current phase progress
	const currentPhaseIndex = $derived(
		plan?.phases.findIndex((p) => p.id === task.current_phase) ?? -1
	);
	const totalPhases = $derived(plan?.phases.length ?? 0);
	const phaseProgress = $derived(
		totalPhases > 0 && currentPhaseIndex >= 0
			? `${currentPhaseIndex + 1}/${totalPhases}`
			: null
	);

	// Check if task can be retried (failed or blocked)
	const canRetry = $derived(task.status === 'failed' || task.status === 'blocked');
</script>

<header class="task-header">
	<div class="task-info">
		<div class="task-meta">
			<span class="task-id">{task.id}</span>
			<span class="weight-badge" style:color={weight.color} style:background={weight.bg}>
				{task.weight}
			</span>
			<div class="status-badge">
				<StatusIndicator status={task.status} size="sm" showLabel />
			</div>
			{#if phaseProgress && task.current_phase}
				<div class="phase-indicator">
					<span class="phase-name">{task.current_phase}</span>
					<span class="phase-progress">{phaseProgress}</span>
				</div>
			{/if}
		</div>
		<h1 class="task-title">{task.title}</h1>
		{#if task.description}
			<p class="task-description">{task.description}</p>
		{/if}
	</div>

	<div class="task-actions">
		{#if task.status === 'running'}
			<button onclick={onPause} title="Pause task">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="currentColor"
					stroke="none"
				>
					<rect x="6" y="4" width="4" height="16" rx="1" />
					<rect x="14" y="4" width="4" height="16" rx="1" />
				</svg>
				Pause
			</button>
			<button class="danger" onclick={onCancel} title="Cancel task">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
				</svg>
				Cancel
			</button>
		{:else if task.status === 'paused'}
			<button class="primary" onclick={onResume} title="Resume task">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="currentColor"
					stroke="none"
				>
					<polygon points="5 3 19 12 5 21 5 3" />
				</svg>
				Resume
			</button>
		{:else if ['created', 'planned'].includes(task.status)}
			<button class="primary" onclick={onRun} title="Run task">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="currentColor"
					stroke="none"
				>
					<polygon points="5 3 19 12 5 21 5 3" />
				</svg>
				Run Task
			</button>
		{/if}

		{#if canRetry && onRetry}
			<button onclick={onRetry} title="Retry task">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<polyline points="23 4 23 10 17 10" />
					<polyline points="1 20 1 14 7 14" />
					<path
						d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"
					/>
				</svg>
				Retry
			</button>
		{/if}

		{#if task.status !== 'running'}
			<button class="icon-btn delete-btn" onclick={onDelete} title="Delete task">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<polyline points="3 6 5 6 21 6"></polyline>
					<path
						d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
					></path>
				</svg>
			</button>
		{/if}
	</div>
</header>

<style>
	.task-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--space-6);
		margin-bottom: var(--space-6);
		padding-bottom: var(--space-6);
		border-bottom: 1px solid var(--border-subtle);
	}

	.task-info {
		flex: 1;
		min-width: 0;
	}

	.task-meta {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-3);
		flex-wrap: wrap;
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		letter-spacing: var(--tracking-wide);
	}

	.weight-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-sm);
	}

	.status-badge {
		margin-left: var(--space-2);
	}

	.phase-indicator {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-1) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		font-size: var(--text-xs);
	}

	.phase-name {
		color: var(--text-secondary);
		font-weight: var(--font-medium);
	}

	.phase-progress {
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.task-title {
		font-size: var(--text-2xl);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0 0 var(--space-2) 0;
		letter-spacing: normal;
		text-transform: none;
	}

	.task-description {
		font-size: var(--text-base);
		color: var(--text-secondary);
		line-height: var(--leading-relaxed);
		margin: 0;
	}

	.task-actions {
		display: flex;
		gap: var(--space-2);
		flex-shrink: 0;
		flex-wrap: wrap;
	}

	.task-actions button {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.delete-btn {
		background: transparent;
		border: 1px solid var(--border-default);
		color: var(--text-muted);
	}

	.delete-btn:hover {
		background: var(--status-danger-bg);
		border-color: var(--status-danger);
		color: var(--status-danger);
	}
</style>
