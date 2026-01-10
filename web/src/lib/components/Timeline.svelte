<script lang="ts">
	import type { Phase, TaskState } from '$lib/types';

	interface Props {
		phases: Phase[];
		currentPhase?: string;
		state?: TaskState | null;
	}

	let { phases, currentPhase, state }: Props = $props();

	function getPhaseStatus(phaseId: string): string {
		if (state?.phases && state.phases[phaseId]) {
			return state.phases[phaseId].status;
		}
		return 'pending';
	}

	function getPhaseIterations(phaseId: string): number {
		return state?.phases?.[phaseId]?.iterations || 0;
	}

	const statusIcons: Record<string, string> = {
		pending: '\u25cb', // ○
		running: '\u25cf', // ●
		completed: '\u2714', // ✔
		failed: '\u2718', // ✘
		skipped: '\u2212'  // −
	};

	const statusColors: Record<string, string> = {
		pending: 'var(--text-muted)',
		running: 'var(--accent-primary)',
		completed: 'var(--accent-success)',
		failed: 'var(--accent-danger)',
		skipped: 'var(--text-muted)'
	};
</script>

<div class="timeline">
	{#each phases as phase, i (phase.id)}
		{@const status = getPhaseStatus(phase.id)}
		{@const isCurrent = phase.id === currentPhase}
		{@const iterations = getPhaseIterations(phase.id)}

		<div class="phase" class:current={isCurrent}>
			<div class="phase-indicator" style="color: {statusColors[status]}">
				<span class="phase-icon">{statusIcons[status]}</span>
				{#if i < phases.length - 1}
					<div class="phase-line" class:completed={status === 'completed'}></div>
				{/if}
			</div>

			<div class="phase-content">
				<div class="phase-header">
					<span class="phase-name">{phase.name || phase.id}</span>
					{#if iterations > 0}
						<span class="phase-iterations">{iterations} iteration{iterations !== 1 ? 's' : ''}</span>
					{/if}
				</div>
				{#if state?.phases?.[phase.id]?.error}
					<div class="phase-error">{state.phases[phase.id].error}</div>
				{/if}
			</div>
		</div>
	{/each}
</div>

<style>
	.timeline {
		display: flex;
		flex-direction: column;
		gap: 0;
	}

	.phase {
		display: flex;
		gap: 1rem;
		padding: 0.5rem 0;
	}

	.phase.current {
		background: rgba(88, 166, 255, 0.05);
		border-radius: 6px;
		margin: 0 -0.5rem;
		padding: 0.5rem;
	}

	.phase-indicator {
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 20px;
	}

	.phase-icon {
		font-size: 1rem;
		line-height: 1;
	}

	.phase-line {
		flex: 1;
		width: 2px;
		background: var(--border-color);
		min-height: 20px;
		margin-top: 4px;
	}

	.phase-line.completed {
		background: var(--accent-success);
	}

	.phase-content {
		flex: 1;
		padding-top: 0.125rem;
	}

	.phase-header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.phase-name {
		font-weight: 500;
		font-size: 0.9375rem;
	}

	.phase-iterations {
		font-size: 0.75rem;
		color: var(--text-secondary);
		font-family: var(--font-mono);
	}

	.phase-error {
		font-size: 0.75rem;
		color: var(--accent-danger);
		margin-top: 0.25rem;
	}
</style>
