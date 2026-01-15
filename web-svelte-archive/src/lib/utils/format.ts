/**
 * Formatting utilities for display values
 */

/**
 * Formats a date as a relative time string (e.g., "2 hours ago")
 * @param date - Date object or ISO string
 * @returns Human-readable relative time
 */
export function formatRelativeTime(date: Date | string): string {
	const d = typeof date === 'string' ? new Date(date) : date;
	const now = new Date();
	const diffMs = now.getTime() - d.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);

	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	if (diffHours < 24) return `${diffHours}h ago`;
	if (diffDays < 7) return `${diffDays}d ago`;
	return d.toLocaleDateString();
}

/**
 * Formats a duration in milliseconds to a human-readable string (e.g., "1h 23m")
 * @param ms - Duration in milliseconds
 * @returns Formatted duration string
 */
export function formatDuration(ms: number): string {
	if (ms < 1000) return `${ms}ms`;

	const seconds = Math.floor(ms / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);

	if (hours > 0) {
		const remainingMins = minutes % 60;
		return remainingMins > 0 ? `${hours}h ${remainingMins}m` : `${hours}h`;
	}

	if (minutes > 0) {
		const remainingSecs = seconds % 60;
		return remainingSecs > 0 ? `${minutes}m ${remainingSecs}s` : `${minutes}m`;
	}

	return `${seconds}s`;
}

/**
 * Formats a number to a compact string with K/M/B suffixes (e.g., "1.2K")
 * @param n - Number to format
 * @returns Compact formatted string
 */
export function formatCompactNumber(n: number): string {
	if (n >= 1_000_000_000) {
		return `${(n / 1_000_000_000).toFixed(1)}B`;
	}
	if (n >= 1_000_000) {
		return `${(n / 1_000_000).toFixed(1)}M`;
	}
	if (n >= 1_000) {
		return `${(n / 1_000).toFixed(1)}K`;
	}
	return String(n);
}
