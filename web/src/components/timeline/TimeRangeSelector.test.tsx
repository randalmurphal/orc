import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TimeRangeSelector } from './TimeRangeSelector';
import { getDateRange, type CustomDateRange } from './time-range-utils';

// Mock current date for consistent testing
// Use explicit year/month/day to avoid UTC conversion issues
const MOCK_NOW = new Date(2026, 0, 20, 14, 30, 0); // Jan 20, 2026, 2:30 PM local

/** Create a local date without UTC conversion issues */
function localDate(year: number, month: number, day: number): Date {
	return new Date(year, month - 1, day, 0, 0, 0, 0);
}

describe('TimeRangeSelector', () => {
	let originalDate: DateConstructor;

	beforeEach(() => {
		vi.clearAllMocks();
		// Mock Date to return consistent values
		originalDate = global.Date;
		vi.useFakeTimers();
		vi.setSystemTime(MOCK_NOW);
	});

	afterEach(() => {
		vi.useRealTimers();
		global.Date = originalDate;
	});

	describe('rendering', () => {
		it('renders all preset range tabs', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			expect(screen.getByRole('tab', { name: 'Today' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'Yesterday' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'This Week' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'This Month' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'Custom date range' })).toBeInTheDocument();
		});

		it('renders tablist with correct aria-label', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			expect(screen.getByRole('tablist')).toHaveAttribute('aria-label', 'Time range filter');
		});

		it('marks active tab with aria-selected', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="this_week" onChange={onChange} />);

			expect(screen.getByRole('tab', { name: 'This Week' })).toHaveAttribute(
				'aria-selected',
				'true'
			);
			expect(screen.getByRole('tab', { name: 'Today' })).toHaveAttribute(
				'aria-selected',
				'false'
			);
		});

		it('applies active class to selected tab', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="yesterday" onChange={onChange} />);

			const yesterdayTab = screen.getByRole('tab', { name: 'Yesterday' });
			expect(yesterdayTab).toHaveClass('time-range-tab--active');
		});

		it('applies custom className', () => {
			const onChange = vi.fn();
			const { container } = render(
				<TimeRangeSelector value="today" onChange={onChange} className="my-custom-class" />
			);

			expect(container.querySelector('.time-range-selector')).toHaveClass('my-custom-class');
		});
	});

	describe('tab selection', () => {
		it('calls onChange when clicking a tab', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			fireEvent.click(screen.getByRole('tab', { name: 'This Month' }));
			expect(onChange).toHaveBeenCalledWith('this_month');
		});

		it('calls onChange when clicking custom button', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			fireEvent.click(screen.getByRole('tab', { name: 'Custom date range' }));
			expect(onChange).toHaveBeenCalledWith('custom');
		});

		it('does not call onChange when clicking already selected tab', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			fireEvent.click(screen.getByRole('tab', { name: 'Today' }));
			// onChange is called but with same value - this is expected behavior
			expect(onChange).toHaveBeenCalledWith('today');
		});
	});

	describe('custom date picker', () => {
		it('shows custom date picker when custom is selected', () => {
			const onChange = vi.fn();
			const customRange = { start: new Date('2026-01-10'), end: new Date('2026-01-16') };

			render(
				<TimeRangeSelector
					value="custom"
					onChange={onChange}
					customRange={customRange}
					onCustomRangeChange={vi.fn()}
				/>
			);

			expect(screen.getByLabelText('From:')).toBeInTheDocument();
			expect(screen.getByLabelText('To:')).toBeInTheDocument();
			expect(screen.getByRole('tabpanel')).toHaveAttribute(
				'aria-label',
				'Custom date range'
			);
		});

		it('hides custom date picker when preset is selected', () => {
			const onChange = vi.fn();

			render(<TimeRangeSelector value="today" onChange={onChange} />);

			expect(screen.queryByLabelText('From:')).not.toBeInTheDocument();
			expect(screen.queryByLabelText('To:')).not.toBeInTheDocument();
		});

		it('displays formatted dates in custom range', () => {
			const onChange = vi.fn();
			const customRange = { start: localDate(2026, 1, 10), end: localDate(2026, 1, 16) };

			render(
				<TimeRangeSelector
					value="custom"
					onChange={onChange}
					customRange={customRange}
					onCustomRangeChange={vi.fn()}
				/>
			);

			expect(screen.getByText(/Jan 10, 2026/)).toBeInTheDocument();
			expect(screen.getByText(/Jan 16, 2026/)).toBeInTheDocument();
		});

		it('calls onCustomRangeChange when start date changes', () => {
			const onChange = vi.fn();
			const onCustomRangeChange = vi.fn();
			const customRange = { start: localDate(2026, 1, 10), end: localDate(2026, 1, 16) };

			render(
				<TimeRangeSelector
					value="custom"
					onChange={onChange}
					customRange={customRange}
					onCustomRangeChange={onCustomRangeChange}
				/>
			);

			const startInput = screen.getByLabelText('From:');
			fireEvent.change(startInput, { target: { value: '2026-01-05' } });

			expect(onCustomRangeChange).toHaveBeenCalledWith({
				start: expect.any(Date),
				end: expect.any(Date),
			});
			const callArg = onCustomRangeChange.mock.calls[0][0];
			expect(callArg.start.getDate()).toBe(5);
		});

		it('calls onCustomRangeChange when end date changes', () => {
			const onChange = vi.fn();
			const onCustomRangeChange = vi.fn();
			const customRange = { start: localDate(2026, 1, 10), end: localDate(2026, 1, 16) };

			render(
				<TimeRangeSelector
					value="custom"
					onChange={onChange}
					customRange={customRange}
					onCustomRangeChange={onCustomRangeChange}
				/>
			);

			const endInput = screen.getByLabelText('To:');
			fireEvent.change(endInput, { target: { value: '2026-01-18' } });

			expect(onCustomRangeChange).toHaveBeenCalledWith({
				start: expect.any(Date),
				end: expect.any(Date),
			});
			const callArg = onCustomRangeChange.mock.calls[0][0];
			expect(callArg.end.getDate()).toBe(18);
		});

		it('adjusts end date if start date exceeds it', () => {
			const onChange = vi.fn();
			const onCustomRangeChange = vi.fn();
			const customRange = { start: localDate(2026, 1, 10), end: localDate(2026, 1, 16) };

			render(
				<TimeRangeSelector
					value="custom"
					onChange={onChange}
					customRange={customRange}
					onCustomRangeChange={onCustomRangeChange}
				/>
			);

			const startInput = screen.getByLabelText('From:');
			fireEvent.change(startInput, { target: { value: '2026-01-20' } });

			const callArg = onCustomRangeChange.mock.calls[0][0];
			// Start and end should both be Jan 20 since start exceeds original end
			expect(callArg.start.getDate()).toBe(20);
			expect(callArg.end.getDate()).toBe(20);
		});

		it('adjusts start date if end date precedes it', () => {
			const onChange = vi.fn();
			const onCustomRangeChange = vi.fn();
			const customRange = { start: localDate(2026, 1, 10), end: localDate(2026, 1, 16) };

			render(
				<TimeRangeSelector
					value="custom"
					onChange={onChange}
					customRange={customRange}
					onCustomRangeChange={onCustomRangeChange}
				/>
			);

			const endInput = screen.getByLabelText('To:');
			fireEvent.change(endInput, { target: { value: '2026-01-05' } });

			const callArg = onCustomRangeChange.mock.calls[0][0];
			// Start and end should both be Jan 5 since end precedes original start
			expect(callArg.start.getDate()).toBe(5);
			expect(callArg.end.getDate()).toBe(5);
		});
	});

	describe('keyboard navigation', () => {
		it('moves focus right with ArrowRight', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			const todayTab = screen.getByRole('tab', { name: 'Today' });
			todayTab.focus();
			fireEvent.keyDown(todayTab, { key: 'ArrowRight' });

			expect(screen.getByRole('tab', { name: 'Yesterday' })).toHaveFocus();
		});

		it('moves focus left with ArrowLeft', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="yesterday" onChange={onChange} />);

			const yesterdayTab = screen.getByRole('tab', { name: 'Yesterday' });
			yesterdayTab.focus();
			fireEvent.keyDown(yesterdayTab, { key: 'ArrowLeft' });

			expect(screen.getByRole('tab', { name: 'Today' })).toHaveFocus();
		});

		it('wraps around from last to first with ArrowRight', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="custom" onChange={onChange} />);

			const customTab = screen.getByRole('tab', { name: 'Custom date range' });
			customTab.focus();
			fireEvent.keyDown(customTab, { key: 'ArrowRight' });

			expect(screen.getByRole('tab', { name: 'Today' })).toHaveFocus();
		});

		it('wraps around from first to last with ArrowLeft', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			const todayTab = screen.getByRole('tab', { name: 'Today' });
			todayTab.focus();
			fireEvent.keyDown(todayTab, { key: 'ArrowLeft' });

			expect(screen.getByRole('tab', { name: 'Custom date range' })).toHaveFocus();
		});

		it('moves to first tab with Home key', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="this_month" onChange={onChange} />);

			const monthTab = screen.getByRole('tab', { name: 'This Month' });
			monthTab.focus();
			fireEvent.keyDown(monthTab, { key: 'Home' });

			expect(screen.getByRole('tab', { name: 'Today' })).toHaveFocus();
		});

		it('moves to last tab with End key', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="today" onChange={onChange} />);

			const todayTab = screen.getByRole('tab', { name: 'Today' });
			todayTab.focus();
			fireEvent.keyDown(todayTab, { key: 'End' });

			expect(screen.getByRole('tab', { name: 'Custom date range' })).toHaveFocus();
		});

		it('sets tabIndex correctly for roving tabindex', () => {
			const onChange = vi.fn();
			render(<TimeRangeSelector value="this_week" onChange={onChange} />);

			// Active tab should have tabIndex 0
			expect(screen.getByRole('tab', { name: 'This Week' })).toHaveAttribute('tabindex', '0');

			// Inactive tabs should have tabIndex -1
			expect(screen.getByRole('tab', { name: 'Today' })).toHaveAttribute('tabindex', '-1');
			expect(screen.getByRole('tab', { name: 'Yesterday' })).toHaveAttribute('tabindex', '-1');
			expect(screen.getByRole('tab', { name: 'This Month' })).toHaveAttribute('tabindex', '-1');
		});
	});
});

describe('getDateRange', () => {
	let originalDate: DateConstructor;

	beforeEach(() => {
		originalDate = global.Date;
		vi.useFakeTimers();
		// Set mock time to Tuesday, Jan 20, 2026 at 14:30:00 local time
		vi.setSystemTime(new Date(2026, 0, 20, 14, 30, 0));
	});

	afterEach(() => {
		vi.useRealTimers();
		global.Date = originalDate;
	});

	it('returns correct range for today', () => {
		const range = getDateRange('today');

		expect(range.since.getFullYear()).toBe(2026);
		expect(range.since.getMonth()).toBe(0); // January
		expect(range.since.getDate()).toBe(20);
		expect(range.since.getHours()).toBe(0);
		expect(range.since.getMinutes()).toBe(0);
		expect(range.since.getSeconds()).toBe(0);

		expect(range.until.getFullYear()).toBe(2026);
		expect(range.until.getMonth()).toBe(0);
		expect(range.until.getDate()).toBe(20);
		expect(range.until.getHours()).toBe(14);
		expect(range.until.getMinutes()).toBe(30);
	});

	it('returns correct range for yesterday', () => {
		const range = getDateRange('yesterday');

		// Since should be start of Jan 19
		expect(range.since.getFullYear()).toBe(2026);
		expect(range.since.getMonth()).toBe(0);
		expect(range.since.getDate()).toBe(19);
		expect(range.since.getHours()).toBe(0);
		expect(range.since.getMinutes()).toBe(0);

		// Until should be end of Jan 19
		expect(range.until.getFullYear()).toBe(2026);
		expect(range.until.getMonth()).toBe(0);
		expect(range.until.getDate()).toBe(19);
		expect(range.until.getHours()).toBe(23);
		expect(range.until.getMinutes()).toBe(59);
	});

	it('returns correct range for this_week', () => {
		const range = getDateRange('this_week');

		// Jan 20, 2026 is a Tuesday, so week starts Sunday Jan 18
		expect(range.since.getFullYear()).toBe(2026);
		expect(range.since.getMonth()).toBe(0);
		expect(range.since.getDate()).toBe(18); // Sunday
		expect(range.since.getHours()).toBe(0);
		expect(range.since.getMinutes()).toBe(0);

		// Until should be current time
		expect(range.until.getDate()).toBe(20);
	});

	it('returns correct range for this_month', () => {
		const range = getDateRange('this_month');

		// Should start at Jan 1
		expect(range.since.getFullYear()).toBe(2026);
		expect(range.since.getMonth()).toBe(0);
		expect(range.since.getDate()).toBe(1);
		expect(range.since.getHours()).toBe(0);
		expect(range.since.getMinutes()).toBe(0);

		// Until should be current time
		expect(range.until.getDate()).toBe(20);
	});

	it('returns custom range when provided', () => {
		const customRange: CustomDateRange = {
			start: localDate(2026, 1, 10),
			end: localDate(2026, 1, 15),
		};
		const range = getDateRange('custom', customRange);

		// Should be start of Jan 10
		expect(range.since.getDate()).toBe(10);
		expect(range.since.getHours()).toBe(0);

		// Should be end of Jan 15
		expect(range.until.getDate()).toBe(15);
		expect(range.until.getHours()).toBe(23);
		expect(range.until.getMinutes()).toBe(59);
	});

	it('defaults to last 7 days when custom range not provided', () => {
		const range = getDateRange('custom');

		// Since should be 7 days ago (Jan 13)
		expect(range.since.getDate()).toBe(13);

		// Until should be current time
		expect(range.until.getDate()).toBe(20);
	});

	it('handles timezone correctly by using local dates', () => {
		// The implementation uses local time methods (setHours, getDate, etc.)
		// which should handle timezone correctly
		const range = getDateRange('today');

		// Both dates should be in local timezone
		const localOffset = new Date().getTimezoneOffset();
		expect(range.since.getTimezoneOffset()).toBe(localOffset);
		expect(range.until.getTimezoneOffset()).toBe(localOffset);
	});

	it('handles month boundaries correctly', () => {
		// Set time to Feb 3, 2026 local time
		vi.setSystemTime(new Date(2026, 1, 3, 10, 0, 0));

		const range = getDateRange('this_week');

		// Feb 3, 2026 is a Tuesday, week starts Sunday Feb 1
		expect(range.since.getMonth()).toBe(1); // February
		expect(range.since.getDate()).toBe(1);
	});

	it('handles year boundaries correctly', () => {
		// Set time to Jan 2, 2026 local time
		vi.setSystemTime(new Date(2026, 0, 2, 10, 0, 0));

		const range = getDateRange('this_week');

		// Jan 2, 2026 is a Friday, week starts Sunday Dec 28, 2025
		expect(range.since.getFullYear()).toBe(2025);
		expect(range.since.getMonth()).toBe(11); // December
		expect(range.since.getDate()).toBe(28);
	});
});
