import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { DetailedPhaseProgress } from '../DetailedPhaseProgress';
import { PhaseStatus, ExecutionState } from '../../../gen/orc/v1/task_pb';
import { Timestamp } from '../../../gen/google/protobuf/timestamp_pb';

// Mock the progress estimation utilities
vi.mock('../../../lib/utils/progressEstimation', () => ({
  estimatePhaseCompletion: vi.fn(),
  calculateTimeRemaining: vi.fn(),
  getPhaseSubSteps: vi.fn(),
}));

// Mock the phase activity mapper
vi.mock('../../../lib/mappers/phaseActivityMapper', () => ({
  mapActivityToProgress: vi.fn(),
  getActivityDescription: vi.fn(),
}));

describe('DetailedPhaseProgress Enhanced Progress Indicators', () => {
  const mockEstimatePhaseCompletion = vi.mocked(
    require('../../../lib/utils/progressEstimation').estimatePhaseCompletion
  );
  const mockCalculateTimeRemaining = vi.mocked(
    require('../../../lib/utils/progressEstimation').calculateTimeRemaining
  );
  const mockGetPhaseSubSteps = vi.mocked(
    require('../../../lib/utils/progressEstimation').getPhaseSubSteps
  );
  const mockMapActivityToProgress = vi.mocked(
    require('../../../lib/mappers/phaseActivityMapper').mapActivityToProgress
  );
  const mockGetActivityDescription = vi.mocked(
    require('../../../lib/mappers/phaseActivityMapper').getActivityDescription
  );

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('SC-5.1: displays detailed phase progress with sub-steps', () => {
    const executionState = new ExecutionState({
      currentIteration: 2,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          startedAt: Timestamp.fromDate(new Date(Date.now() - 300000)), // 5 minutes ago
          iterations: 2,
        }
      }
    });

    // Mock progress estimation
    mockEstimatePhaseCompletion.mockReturnValue(0.65); // 65% complete
    mockCalculateTimeRemaining.mockReturnValue(120); // 2 minutes remaining
    mockGetPhaseSubSteps.mockReturnValue([
      { step: 'Analyzing code structure', completed: true },
      { step: 'Writing implementation', completed: true },
      { step: 'Running tests', completed: false, inProgress: true },
      { step: 'Validating results', completed: false },
    ]);

    mockMapActivityToProgress.mockReturnValue({
      currentStep: 2,
      progressWithinStep: 0.75,
    });

    mockGetActivityDescription.mockReturnValue('Running implementation tests...');

    render(
      <DetailedPhaseProgress
        taskId="TASK-001"
        currentPhase="implement"
        executionState={executionState}
        currentActivity="running_tool"
        showSubSteps={true}
      />
    );

    // Should show overall phase progress
    expect(screen.getByTestId('phase-name')).toHaveTextContent('Implementation');
    expect(screen.getByTestId('phase-progress')).toHaveTextContent('65%');
    expect(screen.getByTestId('time-remaining')).toHaveTextContent('~2 min remaining');

    // Should show sub-steps with status
    const subSteps = screen.getAllByTestId('progress-substep');
    expect(subSteps).toHaveLength(4);

    expect(subSteps[0]).toHaveClass('completed');
    expect(subSteps[0]).toHaveTextContent('✓ Analyzing code structure');

    expect(subSteps[1]).toHaveClass('completed');
    expect(subSteps[1]).toHaveTextContent('✓ Writing implementation');

    expect(subSteps[2]).toHaveClass('in-progress');
    expect(subSteps[2]).toHaveTextContent('⟳ Running tests');

    expect(subSteps[3]).toHaveClass('pending');
    expect(subSteps[3]).toHaveTextContent('○ Validating results');

    // Should show current activity
    expect(screen.getByTestId('current-activity')).toHaveTextContent('Running implementation tests...');
  });

  it('SC-5.2: shows iteration progress and retry indicators', () => {
    const executionState = new ExecutionState({
      currentIteration: 3,
      phases: {
        'review': {
          status: PhaseStatus.PENDING,
          startedAt: Timestamp.fromDate(new Date(Date.now() - 600000)), // 10 minutes ago
          iterations: 3,
        }
      }
    });

    mockEstimatePhaseCompletion.mockReturnValue(0.40); // 40% complete
    mockGetPhaseSubSteps.mockReturnValue([
      { step: 'Code quality analysis', completed: true },
      { step: 'Security review', completed: false, inProgress: true, retryCount: 2 },
      { step: 'Documentation review', completed: false },
    ]);

    render(
      <DetailedPhaseProgress
        taskId="TASK-002"
        currentPhase="review"
        executionState={executionState}
        currentActivity="spec_analyzing"
        showSubSteps={true}
      />
    );

    // Should show iteration count
    expect(screen.getByTestId('iteration-count')).toHaveTextContent('Iteration 3');
    expect(screen.getByTestId('iteration-indicator')).toHaveClass('text-yellow-600'); // Multiple iterations

    // Should show retry indicator on problematic step
    const securityStep = screen.getByTestId('substep-security-review');
    expect(securityStep).toContainElement(screen.getByTestId('retry-indicator'));
    expect(screen.getByTestId('retry-count')).toHaveTextContent('Retry 2');
  });

  it('SC-5.3: displays phase duration and performance metrics', () => {
    const executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          startedAt: Timestamp.fromDate(new Date(Date.now() - 480000)), // 8 minutes ago
          iterations: 1,
          tokens: {
            input: 5000,
            output: 3500,
            total: 8500,
          }
        }
      }
    });

    mockEstimatePhaseCompletion.mockReturnValue(0.80);
    mockCalculateTimeRemaining.mockReturnValue(60); // 1 minute remaining

    render(
      <DetailedPhaseProgress
        taskId="TASK-003"
        currentPhase="implement"
        executionState={executionState}
        currentActivity="streaming"
        showMetrics={true}
      />
    );

    // Should show duration
    expect(screen.getByTestId('phase-duration')).toHaveTextContent('8:00');

    // Should show token usage
    expect(screen.getByTestId('phase-tokens')).toHaveTextContent('8,500');
    expect(screen.getByTestId('token-breakdown')).toHaveTextContent('5K in / 3.5K out');

    // Should show progress rate
    expect(screen.getByTestId('progress-rate')).toHaveTextContent('10% per min');
  });

  it('SC-5.4: handles long-running phases with heartbeat indicators', () => {
    const executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          startedAt: Timestamp.fromDate(new Date(Date.now() - 1800000)), // 30 minutes ago
          iterations: 1,
        }
      }
    });

    // Mock long-running phase
    mockEstimatePhaseCompletion.mockReturnValue(0.45);
    mockCalculateTimeRemaining.mockReturnValue(2400); // 40 minutes remaining

    render(
      <DetailedPhaseProgress
        taskId="TASK-004"
        currentPhase="implement"
        executionState={executionState}
        currentActivity="waiting_api"
        showHeartbeat={true}
      />
    );

    // Should show long-running indicator
    expect(screen.getByTestId('long-running-indicator')).toBeInTheDocument();
    expect(screen.getByTestId('long-running-warning')).toHaveTextContent(
      'This phase has been running for 30 minutes'
    );

    // Should show heartbeat status
    expect(screen.getByTestId('heartbeat-indicator')).toBeInTheDocument();
    expect(screen.getByTestId('last-heartbeat')).toHaveTextContent('Active');
  });

  it('SC-5.5: shows completion confidence and quality indicators', () => {
    const executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'review': {
          status: PhaseStatus.PENDING,
          startedAt: Timestamp.fromDate(new Date(Date.now() - 120000)), // 2 minutes ago
          iterations: 1,
          validation: [
            { type: 'syntax_check', passed: true },
            { type: 'test_coverage', passed: true },
            { type: 'security_scan', passed: false },
          ]
        }
      }
    });

    mockEstimatePhaseCompletion.mockReturnValue(0.85);

    render(
      <DetailedPhaseProgress
        taskId="TASK-005"
        currentPhase="review"
        executionState={executionState}
        currentActivity="spec_analyzing"
        showQualityIndicators={true}
      />
    );

    // Should show completion confidence
    expect(screen.getByTestId('completion-confidence')).toHaveTextContent('High confidence');
    expect(screen.getByTestId('confidence-indicator')).toHaveClass('text-green-600');

    // Should show quality gate status
    const qualityGates = screen.getAllByTestId('quality-gate');
    expect(qualityGates).toHaveLength(3);

    expect(qualityGates[0]).toHaveClass('passed');
    expect(qualityGates[0]).toHaveTextContent('✓ Syntax Check');

    expect(qualityGates[1]).toHaveClass('passed');
    expect(qualityGates[1]).toHaveTextContent('✓ Test Coverage');

    expect(qualityGates[2]).toHaveClass('failed');
    expect(qualityGates[2]).toHaveTextContent('✗ Security Scan');
  });

  it('SC-5.6: provides expandable detailed view', () => {
    const executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          iterations: 1,
        }
      }
    });

    render(
      <DetailedPhaseProgress
        taskId="TASK-006"
        currentPhase="implement"
        executionState={executionState}
        currentActivity="streaming"
        collapsible={true}
        defaultExpanded={false}
      />
    );

    // Should show compact view initially
    expect(screen.getByTestId('progress-summary')).toBeInTheDocument();
    expect(screen.queryByTestId('detailed-progress')).toBeNull();

    // Should expand when clicked
    const expandButton = screen.getByTestId('expand-button');
    expandButton.click();

    expect(screen.getByTestId('detailed-progress')).toBeInTheDocument();
    expect(screen.getByTestId('progress-substeps')).toBeInTheDocument();
  });

  it('SC-5.7: handles phase transitions with smooth animations', async () => {
    let currentPhase = 'implement';
    let executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': { status: PhaseStatus.PENDING, iterations: 1 },
        'test': { status: PhaseStatus.UNSPECIFIED },
      }
    });

    const { rerender } = render(
      <DetailedPhaseProgress
        taskId="TASK-007"
        currentPhase={currentPhase}
        executionState={executionState}
        currentActivity="running_tool"
        animateTransitions={true}
      />
    );

    // Simulate phase transition
    currentPhase = 'test';
    executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': { status: PhaseStatus.COMPLETED, iterations: 1 },
        'test': { status: PhaseStatus.PENDING, iterations: 1 },
      }
    });

    rerender(
      <DetailedPhaseProgress
        taskId="TASK-007"
        currentPhase={currentPhase}
        executionState={executionState}
        currentActivity="running_tool"
        animateTransitions={true}
      />
    );

    // Should show transition animation
    await waitFor(() => {
      expect(screen.getByTestId('phase-transition')).toHaveClass('animate-fade-in');
    });

    expect(screen.getByTestId('phase-name')).toHaveTextContent('Test');
  });

  it('SC-5.8: integrates with pause/resume controls', () => {
    const executionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          iterations: 1,
        }
      }
    });

    const mockOnPause = vi.fn();
    const mockOnResume = vi.fn();

    render(
      <DetailedPhaseProgress
        taskId="TASK-008"
        currentPhase="implement"
        executionState={executionState}
        currentActivity="streaming"
        isPaused={false}
        onPause={mockOnPause}
        onResume={mockOnResume}
        showControls={true}
      />
    );

    // Should show pause control when running
    const pauseButton = screen.getByTestId('pause-button');
    expect(pauseButton).toBeInTheDocument();

    pauseButton.click();
    expect(mockOnPause).toHaveBeenCalledWith('TASK-008');

    // Test resume button
    rerender(
      <DetailedPhaseProgress
        taskId="TASK-008"
        currentPhase="implement"
        executionState={executionState}
        currentActivity="idle"
        isPaused={true}
        onPause={mockOnPause}
        onResume={mockOnResume}
        showControls={true}
      />
    );

    const resumeButton = screen.getByTestId('resume-button');
    resumeButton.click();
    expect(mockOnResume).toHaveBeenCalledWith('TASK-008');
  });
});