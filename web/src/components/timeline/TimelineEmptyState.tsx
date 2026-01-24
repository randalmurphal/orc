/**
 * TimelineEmptyState component - Empty state display for timeline view.
 *
 * Shows when no events match the current filters or time range.
 * Provides clear messaging and optional action to clear filters.
 */

import { Icon } from '@/components/ui';
import './TimelineEmptyState.css';

export interface TimelineEmptyStateProps {
	/** Whether filters are currently active */
	hasFilters?: boolean;
	/** Callback to clear all filters */
	onClearFilters?: () => void;
	/** Custom message to display */
	message?: string;
}

/**
 * TimelineEmptyState displays a message when no events are shown.
 *
 * @example
 * <TimelineEmptyState />
 *
 * @example
 * // With active filters
 * <TimelineEmptyState hasFilters onClearFilters={clearFilters} />
 */
export function TimelineEmptyState({
	hasFilters = false,
	onClearFilters,
	message,
}: TimelineEmptyStateProps) {
	const defaultMessage = hasFilters
		? 'No events found. Try adjusting your filters.'
		: 'No events found for this time period.';

	return (
		<div className="timeline-empty-state" role="status" aria-live="polite">
			<div className="timeline-empty-state-icon">
				<Icon name="calendar" size={48} />
			</div>
			<p className="timeline-empty-state-message">{message || defaultMessage}</p>
			{hasFilters && onClearFilters && (
				<button
					type="button"
					className="timeline-empty-state-action"
					onClick={onClearFilters}
				>
					Clear filters
				</button>
			)}
		</div>
	);
}
