/**
 * TimelineEvent component - displays individual events in the timeline feed.
 * Renders event icon, title, metadata, and expandable details.
 */

import { useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { Icon, type IconName } from '@/components/ui/Icon';
import { formatDate } from '@/lib/formatDate';
import { usePreferencesStore } from '@/stores/preferencesStore';
import './TimelineEvent.css';

// Event types that can occur in the timeline
export type EventType =
	| 'phase_started'
	| 'phase_completed'
	| 'phase_failed'
	| 'task_created'
	| 'task_started'
	| 'task_paused'
	| 'task_completed'
	| 'activity_changed'
	| 'error_occurred'
	| 'warning_issued'
	| 'token_update'
	| 'gate_decision';

// Event data structure
export interface TimelineEventData {
	id: number;
	task_id: string;
	task_title: string;
	phase?: string;
	iteration?: number;
	event_type: EventType;
	data: Record<string, unknown>;
	source: 'executor' | 'api' | 'cli' | 'manual';
	created_at: string;
}

export interface TimelineEventProps {
	event: TimelineEventData;
	showTask?: boolean; // Whether to show task info (false in task detail view)
}

// Event type configuration for icon, color, and label
type EventStyle = 'success' | 'error' | 'warning' | 'info' | 'default';

interface EventConfig {
	icon: IconName;
	style: EventStyle;
	getLabel: (event: TimelineEventData) => string;
}

const EVENT_CONFIG: Record<EventType, EventConfig> = {
	phase_completed: {
		icon: 'check-circle',
		style: 'success',
		getLabel: (e) => `Phase completed: ${e.phase || 'unknown'}`,
	},
	phase_failed: {
		icon: 'x-circle',
		style: 'error',
		getLabel: (e) => `Phase failed: ${e.phase || 'unknown'}`,
	},
	phase_started: {
		icon: 'play-circle',
		style: 'info',
		getLabel: (e) => `Phase started: ${e.phase || 'unknown'}`,
	},
	task_created: {
		icon: 'plus',
		style: 'default',
		getLabel: () => 'Task created',
	},
	task_started: {
		icon: 'play',
		style: 'info',
		getLabel: () => 'Task started',
	},
	task_paused: {
		icon: 'pause',
		style: 'warning',
		getLabel: () => 'Task paused',
	},
	task_completed: {
		icon: 'check-circle',
		style: 'success',
		getLabel: () => 'Task completed',
	},
	activity_changed: {
		icon: 'activity',
		style: 'info',
		getLabel: (e) => `Activity: ${String(e.data?.activity || 'changed')}`,
	},
	error_occurred: {
		icon: 'alert-triangle',
		style: 'error',
		getLabel: () => 'Error occurred',
	},
	warning_issued: {
		icon: 'alert-circle',
		style: 'warning',
		getLabel: () => 'Warning issued',
	},
	token_update: {
		icon: 'cpu',
		style: 'default',
		getLabel: () => 'Token usage updated',
	},
	gate_decision: {
		icon: 'shield',
		style: 'info',
		getLabel: (e) => `Gate: ${e.data?.approved ? 'approved' : 'rejected'}`,
	},
};

// Format numbers with commas
function formatNumber(num: number | undefined): string {
	if (num === undefined || num === null) return '0';
	return num.toLocaleString();
}

// Format duration in seconds to human readable
function formatDuration(seconds: number | undefined): string {
	if (seconds === undefined || seconds === null) return '';
	if (seconds < 60) return `${seconds}s`;
	if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
	const hours = Math.floor(seconds / 3600);
	const mins = Math.floor((seconds % 3600) / 60);
	return `${hours}h ${mins}m`;
}

// Extract relevant details from event data
function extractDetails(event: TimelineEventData): Array<{ label: string; value: string }> {
	const details: Array<{ label: string; value: string }> = [];
	const data = event.data || {};

	// Duration
	if (typeof data.duration === 'number') {
		details.push({ label: 'Duration', value: formatDuration(data.duration) });
	}

	// Tokens
	if (typeof data.input_tokens === 'number' || typeof data.output_tokens === 'number') {
		const input = formatNumber(data.input_tokens as number);
		const output = formatNumber(data.output_tokens as number);
		details.push({ label: 'Tokens', value: `${input} input / ${output} output` });
	}

	// Commit
	if (typeof data.commit_sha === 'string' && data.commit_sha) {
		details.push({ label: 'Commit', value: data.commit_sha.substring(0, 7) });
	}

	// Error message
	if (typeof data.error === 'string' && data.error) {
		details.push({ label: 'Error', value: data.error });
	}

	// Warning message
	if (typeof data.message === 'string' && data.message) {
		details.push({ label: 'Message', value: data.message });
	}

	// Iteration
	if (event.iteration !== undefined && event.iteration > 1) {
		details.push({ label: 'Iteration', value: String(event.iteration) });
	}

	// Gate decision details
	if (event.event_type === 'gate_decision') {
		const approved = !!data.approved;
		const reason = typeof data.reason === 'string' && data.reason ? data.reason : '';
		const hasSource = typeof data.source === 'string' && !!data.source;
		const hasRetry = typeof data.retry_from === 'string' && !!data.retry_from;
		const outputKeys =
			data.output_data && typeof data.output_data === 'object'
				? Object.keys(data.output_data as Record<string, unknown>)
				: [];
		const hasEnhancedDetails = hasSource || hasRetry || outputKeys.length > 0;

		// Decision line: show reason as value when no enhanced details, otherwise just status
		if (!hasEnhancedDetails && reason) {
			details.push({ label: 'Decision', value: reason });
		} else {
			details.push({ label: 'Decision', value: approved ? 'approved' : 'rejected' });
		}
		if (typeof data.gate_type === 'string' && data.gate_type) {
			details.push({ label: 'Gate Type', value: data.gate_type });
		}
		if (hasSource) {
			details.push({ label: 'Source', value: data.source as string });
		}
		if (hasRetry) {
			details.push({ label: 'Retry From', value: data.retry_from as string });
		}
		if (outputKeys.length > 0) {
			details.push({ label: 'Output Data', value: outputKeys.join(', ') });
		}
	}

	// Reason (non-gate events only; gate_decision handles reason above)
	if (event.event_type !== 'gate_decision' && typeof data.reason === 'string' && data.reason) {
		details.push({ label: 'Reason', value: data.reason });
	}

	return details;
}

export function TimelineEvent({ event, showTask = true }: TimelineEventProps) {
	const [isExpanded, setIsExpanded] = useState(false);
	const dateFormat = usePreferencesStore((s) => s.dateFormat);

	const config = EVENT_CONFIG[event.event_type] || {
		icon: 'circle',
		style: 'default' as EventStyle,
		getLabel: () => event.event_type,
	};

	const label = config.getLabel(event);
	const details = extractDetails(event);
	const hasDetails = details.length > 0;

	const toggleExpanded = useCallback(() => {
		if (hasDetails) {
			setIsExpanded((prev) => !prev);
		}
	}, [hasDetails]);

	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if ((e.key === 'Enter' || e.key === ' ') && hasDetails) {
				e.preventDefault();
				toggleExpanded();
			}
		},
		[hasDetails, toggleExpanded]
	);

	// Build class names
	const eventClasses = [
		'timeline-event',
		`timeline-event--${config.style}`,
		isExpanded && 'timeline-event--expanded',
		hasDetails && 'timeline-event--expandable',
	]
		.filter(Boolean)
		.join(' ');

	const iconClasses = ['timeline-event-icon', `timeline-event-icon--${config.style}`].join(' ');

	return (
		<article
			className={eventClasses}
			onClick={toggleExpanded}
			onKeyDown={handleKeyDown}
			tabIndex={hasDetails ? 0 : undefined}
			role={hasDetails ? 'button' : undefined}
			aria-expanded={hasDetails ? isExpanded : undefined}
			aria-label={`${label}${showTask ? `, ${event.task_id}: ${event.task_title}` : ''}`}
		>
			{/* Icon */}
			<div className={iconClasses}>
				<Icon name={config.icon} size={16} />
			</div>

			{/* Content */}
			<div className="timeline-event-content">
				{/* Header row */}
				<div className="timeline-event-header">
					<span className="timeline-event-label">{label}</span>
					<span className="timeline-event-time">
						{formatDate(event.created_at, dateFormat)}
					</span>
				</div>

				{/* Task info (when showTask is true) */}
				{showTask && (
					<Link
						to={`/tasks/${event.task_id}`}
						className="timeline-event-task"
						onClick={(e) => e.stopPropagation()}
					>
						<span className="timeline-event-task-id">{event.task_id}</span>
						<span className="timeline-event-task-title">{event.task_title}</span>
					</Link>
				)}

				{/* Expandable details */}
				{hasDetails && isExpanded && (
					<div className="timeline-event-details">
						{details.map((detail, index) => (
							<div key={index} className="timeline-event-detail">
								<span className="timeline-event-detail-prefix">
									{index === details.length - 1 ? '└─' : '├─'}
								</span>
								<span className="timeline-event-detail-label">{detail.label}:</span>
								<span className="timeline-event-detail-value">{detail.value}</span>
							</div>
						))}
					</div>
				)}

				{/* Expand indicator */}
				{hasDetails && !isExpanded && (
					<span className="timeline-event-expand-hint">Click to expand</span>
				)}
			</div>
		</article>
	);
}
