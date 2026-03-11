/**
 * TDD Tests for Event Handlers - TASK-555
 *
 * Tests for bug fix: Board doesn't update in real-time after creating task from modal
 *
 * These tests verify that event handlers properly update the Zustand stores
 * when real-time events are received.
 *
 * Success Criteria Coverage:
 * - SC-1: taskCreated event should add task to store (or trigger fetch)
 * - SC-2: taskCreated event should not create duplicates if task already exists
 * - SC-3: Other event types (taskUpdated, taskDeleted) should continue to work
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { handleEvent } from './handlers';
import { onRecommendationSignal } from './recommendationSignals';
import { onAttentionDashboardSignal } from './attentionDashboardSignals';
import { useTaskStore } from '@/stores/taskStore';
import { useInitiativeStore } from '@/stores/initiativeStore';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import {
	DecisionRequiredEventSchema,
	DecisionResolvedEventSchema,
	EventSchema,
	RecommendationCreatedEventSchema,
	TaskCreatedEventSchema,
	TaskDeletedEventSchema,
	TaskUpdatedEventSchema,
	InitiativeCreatedEventSchema,
	type Event,
} from '@/gen/orc/v1/events_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { RecommendationKind, RecommendationStatus } from '@/gen/orc/v1/recommendation_pb';
import { createMockTask } from '@/test/factories';

// Helper to create a timestamp
function createTimestamp(): ReturnType<typeof create<typeof TimestampSchema>> {
	const now = Date.now();
	return create(TimestampSchema, {
		seconds: BigInt(Math.floor(now / 1000)),
		nanos: (now % 1000) * 1_000_000,
	});
}

// Helper to create a taskCreated event
function createTaskCreatedEvent(taskId: string, title: string, initiativeId?: string): Event {
	return create(EventSchema, {
		id: `evt-${Date.now()}`,
		timestamp: createTimestamp(),
		taskId,
		payload: {
			case: 'taskCreated',
			value: create(TaskCreatedEventSchema, {
				taskId,
				title,
				initiativeId,
			}),
		},
	});
}

// Helper to create a taskUpdated event
function createTaskUpdatedEvent(taskId: string, task: ReturnType<typeof createMockTask>, changedFields: string[] = []): Event {
	return create(EventSchema, {
		id: `evt-${Date.now()}`,
		timestamp: createTimestamp(),
		taskId,
		payload: {
			case: 'taskUpdated',
			value: create(TaskUpdatedEventSchema, {
				taskId,
				task,
				changedFields,
			}),
		},
	});
}

// Helper to create a taskDeleted event
function createTaskDeletedEvent(taskId: string): Event {
	return create(EventSchema, {
		id: `evt-${Date.now()}`,
		timestamp: createTimestamp(),
		taskId,
		payload: {
			case: 'taskDeleted',
			value: {
				$typeName: 'orc.v1.TaskDeletedEvent',
				taskId,
			},
		},
	});
}

describe('handleEvent - taskCreated', () => {
	beforeEach(() => {
		// Reset stores before each test
		useTaskStore.getState().reset();
		useInitiativeStore.getState().reset();
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-1: taskCreated event should add task to store', () => {
		it('should add a new task to the store when taskCreated event is received', () => {
			// Arrange: Store starts empty
			expect(useTaskStore.getState().tasks).toHaveLength(0);

			// Act: Handle a taskCreated event
			const event = createTaskCreatedEvent('TASK-001', 'New Task Title');
			handleEvent(event);

			// Assert: Task should be in the store
			const tasks = useTaskStore.getState().tasks;
			expect(tasks).toHaveLength(1);
			expect(tasks[0].id).toBe('TASK-001');
			expect(tasks[0].title).toBe('New Task Title');
		});

		it('should add task with correct initial status (CREATED or PLANNED)', () => {
			// Arrange
			const event = createTaskCreatedEvent('TASK-002', 'Another Task');

			// Act
			handleEvent(event);

			// Assert: New tasks should have CREATED or PLANNED status
			const task = useTaskStore.getState().getTask('TASK-002');
			expect(task).toBeDefined();
			// Accept either CREATED or PLANNED as valid initial statuses
			expect([TaskStatus.CREATED, TaskStatus.PLANNED]).toContain(task!.status);
		});

		it('should include initiative_id when provided in the event', () => {
			// Arrange
			const event = createTaskCreatedEvent('TASK-003', 'Initiative Task', 'INIT-001');

			// Act
			handleEvent(event);

			// Assert
			const task = useTaskStore.getState().getTask('TASK-003');
			expect(task).toBeDefined();
			expect(task!.initiativeId).toBe('INIT-001');
		});

		it('should handle task without initiative_id', () => {
			// Arrange
			const event = createTaskCreatedEvent('TASK-004', 'Standalone Task');

			// Act
			handleEvent(event);

			// Assert
			const task = useTaskStore.getState().getTask('TASK-004');
			expect(task).toBeDefined();
			expect(task!.initiativeId).toBeUndefined();
		});
	});

	describe('SC-2: taskCreated event should not create duplicates', () => {
		it('should not add duplicate task if task already exists in store', () => {
			// Arrange: Task already exists (e.g., from API response)
			const existingTask = createMockTask({
				id: 'TASK-005',
				title: 'Existing Task',
				status: TaskStatus.CREATED,
			});
			useTaskStore.getState().addTask(existingTask);
			expect(useTaskStore.getState().tasks).toHaveLength(1);

			// Act: Receive taskCreated event for same task
			const event = createTaskCreatedEvent('TASK-005', 'Existing Task');
			handleEvent(event);

			// Assert: Should still have only one task
			expect(useTaskStore.getState().tasks).toHaveLength(1);
		});

		it('should not overwrite existing task data with partial event data', () => {
			// Arrange: Task already exists with full data
			const existingTask = createMockTask({
				id: 'TASK-006',
				title: 'Full Task',
				description: 'This has a description',
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});
			useTaskStore.getState().addTask(existingTask);

			// Act: Receive taskCreated event (which has minimal data)
			const event = createTaskCreatedEvent('TASK-006', 'Full Task');
			handleEvent(event);

			// Assert: Original data should be preserved
			const task = useTaskStore.getState().getTask('TASK-006');
			expect(task?.description).toBe('This has a description');
			expect(task?.status).toBe(TaskStatus.RUNNING);
			expect(task?.currentPhase).toBe('implement');
		});
	});

	describe('SC-3: Other event types should continue to work', () => {
		it('should update task when taskUpdated event is received', () => {
			// Arrange: Task exists in store
			const existingTask = createMockTask({
				id: 'TASK-007',
				title: 'Original Title',
				status: TaskStatus.PLANNED,
			});
			useTaskStore.getState().addTask(existingTask);

			// Create updated task
			const updatedTask = createMockTask({
				id: 'TASK-007',
				title: 'Original Title',
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});

			// Act: Handle taskUpdated event
			const event = createTaskUpdatedEvent('TASK-007', updatedTask, ['status', 'currentPhase']);
			handleEvent(event);

			// Assert
			const task = useTaskStore.getState().getTask('TASK-007');
			expect(task?.status).toBe(TaskStatus.RUNNING);
			expect(task?.currentPhase).toBe('implement');
		});

		it('should remove task when taskDeleted event is received', () => {
			// Arrange
			const existingTask = createMockTask({ id: 'TASK-008', title: 'To Delete' });
			useTaskStore.getState().addTask(existingTask);
			expect(useTaskStore.getState().tasks).toHaveLength(1);

			// Act
			const event = createTaskDeletedEvent('TASK-008');
			handleEvent(event);

			// Assert
			expect(useTaskStore.getState().tasks).toHaveLength(0);
		});
	});

	describe('Edge cases', () => {
		it('should handle multiple rapid taskCreated events for different tasks', () => {
			// Arrange & Act: Send multiple events in rapid succession
			handleEvent(createTaskCreatedEvent('TASK-A', 'Task A'));
			handleEvent(createTaskCreatedEvent('TASK-B', 'Task B'));
			handleEvent(createTaskCreatedEvent('TASK-C', 'Task C'));

			// Assert: All tasks should be in the store
			const tasks = useTaskStore.getState().tasks;
			expect(tasks).toHaveLength(3);
			expect(tasks.map(t => t.id).sort()).toEqual(['TASK-A', 'TASK-B', 'TASK-C']);
		});

	});
});

describe('handleEvent - initiativeCreated', () => {
	beforeEach(() => {
		useInitiativeStore.getState().reset();
		vi.clearAllMocks();
	});

	it('should add a new initiative to the store when initiativeCreated event is received', () => {
		// Arrange: Store starts empty (initiatives is a Map)
		expect(useInitiativeStore.getState().initiatives.size).toBe(0);

		// Create initiativeCreated event
		const event = create(EventSchema, {
			id: `evt-${Date.now()}`,
			timestamp: createTimestamp(),
			payload: {
				case: 'initiativeCreated',
				value: create(InitiativeCreatedEventSchema, {
					initiativeId: 'INIT-001',
					title: 'New Initiative',
				}),
			},
		});

		// Act
		handleEvent(event);

		// Assert: Initiative should be in the store
		// Note: This test will fail because the current handler only logs
		// The implementation should add the initiative to the store
		const initiatives = useInitiativeStore.getState().initiatives;
		expect(initiatives.size).toBe(1);
		expect(initiatives.get('INIT-001')).toBeDefined();
		expect(initiatives.get('INIT-001')?.title).toBe('New Initiative');
	});
});

describe('handleEvent - recommendation events', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('emits a recommendation signal for recommendationCreated events', () => {
		const received: Array<{ projectId: string; recommendationId: string; type: string }> = [];
		const unsubscribe = onRecommendationSignal((signal) => {
			received.push(signal);
		});
		const event = create(EventSchema, {
			id: 'evt-rec-created',
			timestamp: createTimestamp(),
			projectId: 'proj-001',
			payload: {
				case: 'recommendationCreated',
				value: create(RecommendationCreatedEventSchema, {
					recommendationId: 'REC-001',
					kind: RecommendationKind.CLEANUP,
					status: RecommendationStatus.PENDING,
					title: 'Cleanup duplicate polling',
					summary: 'Two loops are doing the same work.',
					sourceTaskId: 'TASK-001',
					sourceRunId: 'RUN-001',
					sourceThreadId: 'THR-001',
				}),
			},
		});

		handleEvent(event);
		unsubscribe();

		expect(received).toEqual([
			{
				projectId: 'proj-001',
				recommendationId: 'REC-001',
				type: 'created',
			},
		]);
	});
});

describe('handleEvent - attention dashboard signals', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('emits a dashboard signal for taskCreated events', () => {
		const received: Array<{ projectId: string; taskId?: string; type: string }> = [];
		const unsubscribe = onAttentionDashboardSignal((signal) => {
			received.push(signal);
		});
		const event = create(EventSchema, {
			id: 'evt-task-created',
			timestamp: createTimestamp(),
			projectId: 'proj-001',
			taskId: 'TASK-199',
			payload: {
				case: 'taskCreated',
				value: create(TaskCreatedEventSchema, {
					taskId: 'TASK-199',
					title: 'Created task',
					initiativeId: 'INIT-001',
				}),
			},
		});

		handleEvent(event);
		unsubscribe();

		expect(received).toEqual([
			{
				projectId: 'proj-001',
				taskId: 'TASK-199',
				type: 'task-updated',
			},
		]);
	});

	it('emits a dashboard signal for taskUpdated events', () => {
		const received: Array<{ projectId: string; taskId?: string; type: string }> = [];
		const unsubscribe = onAttentionDashboardSignal((signal) => {
			received.push(signal);
		});
		const updatedTask = createMockTask({
			id: 'TASK-200',
			title: 'Update me',
		});
		const event = create(EventSchema, {
			id: 'evt-task-updated',
			timestamp: createTimestamp(),
			projectId: 'proj-001',
			taskId: 'TASK-200',
			payload: {
				case: 'taskUpdated',
				value: create(TaskUpdatedEventSchema, {
					taskId: 'TASK-200',
					task: updatedTask,
					changedFields: ['status'],
				}),
			},
		});

		handleEvent(event);
		unsubscribe();

		expect(received).toEqual([
			{
				projectId: 'proj-001',
				taskId: 'TASK-200',
				type: 'task-updated',
			},
		]);
	});

	it('emits a dashboard signal for taskDeleted events', () => {
		const received: Array<{ projectId: string; taskId?: string; type: string }> = [];
		const unsubscribe = onAttentionDashboardSignal((signal) => {
			received.push(signal);
		});
		const event = create(EventSchema, {
			id: 'evt-task-deleted',
			timestamp: createTimestamp(),
			projectId: 'proj-001',
			payload: {
				case: 'taskDeleted',
				value: create(TaskDeletedEventSchema, {
					taskId: 'TASK-200',
				}),
			},
		});

		handleEvent(event);
		unsubscribe();

		expect(received).toEqual([
			{
				projectId: 'proj-001',
				taskId: 'TASK-200',
				type: 'task-updated',
			},
		]);
	});

	it('emits a dashboard signal for decisionRequired events', () => {
		const received: Array<{ projectId: string; taskId?: string; type: string }> = [];
		const unsubscribe = onAttentionDashboardSignal((signal) => {
			received.push(signal);
		});
		const event = create(EventSchema, {
			id: 'evt-decision-required',
			timestamp: createTimestamp(),
			projectId: 'proj-001',
			payload: {
				case: 'decisionRequired',
				value: create(DecisionRequiredEventSchema, {
					decisionId: 'DEC-001',
					taskId: 'TASK-201',
					taskTitle: 'Needs a call',
					phase: 'review',
					gateType: 'human',
					question: 'Ship it?',
					context: 'Needs approval',
					requestedAt: createTimestamp(),
				}),
			},
		});

		handleEvent(event);
		unsubscribe();

		expect(received).toEqual([
			{
				projectId: 'proj-001',
				taskId: 'TASK-201',
				type: 'decision-required',
			},
		]);
	});

	it('emits a dashboard signal for decisionResolved events', () => {
		const received: Array<{ projectId: string; taskId?: string; type: string }> = [];
		const unsubscribe = onAttentionDashboardSignal((signal) => {
			received.push(signal);
		});
		const event = create(EventSchema, {
			id: 'evt-decision-resolved',
			timestamp: createTimestamp(),
			projectId: 'proj-001',
			payload: {
				case: 'decisionResolved',
				value: create(DecisionResolvedEventSchema, {
					decisionId: 'DEC-001',
					taskId: 'TASK-201',
					approved: true,
					resolvedAt: createTimestamp(),
				}),
			},
		});

		handleEvent(event);
		unsubscribe();

		expect(received).toEqual([
			{
				projectId: 'proj-001',
				taskId: 'TASK-201',
				type: 'decision-resolved',
			},
		]);
	});
});
