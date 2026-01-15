/**
 * DashboardQuickActions component - displays quick action buttons.
 * New Task and View All Tasks buttons.
 */

import { Button } from '@/components/ui/Button';
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
				<Button
					variant="primary"
					size="md"
					leftIcon={<Icon name="plus" size={16} />}
					onClick={onNewTask}
				>
					New Task
				</Button>
				<Button
					variant="secondary"
					size="md"
					leftIcon={<Icon name="tasks" size={16} />}
					onClick={onViewTasks}
				>
					View All Tasks
				</Button>
			</div>
		</section>
	);
}
