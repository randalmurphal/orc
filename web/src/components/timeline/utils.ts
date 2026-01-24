/**
 * Timeline utilities for date grouping and formatting.
 *
 * Provides functions to:
 * - Determine which date group an event belongs to (today, yesterday, this_week, etc.)
 * - Group arrays of events by their date group
 * - Format date group labels for display
 */

import { isToday, isYesterday, isThisWeek, isThisMonth, format, formatDistanceToNow } from 'date-fns';
import type { TimelineEventData } from './TimelineEvent';

/**
 * Date group identifiers for timeline grouping.
 * Special values: 'today', 'yesterday', 'this_week', 'this_month'
 * Or a formatted date string like 'December 15, 2024' for older dates.
 */
export type DateGroup = 'today' | 'yesterday' | 'this_week' | 'this_month' | string;

/**
 * Determines which date group a given date belongs to.
 *
 * @param date - The date to categorize
 * @returns The date group identifier
 *
 * @example
 * getDateGroup(new Date()) // Returns 'today'
 * getDateGroup(new Date(Date.now() - 86400000)) // Returns 'yesterday' (if applicable)
 */
export function getDateGroup(date: Date): DateGroup {
	if (isToday(date)) return 'today';
	if (isYesterday(date)) return 'yesterday';
	if (isThisWeek(date, { weekStartsOn: 0 })) return 'this_week';
	if (isThisMonth(date)) return 'this_month';
	return format(date, 'MMMM d, yyyy');
}

/**
 * Groups an array of timeline events by their date group.
 *
 * @param events - Array of timeline events to group
 * @returns Map of date groups to arrays of events in that group
 *
 * @example
 * const events = [{ created_at: '2024-01-15T10:00:00Z', ... }];
 * const grouped = groupEventsByDate(events);
 * // Map { 'today' => [...], 'yesterday' => [...] }
 */
export function groupEventsByDate(events: TimelineEventData[]): Map<DateGroup, TimelineEventData[]> {
	const groups = new Map<DateGroup, TimelineEventData[]>();

	for (const event of events) {
		const date = new Date(event.created_at);
		const group = getDateGroup(date);

		if (!groups.has(group)) {
			groups.set(group, []);
		}
		groups.get(group)!.push(event);
	}

	return groups;
}

/**
 * Formats a date group identifier into a human-readable label with event count.
 *
 * @param group - The date group identifier
 * @param count - Number of events in the group
 * @returns Formatted label string
 *
 * @example
 * getDateGroupLabel('today', 5) // Returns 'Today (5 events)'
 * getDateGroupLabel('yesterday', 1) // Returns 'Yesterday (1 event)'
 * getDateGroupLabel('December 15, 2024', 3) // Returns 'December 15, 2024 (3 events)'
 */
export function getDateGroupLabel(group: DateGroup, count: number): string {
	const eventWord = count === 1 ? 'event' : 'events';
	const countSuffix = `(${count} ${eventWord})`;

	switch (group) {
		case 'today':
			return `Today ${countSuffix}`;
		case 'yesterday':
			return `Yesterday ${countSuffix}`;
		case 'this_week':
			return `This Week ${countSuffix}`;
		case 'this_month':
			return `This Month ${countSuffix}`;
		default:
			// It's a formatted date string
			return `${group} ${countSuffix}`;
	}
}

/**
 * Returns the order priority for date groups.
 * Lower numbers appear first.
 *
 * @param group - The date group identifier
 * @returns Order priority (0-4, with 4 being specific dates)
 */
export function getDateGroupOrder(group: DateGroup): number {
	switch (group) {
		case 'today':
			return 0;
		case 'yesterday':
			return 1;
		case 'this_week':
			return 2;
		case 'this_month':
			return 3;
		default:
			return 4;
	}
}

/**
 * Sorts date groups by their natural order (most recent first).
 *
 * @param groups - Array of date group identifiers
 * @returns Sorted array of date groups
 */
export function sortDateGroups(groups: DateGroup[]): DateGroup[] {
	return [...groups].sort((a, b) => {
		const orderA = getDateGroupOrder(a);
		const orderB = getDateGroupOrder(b);

		if (orderA !== orderB) {
			return orderA - orderB;
		}

		// Both are specific dates, sort by date descending
		if (orderA === 4) {
			return new Date(b).getTime() - new Date(a).getTime();
		}

		return 0;
	});
}

/**
 * Formats a timestamp into a relative time string.
 *
 * @param timestamp - ISO8601 timestamp string
 * @returns Relative time string like "2 minutes ago", "1 hour ago", etc.
 *
 * @example
 * formatRelativeTime('2024-01-15T10:00:00Z') // Returns "5 minutes ago"
 */
export function formatRelativeTime(timestamp: string): string {
	return formatDistanceToNow(new Date(timestamp), { addSuffix: true });
}
