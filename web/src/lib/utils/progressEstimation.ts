// Progress estimation utilities for task phases

export interface PhaseSubStep {
  step: string;
  completed: boolean;
  inProgress?: boolean;
  retryCount?: number;
}

/**
 * Estimates the completion percentage of a phase (0-1)
 */
export function estimatePhaseCompletion(
  currentActivity: string,
  phaseStartTime?: number,
  phaseDuration?: number
): number {
  // Basic estimation based on activity type and duration
  const activityBaseProgress: Record<string, number> = {
    'waiting_api': 0.1,
    'streaming': 0.3,
    'running_tool': 0.6,
    'spec_analyzing': 0.2,
    'spec_writing': 0.4,
    'implement_analyzing': 0.1,
    'implement_writing': 0.5,
    'implement_testing': 0.8,
    'review_analyzing': 0.3,
    'review_feedback': 0.7,
    'idle': 0.9,
  };

  let baseProgress = activityBaseProgress[currentActivity] || 0.5;

  // Add time-based progression
  if (phaseStartTime && phaseDuration) {
    const elapsed = Date.now() - phaseStartTime;
    const timeProgress = Math.min(elapsed / (phaseDuration * 1000), 1);
    baseProgress = Math.max(baseProgress, timeProgress * 0.8); // Cap time-based progress at 80%
  }

  return Math.min(baseProgress, 0.99); // Never reach 100% until actually complete
}

/**
 * Calculates estimated time remaining for a phase in seconds
 */
export function calculateTimeRemaining(
  currentActivity: string,
  estimatedCompletion: number,
  phaseStartTime?: number
): number {
  if (!phaseStartTime || estimatedCompletion >= 0.99) {
    return 0;
  }

  const elapsed = Date.now() - phaseStartTime;
  const elapsedSeconds = elapsed / 1000;

  if (estimatedCompletion <= 0) {
    return 300; // Default 5 minutes
  }

  const totalEstimated = elapsedSeconds / estimatedCompletion;
  const remaining = totalEstimated - elapsedSeconds;

  return Math.max(Math.round(remaining), 0);
}

/**
 * Gets the sub-steps for a phase based on current activity
 */
export function getPhaseSubSteps(
  phase: string,
  currentActivity: string,
  iterations: number = 1
): PhaseSubStep[] {
  const phaseSteps: Record<string, string[]> = {
    'implement': [
      'Analyzing code structure',
      'Writing implementation',
      'Running tests',
      'Validating results'
    ],
    'spec': [
      'Analyzing requirements',
      'Writing specification',
      'Defining success criteria',
      'Creating test plan'
    ],
    'review': [
      'Code quality analysis',
      'Security review',
      'Documentation review',
      'Performance assessment'
    ],
    'test': [
      'Setting up test environment',
      'Running unit tests',
      'Running integration tests',
      'Generating coverage report'
    ],
    'breakdown': [
      'Analyzing scope',
      'Creating sub-tasks',
      'Defining dependencies',
      'Estimating effort'
    ]
  };

  const steps = phaseSteps[phase] || ['Processing...'];

  return steps.map((step, index) => {
    const stepName = step.toLowerCase();
    const isAnalyzing = currentActivity.includes('analyzing') && stepName.includes('analyzing');
    const isWriting = currentActivity.includes('writing') && stepName.includes('writing');
    const isRunning = currentActivity.includes('running') && stepName.includes('running');

    const inProgress = isAnalyzing || isWriting || isRunning;

    // Mark steps as completed based on current activity position
    let completed = false;
    if (currentActivity === 'running_tool' && index < 2) completed = true;
    if (currentActivity === 'spec_writing' && stepName.includes('analyzing')) completed = true;
    if (currentActivity === 'implement_testing' && index < 2) completed = true;

    const retryCount = iterations > 1 && inProgress ? iterations - 1 : undefined;

    return {
      step,
      completed,
      inProgress,
      retryCount
    };
  });
}