/**
 * Tests for timeline date grouping utilities
 *
 * These tests verify the date grouping logic that organizes timeline events
 * into collapsible date groups (Today, Yesterday, This Week, This Month, etc.)
 *
 * Success Criteria covered:
 * - SC-4: Events are grouped by date with collapsible headers
 *
 * TDD Note: These tests are written BEFORE the implementation exists.
 * The utils.ts file and its functions do not yet exist.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { TimelineEventData } from './TimelineEvent';

// Import from the file we're going to create
// This will fail until implementation exists
import { getDateGroup, groupEventsByDate, getDateGroupLabel } from './utils';

// Mock current date for consistent tests
const MOCK_NOW = new Date('2024-03-15T14:00:00Z'); // Friday

describe('utils - date grouping', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(MOCK_NOW);
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	// Helper to create a mock event at a specific date
	function createEventAt(dateStr: string, id = 1): TimelineEventData {
		return {
			id,
			task_id: 'TASK-001',
			task_title: 'Test Task',
			event_type: 'phase_completed',
			data: {},
			source: 'executor',
			created_at: dateStr,
		};
	}

	describe('getDateGroup', () => {
		it('returns "today" for dates within the current day', () => {
			// Today morning
			expect(getDateGroup(new Date('2024-03-15T08:00:00Z'))).toBe('today');
			// Today afternoon (same as MOCK_NOW)
			expect(getDateGroup(new Date('2024-03-15T14:00:00Z'))).toBe('today');
			// Today evening
			expect(getDateGroup(new Date('2024-03-15T20:00:00Z'))).toBe('today');
		});

		it('returns "yesterday" for dates within the previous day', () => {
			// Yesterday morning
			expect(getDateGroup(new Date('2024-03-14T08:00:00Z'))).toBe('yesterday');
			// Yesterday evening
			expect(getDateGroup(new Date('2024-03-14T23:59:59Z'))).toBe('yesterday');
		});

		it('returns "this_week" for dates earlier this week (but not yesterday or today)', () => {
			// Wednesday (2 days ago)
			expect(getDateGroup(new Date('2024-03-13T12:00:00Z'))).toBe('this_week');
			// Monday (4 days ago, start of week)
			expect(getDateGroup(new Date('2024-03-11T12:00:00Z'))).toBe('this_week');
		});

		it('returns "this_month" for dates in current month but outside this week', () => {
			// First day of March
			expect(getDateGroup(new Date('2024-03-01T12:00:00Z'))).toBe('this_month');
			// March 5 (last week)
			expect(getDateGroup(new Date('2024-03-05T12:00:00Z'))).toBe('this_month');
		});

		it('returns formatted date string for dates outside this month', () => {
			// February
			const febGroup = getDateGroup(new Date('2024-02-15T12:00:00Z'));
			expect(febGroup).not.toBe('today');
			expect(febGroup).not.toBe('yesterday');
			expect(febGroup).not.toBe('this_week');
			expect(febGroup).not.toBe('this_month');
			// Should be a formatted date like "February 15, 2024" or similar
			expect(typeof febGroup).toBe('string');
		});

		it('handles dates from previous years', () => {
			const oldGroup = getDateGroup(new Date('2023-06-15T12:00:00Z'));
			expect(oldGroup).not.toBe('today');
			expect(oldGroup).not.toBe('yesterday');
			expect(oldGroup).not.toBe('this_week');
			expect(oldGroup).not.toBe('this_month');
		});
	});

	describe('groupEventsByDate', () => {
		it('returns empty Map for empty events array', () => {
			const groups = groupEventsByDate([]);
			expect(groups).toBeInstanceOf(Map);
			expect(groups.size).toBe(0);
		});

		it('groups events occurring today under "today"', () => {
			const events = [
				createEventAt('2024-03-15T10:00:00Z', 1),
				createEventAt('2024-03-15T12:00:00Z', 2),
			];

			const groups = groupEventsByDate(events);
			expect(groups.has('today')).toBe(true);
			expect(groups.get('today')).toHaveLength(2);
		});

		it('groups events from multiple days correctly', () => {
			const events = [
				createEventAt('2024-03-15T10:00:00Z', 1), // Today
				createEventAt('2024-03-14T10:00:00Z', 2), // Yesterday
				createEventAt('2024-03-13T10:00:00Z', 3), // This week
			];

			const groups = groupEventsByDate(events);
			expect(groups.has('today')).toBe(true);
			expect(groups.has('yesterday')).toBe(true);
			expect(groups.has('this_week')).toBe(true);

			expect(groups.get('today')).toHaveLength(1);
			expect(groups.get('yesterday')).toHaveLength(1);
			expect(groups.get('this_week')).toHaveLength(1);
		});

		it('preserves event order within each group (most recent first)', () => {
			const events = [
				createEventAt('2024-03-15T16:00:00Z', 1), // Later today
				createEventAt('2024-03-15T10:00:00Z', 2), // Earlier today
			];

			const groups = groupEventsByDate(events);
			const todayEvents = groups.get('today');
			expect(todayEvents?.[0].id).toBe(1); // Most recent first
			expect(todayEvents?.[1].id).toBe(2);
		});

		it('groups all events in same day into one group when all are today', () => {
			const events = Array.from({ length: 5 }, (_, i) =>
				createEventAt(`2024-03-15T${10 + i}:00:00Z`, i + 1)
			);

			const groups = groupEventsByDate(events);
			expect(groups.size).toBe(1);
			expect(groups.get('today')).toHaveLength(5);
		});

		it('creates separate groups for events spanning multiple months', () => {
			const events = [
				createEventAt('2024-03-15T10:00:00Z', 1), // Today
				createEventAt('2024-02-15T10:00:00Z', 2), // Last month
				createEventAt('2024-01-15T10:00:00Z', 3), // Two months ago
			];

			const groups = groupEventsByDate(events);
			expect(groups.size).toBe(3);
			expect(groups.has('today')).toBe(true);
		});
	});

	describe('getDateGroupLabel', () => {
		it('returns "Today" for today group with event count', () => {
			const label = getDateGroupLabel('today', 5);
			expect(label).toContain('Today');
			expect(label).toContain('5');
		});

		it('returns "Yesterday" for yesterday group with event count', () => {
			const label = getDateGroupLabel('yesterday', 3);
			expect(label).toContain('Yesterday');
			expect(label).toContain('3');
		});

		it('returns "This Week" for this_week group', () => {
			const label = getDateGroupLabel('this_week', 10);
			expect(label).toContain('This Week');
			expect(label).toContain('10');
		});

		it('returns "This Month" for this_month group', () => {
			const label = getDateGroupLabel('this_month', 25);
			expect(label).toContain('This Month');
			expect(label).toContain('25');
		});

		it('returns formatted date string for specific date groups', () => {
			// Passing a specific date string as group
			const label = getDateGroupLabel('February 15, 2024', 2);
			expect(label).toContain('February 15');
		});

		it('pluralizes "event" correctly for count of 1', () => {
			const label = getDateGroupLabel('today', 1);
			expect(label).toMatch(/1\s+event(?!\s*s)/);
		});

		it('pluralizes "events" correctly for count > 1', () => {
			const label = getDateGroupLabel('today', 2);
			expect(label).toMatch(/2\s+events/);
		});
	});
});
