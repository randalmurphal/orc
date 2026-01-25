/**
 * Shared formatting utilities for consistent display of numbers, costs, and durations.
 */

/**
 * Format large numbers with K/M/B suffixes.
 * - Values >= 1B display as "1.23B"
 * - Values >= 1M display as "1.5M" (removes trailing zeros)
 * - Values >= 1K display as "12.5K" (removes trailing zeros)
 * - Values < 1K display as-is
 */
export function formatNumber(value: number): string {
	const absValue = Math.abs(value);
	const sign = value < 0 ? '-' : '';

	if (absValue >= 1_000_000_000) {
		const formatted = absValue / 1_000_000_000;
		return `${sign}${formatted.toFixed(2).replace(/\.?0+$/, '')}B`;
	}
	if (absValue >= 1_000_000) {
		const formatted = absValue / 1_000_000;
		return `${sign}${formatted.toFixed(1).replace(/\.0$/, '')}M`;
	}
	if (absValue >= 1_000) {
		const formatted = absValue / 1_000;
		return `${sign}${formatted.toFixed(1).replace(/\.0$/, '')}K`;
	}
	return String(value);
}


/**
 * Format cost in USD with appropriate precision.
 * - Values >= 1M display as "$1.50M" (always 2 decimal places)
 * - Values >= 1K display as "$1.5K" (removes trailing zeros)
 * - Values < 1K display as "$1.23"
 */
export function formatCost(cost: number): string {
	if (cost >= 1_000_000) {
		const formatted = cost / 1_000_000;
		return `$${formatted.toFixed(2)}M`;
	}
	if (cost >= 1_000) {
		const formatted = cost / 1_000;
		return `$${formatted.toFixed(1).replace(/\.0$/, '')}K`;
	}
	return `$${cost.toFixed(2)}`;
}

/**
 * Format duration from a start time to now in human-readable form.
 * Returns "Xh Ym" for hours, "Xm" for minutes, "Xs" for seconds.
 */
export function formatDuration(startTime: Date | null): string {
	if (!startTime) return '0m';

	const now = new Date();
	const diffMs = now.getTime() - startTime.getTime();

	// Don't show negative time
	if (diffMs < 0) return '0m';

	const seconds = Math.floor(diffMs / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);

	if (hours > 0) {
		return `${hours}h ${minutes % 60}m`;
	}
	if (minutes > 0) {
		return `${minutes}m`;
	}
	return `${seconds}s`;
}

/**
 * Format duration in milliseconds to human-readable form.
 * Returns "Xh Ym Zs" for hours, "Xm Ys" for minutes, "Xs" for seconds.
 */
export function formatDurationMs(ms: number): string {
	const seconds = Math.floor(ms / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);

	if (hours > 0) {
		const remainingMinutes = minutes % 60;
		const remainingSeconds = seconds % 60;
		if (remainingSeconds > 0) {
			return `${hours}h ${remainingMinutes}m ${remainingSeconds}s`;
		}
		return `${hours}h ${remainingMinutes}m`;
	}
	if (minutes > 0) {
		const remainingSeconds = seconds % 60;
		if (remainingSeconds > 0) {
			return `${minutes}m ${remainingSeconds}s`;
		}
		return `${minutes}m`;
	}
	return `${seconds}s`;
}

/**
 * Format percentage value.
 */
export function formatPercentage(value: number): string {
	return `${Math.round(value)}%`;
}

/**
 * Format trend value with sign.
 */
export function formatTrend(value: number): string {
	if (value > 0) return `+${value}`;
	return String(value);
}

/**
 * Format large numbers for display with K/M/B suffixes and comma formatting.
 * - Values >= 1B display as "1.23B"
 * - Values >= 1M display as "1.23M"
 * - Values >= 10K display as "127.5K" (with decimals if needed)
 * - Values 1K-10K display as "1,234" (comma-formatted)
 * - Values < 1K display as-is
 *
 * This is the legacy format used by Stat component.
 */
export function formatLargeNumber(value: number): string {
	const absValue = Math.abs(value);
	const sign = value < 0 ? '-' : '';

	if (absValue >= 1_000_000_000) {
		const formatted = absValue / 1_000_000_000;
		return `${sign}${formatted.toFixed(2).replace(/\.?0+$/, '')}B`;
	}
	if (absValue >= 1_000_000) {
		const formatted = absValue / 1_000_000;
		return `${sign}${formatted.toFixed(2).replace(/\.?0+$/, '')}M`;
	}
	if (absValue >= 10_000) {
		const formatted = absValue / 1_000;
		// Remove trailing zeros but keep meaningful decimals
		const str = formatted.toFixed(1).replace(/\.0$/, '');
		return `${sign}${str}K`;
	}
	if (absValue >= 1_000) {
		// Comma-format for 1K-10K range
		return value.toLocaleString('en-US');
	}
	return String(value);
}
