<script lang="ts">
	interface Props {
		status: 'created' | 'classifying' | 'planned' | 'running' | 'paused' | 'blocked' | 'completed' | 'failed';
		size?: 'sm' | 'md' | 'lg';
		showLabel?: boolean;
	}

	let { status, size = 'md', showLabel = false }: Props = $props();

	const statusConfig: Record<string, { color: string; glow: string; label: string }> = {
		created: {
			color: 'var(--text-muted)',
			glow: 'transparent',
			label: 'Created'
		},
		classifying: {
			color: 'var(--status-warning)',
			glow: 'var(--status-warning-glow)',
			label: 'Classifying'
		},
		planned: {
			color: 'var(--text-secondary)',
			glow: 'transparent',
			label: 'Planned'
		},
		running: {
			color: 'var(--accent-primary)',
			glow: 'var(--accent-glow)',
			label: 'Running'
		},
		paused: {
			color: 'var(--status-warning)',
			glow: 'var(--status-warning-glow)',
			label: 'Paused'
		},
		blocked: {
			color: 'var(--status-danger)',
			glow: 'var(--status-danger-glow)',
			label: 'Blocked'
		},
		completed: {
			color: 'var(--status-success)',
			glow: 'transparent',
			label: 'Completed'
		},
		failed: {
			color: 'var(--status-danger)',
			glow: 'transparent',
			label: 'Failed'
		}
	};

	const config = $derived(statusConfig[status] || statusConfig.created);
	const isAnimated = $derived(status === 'running');
	const isPaused = $derived(status === 'paused');

	const sizeClasses: Record<string, string> = {
		sm: 'size-sm',
		md: 'size-md',
		lg: 'size-lg'
	};
</script>

<div class="status-indicator {sizeClasses[size]}" class:animated={isAnimated} class:paused={isPaused}>
	<span
		class="orb"
		style:--status-color={config.color}
		style:--status-glow={config.glow}
	></span>
	{#if showLabel}
		<span class="label" style:color={config.color}>{config.label}</span>
	{/if}
</div>

<style>
	.status-indicator {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
	}

	.orb {
		border-radius: 50%;
		background: var(--status-color);
		box-shadow: 0 0 0 var(--status-glow);
		transition: all var(--duration-normal) var(--ease-out);
	}

	/* Sizes */
	.size-sm .orb {
		width: 6px;
		height: 6px;
	}

	.size-md .orb {
		width: 8px;
		height: 8px;
	}

	.size-lg .orb {
		width: 10px;
		height: 10px;
	}

	.label {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	/* Running animation */
	.animated .orb {
		animation: status-pulse 2s ease-in-out infinite, status-glow 2s ease-in-out infinite;
	}

	/* Paused animation */
	.paused .orb {
		animation: status-blink 1.5s ease-in-out infinite;
	}
</style>
