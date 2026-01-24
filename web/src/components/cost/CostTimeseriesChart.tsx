/**
 * CostTimeseriesChart - Line chart showing cost over time.
 * Supports single aggregated view or per-model breakdown (opus/sonnet/haiku).
 */

import { useMemo, type HTMLAttributes } from 'react';
import {
	LineChart,
	Line,
	XAxis,
	YAxis,
	CartesianGrid,
	Tooltip,
	Legend,
	ResponsiveContainer,
} from 'recharts';
import './CostTimeseriesChart.css';

export interface CostDataPoint {
	/** Date string (ISO format or formatted) */
	date: string;
	/** Cost in dollars */
	cost: number;
	/** Token count */
	tokens: number;
	/** Model name (opus/sonnet/haiku) - only when showModels=true */
	model?: 'opus' | 'sonnet' | 'haiku';
}

export interface CostTimeseriesChartProps extends HTMLAttributes<HTMLDivElement> {
	/** Array of cost data points */
	data: CostDataPoint[];
	/** Time granularity for axis formatting */
	granularity: 'hour' | 'day' | 'week';
	/** Time period for the chart */
	period: {
		start: Date;
		end: Date;
	};
	/** Show separate lines for each model (opus/sonnet/haiku) */
	showModels?: boolean;
}

/** Model color configuration using design tokens */
const MODEL_COLORS = {
	opus: 'var(--primary)',
	sonnet: 'var(--cyan)',
	haiku: 'var(--green)',
} as const;

/** Aggregated data point for chart rendering */
interface ChartDataPoint {
	date: string;
	cost?: number;
	tokens?: number;
	opus?: number;
	sonnet?: number;
	haiku?: number;
	opusTokens?: number;
	sonnetTokens?: number;
	haikuTokens?: number;
}

/** Recharts payload entry type */
interface TooltipPayloadEntry {
	name: string;
	value: number;
	color: string;
	dataKey: string;
	payload: ChartDataPoint;
}

/**
 * Custom tooltip component for the chart
 */
function CustomTooltip({
	active,
	payload,
	label,
	showModels,
}: {
	active?: boolean;
	payload?: TooltipPayloadEntry[];
	label?: string;
	showModels?: boolean;
}) {
	if (!active || !payload || payload.length === 0) {
		return null;
	}

	// Find tokens from the payload data
	const dataPoint = payload[0]?.payload;

	return (
		<div className="cost-tooltip">
			<div className="cost-tooltip-date">{label}</div>
			{payload.map((entry, index) => {
				// Skip token entries
				if (entry.dataKey.includes('Tokens')) return null;

				let tokenCount: number | undefined;
				if (showModels) {
					const tokenKey = `${entry.dataKey}Tokens` as keyof ChartDataPoint;
					tokenCount = dataPoint?.[tokenKey] as number | undefined;
				} else {
					tokenCount = dataPoint?.tokens;
				}

				return (
					<div key={index} className="cost-tooltip-row">
						<span className="cost-tooltip-label" style={{ color: entry.color }}>
							{showModels ? entry.name : 'Cost'}
						</span>
						<span className="cost-tooltip-value">${entry.value.toFixed(2)}</span>
						{tokenCount !== undefined && (
							<span className="cost-tooltip-tokens">
								{tokenCount.toLocaleString()} tokens
							</span>
						)}
					</div>
				);
			})}
		</div>
	);
}

/**
 * Format date based on granularity
 */
function formatDate(dateStr: string, granularity: 'hour' | 'day' | 'week'): string {
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return dateStr;

	switch (granularity) {
		case 'hour':
			return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
		case 'week':
			return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
		case 'day':
		default:
			return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
	}
}

/**
 * Format Y-axis tick value as currency
 */
function formatCost(value: number): string {
	return `$${value.toFixed(0)}`;
}

/**
 * CostTimeseriesChart component
 */
export function CostTimeseriesChart({
	data,
	granularity,
	period: _period, // Destructure to avoid spreading to DOM
	showModels = false,
	className = '',
	...props
}: CostTimeseriesChartProps) {
	/**
	 * Transform data for chart rendering.
	 * When showModels=true, pivot data by date with model columns.
	 * When showModels=false, aggregate all models into single cost/tokens.
	 */
	const chartData = useMemo<ChartDataPoint[]>(() => {
		if (data.length === 0) return [];

		if (showModels) {
			// Pivot data by date, with separate columns for each model
			const byDate = new Map<string, ChartDataPoint>();

			for (const point of data) {
				const existing = byDate.get(point.date) || { date: point.date };
				if (point.model) {
					// Assign model cost and tokens using type-safe approach
					switch (point.model) {
						case 'opus':
							existing.opus = point.cost;
							existing.opusTokens = point.tokens;
							break;
						case 'sonnet':
							existing.sonnet = point.cost;
							existing.sonnetTokens = point.tokens;
							break;
						case 'haiku':
							existing.haiku = point.cost;
							existing.haikuTokens = point.tokens;
							break;
					}
				}
				byDate.set(point.date, existing);
			}

			return Array.from(byDate.values());
		} else {
			// Aggregate by date
			const byDate = new Map<string, ChartDataPoint>();

			for (const point of data) {
				const existing = byDate.get(point.date) || { date: point.date, cost: 0, tokens: 0 };
				existing.cost = (existing.cost || 0) + point.cost;
				existing.tokens = (existing.tokens || 0) + point.tokens;
				byDate.set(point.date, existing);
			}

			return Array.from(byDate.values());
		}
	}, [data, showModels]);

	const classes = ['cost-timeseries-chart', className].filter(Boolean).join(' ');

	return (
		<div className={classes} {...props}>
			<ResponsiveContainer width="100%" height={300}>
				<LineChart data={chartData} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
					<CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
					<XAxis
						dataKey="date"
						tickFormatter={(value) => formatDate(value, granularity)}
						stroke="var(--text-muted)"
						tick={{ fontSize: 10 }}
						axisLine={{ stroke: 'var(--border)' }}
						tickLine={{ stroke: 'var(--border)' }}
					/>
					<YAxis
						tickFormatter={formatCost}
						stroke="var(--text-muted)"
						tick={{ fontSize: 10 }}
						axisLine={{ stroke: 'var(--border)' }}
						tickLine={{ stroke: 'var(--border)' }}
						width={50}
					/>
					<Tooltip content={<CustomTooltip showModels={showModels} />} />

					{/* Conditionally render legend for multi-model view */}
					{showModels && (
						<Legend wrapperStyle={{ fontSize: '11px' }} iconType="circle" iconSize={8} />
					)}

					{/* Render model-specific lines when showModels=true */}
					{showModels && (
						<Line
							type="monotone"
							dataKey="opus"
							name="opus"
							stroke={MODEL_COLORS.opus}
							strokeWidth={2}
							dot={{ fill: MODEL_COLORS.opus, r: 3 }}
							activeDot={{ r: 5 }}
						/>
					)}
					{showModels && (
						<Line
							type="monotone"
							dataKey="sonnet"
							name="sonnet"
							stroke={MODEL_COLORS.sonnet}
							strokeWidth={2}
							dot={{ fill: MODEL_COLORS.sonnet, r: 3 }}
							activeDot={{ r: 5 }}
						/>
					)}
					{showModels && (
						<Line
							type="monotone"
							dataKey="haiku"
							name="haiku"
							stroke={MODEL_COLORS.haiku}
							strokeWidth={2}
							dot={{ fill: MODEL_COLORS.haiku, r: 3 }}
							activeDot={{ r: 5 }}
						/>
					)}

					{/* Render single aggregated line when showModels=false */}
					{!showModels && (
						<Line
							type="monotone"
							dataKey="cost"
							name="Cost"
							stroke="var(--primary)"
							strokeWidth={2}
							dot={{ fill: 'var(--primary)', r: 3 }}
							activeDot={{ r: 5 }}
						/>
					)}
				</LineChart>
			</ResponsiveContainer>
		</div>
	);
}
