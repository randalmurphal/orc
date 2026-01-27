/**
 * DashboardStats component - displays quick stats cards with live connection indicator.
 * Shows running, blocked, today's completed, and token usage.
 */

import type { ConnectionStatus } from '@/lib/events';
import type { DashboardStats as ProtoDashboardStats } from '@/gen/orc/v1/dashboard_pb';
import { formatNumber } from '@/lib/format';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import './DashboardStats.css';

interface DashboardStatsProps {
	stats: ProtoDashboardStats;
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
	const tokens = stats.todayTokens;
	const totalTokens = tokens?.totalTokens ?? 0;
	const cacheTotal =
		(tokens?.cacheCreationInputTokens ?? 0) + (tokens?.cacheReadInputTokens ?? 0);
	const taskCounts = stats.taskCounts;
	const todayCompleted = stats.recentCompletions?.length ?? 0;

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
			? `Total: ${totalTokens.toLocaleString()}\nCached: ${cacheTotal.toLocaleString()} (${formatNumber(tokens?.cacheCreationInputTokens ?? 0)} creation, ${formatNumber(tokens?.cacheReadInputTokens ?? 0)} read)`
			: `Total: ${totalTokens.toLocaleString()}`;

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
				<Button variant="ghost" className="stat-card running" onClick={() => onFilterClick('running')}>
					<div className="stat-icon">
						<Icon name="clock" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{taskCounts?.running ?? 0}</span>
						<span className="stat-label">Running</span>
					</div>
				</Button>

				<Button
					variant="ghost"
					className="stat-card blocked"
					onClick={() =>
						onDependencyFilterClick?.('blocked') ?? onFilterClick('blocked')
					}
				>
					<div className="stat-icon">
						<Icon name="blocked" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{taskCounts?.blocked ?? 0}</span>
						<span className="stat-label">Blocked</span>
					</div>
				</Button>

				<Button variant="ghost" className="stat-card today" onClick={() => onFilterClick('all')}>
					<div className="stat-icon">
						<Icon name="calendar" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{todayCompleted}</span>
						<span className="stat-label">Today</span>
					</div>
				</Button>

				<div className="stat-card tokens" title={tokenTooltip}>
					<div className="stat-icon">
						<Icon name="dollar" size={24} />
					</div>
					<div className="stat-content">
						<span className="stat-value">{formatNumber(totalTokens)}</span>
						<span className="stat-label">
							Tokens{cacheTotal > 0 ? ` (${formatNumber(cacheTotal)} cached)` : ''}
						</span>
					</div>
				</div>
			</div>
		</section>
	);
}
