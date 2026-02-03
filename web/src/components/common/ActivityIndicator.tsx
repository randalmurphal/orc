import React from 'react';

export interface ActivityIndicatorProps {
  activity: string;
  phase: string;
  className?: string;
  ariaLabel?: string;
}

interface ActivityState {
  icon: string;
  text: string;
  color: string;
  isActive: boolean;
  iconType: string;
  iconAnimation: string;
}

export function ActivityIndicator({
  activity,
  phase,
  className = '',
  ariaLabel
}: ActivityIndicatorProps) {
  const getActivityState = (): ActivityState => {
    const activityStates: Record<string, Omit<ActivityState, 'text'>> = {
      'waiting_api': {
        icon: '⏳',
        color: 'text-blue-600',
        isActive: true,
        iconType: 'waiting',
        iconAnimation: 'animate-pulse'
      },
      'streaming': {
        icon: '📡',
        color: 'text-blue-600',
        isActive: true,
        iconType: 'streaming',
        iconAnimation: 'animate-spin animate-bounce animate-pulse'
      },
      'running_tool': {
        icon: '🔧',
        color: 'text-purple-600',
        isActive: true,
        iconType: 'tool',
        iconAnimation: 'animate-spin animate-bounce animate-pulse'
      },
      'spec_analyzing': {
        icon: '🔍',
        color: 'text-green-600',
        isActive: true,
        iconType: 'analyze',
        iconAnimation: 'animate-pulse'
      },
      'spec_writing': {
        icon: '✏️',
        color: 'text-yellow-600',
        isActive: true,
        iconType: 'write',
        iconAnimation: 'animate-pulse'
      },
      'implement_writing': {
        icon: '💻',
        color: 'text-blue-600',
        isActive: true,
        iconType: 'write',
        iconAnimation: 'animate-pulse'
      },
      'implement_testing': {
        icon: '🧪',
        color: 'text-green-600',
        isActive: true,
        iconType: 'test',
        iconAnimation: 'animate-pulse'
      },
      'review_analyzing': {
        icon: '🕵️',
        color: 'text-orange-600',
        isActive: true,
        iconType: 'analyze',
        iconAnimation: 'animate-pulse'
      },
      'review_feedback': {
        icon: '📝',
        color: 'text-orange-600',
        isActive: true,
        iconType: 'write',
        iconAnimation: 'animate-pulse'
      },
      'idle': {
        icon: '⏸️',
        color: 'text-gray-500',
        isActive: false,
        iconType: 'idle',
        iconAnimation: ''
      }
    };

    const baseState = activityStates[activity] || {
      icon: '🔄',
      color: 'text-gray-500',
      isActive: false,
      iconType: 'default',
      iconAnimation: ''
    };

    const text = getActivityText(activity, phase);

    return {
      ...baseState,
      text
    };
  };

  const getActivityText = (activity: string, phase: string): string => {
    const activityTexts: Record<string, string> = {
      'waiting_api': 'Waiting for API response...',
      'streaming': 'Streaming response...',
      'spec_analyzing': 'Analyzing specification...',
      'spec_writing': 'Writing specification...',
      'implement_writing': 'Writing implementation...',
      'implement_testing': 'Testing implementation...',
      'review_analyzing': 'Analyzing code...',
      'review_feedback': 'Generating feedback...',
      'idle': 'Idle',
      'unknown_activity': 'Processing...'
    };

    // Handle running_tool with phase context
    if (activity === 'running_tool') {
      switch (phase) {
        case 'implement':
          return 'Running implementation tool...';
        case 'test':
          return 'Running test tool...';
        case 'review':
          return 'Running review tool...';
        default:
          return 'Running tool...';
      }
    }

    return activityTexts[activity] || 'Processing...';
  };

  const activityState = getActivityState();
  const displayAriaLabel = ariaLabel || activityState.text;

  return (
    <div
      data-testid="activity-indicator"
      data-activity={activity}
      data-phase={phase}
      className={`flex items-center gap-2 ${className} ${activityState.color} ${
        activityState.isActive ? 'animate-pulse' : ''
      }`}
      role="status"
      aria-live="polite"
      aria-label={displayAriaLabel}
    >
      <span
        data-testid="activity-icon"
        data-icon-type={activityState.iconType}
        className={`text-lg ${activityState.color} ${activityState.iconAnimation}`}
      >
        {activityState.icon}
      </span>
      <span
        data-testid="activity-text"
        id="activity-description"
        className={`text-sm font-medium ${activityState.color}`}
        aria-describedby="activity-description"
      >
        {activityState.text}
      </span>
    </div>
  );
}