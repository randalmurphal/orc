/**
 * DashboardRecentActivity component - displays recently completed/failed tasks.
 * Shows status indicator, task ID, title, and relative timestamp.
 */

import { Link } from 'react-router-dom';
import type { Task } from '@/lib/types';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import './DashboardRecentActivity.css';

interface DashboardRecentActivityProps {
	tasks: Task[];
}

function formatRelativeTime(dateStr: string): string {
	if (!dateStr) return '';
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return '';

	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);

	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	if (diffHours < 24) return `${diffHours}h ago`;
	if (diffDays < 7) return `${diffDays}d ago`;
	// Use explicit options to ensure 4-digit year display
	return date.toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'numeric',
		day: 'numeric',
	});
}

export function DashboardRecentActivity({ tasks }: DashboardRecentActivityProps) {
	if (tasks.length === 0) {
		return null;
	}

	return (
		<section className="tasks-section recent-activity-section">
			<div className="section-header">
				<h2 className="section-title">Recent Activity</h2>
			</div>
			<div className="activity-list">
				{tasks.map((task) => (
					<Link key={task.id} to={`/tasks/${task.id}`} className="activity-item">
						<div className="activity-status">
							<StatusIndicator status={task.status} size="md" />
						</div>
						<div className="activity-content">
							<span className="activity-id">{task.id}</span>
							<span className="activity-title">{task.title}</span>
						</div>
						<span className="activity-time">{formatRelativeTime(task.updated_at)}</span>
					</Link>
				))}
			</div>
		</section>
	);
}
