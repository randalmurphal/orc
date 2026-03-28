import type { RunningTask } from '@/gen/orc/v1/attention_dashboard_pb';

export interface RunningTaskItemProps {
	task: RunningTask;
	onOpen: (projectId: string, taskId: string) => void;
}

function formatElapsed(elapsedTimeSeconds: bigint): string {
	const totalSeconds = Number(elapsedTimeSeconds);
	const hours = Math.floor(totalSeconds / 3600);
	const minutes = Math.floor((totalSeconds % 3600) / 60);

	if (hours > 0) {
		return `${hours}h ${minutes}m`;
	}

	if (minutes > 0) {
		return `${minutes}m`;
	}

	return `${totalSeconds}s`;
}

export function RunningTaskItem({ task, onOpen }: RunningTaskItemProps) {
	return (
		<button
			type="button"
			className="command-center-running-item"
			onClick={() => onOpen(task.projectId, task.id)}
		>
			<div className="command-center-running-item__header">
				<span className="command-center-running-item__project">{task.projectName}</span>
				<span className="command-center-running-item__elapsed">{formatElapsed(task.elapsedTimeSeconds)}</span>
			</div>
			<div className="command-center-running-item__title-row">
				<span className="command-center-running-item__id">{task.id}</span>
				<span className="command-center-running-item__title">{task.title}</span>
			</div>
			<div className="command-center-running-item__meta">
				<span className="command-center-running-item__phase">
					{task.currentPhase || 'running'}
				</span>
				{task.initiativeTitle ? (
					<span className="command-center-running-item__initiative">{task.initiativeTitle}</span>
				) : null}
			</div>
		</button>
	);
}
