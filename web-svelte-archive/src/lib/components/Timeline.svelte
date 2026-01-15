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

	function getPhaseDuration(phaseId: string): string | null {
		const phaseState = state?.phases?.[phaseId];
		if (!phaseState?.started_at) return null;

		const start = new Date(phaseState.started_at);
		const end = phaseState.completed_at ? new Date(phaseState.completed_at) : new Date();
		const diffMs = end.getTime() - start.getTime();
		const diffMins = Math.floor(diffMs / 60000);

		if (diffMins < 1) return '<1m';
		if (diffMins < 60) return `${diffMins}m`;
		const hours = Math.floor(diffMins / 60);
		const mins = diffMins % 60;
		return `${hours}h ${mins}m`;
	}

	// Calculate progress percentage
	const completedCount = $derived(
		phases.filter((p) => getPhaseStatus(p.id) === 'completed').length
	);
	const progressPercent = $derived(
		phases.length > 0 ? Math.round((completedCount / phases.length) * 100) : 0
	);

	const statusConfig: Record<string, { icon: string; color: string; bg: string }> = {
		pending: {
			icon: '',
			color: 'var(--text-muted)',
			bg: 'var(--bg-tertiary)'
		},
		running: {
			icon: '',
			color: 'var(--accent-primary)',
			bg: 'var(--accent-subtle)'
		},
		completed: {
			icon: '\u2713',
			color: 'var(--status-success)',
			bg: 'var(--status-success-bg)'
		},
		failed: {
			icon: '\u2717',
			color: 'var(--status-danger)',
			bg: 'var(--status-danger-bg)'
		},
		skipped: {
			icon: '\u2212',
			color: 'var(--text-muted)',
			bg: 'var(--bg-tertiary)'
		}
	};
</script>

<div class="timeline-container">
	<div class="timeline-header">
		<h2>Execution Timeline</h2>
		{#if currentPhase}
			<span class="current-phase-label">
				Current: <span class="phase-name">{currentPhase}</span>
			</span>
		{/if}
	</div>

	<!-- Horizontal Phase Timeline -->
	<div class="timeline">
		{#each phases as phase, i (phase.id)}
			{@const status = getPhaseStatus(phase.id)}
			{@const isCurrent = phase.id === currentPhase}
			{@const iterations = getPhaseIterations(phase.id)}
			{@const duration = getPhaseDuration(phase.id)}
			{@const config = statusConfig[status] || statusConfig.pending}

			<!-- Connector Line (before all except first) -->
			{#if i > 0}
				{@const prevStatus = getPhaseStatus(phases[i - 1].id)}
				<div
					class="connector"
					class:completed={prevStatus === 'completed'}
					class:active={prevStatus === 'completed' && status === 'running'}
				></div>
			{/if}

			<!-- Phase Node -->
			<div class="phase-node" class:current={isCurrent} class:running={status === 'running'}>
				<div
					class="node-circle"
					style:--node-color={config.color}
					style:--node-bg={config.bg}
				>
					{#if config.icon}
						<span class="node-icon">{config.icon}</span>
					{:else if status === 'running'}
						<span class="node-pulse"></span>
					{/if}
				</div>

				<div class="node-label">{phase.name || phase.id}</div>

				<div class="node-meta">
					{#if duration}
						<span class="node-duration">{duration}</span>
					{/if}
					{#if iterations > 0}
						<span class="node-iterations">x{iterations}</span>
					{/if}
				</div>

				{#if state?.phases?.[phase.id]?.error}
					<div class="node-error" title={state.phases[phase.id].error}>!</div>
				{/if}
			</div>
		{/each}
	</div>

	<!-- Progress Bar -->
	<div class="progress-section">
		<div class="progress-bar">
			<div
				class="progress-fill"
				class:active={currentPhase}
				style:width="{progressPercent}%"
			></div>
		</div>
		<span class="progress-label">{progressPercent}%</span>
	</div>
</div>

<style>
	.timeline-container {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-5);
	}

	.timeline-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: var(--space-5);
	}

	.timeline-header h2 {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		margin: 0;
	}

	.current-phase-label {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.current-phase-label .phase-name {
		color: var(--accent-primary);
		font-weight: var(--font-medium);
	}

	/* Timeline Track */
	.timeline {
		display: flex;
		align-items: flex-start;
		justify-content: center;
		gap: 0;
		padding: var(--space-4) 0;
		overflow-x: auto;
	}

	/* Connector Lines */
	.connector {
		width: 40px;
		height: 2px;
		background: var(--border-default);
		margin-top: 19px; /* Center with node circle */
		flex-shrink: 0;
	}

	.connector.completed {
		background: var(--status-success);
	}

	.connector.active {
		background: linear-gradient(90deg, var(--status-success), var(--accent-primary));
		animation: progress-flow 1.5s linear infinite;
	}

	/* Phase Node */
	.phase-node {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		min-width: 70px;
		position: relative;
	}

	.phase-node.current {
		transform: scale(1.05);
	}

	.node-circle {
		width: 40px;
		height: 40px;
		border-radius: 50%;
		background: var(--node-bg);
		border: 2px solid var(--node-color);
		display: flex;
		align-items: center;
		justify-content: center;
		position: relative;
		transition: all var(--duration-normal) var(--ease-out);
	}

	.phase-node.running .node-circle {
		box-shadow: var(--shadow-glow);
		animation: status-glow 2s ease-in-out infinite;
	}

	.node-icon {
		font-size: var(--text-base);
		color: var(--node-color);
		font-weight: var(--font-bold);
	}

	.node-pulse {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--accent-primary);
		animation: status-pulse 1.5s ease-in-out infinite;
	}

	.node-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		text-align: center;
		white-space: nowrap;
	}

	.phase-node.current .node-label {
		color: var(--accent-primary);
	}

	.node-meta {
		display: flex;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.node-duration {
		color: var(--text-secondary);
	}

	.node-iterations {
		color: var(--text-muted);
	}

	.node-error {
		position: absolute;
		top: -4px;
		right: -4px;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background: var(--status-danger);
		color: white;
		font-size: var(--text-2xs);
		font-weight: var(--font-bold);
		display: flex;
		align-items: center;
		justify-content: center;
	}

	/* Progress Section */
	.progress-section {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-top: var(--space-4);
		padding-top: var(--space-4);
		border-top: 1px solid var(--border-subtle);
	}

	.progress-bar {
		flex: 1;
		height: 4px;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background: var(--status-success);
		border-radius: var(--radius-full);
		transition: width var(--duration-slow) var(--ease-out);
	}

	.progress-fill.active {
		background: linear-gradient(
			90deg,
			var(--status-success),
			var(--accent-primary),
			var(--status-success)
		);
		background-size: 200% 100%;
		animation: progress-flow 2s linear infinite;
	}

	.progress-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		font-family: var(--font-mono);
		min-width: 40px;
		text-align: right;
	}
</style>
