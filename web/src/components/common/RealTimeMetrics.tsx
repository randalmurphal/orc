import React from 'react';
import { formatCurrency, formatTokens, formatDuration, formatPercentage, formatTimeRemaining } from '../../lib/utils/formatting';

export interface SessionMetrics {
  totalTokens: number;
  estimatedCostUSD: number;
  inputTokens?: number;
  outputTokens?: number;
  durationSeconds: number;
  tasksRunning: number;
  isPaused?: boolean;
  costTrend?: 'increasing' | 'decreasing';
  tokenRate?: number;
}

export interface PhaseProgress {
  iterations: number;
  estimatedCompletion: number;
  currentActivity?: string;
  phaseStartTime?: number;
  estimatedTimeRemaining?: number;
}

export interface RealTimeMetricsProps {
  taskId: string;
  sessionMetrics?: SessionMetrics;
  phaseProgress?: PhaseProgress;
  showDetailed?: boolean;
}

export function RealTimeMetrics({
  taskId: _taskId,
  sessionMetrics,
  phaseProgress,
  showDetailed = false
}: RealTimeMetricsProps) {
  if (!sessionMetrics && !phaseProgress) {
    return (
      <div data-testid="realtime-metrics" className="text-sm text-gray-500">
        <span data-testid="no-metrics-message">No metrics available</span>
      </div>
    );
  }

  const {
    totalTokens = 0,
    estimatedCostUSD = 0,
    inputTokens = 0,
    outputTokens = 0,
    durationSeconds = 0,
    tasksRunning = 0,
    isPaused = false,
    costTrend,
    tokenRate = 0
  } = sessionMetrics || {};

  const getTrendIcon = (trend?: string) => {
    switch (trend) {
      case 'increasing':
        return <span data-testid="trend-icon" data-trend="up" className="text-red-600">↗</span>;
      case 'decreasing':
        return <span data-testid="trend-icon" data-trend="down" className="text-green-600">↘</span>;
      default:
        return null;
    }
  };

  const getTrendColor = (trend?: string) => {
    switch (trend) {
      case 'increasing':
        return 'text-red-600';
      case 'decreasing':
        return 'text-green-600';
      default:
        return 'text-gray-700';
    }
  };

  if (!showDetailed) {
    return (
      <div data-testid="compact-metrics" className="flex items-center gap-4 text-sm">
        {sessionMetrics && (
          <>
            <div className="flex items-center gap-1">
              <span className="text-gray-600">Tokens:</span>
              <span data-testid="session-tokens" className="font-medium">{formatTokens(totalTokens)}</span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-gray-600">Cost:</span>
              <span data-testid="session-cost" className={`font-medium ${getTrendColor(costTrend)}`}>
                {formatCurrency(estimatedCostUSD)}
              </span>
              {getTrendIcon(costTrend)}
            </div>
            <div className="flex items-center gap-1">
              <span className="text-gray-600">Time:</span>
              <span className="font-medium">{formatDuration(durationSeconds)}</span>
            </div>
            {isPaused && (
              <div className="flex items-center gap-1 text-yellow-600">
                <span>⏸</span>
                <span className="text-xs">Paused</span>
              </div>
            )}
          </>
        )}
        {phaseProgress && (
          <div className="flex items-center gap-1">
            <span className="text-gray-600">Progress:</span>
            <span className="font-medium">{formatPercentage(phaseProgress.estimatedCompletion)}</span>
          </div>
        )}
      </div>
    );
  }

  return (
    <div data-testid="realtime-metrics" className="space-y-3">
      {/* Header - only show if we have session metrics */}
      {sessionMetrics && (
        <>
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-800">Session Metrics</h3>
            {isPaused && (
              <span data-testid="session-status" className="text-xs text-yellow-600 bg-yellow-50 px-2 py-1 rounded">
                Paused
              </span>
            )}
            {!isPaused && (
              <span data-testid="session-status" className="text-xs text-green-600">
                Running
              </span>
            )}
          </div>

          {/* Main metrics grid */}
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-gray-600">Total Tokens:</span>
              <div data-testid="session-tokens" className="font-medium">{formatTokens(totalTokens)}</div>
              {showDetailed && (
                <div className="text-xs text-gray-500">
                  <span data-testid="input-tokens">{formatTokens(inputTokens)}</span> in / <span data-testid="output-tokens">{formatTokens(outputTokens)}</span> out
                </div>
              )}
            </div>

            <div>
              <span className="text-gray-600">Estimated Cost:</span>
              <div className={`font-medium flex items-center gap-1 ${getTrendColor(costTrend)}`}>
                <span data-testid="session-cost">{formatCurrency(estimatedCostUSD)}</span>
                {costTrend && <span data-testid="cost-trend" className={getTrendColor(costTrend)}>{getTrendIcon(costTrend)}</span>}
              </div>
            </div>

            <div>
              <span className="text-gray-600">Duration:</span>
              <div data-testid="session-duration" className="font-medium">{formatDuration(durationSeconds)}</div>
            </div>

            <div>
              <span className="text-gray-600">Active Tasks:</span>
              <div data-testid="tasks-running" className="font-medium">{tasksRunning}</div>
            </div>

            {tokenRate > 0 && (
              <div className="col-span-2">
                <span className="text-gray-600">Token Rate:</span>
                <div data-testid="token-rate" className="font-medium">
                  {formatTokens(Math.round(tokenRate))} tokens/min
                </div>
              </div>
            )}
          </div>
        </>
      )}

      {/* Phase progress */}
      {phaseProgress && (
        <div className={`${sessionMetrics ? 'border-t pt-3' : ''} space-y-2`}>
          <h4 className="text-xs font-medium text-gray-700">Phase Progress</h4>

          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-600">Iterations:</span>
            <span data-testid="phase-iterations" className="font-medium">{phaseProgress.iterations}</span>
          </div>

          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-600">Completion:</span>
            <span data-testid="completion-percentage" className="font-medium">
              {formatPercentage(phaseProgress.estimatedCompletion)}
            </span>
          </div>

          <div className="w-full bg-gray-200 rounded-full h-1.5">
            <div
              data-testid="progress-bar"
              className="bg-blue-600 h-1.5 rounded-full transition-all duration-300"
              style={{ width: formatPercentage(phaseProgress.estimatedCompletion) }}
            />
          </div>

          {phaseProgress.estimatedTimeRemaining && (
            <div className="flex items-center justify-between text-xs text-gray-600">
              <span>Est. remaining:</span>
              <span data-testid="estimated-remaining">{formatTimeRemaining(phaseProgress.estimatedTimeRemaining)}</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}