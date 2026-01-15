/**
 * DashboardQuickActions component - displays quick action buttons.
 * New Task and View All Tasks buttons.
 */

import { Icon } from '@/components/ui/Icon';
import './DashboardQuickActions.css';

interface DashboardQuickActionsProps {
	onNewTask: () => void;
	onViewTasks: () => void;
}

export function DashboardQuickActions({ onNewTask, onViewTasks }: DashboardQuickActionsProps) {
	return (
		<section className="actions-section">
			<div className="quick-actions">
				<button className="action-btn primary" onClick={onNewTask}>
					<Icon name="plus" size={16} />
					New Task
				</button>
				<button className="action-btn" onClick={onViewTasks}>
					<Icon name="tasks" size={16} />
					View All Tasks
				</button>
			</div>
		</section>
	);
}
