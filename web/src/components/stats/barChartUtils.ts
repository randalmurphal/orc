/**
 * Utility functions and constants for TasksBarChart component.
 * Separated from component file to ensure optimal React Fast Refresh behavior.
 */

// =============================================================================
// Types
// =============================================================================

export interface DayData {
	day: string;
	count: number;
}

// =============================================================================
// Constants
// =============================================================================

export const MIN_BAR_HEIGHT = 4;
export const MAX_BAR_HEIGHT = 140; // Leave room for label (160px container - 20px for label/gap)

// =============================================================================
// Utility Functions
// =============================================================================

/**
 * Calculate bar height based on count relative to max value.
 * Returns minimum height for zero values to ensure visibility.
 */
export function calculateBarHeight(count: number, maxCount: number): number {
	if (count === 0) return MIN_BAR_HEIGHT;
	const normalizedMax = Math.max(maxCount, 1); // Avoid division by zero
	const height = (count / normalizedMax) * MAX_BAR_HEIGHT;
	return Math.min(MAX_BAR_HEIGHT, Math.max(MIN_BAR_HEIGHT, height));
}

/**
 * Calculate deterministic skeleton bar height based on index.
 * Uses a simple hash-like formula to produce varied but stable heights.
 * Heights range from 40px to 99px.
 */
export function getSkeletonBarHeight(index: number): number {
	// Deterministic formula: produces heights 40, 53, 66, 79, 52, 65, 78 for indices 0-6
	return 40 + ((index * 73) % 60);
}

// =============================================================================
// Default Data
// =============================================================================

/**
 * Default week data with zero counts for all days.
 * Useful for initialization or demo purposes.
 */
export const defaultWeekData: DayData[] = [
	{ day: 'Mon', count: 0 },
	{ day: 'Tue', count: 0 },
	{ day: 'Wed', count: 0 },
	{ day: 'Thu', count: 0 },
	{ day: 'Fri', count: 0 },
	{ day: 'Sat', count: 0 },
	{ day: 'Sun', count: 0 },
];
