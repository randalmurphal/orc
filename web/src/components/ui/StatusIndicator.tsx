/**
 * StatusIndicator component - displays a colored orb indicating task status.
 * Includes animation for running and paused states.
 */

import type { TaskStatus } from '@/lib/types';
import './StatusIndicator.css';

export type StatusIndicatorSize = 'sm' | 'md' | 'lg';

interface StatusConfig {
	color: string;
	glow: string;
	label: string;
}

const statusConfig: Record<TaskStatus, StatusConfig> = {
	created: {
		color: 'var(--text-muted)',
		glow: 'transparent',
		label: 'Created',
	},
	classifying: {
		color: 'var(--status-warning)',
		glow: 'var(--status-warning-glow)',
		label: 'Classifying',
	},
	planned: {
		color: 'var(--text-secondary)',
		glow: 'transparent',
		label: 'Planned',
	},
	running: {
		color: 'var(--accent-primary)',
		glow: 'var(--accent-glow)',
		label: 'Running',
	},
	paused: {
		color: 'var(--status-warning)',
		glow: 'var(--status-warning-glow)',
		label: 'Paused',
	},
	blocked: {
		color: 'var(--status-danger)',
		glow: 'var(--status-danger-glow)',
		label: 'Blocked',
	},
	finalizing: {
		color: 'var(--status-info)',
		glow: 'var(--status-info-glow)',
		label: 'Finalizing',
	},
	completed: {
		color: 'var(--status-success)',
		glow: 'transparent',
		label: 'Completed',
	},
	failed: {
		color: 'var(--status-danger)',
		glow: 'transparent',
		label: 'Failed',
	},
};

interface StatusIndicatorProps {
	status: TaskStatus;
	size?: StatusIndicatorSize;
	showLabel?: boolean;
}

export function StatusIndicator({ status, size = 'md', showLabel = false }: StatusIndicatorProps) {
	const config = statusConfig[status] || statusConfig.created;
	const isAnimated = status === 'running';
	const isPaused = status === 'paused';

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
