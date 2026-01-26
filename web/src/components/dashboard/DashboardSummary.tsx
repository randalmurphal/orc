/**
 * DashboardSummary component - displays overall task counts.
 * Total tasks, completed count, and failed count.
 */

import type { DashboardStats } from '@/gen/orc/v1/dashboard_pb';
import './DashboardSummary.css';

interface DashboardSummaryProps {
	stats: DashboardStats;
}

export function DashboardSummary({ stats }: DashboardSummaryProps) {
	const taskCounts = stats.taskCounts;
	return (
		<section className="summary-section">
			<div className="summary-stats">
				<div className="summary-item">
					<span className="summary-label">Total Tasks</span>
					<span className="summary-value">{taskCounts?.all ?? 0}</span>
				</div>
				<div className="summary-item">
					<span className="summary-label">Completed</span>
					<span className="summary-value success">{taskCounts?.completed ?? 0}</span>
				</div>
				<div className="summary-item">
					<span className="summary-label">Failed</span>
					<span className="summary-value danger">{taskCounts?.failed ?? 0}</span>
				</div>
			</div>
		</section>
	);
}
