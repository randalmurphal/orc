/**
 * TimelineGroup component - Collapsible container for grouped timeline events.
 *
 * Groups timeline events by date (Today, Yesterday, This Week, etc.) with
 * a collapsible header showing the date label and event count.
 *
 * Features:
 * - Collapsible/expandable header with chevron indicator
 * - localStorage persistence for collapse state
 * - Keyboard navigation (Enter/Space to toggle)
 * - Accessible with ARIA attributes
 */

import { useState, useCallback, useId, useMemo } from 'react';
import { Icon } from '@/components/ui';
import { TimelineEvent, type TimelineEventData } from './TimelineEvent';
import './TimelineGroup.css';

const STORAGE_KEY = 'timeline-collapsed-groups';

export interface TimelineGroupProps {
	/** Unique identifier for the group (e.g., 'today', 'yesterday') */
	groupId: string;
	/** Display label including event count (e.g., "Today (5 events)") */
	label: string;
	/** Array of events to display in this group */
	events: TimelineEventData[];
	/** Initial expanded state. Defaults to true. */
	defaultExpanded?: boolean;
	/** Callback when expand state changes */
	onToggle?: (groupId: string, isExpanded: boolean) => void;
}

/**
 * Reads collapsed groups from localStorage.
 * Returns empty array if localStorage is unavailable or data is invalid.
 */
function readCollapsedGroups(): string[] {
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (!stored) return [];
		const parsed = JSON.parse(stored);
		return Array.isArray(parsed) ? parsed : [];
	} catch {
		return [];
	}
}

/**
 * Saves collapsed groups to localStorage.
 * Silently fails if localStorage is unavailable.
 */
function saveCollapsedGroups(groups: string[]): void {
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(groups));
	} catch {
		// Ignore localStorage errors
	}
}

/**
 * TimelineGroup displays a collapsible section of timeline events.
 *
 * @example
 * <TimelineGroup
 *   groupId="today"
 *   label="Today (5 events)"
 *   events={todayEvents}
 *   defaultExpanded={true}
 * />
 */
export function TimelineGroup({
	groupId,
	label,
	events,
	defaultExpanded = true,
	onToggle,
}: TimelineGroupProps) {
	// Generate unique IDs for accessibility
	const headerId = useId();
	const contentId = useId();

	// Determine initial expanded state from localStorage or default
	const initialExpanded = useMemo(() => {
		const collapsedGroups = readCollapsedGroups();
		if (collapsedGroups.includes(groupId)) {
			return false;
		}
		return defaultExpanded;
	}, [groupId, defaultExpanded]);

	const [isExpanded, setIsExpanded] = useState(initialExpanded);

	// Toggle expand state
	const handleToggle = useCallback(() => {
		setIsExpanded((prev) => {
			const newValue = !prev;

			// Update localStorage
			const collapsedGroups = readCollapsedGroups();
			if (newValue) {
				// Expanding - remove from collapsed list
				const updated = collapsedGroups.filter((g) => g !== groupId);
				saveCollapsedGroups(updated);
			} else {
				// Collapsing - add to collapsed list
				if (!collapsedGroups.includes(groupId)) {
					collapsedGroups.push(groupId);
					saveCollapsedGroups(collapsedGroups);
				}
			}

			// Call callback
			onToggle?.(groupId, newValue);

			return newValue;
		});
	}, [groupId, onToggle]);

	// Handle keyboard navigation
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				handleToggle();
			}
		},
		[handleToggle]
	);

	// Build class names
	const groupClasses = [
		'timeline-group',
		isExpanded ? 'timeline-group--expanded' : '',
	]
		.filter(Boolean)
		.join(' ');

	return (
		<section className={groupClasses} role="region" aria-labelledby={headerId}>
			{/* Header */}
			<button
				type="button"
				id={headerId}
				className="timeline-group-header"
				onClick={handleToggle}
				onKeyDown={handleKeyDown}
				aria-expanded={isExpanded}
				aria-controls={contentId}
			>
				<span data-icon className="timeline-group-icon-wrapper">
					<Icon
						name={isExpanded ? 'chevron-down' : 'chevron-right'}
						size={16}
						className="timeline-group-chevron"
					/>
				</span>
				<span className="timeline-group-label">{label}</span>
			</button>

			{/* Content */}
			<div
				id={contentId}
				className="timeline-group-content"
				aria-hidden={!isExpanded}
			>
				{isExpanded &&
					events.map((event) => (
						<TimelineEvent key={event.id} event={event} />
					))}
			</div>
		</section>
	);
}
