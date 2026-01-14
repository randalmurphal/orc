<script lang="ts">
	import type { DashboardStats } from '$lib/api';
	import type { ConnectionStatus } from '$lib/websocket';

	interface Props {
		stats: DashboardStats;
		wsStatus: ConnectionStatus;
		onFilterClick: (status: string) => void;
		onDependencyFilterClick?: (status: string) => void;
	}

	let { stats, wsStatus, onFilterClick, onDependencyFilterClick }: Props = $props();

	const cacheTotal = $derived((stats.cache_creation_input_tokens || 0) + (stats.cache_read_input_tokens || 0));

	function formatTokens(tokens: number): string {
		if (tokens >= 1_000_000) {
			return `${(tokens / 1_000_000).toFixed(1)}M`;
		}
		if (tokens >= 1_000) {
			return `${(tokens / 1_000).toFixed(1)}K`;
		}
		return String(tokens);
	}
</script>

<section class="stats-section">
	<div class="section-header">
		<h2 class="section-title">Quick Stats</h2>
		<div class="connection-status" class:connected={wsStatus === 'connected'} class:connecting={wsStatus === 'connecting' || wsStatus === 'reconnecting'}>
			<span class="status-dot"></span>
			<span class="status-text">{wsStatus === 'connected' ? 'Live' : wsStatus === 'connecting' ? 'Connecting' : wsStatus === 'reconnecting' ? 'Reconnecting' : 'Offline'}</span>
		</div>
	</div>
	<div class="stats-grid">
		<button class="stat-card running" onclick={() => onFilterClick('running')}>
			<div class="stat-icon">
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="10" />
					<polyline points="12 6 12 12 16 14" />
				</svg>
			</div>
			<div class="stat-content">
				<span class="stat-value">{stats.running}</span>
				<span class="stat-label">Running</span>
			</div>
		</button>

		<button class="stat-card blocked" onclick={() => onDependencyFilterClick?.('blocked') ?? onFilterClick('blocked')}>
			<div class="stat-icon">
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="10" />
					<line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
				</svg>
			</div>
			<div class="stat-content">
				<span class="stat-value">{stats.blocked}</span>
				<span class="stat-label">Blocked</span>
			</div>
		</button>

		<button class="stat-card today" onclick={() => onFilterClick('all')}>
			<div class="stat-icon">
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
					<line x1="16" y1="2" x2="16" y2="6" />
					<line x1="8" y1="2" x2="8" y2="6" />
					<line x1="3" y1="10" x2="21" y2="10" />
				</svg>
			</div>
			<div class="stat-content">
				<span class="stat-value">{stats.today}</span>
				<span class="stat-label">Today</span>
			</div>
		</button>

		<div
			class="stat-card tokens"
			title={cacheTotal > 0
				? `Total: ${stats.tokens.toLocaleString()}\nCached: ${cacheTotal.toLocaleString()} (${formatTokens(stats.cache_creation_input_tokens || 0)} creation, ${formatTokens(stats.cache_read_input_tokens || 0)} read)`
				: `Total: ${stats.tokens.toLocaleString()}`}
		>
			<div class="stat-icon">
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
				</svg>
			</div>
			<div class="stat-content">
				<span class="stat-value">{formatTokens(stats.tokens)}</span>
				<span class="stat-label">Tokens{cacheTotal > 0 ? ` (${formatTokens(cacheTotal)} cached)` : ''}</span>
			</div>
		</div>
	</div>
</section>

<style>
	.section-title {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
		margin-bottom: var(--space-4);
	}

	/* Connection Status */
	.connection-status {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-1) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.status-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--text-muted);
	}

	.connection-status.connected .status-dot {
		background: var(--status-success);
		box-shadow: 0 0 4px var(--status-success);
	}

	.connection-status.connected .status-text {
		color: var(--status-success);
	}

	.connection-status.connecting .status-dot {
		background: var(--status-warning);
		animation: pulse 1s ease-in-out infinite;
	}

	.connection-status.connecting .status-text {
		color: var(--status-warning);
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}

	/* Stats Grid */
	.stats-section .section-header {
		margin-bottom: var(--space-4);
	}

	.stats-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: var(--space-4);
	}

	@media (max-width: 768px) {
		.stats-grid {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	.stat-card {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		text-align: left;
	}

	.stat-card:hover {
		border-color: var(--accent-primary);
		transform: translateY(-2px);
	}

	.stat-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 40px;
		height: 40px;
		border-radius: var(--radius-md);
		color: var(--text-muted);
	}

	.stat-card.running .stat-icon {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.stat-card.blocked .stat-icon {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.stat-card.today .stat-icon {
		background: rgba(168, 85, 247, 0.1);
		color: rgb(168, 85, 247);
	}

	.stat-card.tokens .stat-icon {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.stat-content {
		display: flex;
		flex-direction: column;
	}

	.stat-value {
		font-size: var(--text-xl);
		font-weight: var(--font-bold);
		font-family: var(--font-mono);
		color: var(--text-primary);
	}

	.stat-label {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}
</style>
