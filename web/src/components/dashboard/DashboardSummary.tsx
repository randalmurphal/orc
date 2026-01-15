/**
 * DashboardSummary component - displays overall task counts.
 * Total tasks, completed count, and failed count.
 */

import type { DashboardStats } from '@/lib/api';
import './DashboardSummary.css';

interface DashboardSummaryProps {
	stats: DashboardStats;
}

export function DashboardSummary({ stats }: DashboardSummaryProps) {
	return (
		<section className="summary-section">
			<div className="summary-stats">
				<div className="summary-item">
					<span className="summary-label">Total Tasks</span>
					<span className="summary-value">{stats.total}</span>
				</div>
				<div className="summary-item">
					<span className="summary-label">Completed</span>
					<span className="summary-value success">{stats.completed}</span>
				</div>
				<div className="summary-item">
					<span className="summary-label">Failed</span>
					<span className="summary-value danger">{stats.failed}</span>
				</div>
			</div>
		</section>
	);
}
