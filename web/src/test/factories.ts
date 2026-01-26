/**
 * Test factories for creating proto-compatible mock objects.
 *
 * These factories use the proto Schema types to create properly typed
 * objects with $typeName set correctly.
 */

import { create } from '@bufbuild/protobuf';
import {
	TaskSchema,
	TaskPlanSchema,
	PlanPhaseSchema,
	type Task,
	type TaskPlan,
	type PlanPhase,
	TaskStatus,
	TaskWeight,
	TaskCategory,
	TaskPriority,
	TaskQueue,
	PhaseStatus,
	DependencyStatus,
} from '@/gen/orc/v1/task_pb';
import {
	InitiativeSchema,
	TaskRefSchema,
	type Initiative,
	type TaskRef,
	InitiativeStatus,
} from '@/gen/orc/v1/initiative_pb';
import {
	PendingDecisionSchema,
	DecisionOptionSchema,
	type PendingDecision,
	type DecisionOption,
} from '@/gen/orc/v1/decision_pb';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';

/**
 * Create a proto Timestamp from a date string or Date object
 */
export function createTimestamp(date: string | Date = new Date()): ReturnType<typeof create<typeof TimestampSchema>> {
	const d = typeof date === 'string' ? new Date(date) : date;
	const ms = d.getTime();
	return create(TimestampSchema, {
		seconds: BigInt(Math.floor(ms / 1000)),
		nanos: (ms % 1000) * 1_000_000,
	});
}

/**
 * Create a mock Task with proto-compatible types
 */
export function createMockTask(overrides: Partial<Omit<Task, '$typeName' | '$unknown'>> = {}): Task {
	const base = create(TaskSchema, {
		id: 'TASK-001',
		title: 'Test task',
		weight: TaskWeight.SMALL,
		status: TaskStatus.CREATED,
		branch: 'orc/TASK-001',
		priority: TaskPriority.NORMAL,
		category: TaskCategory.FEATURE,
		queue: TaskQueue.ACTIVE,
		dependencyStatus: DependencyStatus.NONE,
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
	});
	return Object.assign(base, overrides);
}

/**
 * Create a mock Initiative with proto-compatible types
 */
export function createMockInitiative(overrides: Partial<Omit<Initiative, '$typeName' | '$unknown'>> = {}): Initiative {
	const base = create(InitiativeSchema, {
		id: 'INIT-001',
		title: 'Test initiative',
		vision: 'Test vision',
		status: InitiativeStatus.ACTIVE,
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
	});
	return Object.assign(base, overrides);
}

/**
 * Create a mock TaskRef with proto-compatible types
 */
export function createMockTaskRef(overrides: Partial<Omit<TaskRef, '$typeName' | '$unknown'>> = {}): TaskRef {
	const base = create(TaskRefSchema, {
		id: 'TASK-001',
		title: 'Test task',
		status: TaskStatus.CREATED,
		dependsOn: [],
	});
	return Object.assign(base, overrides);
}

/**
 * Create a mock DecisionOption with proto-compatible types
 */
export function createMockDecisionOption(overrides: Partial<Omit<DecisionOption, '$typeName' | '$unknown'>> = {}): DecisionOption {
	const base = create(DecisionOptionSchema, {
		id: 'opt-1',
		label: 'Option 1',
		description: '',
		recommended: false,
	});
	return Object.assign(base, overrides);
}

/**
 * Create a mock PendingDecision with proto-compatible types
 */
export function createMockDecision(overrides: Partial<Omit<PendingDecision, '$typeName' | '$unknown'>> = {}): PendingDecision {
	// Create default options based on gate type
	const gateType = overrides.gateType ?? 'approval';
	const defaultOptions: DecisionOption[] = gateType === 'approval'
		? [
			createMockDecisionOption({ id: 'approve', label: 'Approve', recommended: true }),
			createMockDecisionOption({ id: 'reject', label: 'Reject' }),
		]
		: [
			createMockDecisionOption({ id: 'opt-1', label: 'Option 1', recommended: true }),
			createMockDecisionOption({ id: 'opt-2', label: 'Option 2' }),
		];

	const base = create(PendingDecisionSchema, {
		id: 'DEC-001',
		taskId: 'TASK-001',
		taskTitle: 'Test task',
		phase: 'implement',
		gateType: gateType,
		question: 'Ready to proceed?',
		options: defaultOptions,
		requestedAt: createTimestamp(),
	});
	return Object.assign(base, overrides);
}

/**
 * Create a mock TaskPlan with proto-compatible types
 */
export function createMockTaskPlan(overrides: Partial<Omit<TaskPlan, '$typeName' | '$unknown'>> = {}): TaskPlan {
	const base = create(TaskPlanSchema, {
		phases: [
			create(PlanPhaseSchema, {
				id: 'phase-1',
				name: 'implement',
				status: PhaseStatus.PENDING,
			}),
		],
	});
	return Object.assign(base, overrides);
}

/**
 * Create a mock PlanPhase with proto-compatible types
 */
export function createMockPhase(overrides: Partial<Omit<PlanPhase, '$typeName' | '$unknown'>> = {}): PlanPhase {
	const base = create(PlanPhaseSchema, {
		id: 'phase-1',
		name: 'implement',
		status: PhaseStatus.PENDING,
	});
	return Object.assign(base, overrides);
}

// Status helper functions
export function getStatusLabel(status: TaskStatus): string {
	switch (status) {
		case TaskStatus.CREATED: return 'created';
		case TaskStatus.CLASSIFYING: return 'classifying';
		case TaskStatus.PLANNED: return 'planned';
		case TaskStatus.RUNNING: return 'running';
		case TaskStatus.PAUSED: return 'paused';
		case TaskStatus.BLOCKED: return 'blocked';
		case TaskStatus.FINALIZING: return 'finalizing';
		case TaskStatus.COMPLETED: return 'completed';
		case TaskStatus.FAILED: return 'failed';
		case TaskStatus.RESOLVED: return 'resolved';
		default: return 'created';
	}
}

export function getWeightLabel(weight: TaskWeight): string {
	switch (weight) {
		case TaskWeight.TRIVIAL: return 'trivial';
		case TaskWeight.SMALL: return 'small';
		case TaskWeight.MEDIUM: return 'medium';
		case TaskWeight.LARGE: return 'large';
		default: return 'medium';
	}
}

export function getCategoryLabel(category: TaskCategory): string {
	switch (category) {
		case TaskCategory.FEATURE: return 'feature';
		case TaskCategory.BUG: return 'bug';
		case TaskCategory.REFACTOR: return 'refactor';
		case TaskCategory.CHORE: return 'chore';
		case TaskCategory.DOCS: return 'docs';
		case TaskCategory.TEST: return 'test';
		default: return 'feature';
	}
}
