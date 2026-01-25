/**
 * DashboardStats component - displays quick stats cards with live connection indicator.
 * Shows running, blocked, today's completed, and token usage.
 */

import type { ConnectionStatus } from '@/lib/types';
import type { DashboardStats as DashboardStatsType } from '@/lib/api';
import { formatNumber } from '@/lib/format';
import { Icon } from '@/components/ui/Icon';
import './DashboardStats.css';

interface DashboardStatsProps {
	stats: DashboardStatsType;
	wsStatus: ConnectionStatus;
	onFilterClick: (status: string) => void;
	onDependencyFilterClick?: (status: string) => void;
}

export function DashboardStats({
	stats,
	wsStatus,
	onFilterClick,
	onDependencyFilterClick,
}: DashboardStatsProps) {
	const cacheTotal =
		(stats.cache_creation_input_tokens || 0) + (stats.cache_read_input_tokens || 0);

	const connectionStatusClass = [
		'connection-status',
		wsStatus === 'connected' && 'connected',
		(wsStatus === 'connecting' || wsStatus === 'reconnecting') && 'connecting',
	]
		.filter(Boolean)
		.join(' ');

	const getStatusText = () => {
		switch (wsStatus) {
			case 'connected':
				return 'Live';
			case 'connecting':
				return 'Connecting';
			case 'reconnecting':
				return 'Reconnecting';
			default:
				return 'Offline';
		}
	};

	const tokenTooltip =
		cacheTotal > 0
			? `Total: ${stats.tokens.toLocaleString()}\nCached: ${cacheTotal.toLocaleString()} (${formatNumber(stats.cache_creation_input_tokens || 0)} creation, ${formatNumber(stats.cache_read_input_tokens || 0)} read)`
			: `Total: ${stats.tokens.toLocaleString()}`;

	return (
		<section className="stats-section">
			<div className="section-header">
				<h2 className="section-title">Quick Stats</h2>
				<div className={connectionStatusClass}>
					<span className="status-dot" />
					<span className="status-text">{getStatusText()}</span>
				</div>
			</div>
			<div className="stats-grid">
				<button className="stat-card running" onClick={() => onFilterClick('running')}>
					<div className="stat-icon">
						<Icon name="clock" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{stats.running}</span>
						<span className="stat-label">Running</span>
					</div>
				</button>

				<button
					className="stat-card blocked"
					onClick={() =>
						onDependencyFilterClick?.('blocked') ?? onFilterClick('blocked')
					}
				>
					<div className="stat-icon">
						<Icon name="blocked" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{stats.blocked}</span>
						<span className="stat-label">Blocked</span>
					</div>
				</button>

				<button className="stat-card today" onClick={() => onFilterClick('all')}>
					<div className="stat-icon">
						<Icon name="calendar" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{stats.today}</span>
						<span className="stat-label">Today</span>
					</div>
				</button>

				<div className="stat-card tokens" title={tokenTooltip}>
					<div className="stat-icon">
						<Icon name="dollar" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{formatNumber(stats.tokens)}</span>
						<span className="stat-label">
							Tokens{cacheTotal > 0 ? ` (${formatNumber(cacheTotal)} cached)` : ''}
						</span>
					</div>
				</div>
			</div>
		</section>
	);
}
