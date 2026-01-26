/**
 * DashboardActiveTasks component - displays a list of running/paused/blocked tasks.
 * Each task is clickable to navigate to task detail.
 */

import { Link } from 'react-router-dom';
import type { Task } from '@/gen/orc/v1/task_pb';
import { StatusIndicator } from '@/components/ui/StatusIndicator';
import './DashboardActiveTasks.css';

interface DashboardActiveTasksProps {
	tasks: Task[];
}

export function DashboardActiveTasks({ tasks }: DashboardActiveTasksProps) {
	if (tasks.length === 0) {
		return null;
	}

	return (
		<section className="tasks-section">
			<div className="section-header">
				<h2 className="section-title">Active Tasks</h2>
				<span className="section-count">{tasks.length}</span>
			</div>
			<div className="task-list">
				{tasks.map((task) => (
					<Link key={task.id} to={`/tasks/${task.id}`} className="task-card-compact">
						<div className="task-status">
							<StatusIndicator status={task.status} size="md" />
						</div>
						<div className="task-content">
							<span className="task-id">{task.id}</span>
							<span className="task-title">{task.title}</span>
						</div>
						{task.currentPhase && (
							<span className="task-phase">{task.currentPhase}</span>
						)}
					</Link>
				))}
			</div>
		</section>
	);
}
