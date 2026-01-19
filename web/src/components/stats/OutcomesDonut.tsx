/**
 * OutcomesDonut - CSS-only donut chart for task outcomes visualization.
 * Uses conic-gradient for rendering with animated segment transitions.
 */

import { useMemo } from 'react';
import './OutcomesDonut.css';

export interface OutcomesDonutProps {
	/** Number of successfully completed tasks */
	completed: number;
	/** Number of tasks that completed with retries */
	withRetries: number;
	/** Number of failed tasks */
	failed: number;
}

/**
 * Donut chart visualizing task outcomes with a centered total count.
 * Renders using CSS conic-gradient for smooth segment animations.
 *
 * @example
 * <OutcomesDonut completed={232} withRetries={11} failed={4} />
 */
export function OutcomesDonut({ completed, withRetries, failed }: OutcomesDonutProps) {
	const total = completed + withRetries + failed;

	// Memoize gradient calculation to avoid recalculating on every render
	const gradient = useMemo(() => {
		if (total === 0) {
			// Empty state - neutral background
			return 'var(--bg-surface)';
		}

		const completedDeg = (completed / total) * 360;
		const retriesDeg = (withRetries / total) * 360;
		const failedDeg = (failed / total) * 360;

		// Handle single-category case (full circle)
		if (completed === total) {
			return 'var(--green)';
		}
		if (withRetries === total) {
			return 'var(--amber)';
		}
		if (failed === total) {
			return 'var(--red)';
		}

		// Build conic-gradient with all segments
		const segments: string[] = [];
		let currentDeg = 0;

		if (completed > 0) {
			segments.push(`var(--green) ${currentDeg}deg ${currentDeg + completedDeg}deg`);
			currentDeg += completedDeg;
		}

		if (withRetries > 0) {
			segments.push(`var(--amber) ${currentDeg}deg ${currentDeg + retriesDeg}deg`);
			currentDeg += retriesDeg;
		}

		if (failed > 0) {
			segments.push(`var(--red) ${currentDeg}deg ${currentDeg + failedDeg}deg`);
		}

		return `conic-gradient(${segments.join(', ')})`;
	}, [completed, withRetries, failed, total]);

	return (
		<div className="outcomes-donut-container">
			<div className="outcomes-donut" style={{ background: gradient }}>
				<div className="outcomes-donut-center">
					<span className="outcomes-donut-value">{total}</span>
					<span className="outcomes-donut-label">Total</span>
				</div>
			</div>
			<div className="outcomes-donut-legend">
				<div className="outcomes-donut-legend-item">
					<span className="outcomes-donut-legend-dot outcomes-donut-legend-dot--completed" />
					<span className="outcomes-donut-legend-text">Completed</span>
					<span className="outcomes-donut-legend-count">{completed}</span>
				</div>
				<div className="outcomes-donut-legend-item">
					<span className="outcomes-donut-legend-dot outcomes-donut-legend-dot--retries" />
					<span className="outcomes-donut-legend-text">With Retries</span>
					<span className="outcomes-donut-legend-count">{withRetries}</span>
				</div>
				<div className="outcomes-donut-legend-item">
					<span className="outcomes-donut-legend-dot outcomes-donut-legend-dot--failed" />
					<span className="outcomes-donut-legend-text">Failed</span>
					<span className="outcomes-donut-legend-count">{failed}</span>
				</div>
			</div>
		</div>
	);
}
