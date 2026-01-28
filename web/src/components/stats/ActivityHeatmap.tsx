/**
 * ActivityHeatmap component - GitHub-style activity heatmap.
 * Displays a 16-week grid of task activity with color intensity based on count.
 */

import {
	useMemo,
	useState,
	useCallback,
	useEffect,
	useRef,
	type KeyboardEvent,
	type MouseEvent,
} from 'react';
import './ActivityHeatmap.css';

/* ==========================================================================
   TYPES
   ========================================================================== */

export interface ActivityData {
	/** Date in "YYYY-MM-DD" format */
	date: string;
	/** Number of tasks completed on this date */
	count: number;
}

export interface ActivityHeatmapProps {
	/** Activity data for the heatmap */
	data: ActivityData[];
	/** Number of weeks to display (default: 16) */
	weeks?: number;
	/** Callback when a day cell is clicked */
	onDayClick?: (date: string, count: number) => void;
	/** Additional CSS class name */
	className?: string;
	/** Show loading skeleton */
	loading?: boolean;
	/** Title for the heatmap */
	title?: string;
}

interface HeatmapCell {
	date: string;
	count: number;
	level: 0 | 1 | 2 | 3 | 4;
	dayOfWeek: number;
	weekIndex: number;
	isFuture: boolean;
}

interface TooltipState {
	visible: boolean;
	x: number;
	y: number;
	date: string;
	count: number;
}

/* ==========================================================================
   CONSTANTS
   ========================================================================== */

const DAY_LABELS = ['', 'Mon', '', 'Wed', '', 'Fri', ''];
const MONTH_NAMES = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/**
 * Level thresholds for activity intensity.
 * level 0: 0 tasks (no activity)
 * level 1: 1-3 tasks
 * level 2: 4-6 tasks
 * level 3: 7-9 tasks
 * level 4: 10+ tasks
 */
function getLevel(count: number): 0 | 1 | 2 | 3 | 4 {
	if (count === 0) return 0;
	if (count <= 3) return 1;
	if (count <= 6) return 2;
	if (count <= 9) return 3;
	return 4;
}

/* ==========================================================================
   DATE UTILITIES
   ========================================================================== */

/** Format date as YYYY-MM-DD */
function formatDate(date: Date): string {
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

/** Format date for display (e.g., "January 16, 2026") */
function formatDateDisplay(dateStr: string): string {
	if (!dateStr || !dateStr.includes('-')) return '';
	const [year, month, day] = dateStr.split('-').map(Number);
	if (isNaN(year) || isNaN(month) || isNaN(day)) return '';
	const date = new Date(year, month - 1, day);
	if (isNaN(date.getTime())) return '';
	return date.toLocaleDateString('en-US', {
		month: 'long',
		day: 'numeric',
		year: 'numeric',
	});
}

/** Get the start of week (Sunday) for a given date */
function startOfWeek(date: Date): Date {
	const result = new Date(date);
	const day = result.getDay();
	result.setDate(result.getDate() - day);
	result.setHours(0, 0, 0, 0);
	return result;
}

/** Subtract weeks from a date */
function subWeeks(date: Date, weeks: number): Date {
	const result = new Date(date);
	result.setDate(result.getDate() - weeks * 7);
	return result;
}

/** Add days to a date */
function addDays(date: Date, days: number): Date {
	const result = new Date(date);
	result.setDate(result.getDate() + days);
	return result;
}

/** Check if date1 is after date2 */
function isAfter(date1: Date, date2: Date): boolean {
	return date1.getTime() > date2.getTime();
}

/* ==========================================================================
   GRID BUILDING
   ========================================================================== */

function buildHeatmapGrid(data: ActivityData[], weeks: number, today: Date): HeatmapCell[][] {
	const startDate = subWeeks(today, weeks - 1);
	const alignedStart = startOfWeek(startDate);

	// Create a map for O(1) lookup
	const dataMap = new Map(data.map((d) => [d.date, d.count]));
	const grid: HeatmapCell[][] = [];

	for (let w = 0; w < weeks; w++) {
		const week: HeatmapCell[] = [];
		for (let d = 0; d < 7; d++) {
			const date = addDays(alignedStart, w * 7 + d);
			const dateStr = formatDate(date);
			const count = dataMap.get(dateStr) || 0;
			const isFuture = isAfter(date, today);

			week.push({
				date: dateStr,
				count,
				level: isFuture ? 0 : getLevel(count),
				dayOfWeek: d,
				weekIndex: w,
				isFuture,
			});
		}
		grid.push(week);
	}

	return grid;
}

/** Get unique months that appear in the grid */
function getMonthLabels(grid: HeatmapCell[][]): { month: string; weekIndex: number }[] {
	const labels: { month: string; weekIndex: number }[] = [];
	let lastMonth = -1;

	for (let w = 0; w < grid.length; w++) {
		// Check the first day of the week
		const firstDayStr = grid[w][0].date;
		const [, month] = firstDayStr.split('-').map(Number);

		if (month !== lastMonth) {
			labels.push({
				month: MONTH_NAMES[month - 1],
				weekIndex: w,
			});
			lastMonth = month;
		}
	}

	return labels;
}

/* ==========================================================================
   RESPONSIVE WEEKS
   ========================================================================== */

function useResponsiveWeeks(defaultWeeks: number): number {
	const [weeks, setWeeks] = useState(defaultWeeks);

	useEffect(() => {
		function updateWeeks() {
			const width = window.innerWidth;
			if (width < 480) {
				setWeeks(Math.min(4, defaultWeeks));
			} else if (width < 768) {
				setWeeks(Math.min(8, defaultWeeks));
			} else {
				setWeeks(defaultWeeks);
			}
		}

		updateWeeks();
		window.addEventListener('resize', updateWeeks);
		return () => window.removeEventListener('resize', updateWeeks);
	}, [defaultWeeks]);

	return weeks;
}

/* ==========================================================================
   SKELETON COMPONENT
   ========================================================================== */

function HeatmapSkeleton({ weeks }: { weeks: number }) {
	const cells = useMemo(() => {
		const result: number[] = [];
		for (let i = 0; i < weeks * 7; i++) {
			result.push(i);
		}
		return result;
	}, [weeks]);

	return (
		<div className="heatmap-skeleton">
			<div className="heatmap-skeleton-header">
				<div className="heatmap-skeleton-title" />
				<div className="heatmap-skeleton-legend" />
			</div>
			<div className="heatmap-skeleton-grid">
				{cells.map((i) => (
					<div key={i} className="heatmap-skeleton-cell" />
				))}
			</div>
		</div>
	);
}

/* ==========================================================================
   MAIN COMPONENT
   ========================================================================== */

export function ActivityHeatmap({
	data,
	weeks: defaultWeeks = 16,
	onDayClick,
	className = '',
	loading = false,
	title = 'Task Activity',
}: ActivityHeatmapProps) {
	const weeks = useResponsiveWeeks(defaultWeeks);
	const today = useMemo(() => new Date(), []);
	const gridRef = useRef<HTMLDivElement>(null);
	const tooltipRef = useRef<HTMLDivElement>(null);

	// Tooltip state
	const [tooltip, setTooltip] = useState<TooltipState>({
		visible: false,
		x: 0,
		y: 0,
		date: '',
		count: 0,
	});

	// Build the grid
	const grid = useMemo(() => buildHeatmapGrid(data, weeks, today), [data, weeks, today]);
	const monthLabels = useMemo(() => getMonthLabels(grid), [grid]);

	// Flatten grid for rendering (transpose to column-major for CSS grid)
	const flatCells = useMemo(() => {
		const cells: HeatmapCell[] = [];
		for (let d = 0; d < 7; d++) {
			for (let w = 0; w < grid.length; w++) {
				cells.push(grid[w][d]);
			}
		}
		return cells;
	}, [grid]);

	// Keyboard navigation state
	const [focusIndex, setFocusIndex] = useState<number | null>(null);

	// Event handlers
	const handleCellMouseEnter = useCallback(
		(cell: HeatmapCell, event: MouseEvent<HTMLDivElement>) => {
			if (cell.isFuture) return;

			const rect = event.currentTarget.getBoundingClientRect();
			setTooltip({
				visible: true,
				x: rect.left + rect.width / 2,
				y: rect.top - 8,
				date: cell.date,
				count: cell.count,
			});
		},
		[]
	);

	const handleCellMouseMove = useCallback((event: MouseEvent<HTMLDivElement>) => {
		if (!tooltip.visible) return;

		setTooltip((prev) => ({
			...prev,
			x: event.clientX,
			y: event.clientY - 30,
		}));
	}, [tooltip.visible]);

	const handleCellMouseLeave = useCallback(() => {
		setTooltip((prev) => ({ ...prev, visible: false }));
	}, []);

	const handleCellClick = useCallback(
		(cell: HeatmapCell) => {
			if (cell.isFuture) return;
			onDayClick?.(cell.date, cell.count);
		},
		[onDayClick]
	);

	const handleKeyDown = useCallback(
		(event: KeyboardEvent<HTMLDivElement>) => {
			if (focusIndex === null) return;

			let newIndex = focusIndex;

			switch (event.key) {
				case 'ArrowLeft':
					// Move to previous week (same day)
					newIndex = Math.max(0, focusIndex - 7);
					event.preventDefault();
					break;
				case 'ArrowRight':
					// Move to next week (same day)
					newIndex = Math.min(flatCells.length - 1, focusIndex + 7);
					event.preventDefault();
					break;
				case 'ArrowUp':
					// Move to previous day (same week)
					if (focusIndex % 7 > 0) {
						newIndex = focusIndex - 1;
					}
					event.preventDefault();
					break;
				case 'ArrowDown':
					// Move to next day (same week)
					if (focusIndex % 7 < 6) {
						newIndex = focusIndex + 1;
					}
					event.preventDefault();
					break;
				case 'Enter':
				case ' ': {
					// Activate the cell
					const cell = flatCells[focusIndex];
					if (cell && !cell.isFuture) {
						onDayClick?.(cell.date, cell.count);
					}
					event.preventDefault();
					break;
				}
				default:
					return;
			}

			// Update focus and navigate
			if (newIndex !== focusIndex) {
				setFocusIndex(newIndex);
				const cellElements = gridRef.current?.querySelectorAll('.heatmap-cell');
				const targetCell = cellElements?.[newIndex] as HTMLElement;
				targetCell?.focus();
			}
		},
		[focusIndex, flatCells, onDayClick]
	);

	const handleCellFocus = useCallback((index: number) => {
		setFocusIndex(index);
	}, []);

	// Calculate month label positions using percentage-based positioning
	const monthLabelStyle = useCallback(
		(weekIndex: number) => {
			const percent = (weekIndex / weeks) * 100;
			return {
				position: 'absolute' as const,
				left: `calc(28px + ${percent}%)`,
			};
		},
		[weeks]
	);

	// Loading state
	if (loading) {
		return (
			<div className={`activity-heatmap ${className}`.trim()}>
				<HeatmapSkeleton weeks={weeks} />
			</div>
		);
	}

	// Empty state
	if (data.length === 0 && !loading) {
		return (
			<div className={`activity-heatmap ${className}`.trim()}>
				<div className="heatmap-header">
					<span className="heatmap-title">{title}</span>
				</div>
				<div className="heatmap-empty">
					<span className="heatmap-empty-icon">📊</span>
					<span>No activity data available</span>
				</div>
			</div>
		);
	}

	return (
		<div className={`activity-heatmap ${className}`.trim()}>
			{/* Header with title and legend */}
			<div className="heatmap-header">
				<span className="heatmap-title">{title}</span>
				<div className="heatmap-legend">
					<span>Less</span>
					<div className="heatmap-legend-scale">
						<div className="heatmap-legend-cell" style={{ background: 'var(--bg-surface)' }} />
						<div className="heatmap-legend-cell" style={{ background: 'rgba(16, 185, 129, 0.3)' }} />
						<div className="heatmap-legend-cell" style={{ background: 'rgba(16, 185, 129, 0.5)' }} />
						<div className="heatmap-legend-cell" style={{ background: 'rgba(16, 185, 129, 0.7)' }} />
						<div className="heatmap-legend-cell" style={{ background: 'var(--green)' }} />
					</div>
					<span>More</span>
				</div>
			</div>

			{/* Month labels */}
			<div className="heatmap-months" style={{ position: 'relative', height: '16px' }}>
				{monthLabels.map(({ month, weekIndex }, i) => (
					<span key={`${month}-${i}`} className="month-label" style={monthLabelStyle(weekIndex)}>
						{month}
					</span>
				))}
			</div>

			{/* Grid container with day labels */}
			<div className="heatmap-container">
				<div className="heatmap-grid-wrapper">
					{/* Day labels */}
					<div className="heatmap-days">
						{DAY_LABELS.map((label, i) => (
							<div key={i} className="day-label">
								{label}
							</div>
						))}
					</div>

					{/* Grid */}
					<div
						ref={gridRef}
						className="heatmap-grid heatmap-grid--dense"
						data-weeks={weeks}
						style={{ '--heatmap-weeks': weeks } as React.CSSProperties}
						role="img"
						aria-label={`Task activity heatmap showing ${weeks} weeks of data`}
						onKeyDown={handleKeyDown}
					>
						{flatCells.map((cell, index) => (
							<div
								key={cell.date}
								className={[
									'heatmap-cell',
									`level-${cell.level}`,
									cell.isFuture && 'future',
								]
									.filter(Boolean)
									.join(' ')}
								aria-label={
									cell.isFuture
										? `Future date: ${formatDateDisplay(cell.date)}`
										: `${cell.count} task${cell.count !== 1 ? 's' : ''} on ${formatDateDisplay(cell.date)}`
								}
								tabIndex={cell.isFuture ? -1 : 0}
								onClick={() => handleCellClick(cell)}
								onMouseEnter={(e) => handleCellMouseEnter(cell, e)}
								onMouseMove={handleCellMouseMove}
								onMouseLeave={handleCellMouseLeave}
								onFocus={() => handleCellFocus(index)}
							/>
						))}
					</div>
				</div>
			</div>

			{/* Tooltip */}
			<div
				ref={tooltipRef}
				className={`heatmap-tooltip ${tooltip.visible ? 'visible' : ''}`}
				style={{
					left: `${tooltip.x}px`,
					top: `${tooltip.y}px`,
					transform: 'translateX(-50%)',
				}}
			>
				<span className="heatmap-tooltip-count">{tooltip.count} task{tooltip.count !== 1 ? 's' : ''}</span>
				{' on '}
				{formatDateDisplay(tooltip.date)}
			</div>
		</div>
	);
}

export default ActivityHeatmap;
