import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { RunningCard } from '../RunningCard';
import { Task, ExecutionState, PhaseStatus, TaskStatus } from '../../../gen/orc/v1/task_pb';
import { Timestamp } from '../../../gen/google/protobuf/timestamp_pb';

// Mock the stores
vi.mock('../../../stores/taskStore', () => ({
  useTaskStore: vi.fn(),
}));

// Mock the activity indicator component
vi.mock('../../common/ActivityIndicator', () => ({
  ActivityIndicator: ({ activity, phase }: { activity: string; phase: string }) => (
    <div data-testid="activity-indicator" data-activity={activity} data-phase={phase}>
      {activity}
    </div>
  ),
}));

// Mock the real-time metrics component
vi.mock('../../common/RealTimeMetrics', () => ({
  RealTimeMetrics: ({
    taskId,
    sessionMetrics,
    phaseProgress
  }: {
    taskId: string;
    sessionMetrics: any;
    phaseProgress: any;
  }) => (
    <div data-testid="realtime-metrics" data-task-id={taskId}>
      <span data-testid="tokens">{sessionMetrics?.totalTokens || 0}</span>
      <span data-testid="cost">{sessionMetrics?.estimatedCostUSD || 0}</span>
      <span data-testid="iterations">{phaseProgress?.iterations || 0}</span>
    </div>
  ),
}));

// Mock the live output component
vi.mock('../../common/LiveOutput', () => ({
  LiveOutput: ({ taskId, outputLines }: { taskId: string; outputLines: string[] }) => (
    <div data-testid="live-output" data-task-id={taskId}>
      {outputLines.map((line, index) => (
        <div key={index} data-testid="output-line">{line}</div>
      ))}
    </div>
  ),
}));

describe('RunningCard Real-Time Progress Updates', () => {
  const mockUseTaskStore = vi.mocked(
    require('../../../stores/taskStore').useTaskStore
  );

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('SC-1: displays real-time activity state indicators', async () => {
    // Setup task with current activity state
    const task = new Task({
      id: 'TASK-001',
      title: 'Test Task',
      status: TaskStatus.RUNNING,
      currentPhase: 'implement',
      startedAt: Timestamp.fromDate(new Date(Date.now() - 30000)), // 30s ago
    });

    const executionState = new ExecutionState({
      currentIteration: 2,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING, // Currently running
          startedAt: Timestamp.fromDate(new Date(Date.now() - 15000)), // 15s ago
          iterations: 2,
        },
        'spec': {
          status: PhaseStatus.COMPLETED,
          startedAt: Timestamp.fromDate(new Date(Date.now() - 60000)),
          completedAt: Timestamp.fromDate(new Date(Date.now() - 45000)),
        }
      }
    });

    // Mock store to return activity state
    mockUseTaskStore.mockReturnValue({
      getTaskActivity: vi.fn().mockReturnValue({
        phase: 'implement',
        activity: 'waiting_api', // Real-time activity state
      }),
      getTaskOutputLines: vi.fn().mockReturnValue([]),
      getSessionMetrics: vi.fn().mockReturnValue(null),
    });

    render(
      <RunningCard
        task={task}
        executionState={executionState}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Should display activity indicator with current activity
    const activityIndicator = screen.getByTestId('activity-indicator');
    expect(activityIndicator).toHaveAttribute('data-activity', 'waiting_api');
    expect(activityIndicator).toHaveAttribute('data-phase', 'implement');
    expect(activityIndicator).toHaveTextContent('waiting_api');
  });

  it('SC-2: shows live output lines from transcript events', async () => {
    const task = new Task({
      id: 'TASK-002',
      title: 'Test Task with Output',
      status: TaskStatus.RUNNING,
      currentPhase: 'implement',
      startedAt: Timestamp.fromDate(new Date(Date.now() - 60000)),
    });

    const executionState = new ExecutionState({
      currentIteration: 1,
    });

    // Mock store to return live output lines
    const outputLines = [
      '✓ Reading file: src/main.go',
      '→ Analyzing function signatures...',
      '✗ Error: missing import statement',
      '→ Fixing import statement...',
      '✓ Tests are now passing'
    ];

    mockUseTaskStore.mockReturnValue({
      getTaskActivity: vi.fn().mockReturnValue({ activity: 'streaming', phase: 'implement' }),
      getTaskOutputLines: vi.fn().mockReturnValue(outputLines),
      getSessionMetrics: vi.fn().mockReturnValue(null),
    });

    render(
      <RunningCard
        task={task}
        executionState={executionState}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Should display live output component with output lines
    const liveOutput = screen.getByTestId('live-output');
    expect(liveOutput).toHaveAttribute('data-task-id', 'TASK-002');

    const outputLineElements = screen.getAllByTestId('output-line');
    expect(outputLineElements).toHaveLength(5);
    expect(outputLineElements[0]).toHaveTextContent('✓ Reading file: src/main.go');
    expect(outputLineElements[2]).toHaveTextContent('✗ Error: missing import statement');
    expect(outputLineElements[4]).toHaveTextContent('✓ Tests are now passing');
  });

  it('SC-3: displays real-time session metrics for running task', async () => {
    const task = new Task({
      id: 'TASK-003',
      title: 'Test Task with Metrics',
      status: TaskStatus.RUNNING,
      currentPhase: 'implement',
      startedAt: Timestamp.fromDate(new Date(Date.now() - 120000)), // 2 minutes ago
    });

    const executionState = new ExecutionState({
      currentIteration: 3,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          iterations: 3,
        }
      }
    });

    // Mock store to return session metrics
    const sessionMetrics = {
      totalTokens: 15420,
      estimatedCostUSD: 0.45,
      inputTokens: 8200,
      outputTokens: 7220,
      durationSeconds: 135,
      tasksRunning: 2,
    };

    const phaseProgress = {
      iterations: 3,
      currentActivity: 'streaming',
      estimatedCompletion: 0.75, // 75% complete
    };

    mockUseTaskStore.mockReturnValue({
      getTaskActivity: vi.fn().mockReturnValue({ activity: 'streaming', phase: 'implement' }),
      getTaskOutputLines: vi.fn().mockReturnValue([]),
      getSessionMetrics: vi.fn().mockReturnValue(sessionMetrics),
      getPhaseProgress: vi.fn().mockReturnValue(phaseProgress),
    });

    render(
      <RunningCard
        task={task}
        executionState={executionState}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Should display real-time metrics
    const metricsComponent = screen.getByTestId('realtime-metrics');
    expect(metricsComponent).toHaveAttribute('data-task-id', 'TASK-003');

    expect(screen.getByTestId('tokens')).toHaveTextContent('15420');
    expect(screen.getByTestId('cost')).toHaveTextContent('0.45');
    expect(screen.getByTestId('iterations')).toHaveTextContent('3');
  });

  it('SC-4: updates activity state when events are received', async () => {
    const task = new Task({
      id: 'TASK-004',
      title: 'Test Activity Updates',
      status: TaskStatus.RUNNING,
      currentPhase: 'implement',
    });

    let mockActivity = { phase: 'implement', activity: 'waiting_api' };

    mockUseTaskStore.mockReturnValue({
      getTaskActivity: vi.fn().mockImplementation(() => mockActivity),
      getTaskOutputLines: vi.fn().mockReturnValue([]),
      getSessionMetrics: vi.fn().mockReturnValue(null),
    });

    const { rerender } = render(
      <RunningCard
        task={task}
        executionState={new ExecutionState()}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Initial state
    expect(screen.getByTestId('activity-indicator')).toHaveAttribute('data-activity', 'waiting_api');

    // Simulate activity change via store update
    mockActivity = { phase: 'implement', activity: 'streaming' };

    rerender(
      <RunningCard
        task={task}
        executionState={new ExecutionState()}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Should update to new activity state
    await waitFor(() => {
      expect(screen.getByTestId('activity-indicator')).toHaveAttribute('data-activity', 'streaming');
    });
  });

  it('SC-5: shows enhanced phase progress indicators', async () => {
    const task = new Task({
      id: 'TASK-005',
      title: 'Test Enhanced Progress',
      status: TaskStatus.RUNNING,
      currentPhase: 'review',
    });

    const executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'spec': { status: PhaseStatus.COMPLETED },
        'implement': { status: PhaseStatus.COMPLETED },
        'test': { status: PhaseStatus.COMPLETED },
        'review': {
          status: PhaseStatus.PENDING,
          iterations: 1,
        }
      }
    });

    mockUseTaskStore.mockReturnValue({
      getTaskActivity: vi.fn().mockReturnValue({
        phase: 'review',
        activity: 'spec_analyzing'
      }),
      getTaskOutputLines: vi.fn().mockReturnValue([]),
      getSessionMetrics: vi.fn().mockReturnValue(null),
      getPhaseProgress: vi.fn().mockReturnValue({
        iterations: 1,
        subSteps: ['Analyzing code quality', 'Checking test coverage', 'Reviewing documentation'],
        currentStep: 1,
        estimatedCompletion: 0.33,
      }),
    });

    render(
      <RunningCard
        task={task}
        executionState={executionState}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Should show enhanced progress with detailed phase info
    expect(screen.getByTestId('activity-indicator')).toHaveAttribute('data-activity', 'spec_analyzing');

    const metricsComponent = screen.getByTestId('realtime-metrics');
    expect(metricsComponent).toBeInTheDocument();
    expect(screen.getByTestId('iterations')).toHaveTextContent('1');
  });

  it('SC-6: handles missing real-time data gracefully', () => {
    const task = new Task({
      id: 'TASK-006',
      title: 'Test Missing Data',
      status: TaskStatus.RUNNING,
      currentPhase: 'implement',
    });

    // Mock store to return null/empty data
    mockUseTaskStore.mockReturnValue({
      getTaskActivity: vi.fn().mockReturnValue(null),
      getTaskOutputLines: vi.fn().mockReturnValue([]),
      getSessionMetrics: vi.fn().mockReturnValue(null),
    });

    render(
      <RunningCard
        task={task}
        executionState={new ExecutionState()}
        isExpanded={true}
        onToggleExpand={vi.fn()}
      />
    );

    // Should render without errors and show appropriate defaults
    const activityIndicator = screen.getByTestId('activity-indicator');
    expect(activityIndicator).toHaveAttribute('data-activity', ''); // Default empty activity

    const liveOutput = screen.getByTestId('live-output');
    expect(liveOutput).toBeInTheDocument();
    expect(screen.queryByTestId('output-line')).toBeNull(); // No output lines

    const metrics = screen.getByTestId('realtime-metrics');
    expect(screen.getByTestId('tokens')).toHaveTextContent('0');
    expect(screen.getByTestId('cost')).toHaveTextContent('0');
  });
});