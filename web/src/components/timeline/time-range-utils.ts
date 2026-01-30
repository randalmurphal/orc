/**
 * Time range utility functions and types.
 * Extracted from TimeRangeSelector.tsx to satisfy react-refresh/only-export-components.
 */

// Time range options
export type TimeRange = 'today' | 'yesterday' | 'this_week' | 'this_month' | 'custom';

// Custom date range interface
export interface CustomDateRange {
	start: Date;
	end: Date;
}

// Date range calculation utilities

/** Get start of day (midnight local time) */
function startOfDay(date: Date): Date {
	const result = new Date(date);
	result.setHours(0, 0, 0, 0);
	return result;
}

/** Get end of day (23:59:59.999 local time) */
function endOfDay(date: Date): Date {
	const result = new Date(date);
	result.setHours(23, 59, 59, 999);
	return result;
}

/** Subtract days from a date */
export function subDays(date: Date, days: number): Date {
	const result = new Date(date);
	result.setDate(result.getDate() - days);
	return result;
}

/** Get start of week (Sunday local time) */
function startOfWeek(date: Date): Date {
	const result = new Date(date);
	const day = result.getDay();
	result.setDate(result.getDate() - day);
	result.setHours(0, 0, 0, 0);
	return result;
}

/** Get start of month */
function startOfMonth(date: Date): Date {
	const result = new Date(date);
	result.setDate(1);
	result.setHours(0, 0, 0, 0);
	return result;
}

/**
 * Calculate the date range for a given TimeRange preset.
 * Returns { since, until } dates for filtering.
 */
export function getDateRange(
	range: TimeRange,
	customRange?: CustomDateRange
): { since: Date; until: Date } {
	const now = new Date();

	switch (range) {
		case 'today':
			return { since: startOfDay(now), until: now };
		case 'yesterday': {
			const yesterday = subDays(now, 1);
			return { since: startOfDay(yesterday), until: endOfDay(yesterday) };
		}
		case 'this_week':
			return { since: startOfWeek(now), until: now };
		case 'this_month':
			return { since: startOfMonth(now), until: now };
		case 'custom':
			if (customRange) {
				return { since: startOfDay(customRange.start), until: endOfDay(customRange.end) };
			}
			// Default to last 7 days if no custom range provided
			return { since: subDays(now, 7), until: now };
		default:
			return { since: startOfDay(now), until: now };
	}
}
