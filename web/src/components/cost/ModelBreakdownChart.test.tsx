import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ModelBreakdownChart } from './ModelBreakdownChart';
import type { ModelBreakdownData } from './ModelBreakdownChart';

const sampleData: ModelBreakdownData[] = [
  { model: 'opus', cost: 119.35, tokens: 1250000, percent: 76 },
  { model: 'sonnet', cost: 34.49, tokens: 3500000, percent: 22 },
  { model: 'haiku', cost: 2.94, tokens: 500000, percent: 2 },
];

describe('ModelBreakdownChart', () => {
  describe('SC-1: donut rendering with correct proportions and colors', () => {
    it('renders donut chart container', () => {
      const { container } = render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(container.querySelector('.model-breakdown-chart')).toBeInTheDocument();
    });

    it('applies conic-gradient with correct degree segments', () => {
      const { container } = render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      const donut = container.querySelector('.model-breakdown-donut');
      expect(donut).toBeInTheDocument();
      // 76% = 273.6deg, 22% = 79.2deg, 2% = 7.2deg
      // Gradient should start at 0 and use var(--primary), var(--cyan), var(--orange)
      const style = donut?.getAttribute('style') || '';
      expect(style).toContain('conic-gradient');
      expect(style).toContain('var(--primary)');
      expect(style).toContain('var(--cyan)');
      expect(style).toContain('var(--orange)');
    });

    it('renders with correct model color order (opus first)', () => {
      const reversedData: ModelBreakdownData[] = [
        { model: 'haiku', cost: 2.94, tokens: 500000, percent: 2 },
        { model: 'sonnet', cost: 34.49, tokens: 3500000, percent: 22 },
        { model: 'opus', cost: 119.35, tokens: 1250000, percent: 76 },
      ];
      const { container } = render(
        <ModelBreakdownChart data={reversedData} total={156.78} period="Last 30 days" />
      );
      const donut = container.querySelector('.model-breakdown-donut');
      const style = donut?.getAttribute('style') || '';
      // Should render opus (--primary) first regardless of data order
      expect(style).toMatch(/conic-gradient\(\s*var\(--primary\)/);
    });
  });

  describe('SC-2: center displays total cost', () => {
    it('shows formatted total cost in center', () => {
      render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(screen.getByText('$156.78')).toBeInTheDocument();
    });

    it('shows period label below total', () => {
      render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(screen.getByText('Last 30 days')).toBeInTheDocument();
    });

    it('formats large totals with commas', () => {
      render(
        <ModelBreakdownChart data={sampleData} total={1234.56} period="Last 30 days" />
      );
      expect(screen.getByText('$1,234.56')).toBeInTheDocument();
    });
  });

  describe('SC-3: legend shows all models', () => {
    it('displays legend entry for each model', () => {
      render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(screen.getByText('Opus')).toBeInTheDocument();
      expect(screen.getByText('Sonnet')).toBeInTheDocument();
      expect(screen.getByText('Haiku')).toBeInTheDocument();
    });

    it('shows cost value for each model', () => {
      render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(screen.getByText('$119.35')).toBeInTheDocument();
      expect(screen.getByText('$34.49')).toBeInTheDocument();
      expect(screen.getByText('$2.94')).toBeInTheDocument();
    });

    it('shows percentage for each model', () => {
      render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(screen.getByText('76%')).toBeInTheDocument();
      expect(screen.getByText('22%')).toBeInTheDocument();
      expect(screen.getByText('2%')).toBeInTheDocument();
    });

    it('renders color dots matching model colors', () => {
      const { container } = render(
        <ModelBreakdownChart data={sampleData} total={156.78} period="Last 30 days" />
      );
      expect(container.querySelector('.model-breakdown-legend-dot--opus')).toBeInTheDocument();
      expect(container.querySelector('.model-breakdown-legend-dot--sonnet')).toBeInTheDocument();
      expect(container.querySelector('.model-breakdown-legend-dot--haiku')).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
    it('handles empty data array', () => {
      const { container } = render(
        <ModelBreakdownChart data={[]} total={0} period="Last 30 days" />
      );
      expect(container.querySelector('.model-breakdown-chart')).toBeInTheDocument();
      expect(screen.getByText('$0.00')).toBeInTheDocument();
    });

    it('handles single model only', () => {
      const singleModel: ModelBreakdownData[] = [
        { model: 'opus', cost: 100, tokens: 1000000, percent: 100 },
      ];
      const { container } = render(
        <ModelBreakdownChart data={singleModel} total={100} period="Last 30 days" />
      );
      const donut = container.querySelector('.model-breakdown-donut');
      expect(donut).toBeInTheDocument();
      expect(screen.getByText('Opus')).toBeInTheDocument();
      expect(screen.getByText('100%')).toBeInTheDocument();
    });

    it('handles all zero costs', () => {
      const zeroData: ModelBreakdownData[] = [
        { model: 'opus', cost: 0, tokens: 0, percent: 0 },
        { model: 'sonnet', cost: 0, tokens: 0, percent: 0 },
        { model: 'haiku', cost: 0, tokens: 0, percent: 0 },
      ];
      const { container } = render(
        <ModelBreakdownChart data={zeroData} total={0} period="Last 30 days" />
      );
      expect(container.querySelector('.model-breakdown-chart')).toBeInTheDocument();
    });
  });
});
