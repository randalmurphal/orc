import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { ActivityIndicator } from '../ActivityIndicator';

describe('ActivityIndicator Real-Time Activity Visualization', () => {
  it('SC-2.1: displays activity state with appropriate icon and text', () => {
    render(
      <ActivityIndicator
        activity="waiting_api"
        phase="implement"
        className="test-class"
      />
    );

    const indicator = screen.getByTestId('activity-indicator');
    expect(indicator).toHaveAttribute('data-activity', 'waiting_api');
    expect(indicator).toHaveAttribute('data-phase', 'implement');
    expect(indicator).toHaveClass('test-class');

    // Should display appropriate text and icon for waiting_api
    expect(screen.getByTestId('activity-icon')).toHaveAttribute('data-icon-type', 'waiting');
    expect(screen.getByTestId('activity-text')).toHaveTextContent('Waiting for API response...');
  });

  it('SC-2.2: shows different states for various activity types', () => {
    const activityStates = [
      {
        activity: 'streaming',
        expectedIcon: 'streaming',
        expectedText: 'Streaming response...',
        expectedColor: 'text-blue-600'
      },
      {
        activity: 'running_tool',
        expectedIcon: 'tool',
        expectedText: 'Running tool...',
        expectedColor: 'text-purple-600'
      },
      {
        activity: 'spec_analyzing',
        expectedIcon: 'analyze',
        expectedText: 'Analyzing specification...',
        expectedColor: 'text-green-600'
      },
      {
        activity: 'spec_writing',
        expectedIcon: 'write',
        expectedText: 'Writing specification...',
        expectedColor: 'text-yellow-600'
      },
      {
        activity: 'idle',
        expectedIcon: 'idle',
        expectedText: 'Idle',
        expectedColor: 'text-gray-500'
      }
    ];

    activityStates.forEach(({ activity, expectedIcon, expectedText, expectedColor }) => {
      const { unmount } = render(
        <ActivityIndicator activity={activity} phase="implement" />
      );

      const indicator = screen.getByTestId('activity-indicator');
      expect(indicator).toHaveAttribute('data-activity', activity);
      expect(indicator).toHaveClass(expectedColor);

      const icon = screen.getByTestId('activity-icon');
      expect(icon).toHaveAttribute('data-icon-type', expectedIcon);

      const text = screen.getByTestId('activity-text');
      expect(text).toHaveTextContent(expectedText);

      unmount();
    });
  });

  it('SC-2.3: shows pulsing animation for active states', () => {
    render(
      <ActivityIndicator activity="streaming" phase="implement" />
    );

    const indicator = screen.getByTestId('activity-indicator');
    expect(indicator).toHaveClass('animate-pulse');

    const icon = screen.getByTestId('activity-icon');
    expect(icon).toHaveClass('animate-spin', 'animate-bounce', 'animate-pulse');
  });

  it('SC-2.4: shows static state for idle activity', () => {
    render(
      <ActivityIndicator activity="idle" phase="implement" />
    );

    const indicator = screen.getByTestId('activity-indicator');
    expect(indicator).not.toHaveClass('animate-pulse');

    const icon = screen.getByTestId('activity-icon');
    expect(icon).not.toHaveClass('animate-spin', 'animate-bounce', 'animate-pulse');
  });

  it('SC-2.5: adapts text based on phase context', () => {
    // Same activity in different phases should show contextual text
    const { rerender } = render(
      <ActivityIndicator activity="running_tool" phase="implement" />
    );

    expect(screen.getByTestId('activity-text')).toHaveTextContent('Running implementation tool...');

    rerender(
      <ActivityIndicator activity="running_tool" phase="test" />
    );

    expect(screen.getByTestId('activity-text')).toHaveTextContent('Running test tool...');

    rerender(
      <ActivityIndicator activity="running_tool" phase="review" />
    );

    expect(screen.getByTestId('activity-text')).toHaveTextContent('Running review tool...');
  });

  it('SC-2.6: handles unknown activity gracefully', () => {
    render(
      <ActivityIndicator activity="unknown_activity" phase="implement" />
    );

    const indicator = screen.getByTestId('activity-indicator');
    expect(indicator).toHaveAttribute('data-activity', 'unknown_activity');

    // Should show default state for unknown activity
    expect(screen.getByTestId('activity-icon')).toHaveAttribute('data-icon-type', 'default');
    expect(screen.getByTestId('activity-text')).toHaveTextContent('Processing...');
    expect(indicator).toHaveClass('text-gray-500');
  });

  it('SC-2.7: supports accessibility attributes', () => {
    render(
      <ActivityIndicator
        activity="streaming"
        phase="implement"
        ariaLabel="Current task activity"
      />
    );

    const indicator = screen.getByTestId('activity-indicator');
    expect(indicator).toHaveAttribute('aria-label', 'Current task activity');
    expect(indicator).toHaveAttribute('role', 'status');
    expect(indicator).toHaveAttribute('aria-live', 'polite');

    const text = screen.getByTestId('activity-text');
    expect(text).toHaveAttribute('aria-describedby', 'activity-description');
  });
});