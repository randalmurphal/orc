import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { RealTimeMetrics } from '../RealTimeMetrics';

// Mock the formatting utilities
vi.mock('../../../lib/utils/formatting', () => ({
  formatCurrency: vi.fn((value: number) => `$${value.toFixed(2)}`),
  formatTokens: vi.fn((value: number) => value.toLocaleString()),
  formatDuration: vi.fn((seconds: number) => {
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}:${secs.toString().padStart(2, '0')}`;
  }),
  formatPercentage: vi.fn((value: number) => `${Math.round(value * 100)}%`),
  formatTimeRemaining: vi.fn((seconds: number) => {
    if (seconds <= 0) return 'Complete';
    if (seconds < 60) return `~${seconds}s remaining`;
    if (seconds < 3600) {
      const minutes = Math.ceil(seconds / 60);
      return `~${minutes} min remaining`;
    }
    const hours = Math.ceil(seconds / 3600);
    return `~${hours}h remaining`;
  }),
}));

describe('RealTimeMetrics Enhanced Session Display', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('SC-3.1: displays current session metrics with live updates', () => {
    const sessionMetrics = {
      totalTokens: 15420,
      estimatedCostUSD: 0.45,
      inputTokens: 8200,
      outputTokens: 7220,
      durationSeconds: 135,
      tasksRunning: 2,
      isPaused: false,
    };

    const phaseProgress = {
      iterations: 3,
      estimatedCompletion: 0.75,
      currentActivity: 'streaming',
    };

    render(
      <RealTimeMetrics
        taskId="TASK-001"
        sessionMetrics={sessionMetrics}
        phaseProgress={phaseProgress}
        showDetailed={true}
      />
    );

    // Should display session metrics
    expect(screen.getByTestId('session-tokens')).toHaveTextContent('15,420');
    expect(screen.getByTestId('session-cost')).toHaveTextContent('$0.45');
    expect(screen.getByTestId('session-duration')).toHaveTextContent('2:15');

    // Should show token breakdown
    expect(screen.getByTestId('input-tokens')).toHaveTextContent('8,200');
    expect(screen.getByTestId('output-tokens')).toHaveTextContent('7,220');

    // Should show task progress
    expect(screen.getByTestId('phase-iterations')).toHaveTextContent('3');
    expect(screen.getByTestId('completion-percentage')).toHaveTextContent('75%');

    // Should show running status
    expect(screen.getByTestId('tasks-running')).toHaveTextContent('2');
    expect(screen.getByTestId('session-status')).toHaveTextContent('Running');
  });

  it('SC-3.2: shows paused state when task is paused', () => {
    const sessionMetrics = {
      totalTokens: 8500,
      estimatedCostUSD: 0.22,
      durationSeconds: 90,
      tasksRunning: 1,
      isPaused: true,
    };

    render(
      <RealTimeMetrics
        taskId="TASK-002"
        sessionMetrics={sessionMetrics}
        phaseProgress={undefined}
        showDetailed={true}
      />
    );

    // Should show paused status
    expect(screen.getByTestId('session-status')).toHaveTextContent('Paused');
    expect(screen.getByTestId('session-status')).toHaveClass('text-yellow-600');

    // Metrics should still be displayed
    expect(screen.getByTestId('session-tokens')).toHaveTextContent('8,500');
    expect(screen.getByTestId('session-cost')).toHaveTextContent('$0.22');
  });

  it('SC-3.3: displays compact view when showDetailed is false', () => {
    const sessionMetrics = {
      totalTokens: 12000,
      estimatedCostUSD: 0.35,
      durationSeconds: 180,
      tasksRunning: 1,
      isPaused: false,
    };

    render(
      <RealTimeMetrics
        taskId="TASK-003"
        sessionMetrics={sessionMetrics}
        phaseProgress={undefined}
        showDetailed={false}
      />
    );

    // Should show only essential metrics in compact form
    expect(screen.getByTestId('compact-metrics')).toBeInTheDocument();
    expect(screen.getByTestId('session-tokens')).toHaveTextContent('12,000');
    expect(screen.getByTestId('session-cost')).toHaveTextContent('$0.35');

    // Should not show detailed breakdown
    expect(screen.queryByTestId('input-tokens')).toBeNull();
    expect(screen.queryByTestId('output-tokens')).toBeNull();
    expect(screen.queryByTestId('phase-iterations')).toBeNull();
  });

  it('SC-3.4: updates metrics in real-time when props change', async () => {
    let sessionMetrics = {
      totalTokens: 10000,
      estimatedCostUSD: 0.30,
      durationSeconds: 60,
      tasksRunning: 1,
    };

    const { rerender } = render(
      <RealTimeMetrics
        taskId="TASK-004"
        sessionMetrics={sessionMetrics}
        phaseProgress={undefined}
        showDetailed={true}
      />
    );

    // Initial values
    expect(screen.getByTestId('session-tokens')).toHaveTextContent('10,000');
    expect(screen.getByTestId('session-cost')).toHaveTextContent('$0.30');

    // Update metrics (simulating real-time update)
    sessionMetrics = {
      totalTokens: 12500,
      estimatedCostUSD: 0.38,
      durationSeconds: 75,
      tasksRunning: 1,
    };

    rerender(
      <RealTimeMetrics
        taskId="TASK-004"
        sessionMetrics={sessionMetrics}
        phaseProgress={undefined}
        showDetailed={true}
      />
    );

    // Should show updated values
    await waitFor(() => {
      expect(screen.getByTestId('session-tokens')).toHaveTextContent('12,500');
      expect(screen.getByTestId('session-cost')).toHaveTextContent('$0.38');
    });
  });

  it('SC-3.5: shows cost trend indicators for increasing costs', () => {
    const sessionMetrics = {
      totalTokens: 20000,
      estimatedCostUSD: 0.65,
      durationSeconds: 200,
      tasksRunning: 1,
      costTrend: 'increasing' as const, // Mock trend data
      tokenRate: 100, // tokens per minute
    };

    render(
      <RealTimeMetrics
        taskId="TASK-005"
        sessionMetrics={sessionMetrics}
        phaseProgress={undefined}
        showDetailed={true}
      />
    );

    // Should show cost trend indicator
    expect(screen.getByTestId('cost-trend')).toBeInTheDocument();
    expect(screen.getByTestId('cost-trend')).toHaveClass('text-red-600'); // increasing trend
    expect(screen.getByTestId('trend-icon')).toHaveAttribute('data-trend', 'up');

    // Should show token rate
    expect(screen.getByTestId('token-rate')).toHaveTextContent('100 tokens/min');
  });

  it('SC-3.6: displays phase progress with completion estimates', () => {
    const phaseProgress = {
      iterations: 5,
      estimatedCompletion: 0.85,
      currentActivity: 'streaming',
      phaseStartTime: Date.now() - 300000, // 5 minutes ago
      estimatedTimeRemaining: 60, // 1 minute
    };

    render(
      <RealTimeMetrics
        taskId="TASK-006"
        sessionMetrics={undefined}
        phaseProgress={phaseProgress}
        showDetailed={true}
      />
    );

    // Should show phase progress details
    expect(screen.getByTestId('phase-iterations')).toHaveTextContent('5');
    expect(screen.getByTestId('completion-percentage')).toHaveTextContent('85%');
    expect(screen.getByTestId('estimated-remaining')).toHaveTextContent('~1 min remaining');

    // Should show progress bar
    const progressBar = screen.getByTestId('progress-bar');
    expect(progressBar).toHaveStyle('width: 85%');
  });

  it('SC-3.7: handles missing or null data gracefully', () => {
    render(
      <RealTimeMetrics
        taskId="TASK-007"
        sessionMetrics={undefined}
        phaseProgress={undefined}
        showDetailed={true}
      />
    );

    // Should show placeholder text when no data
    expect(screen.getByTestId('no-metrics-message')).toHaveTextContent('No metrics available');
    expect(screen.queryByTestId('session-tokens')).toBeNull();

    // Container should still render
    expect(screen.getByTestId('realtime-metrics')).toBeInTheDocument();
  });

  it('SC-3.8: formats large numbers appropriately', () => {
    const sessionMetrics = {
      totalTokens: 1500000, // 1.5M
      estimatedCostUSD: 25.75,
      inputTokens: 800000,
      outputTokens: 700000,
      durationSeconds: 3661, // 1 hour, 1 minute, 1 second
      tasksRunning: 5,
    };

    render(
      <RealTimeMetrics
        taskId="TASK-008"
        sessionMetrics={sessionMetrics}
        phaseProgress={undefined}
        showDetailed={true}
      />
    );

    // Should format large numbers with proper separators
    expect(screen.getByTestId('session-tokens')).toHaveTextContent('1,500,000');
    expect(screen.getByTestId('session-cost')).toHaveTextContent('$25.75');

    // Duration should be formatted as hours:minutes:seconds
    expect(screen.getByTestId('session-duration')).toHaveTextContent('61:01');
  });
});