/**
 * ModelBreakdownChart - CSS-only donut chart for model cost breakdown.
 * Uses conic-gradient for rendering with consistent model ordering.
 */

import { useMemo } from 'react';
import './ModelBreakdownChart.css';

export interface ModelBreakdownData {
  model: 'opus' | 'sonnet' | 'haiku';
  cost: number;
  tokens: number;
  percent: number;
}

export interface ModelBreakdownChartProps {
  data: ModelBreakdownData[];
  total: number;
  period: string;
}

/** Model display order and colors */
const MODEL_ORDER: Array<'opus' | 'sonnet' | 'haiku'> = ['opus', 'sonnet', 'haiku'];
const MODEL_COLORS: Record<'opus' | 'sonnet' | 'haiku', string> = {
  opus: 'var(--primary)',
  sonnet: 'var(--cyan)',
  haiku: 'var(--orange)',
};
const MODEL_LABELS: Record<'opus' | 'sonnet' | 'haiku', string> = {
  opus: 'Opus',
  sonnet: 'Sonnet',
  haiku: 'Haiku',
};

/**
 * Format a number as currency with commas.
 */
function formatCurrency(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

/**
 * Donut chart visualizing cost breakdown by model with legend.
 * Renders using CSS conic-gradient for smooth segments.
 */
export function ModelBreakdownChart({ data, total, period }: ModelBreakdownChartProps) {
  // Create a map for quick lookup by model
  const dataByModel = useMemo(() => {
    const map = new Map<string, ModelBreakdownData>();
    for (const item of data) {
      map.set(item.model, item);
    }
    return map;
  }, [data]);

  // Sort data by model order for consistent rendering
  const sortedData = useMemo(() => {
    return MODEL_ORDER.map((model) => dataByModel.get(model)).filter(
      (item): item is ModelBreakdownData => item !== undefined
    );
  }, [dataByModel]);

  // Memoize gradient calculation
  const gradient = useMemo(() => {
    if (sortedData.length === 0 || total === 0) {
      return 'var(--bg-surface)';
    }

    // Handle single model case (full circle)
    if (sortedData.length === 1 && sortedData[0].percent === 100) {
      return MODEL_COLORS[sortedData[0].model];
    }

    // Build conic-gradient with segments in model order
    const segments: string[] = [];
    let currentDeg = 0;

    for (const item of sortedData) {
      if (item.percent > 0) {
        const segmentDeg = (item.percent / 100) * 360;
        const color = MODEL_COLORS[item.model];
        segments.push(`${color} ${currentDeg}deg ${currentDeg + segmentDeg}deg`);
        currentDeg += segmentDeg;
      }
    }

    if (segments.length === 0) {
      return 'var(--bg-surface)';
    }

    return `conic-gradient(${segments.join(', ')})`;
  }, [sortedData, total]);

  return (
    <div className="model-breakdown-chart">
      <div className="model-breakdown-donut" style={{ background: gradient }}>
        <div className="model-breakdown-center">
          <span className="model-breakdown-total">{formatCurrency(total)}</span>
          <span className="model-breakdown-period">{period}</span>
        </div>
      </div>
      <div className="model-breakdown-legend">
        {sortedData.map((item) => (
          <div key={item.model} className="model-breakdown-legend-item">
            <span
              className={`model-breakdown-legend-dot model-breakdown-legend-dot--${item.model}`}
            />
            <span className="model-breakdown-legend-label">{MODEL_LABELS[item.model]}</span>
            <span className="model-breakdown-legend-cost">{formatCurrency(item.cost)}</span>
            <span className="model-breakdown-legend-percent">{item.percent}%</span>
          </div>
        ))}
      </div>
    </div>
  );
}
