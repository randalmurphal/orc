import type { DateFormat } from '@/stores/preferencesStore';

/**
 * Check if a Date represents Go's zero time value (0001-01-01T00:00:00Z).
 *
 * Go's time.Time zero value is year 1 AD, which JavaScript parses as a valid date.
 * This helper detects such dates to handle them as "not set" rather than displaying
 * incorrect values like "Dec 31, 1" (timezone-shifted year 1).
 *
 * Also returns true for invalid dates (NaN time value) for convenience.
 *
 * @param date - Date object to check
 * @returns true if the date is Go's zero time (year 1) or invalid
 */
export function isZeroTime(date: Date): boolean {
	// Invalid dates should also be treated as "zero time" for convenience
	if (isNaN(date.getTime())) return true;

	// Go's zero time is year 1 AD. No legitimate task would have dates from year 1.
	// Use getUTCFullYear() for consistent results across timezones.
	return date.getUTCFullYear() <= 1;
}

/**
 * Format a date according to the user's date format preference.
 *
 * @param date - Date string, Date object, or null/undefined
 * @param format - The date format preference (relative, absolute, absolute24)
 * @param fallback - Value to return if date is null/undefined (default: 'Never')
 * @returns Formatted date string
 */
export function formatDate(
	date: string | Date | null | undefined,
	format: DateFormat,
	fallback = 'Never'
): string {
	if (!date) return fallback;

	const dateObj = typeof date === 'string' ? new Date(date) : date;

	// Check for invalid date or Go's zero time (year 1)
	if (isZeroTime(dateObj)) return fallback;

	switch (format) {
		case 'relative':
			return formatRelative(dateObj);
		case 'absolute':
			return formatAbsolute(dateObj, false);
		case 'absolute24':
			return formatAbsolute(dateObj, true);
		default:
			return formatRelative(dateObj);
	}
}

/**
 * Format a date as relative time (e.g., "5m ago", "2h ago", "3d ago")
 */
function formatRelative(date: Date): string {
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();

	// Handle future dates
	if (diffMs < 0) {
		return 'in the future';
	}

	const diffSecs = Math.floor(diffMs / 1000);
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);
	const diffWeeks = Math.floor(diffDays / 7);
	const diffMonths = Math.floor(diffDays / 30);
	const diffYears = Math.floor(diffDays / 365);

	if (diffSecs < 60) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	if (diffHours < 24) return `${diffHours}h ago`;
	if (diffDays < 7) return `${diffDays}d ago`;
	if (diffWeeks < 4) return `${diffWeeks}w ago`;
	if (diffMonths < 12) return `${diffMonths}mo ago`;
	return `${diffYears}y ago`;
}

/**
 * Format a date as absolute time
 * @param date - The date to format
 * @param use24Hour - Whether to use 24-hour format
 */
function formatAbsolute(date: Date, use24Hour: boolean): string {
	const now = new Date();
	const isToday = isSameDay(date, now);
	const isThisYear = date.getFullYear() === now.getFullYear();

	// Format time
	const timeOptions: Intl.DateTimeFormatOptions = {
		hour: 'numeric',
		minute: '2-digit',
		...(use24Hour ? { hour12: false } : { hour12: true }),
	};
	const timeStr = date.toLocaleTimeString(undefined, timeOptions);

	// If today, just show time
	if (isToday) {
		return timeStr;
	}

	// Format date
	const dateOptions: Intl.DateTimeFormatOptions = {
		month: 'short',
		day: 'numeric',
		...(!isThisYear ? { year: 'numeric' } : {}),
	};
	const dateStr = date.toLocaleDateString(undefined, dateOptions);

	return `${dateStr} ${timeStr}`;
}

/**
 * Check if two dates are the same day
 */
function isSameDay(date1: Date, date2: Date): boolean {
	return (
		date1.getFullYear() === date2.getFullYear() &&
		date1.getMonth() === date2.getMonth() &&
		date1.getDate() === date2.getDate()
	);
}

/**
 * React hook helper to get a formatted date with the current preference.
 * This is a utility function - components should use the useFormattedDate hook
 * from hooks/useFormattedDate.ts for automatic updates.
 */
export function formatDateWithPreference(
	date: string | Date | null | undefined,
	format: DateFormat,
	fallback?: string
): string {
	return formatDate(date, format, fallback);
}
