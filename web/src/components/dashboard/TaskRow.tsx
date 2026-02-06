import { memo } from 'react';
import type { TaskSummary } from '@/gen/orc/v1/project_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { Icon } from '@/components/ui';
import './TaskRow.css';

function getStatusName(status: TaskStatus): string {
	switch (status) {
		case TaskStatus.RUNNING: return 'running';
		case TaskStatus.BLOCKED: return 'blocked';
		case TaskStatus.CREATED: return 'created';
		case TaskStatus.PAUSED: return 'paused';
		case TaskStatus.COMPLETED: return 'completed';
		case TaskStatus.FAILED: return 'failed';
		default: return 'created';
	}
}

export interface TaskRowProps {
	task: TaskSummary;
	onClick: () => void;
}

export const TaskRow = memo(function TaskRow({ task, onClick }: TaskRowProps) {
	const statusName = getStatusName(task.status);

	return (
		<div
			className={`task-row${task.isStale ? ' task-row--stale' : ''}`}
			role="button"
			tabIndex={0}
			onClick={onClick}
			onKeyDown={(e) => {
				if (e.key === 'Enter' || e.key === ' ') {
					e.preventDefault();
					onClick();
				}
			}}
		>
			<span
				className="task-row__status"
				data-status={statusName}
			/>
			<span className="task-row__id">{task.id}</span>
			<span className="task-row__title">{task.title}</span>
			<span className="task-row__right">
				{task.claimedByName && (
					<span className="task-row__claimer">{task.claimedByName}</span>
				)}
				{task.isStale && (
					<span className="task-row__stale" data-stale="true">
						<Icon name="alert-triangle" size={14} />
					</span>
				)}
			</span>
		</div>
	);
});
