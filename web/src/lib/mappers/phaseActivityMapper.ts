// Phase activity mapping utilities

export interface ActivityProgress {
  currentStep: number;
  progressWithinStep: number;
}

/**
 * Maps current activity to progress within a phase
 */
export function mapActivityToProgress(
  phase: string,
  currentActivity: string
): ActivityProgress {
  const phaseActivitySteps: Record<string, Record<string, { step: number; progress: number }>> = {
    'implement': {
      'waiting_api': { step: 0, progress: 0.1 },
      'streaming': { step: 0, progress: 0.5 },
      'spec_analyzing': { step: 0, progress: 0.2 },
      'running_tool': { step: 2, progress: 0.75 },
      'implement_writing': { step: 1, progress: 0.8 },
      'implement_testing': { step: 2, progress: 0.6 },
      'idle': { step: 3, progress: 0.9 }
    },
    'spec': {
      'spec_analyzing': { step: 0, progress: 0.7 },
      'spec_writing': { step: 1, progress: 0.6 },
      'running_tool': { step: 2, progress: 0.8 },
      'streaming': { step: 1, progress: 0.4 },
      'idle': { step: 3, progress: 0.9 }
    },
    'review': {
      'spec_analyzing': { step: 0, progress: 0.8 },
      'review_analyzing': { step: 1, progress: 0.7 },
      'review_feedback': { step: 2, progress: 0.9 },
      'streaming': { step: 1, progress: 0.5 },
      'idle': { step: 3, progress: 0.9 }
    },
    'test': {
      'running_tool': { step: 1, progress: 0.6 },
      'implement_testing': { step: 2, progress: 0.8 },
      'streaming': { step: 0, progress: 0.3 },
      'idle': { step: 3, progress: 0.9 }
    }
  };

  const mapping = phaseActivitySteps[phase]?.[currentActivity] || { step: 0, progress: 0.5 };

  return {
    currentStep: mapping.step,
    progressWithinStep: mapping.progress
  };
}

/**
 * Gets a human-readable description of the current activity
 */
export function getActivityDescription(
  phase: string,
  currentActivity: string
): string {
  const activityDescriptions: Record<string, Record<string, string>> = {
    'implement': {
      'waiting_api': 'Waiting for API response...',
      'streaming': 'Receiving implementation instructions...',
      'spec_analyzing': 'Analyzing implementation requirements...',
      'running_tool': 'Running implementation tests...',
      'implement_writing': 'Writing implementation code...',
      'implement_testing': 'Testing implementation...',
      'idle': 'Implementation ready for review'
    },
    'spec': {
      'spec_analyzing': 'Analyzing task requirements...',
      'spec_writing': 'Writing technical specification...',
      'running_tool': 'Validating specification...',
      'streaming': 'Refining specification details...',
      'idle': 'Specification complete'
    },
    'review': {
      'spec_analyzing': 'Analyzing code for review...',
      'review_analyzing': 'Performing code quality analysis...',
      'review_feedback': 'Generating review feedback...',
      'streaming': 'Processing review comments...',
      'idle': 'Review complete'
    },
    'test': {
      'running_tool': 'Executing test suite...',
      'implement_testing': 'Running comprehensive tests...',
      'streaming': 'Collecting test results...',
      'idle': 'Testing complete'
    },
    'breakdown': {
      'spec_analyzing': 'Analyzing task complexity...',
      'spec_writing': 'Creating task breakdown...',
      'running_tool': 'Validating breakdown structure...',
      'streaming': 'Refining sub-tasks...',
      'idle': 'Breakdown complete'
    }
  };

  const description = activityDescriptions[phase]?.[currentActivity]
    || activityDescriptions['implement'][currentActivity]
    || `Processing ${phase} phase...`;

  return description;
}

/**
 * Gets the phase display name
 */
export function getPhaseDisplayName(phase: string): string {
  const phaseNames: Record<string, string> = {
    'implement': 'Implementation',
    'spec': 'Specification',
    'tiny_spec': 'Quick Spec',
    'review': 'Review',
    'test': 'Testing',
    'tdd_write': 'TDD Write',
    'breakdown': 'Breakdown',
    'docs': 'Documentation'
  };

  return phaseNames[phase] || phase.charAt(0).toUpperCase() + phase.slice(1);
}