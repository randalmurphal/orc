import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import { timestampToDate, formatTimestamp, timestampToRelative, timestampToISO } from './time';

/**
 * Creates a protobuf Timestamp representing Go's zero time (0001-01-01 00:00:00 UTC).
 * Go's zero time is approximately -62135596800 seconds from Unix epoch.
 */
function createZeroTimestamp() {
	return create(TimestampSchema, {
		seconds: BigInt(-62135596800),
		nanos: 0,
	});
}

/**
 * Creates a valid protobuf Timestamp from a date string.
 */
function createTimestamp(date: string) {
	const d = new Date(date);
	const ms = d.getTime();
	return create(TimestampSchema, {
		seconds: BigInt(Math.floor(ms / 1000)),
		nanos: (ms % 1000) * 1_000_000,
	});
}

describe('timestampToDate', () => {
	it('returns null for undefined timestamp', () => {
		expect(timestampToDate(undefined)).toBeNull();
	});

	it('returns null for zero-value Go timestamp (year 1 AD)', () => {
		const zeroTs = createZeroTimestamp();
		expect(timestampToDate(zeroTs)).toBeNull();
	});

	it('returns valid Date for normal timestamps', () => {
		const ts = createTimestamp('2024-06-15T10:30:00Z');
		const result = timestampToDate(ts);
		expect(result).toBeInstanceOf(Date);
		expect(result?.getFullYear()).toBe(2024);
	});

	it('returns valid Date for Unix epoch (1970-01-01)', () => {
		const ts = create(TimestampSchema, { seconds: BigInt(0), nanos: 0 });
		const result = timestampToDate(ts);
		expect(result).toBeInstanceOf(Date);
		// Use UTC year to avoid timezone issues
		expect(result?.getUTCFullYear()).toBe(1970);
	});
});

describe('formatTimestamp', () => {
	it('returns N/A for undefined timestamp', () => {
		expect(formatTimestamp(undefined)).toBe('N/A');
	});

	it('returns N/A for zero-value Go timestamp', () => {
		const zeroTs = createZeroTimestamp();
		expect(formatTimestamp(zeroTs)).toBe('N/A');
	});

	it('returns formatted date for valid timestamp', () => {
		const ts = createTimestamp('2024-06-15T10:30:00Z');
		const result = formatTimestamp(ts);
		// Should contain year 2024
		expect(result).toContain('2024');
	});
});

describe('timestampToRelative', () => {
	it('returns N/A for zero-value Go timestamp', () => {
		const zeroTs = createZeroTimestamp();
		expect(timestampToRelative(zeroTs)).toBe('N/A');
	});
});

describe('timestampToISO', () => {
	it('returns empty string for zero-value Go timestamp', () => {
		const zeroTs = createZeroTimestamp();
		expect(timestampToISO(zeroTs)).toBe('');
	});
});
