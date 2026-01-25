/**
 * Tests for formatDate utility - zero time handling
 *
 * These tests verify that Go's zero time value (0001-01-01T00:00:00Z) is
 * correctly detected and returns the fallback value ("Never") instead of
 * being formatted as "Dec 31, 1" (which is the timezone-shifted display).
 *
 * Success Criteria covered:
 * - SC-3: Global formatDate() function returns fallback for zero timestamps
 *
 * TDD Note: These tests are written BEFORE the isZeroTime() helper and
 * the zero-time check in formatDate() are implemented.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { formatDate, isZeroTime } from './formatDate';

// Mock current date for consistent relative time tests
const MOCK_NOW = new Date('2024-06-15T14:00:00Z');

describe('formatDate - zero time handling', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(MOCK_NOW);
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('isZeroTime helper', () => {
		it('returns true for Go zero time (0001-01-01T00:00:00Z)', () => {
			const zeroDate = new Date('0001-01-01T00:00:00Z');
			expect(isZeroTime(zeroDate)).toBe(true);
		});

		it('returns true for Go zero time with milliseconds (0001-01-01T00:00:00.000Z)', () => {
			const zeroDate = new Date('0001-01-01T00:00:00.000Z');
			expect(isZeroTime(zeroDate)).toBe(true);
		});

		it('returns true for any year 1 date (0001-12-31T23:59:59Z)', () => {
			// Any date in year 1 should be considered "zero time" since no real
			// task would ever have a date from year 1 AD
			const year1Date = new Date('0001-12-31T23:59:59Z');
			expect(isZeroTime(year1Date)).toBe(true);
		});

		it('returns true for dates parsed as year 1 due to browser handling', () => {
			// Some browsers might parse zero time slightly differently
			const zeroDate = new Date('0001-01-01T00:00:00.000Z');
			expect(isZeroTime(zeroDate)).toBe(true);
		});

		it('returns false for valid modern dates', () => {
			const modernDate = new Date('2024-06-15T10:30:00Z');
			expect(isZeroTime(modernDate)).toBe(false);
		});

		it('returns false for Unix epoch (1970-01-01T00:00:00Z)', () => {
			// Unix epoch is a valid date, not Go's zero time
			const unixEpoch = new Date('1970-01-01T00:00:00Z');
			expect(isZeroTime(unixEpoch)).toBe(false);
		});

		it('returns false for year 2 dates', () => {
			const year2Date = new Date('0002-01-01T00:00:00Z');
			expect(isZeroTime(year2Date)).toBe(false);
		});

		it('returns true for invalid dates (NaN time value)', () => {
			const invalidDate = new Date('not-a-date');
			expect(isZeroTime(invalidDate)).toBe(true);
		});
	});

	describe('formatDate with zero timestamps', () => {
		it('returns fallback "Never" for Go zero time string (SC-3)', () => {
			const result = formatDate('0001-01-01T00:00:00Z', 'relative');
			expect(result).toBe('Never');
		});

		it('returns fallback "Never" for Go zero time string with absolute format', () => {
			const result = formatDate('0001-01-01T00:00:00Z', 'absolute');
			expect(result).toBe('Never');
		});

		it('returns fallback "Never" for Go zero time string with absolute24 format', () => {
			const result = formatDate('0001-01-01T00:00:00Z', 'absolute24');
			expect(result).toBe('Never');
		});

		it('returns fallback for zero time with milliseconds', () => {
			const result = formatDate('0001-01-01T00:00:00.000Z', 'relative');
			expect(result).toBe('Never');
		});

		it('returns custom fallback when provided for zero time', () => {
			const result = formatDate('0001-01-01T00:00:00Z', 'relative', 'Not set');
			expect(result).toBe('Not set');
		});
	});

	describe('formatDate with valid timestamps (preservation)', () => {
		it('formats valid date string correctly with relative format (SC-2 preservation)', () => {
			// 3 hours before MOCK_NOW
			const result = formatDate('2024-06-15T11:00:00Z', 'relative');
			expect(result).toBe('3h ago');
		});

		it('formats valid date string correctly with absolute format', () => {
			const result = formatDate('2024-06-15T11:00:00Z', 'absolute');
			// Should contain date components (actual format depends on locale)
			expect(result).not.toBe('Never');
			expect(result).not.toBe('');
		});

		it('formats valid date string correctly with absolute24 format', () => {
			const result = formatDate('2024-06-15T11:00:00Z', 'absolute24');
			expect(result).not.toBe('Never');
			expect(result).not.toBe('');
		});

		it('formats Unix epoch correctly (1970-01-01T00:00:00Z)', () => {
			const result = formatDate('1970-01-01T00:00:00Z', 'absolute');
			// Unix epoch is a valid date, should not return "Never"
			expect(result).not.toBe('Never');
			expect(result).not.toBe('');
		});

		it('formats Date object correctly', () => {
			const dateObj = new Date('2024-06-15T11:00:00Z');
			const result = formatDate(dateObj, 'relative');
			expect(result).toBe('3h ago');
		});
	});

	describe('formatDate edge cases (existing behavior preservation)', () => {
		it('returns fallback for null input', () => {
			const result = formatDate(null, 'relative');
			expect(result).toBe('Never');
		});

		it('returns fallback for undefined input', () => {
			const result = formatDate(undefined, 'relative');
			expect(result).toBe('Never');
		});

		it('returns fallback for empty string', () => {
			const result = formatDate('', 'relative');
			expect(result).toBe('Never');
		});

		it('returns fallback for invalid date string', () => {
			const result = formatDate('invalid-date', 'relative');
			expect(result).toBe('Never');
		});

		it('returns custom fallback when provided for null', () => {
			const result = formatDate(null, 'relative', 'N/A');
			expect(result).toBe('N/A');
		});

		it('returns custom fallback when provided for invalid date', () => {
			const result = formatDate('garbage', 'relative', 'Unknown');
			expect(result).toBe('Unknown');
		});
	});
});
