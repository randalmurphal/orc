// Formatting utilities for display values

/**
 * Formats a currency value to a readable string
 */
export function formatCurrency(value: number): string {
  if (value === 0) return '$0.00';

  if (value < 0.01) {
    return `<$0.01`;
  }

  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value);
}

/**
 * Formats a token count to a readable string with commas
 */
export function formatTokens(value: number): string {
  if (value === 0) return '0';

  if (value < 1000) {
    return value.toString();
  }

  if (value < 1000000) {
    const thousands = (value / 1000).toFixed(1);
    return thousands.endsWith('.0')
      ? `${Math.floor(value / 1000)}K`
      : `${thousands}K`;
  }

  return new Intl.NumberFormat('en-US').format(value);
}

/**
 * Formats a duration in seconds to MM:SS or HH:MM:SS format
 */
export function formatDuration(seconds: number): string {
  if (seconds < 0) return '0:00';

  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  }

  return `${minutes}:${secs.toString().padStart(2, '0')}`;
}

/**
 * Formats a time remaining estimate
 */
export function formatTimeRemaining(seconds: number): string {
  if (seconds <= 0) return 'Complete';

  if (seconds < 60) {
    return `~${seconds}s remaining`;
  }

  if (seconds < 3600) {
    const minutes = Math.ceil(seconds / 60);
    return `~${minutes} min remaining`;
  }

  const hours = Math.ceil(seconds / 3600);
  return `~${hours}h remaining`;
}

/**
 * Formats a percentage value
 */
export function formatPercentage(value: number): string {
  if (value < 0) return '0%';
  if (value > 1) return '100%';

  return `${Math.round(value * 100)}%`;
}

/**
 * Formats a token breakdown display
 */
export function formatTokenBreakdown(input: number, output: number): string {
  return `${formatTokens(input)} in / ${formatTokens(output)} out`;
}

/**
 * Formats a progress rate
 */
export function formatProgressRate(progressPerSecond: number): string {
  const progressPerMinute = progressPerSecond * 60;
  const percentagePerMinute = Math.round(progressPerMinute * 100);

  if (percentagePerMinute <= 0) return 'Calculating...';

  return `${percentagePerMinute}% per min`;
}