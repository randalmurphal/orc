/**
 * Timestamp conversion utilities for protobuf Timestamp <-> JavaScript Date
 */

import type { Timestamp } from '@bufbuild/protobuf/wkt';
import { timestampFromDate, timestampDate } from '@bufbuild/protobuf/wkt';

/**
 * Convert protobuf Timestamp to JavaScript Date.
 * Returns null if timestamp is undefined or represents Go's zero time.
 */
export function timestampToDate(ts: Timestamp | undefined): Date | null {
	if (!ts) return null;
	// Go's zero time (0001-01-01) has negative seconds - treat as unset
	if (ts.seconds < 0n) return null;
	return timestampDate(ts);
}

/**
 * Convert JavaScript Date to protobuf Timestamp.
 */
export function dateToTimestamp(date: Date): Timestamp {
	return timestampFromDate(date);
}

/**
 * Convert protobuf Timestamp to ISO 8601 string.
 * Returns empty string if timestamp is undefined.
 */
export function timestampToISO(ts: Timestamp | undefined): string {
	const date = timestampToDate(ts);
	return date?.toISOString() ?? '';
}

/**
 * Format a protobuf Timestamp for display.
 * Returns 'N/A' if timestamp is undefined.
 */
export function formatTimestamp(
	ts: Timestamp | undefined,
	options?: Intl.DateTimeFormatOptions
): string {
	const date = timestampToDate(ts);
	if (!date) return 'N/A';
	return date.toLocaleString(undefined, options);
}

/**
 * Get relative time string (e.g., "2 hours ago").
 * Returns 'N/A' if timestamp is undefined.
 */
export function timestampToRelative(ts: Timestamp | undefined): string {
	const date = timestampToDate(ts);
	if (!date) return 'N/A';

	const now = Date.now();
	const diff = now - date.getTime();
	const seconds = Math.floor(diff / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);
	const days = Math.floor(hours / 24);

	if (days > 0) return `${days}d ago`;
	if (hours > 0) return `${hours}h ago`;
	if (minutes > 0) return `${minutes}m ago`;
	if (seconds > 0) return `${seconds}s ago`;
	return 'just now';
}
