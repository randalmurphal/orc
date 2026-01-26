/**
 * StatusIndicator component - displays a colored orb indicating task status.
 * Includes animation for running and paused states.
 */

import { TaskStatus } from '@/gen/orc/v1/task_pb';
import './StatusIndicator.css';

export type StatusIndicatorSize = 'sm' | 'md' | 'lg';

interface StatusConfig {
	color: string;
	glow: string;
	label: string;
}

const statusConfig: Record<TaskStatus, StatusConfig> = {
	[TaskStatus.UNSPECIFIED]: {
		color: 'var(--text-muted)',
		glow: 'transparent',
		label: 'Unknown',
	},
	[TaskStatus.CREATED]: {
		color: 'var(--text-muted)',
		glow: 'transparent',
		label: 'Created',
	},
	[TaskStatus.CLASSIFYING]: {
		color: 'var(--status-warning)',
		glow: 'var(--status-warning-glow)',
		label: 'Classifying',
	},
	[TaskStatus.PLANNED]: {
		color: 'var(--text-secondary)',
		glow: 'transparent',
		label: 'Planned',
	},
	[TaskStatus.RUNNING]: {
		color: 'var(--primary)',
		glow: 'var(--primary-glow)',
		label: 'Running',
	},
	[TaskStatus.PAUSED]: {
		color: 'var(--status-warning)',
		glow: 'var(--status-warning-glow)',
		label: 'Paused',
	},
	[TaskStatus.BLOCKED]: {
		color: 'var(--status-danger)',
		glow: 'var(--status-danger-glow)',
		label: 'Blocked',
	},
	[TaskStatus.FINALIZING]: {
		color: 'var(--status-info)',
		glow: 'var(--status-info-glow)',
		label: 'Finalizing',
	},
	[TaskStatus.COMPLETED]: {
		color: 'var(--status-success)',
		glow: 'transparent',
		label: 'Completed',
	},
	[TaskStatus.FAILED]: {
		color: 'var(--status-danger)',
		glow: 'transparent',
		label: 'Failed',
	},
	[TaskStatus.RESOLVED]: {
		color: 'var(--status-warning)',
		glow: 'transparent',
		label: 'Resolved',
	},
};

interface StatusIndicatorProps {
	status: TaskStatus;
	size?: StatusIndicatorSize;
	showLabel?: boolean;
}

export function StatusIndicator({ status, size = 'md', showLabel = false }: StatusIndicatorProps) {
	const config = statusConfig[status] || statusConfig[TaskStatus.CREATED];
	const isAnimated = status === TaskStatus.RUNNING;
	const isPaused = status === TaskStatus.PAUSED;

	const containerClasses = [
		'status-indicator',
		`size-${size}`,
		isAnimated && 'animated',
		isPaused && 'paused',
	]
		.filter(Boolean)
		.join(' ');

	return (
		<div className={containerClasses}>
			<span
				className="orb"
				style={
					{
						'--status-color': config.color,
						'--status-glow': config.glow,
					} as React.CSSProperties
				}
			/>
			{showLabel && (
				<span className="label" style={{ color: config.color }}>
					{config.label}
				</span>
			)}
		</div>
	);
}
