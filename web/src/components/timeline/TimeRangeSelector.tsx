/**
 * TimeRangeSelector component for collapsible date grouping in the timeline.
 * Provides preset time ranges (today, yesterday, this week, this month) and custom date selection.
 */

import { useCallback, useRef } from 'react';
import { Icon } from '@/components/ui/Icon';
import { subDays, type TimeRange, type CustomDateRange } from './time-range-utils';
import './TimeRangeSelector.css';

export type { TimeRange, CustomDateRange } from './time-range-utils';

export interface TimeRangeSelectorProps {
	value: TimeRange;
	onChange: (range: TimeRange) => void;
	customRange?: CustomDateRange;
	onCustomRangeChange?: (range: CustomDateRange) => void;
	className?: string;
}

// Tab configuration
interface TabConfig {
	id: TimeRange;
	label: string;
}

const RANGE_TABS: TabConfig[] = [
	{ id: 'today', label: 'Today' },
	{ id: 'yesterday', label: 'Yesterday' },
	{ id: 'this_week', label: 'This Week' },
	{ id: 'this_month', label: 'This Month' },
];

/** Format date for display (e.g., "Jan 10, 2026") */
function formatDateDisplay(date: Date): string {
	return date.toLocaleDateString(undefined, {
		month: 'short',
		day: 'numeric',
		year: 'numeric',
	});
}

/** Format date for input[type="date"] (YYYY-MM-DD) */
function formatDateInput(date: Date): string {
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

/** Parse date from input[type="date"] value */
function parseDateInput(value: string): Date {
	const [year, month, day] = value.split('-').map(Number);
	return new Date(year, month - 1, day);
}

/**
 * TimeRangeSelector - Tab-based time range selection with optional custom date picker.
 *
 * @example
 * ```tsx
 * const [range, setRange] = useState<TimeRange>('today');
 * const [customRange, setCustomRange] = useState<CustomDateRange>({
 *   start: new Date(),
 *   end: new Date()
 * });
 *
 * <TimeRangeSelector
 *   value={range}
 *   onChange={setRange}
 *   customRange={customRange}
 *   onCustomRangeChange={setCustomRange}
 * />
 * ```
 */
export function TimeRangeSelector({
	value,
	onChange,
	customRange,
	onCustomRangeChange,
	className = '',
}: TimeRangeSelectorProps) {
	const tabsRef = useRef<HTMLDivElement>(null);

	// Handle keyboard navigation between tabs
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent, currentIndex: number) => {
			const totalTabs = RANGE_TABS.length + 1; // +1 for custom button

			let nextIndex: number | null = null;

			switch (e.key) {
				case 'ArrowLeft':
					e.preventDefault();
					nextIndex = currentIndex > 0 ? currentIndex - 1 : totalTabs - 1;
					break;
				case 'ArrowRight':
					e.preventDefault();
					nextIndex = currentIndex < totalTabs - 1 ? currentIndex + 1 : 0;
					break;
				case 'Home':
					e.preventDefault();
					nextIndex = 0;
					break;
				case 'End':
					e.preventDefault();
					nextIndex = totalTabs - 1;
					break;
			}

			if (nextIndex !== null && tabsRef.current) {
				const buttons = tabsRef.current.querySelectorAll<HTMLButtonElement>(
					'.time-range-tab, .time-range-custom-btn'
				);
				const targetButton = buttons[nextIndex];
				if (targetButton) {
					targetButton.focus();
				}
			}
		},
		[]
	);

	// Handle tab selection
	const handleTabClick = useCallback(
		(range: TimeRange) => {
			onChange(range);
		},
		[onChange]
	);

	// Handle custom date changes
	const handleStartDateChange = useCallback(
		(e: React.ChangeEvent<HTMLInputElement>) => {
			if (onCustomRangeChange && customRange) {
				const newStart = parseDateInput(e.target.value);
				// Ensure start doesn't exceed end
				const newEnd = newStart > customRange.end ? newStart : customRange.end;
				onCustomRangeChange({ start: newStart, end: newEnd });
			}
		},
		[customRange, onCustomRangeChange]
	);

	const handleEndDateChange = useCallback(
		(e: React.ChangeEvent<HTMLInputElement>) => {
			if (onCustomRangeChange && customRange) {
				const newEnd = parseDateInput(e.target.value);
				// Ensure end doesn't precede start
				const newStart = newEnd < customRange.start ? newEnd : customRange.start;
				onCustomRangeChange({ start: newStart, end: newEnd });
			}
		},
		[customRange, onCustomRangeChange]
	);

	const isCustom = value === 'custom';
	const containerClasses = ['time-range-selector', className].filter(Boolean).join(' ');

	// Default custom range for display
	const displayCustomRange = customRange || {
		start: subDays(new Date(), 7),
		end: new Date(),
	};

	return (
		<div className={containerClasses}>
			{/* Tab bar */}
			<div
				ref={tabsRef}
				className="time-range-tabs"
				role="tablist"
				aria-label="Time range filter"
			>
				{RANGE_TABS.map((tab, index) => {
					const isActive = value === tab.id;
					return (
						<button
							key={tab.id}
							type="button"
							role="tab"
							aria-selected={isActive}
							tabIndex={isActive ? 0 : -1}
							className={`time-range-tab ${isActive ? 'time-range-tab--active' : ''}`}
							onClick={() => handleTabClick(tab.id)}
							onKeyDown={(e) => handleKeyDown(e, index)}
						>
							{tab.label}
						</button>
					);
				})}

				{/* Custom button with settings icon */}
				<button
					type="button"
					role="tab"
					aria-selected={isCustom}
					tabIndex={isCustom ? 0 : -1}
					className={`time-range-tab time-range-custom-btn ${isCustom ? 'time-range-tab--active' : ''}`}
					onClick={() => handleTabClick('custom')}
					onKeyDown={(e) => handleKeyDown(e, RANGE_TABS.length)}
					aria-label="Custom date range"
				>
					<Icon name="settings" size={14} />
				</button>
			</div>

			{/* Custom date picker (shown when custom is selected) */}
			{isCustom && (
				<div className="time-range-custom" role="tabpanel" aria-label="Custom date range">
					<div className="time-range-date-field">
						<label htmlFor="time-range-start" className="time-range-date-label">
							From:
						</label>
						<div className="time-range-date-input-wrapper">
							<input
								id="time-range-start"
								type="date"
								className="time-range-date-input"
								value={formatDateInput(displayCustomRange.start)}
								onChange={handleStartDateChange}
								max={formatDateInput(new Date())}
								aria-describedby="time-range-start-display"
							/>
							<span id="time-range-start-display" className="time-range-date-display">
								{formatDateDisplay(displayCustomRange.start)}
							</span>
						</div>
					</div>

					<div className="time-range-date-field">
						<label htmlFor="time-range-end" className="time-range-date-label">
							To:
						</label>
						<div className="time-range-date-input-wrapper">
							<input
								id="time-range-end"
								type="date"
								className="time-range-date-input"
								value={formatDateInput(displayCustomRange.end)}
								onChange={handleEndDateChange}
								min={formatDateInput(displayCustomRange.start)}
								max={formatDateInput(new Date())}
								aria-describedby="time-range-end-display"
							/>
							<span id="time-range-end-display" className="time-range-date-display">
								{formatDateDisplay(displayCustomRange.end)}
							</span>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
