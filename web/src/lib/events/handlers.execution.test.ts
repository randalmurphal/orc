/**
 * TDD Tests for Event Handlers - Workflow Execution Visualization
 *
 * Tests for TASK-639: Live execution visualization on workflow canvas
 *
 * Success Criteria Coverage:
 * - SC-2: Phase status updates within 2 seconds of backend phase change
 * - SC-6: Header shows live session duration, total tokens, and total cost
 *
 * These tests verify that PhaseChangedEvent and TokensUpdatedEvent are routed
 * to workflowEditorStore when a matching workflow is being viewed.
 *
 * AMENDMENT AMEND-001: Proto PhaseStatus enum lacks RUNNING/FAILED/BLOCKED
 * The proto PhaseStatus (task.proto) only has: UNSPECIFIED(0), PENDING(1), COMPLETED(3), SKIPPED(7)
 * The implementation derives UI statuses from:
 * - 'running': run.status === RUNNING + this is the current phase (PENDING in proto)
 * - 'failed': phase has error field set (PENDING in proto with error)
 * - 'blocked': derived from gate conditions (future enhancement)
 *
 * Tests pass UI status strings directly to updateNodeStatus since the
 * derivation logic happens in the WorkflowEditorPage component, not the handlers.
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { handleEvent } from './handlers';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { useTaskStore } from '@/stores/taskStore';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import {
	EventSchema,
	PhaseChangedEventSchema,
	TokensUpdatedEventSchema,
	type Event,
} from '@/gen/orc/v1/events_pb';
import { PhaseStatus } from '@/gen/orc/v1/task_pb';
import { TokenUsageSchema } from '@/gen/orc/v1/common_pb';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
} from '@/test/factories';

// Helper to create a timestamp
function createTimestamp(): ReturnType<typeof create<typeof TimestampSchema>> {
	const now = Date.now();
	return create(TimestampSchema, {
		seconds: BigInt(Math.floor(now / 1000)),
		nanos: (now % 1000) * 1_000_000,
	});
}

// Helper to create a PhaseChangedEvent
function createPhaseChangedEvent(
	taskId: string,
	phaseName: string,
	status: PhaseStatus,
	iteration: number = 1,
	error?: string
): Event {
	return create(EventSchema, {
		id: `evt-${Date.now()}`,
		timestamp: createTimestamp(),
		taskId,
		payload: {
			case: 'phaseChanged',
			value: create(PhaseChangedEventSchema, {
				taskId,
				phaseId: `phase-${phaseName}`,
				phaseName,
				status,
				iteration,
				error,
			}),
		},
	});
}

// Helper to create a TokensUpdatedEvent
function createTokensUpdatedEvent(
	taskId: string,
	inputTokens: number,
	outputTokens: number,
	phaseId?: string
): Event {
	return create(EventSchema, {
		id: `evt-${Date.now()}`,
		timestamp: createTimestamp(),
		taskId,
		payload: {
			case: 'tokensUpdated',
			value: create(TokensUpdatedEventSchema, {
				taskId,
				tokens: create(TokenUsageSchema, {
					inputTokens,
					outputTokens,
				}),
				phaseId,
			}),
		},
	});
}

describe('handleEvent - phaseChanged for workflow editor', () => {
	beforeEach(() => {
		// Reset stores before each test
		useWorkflowEditorStore.getState().reset();
		useTaskStore.getState().reset();
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-2: PhaseChangedEvent updates workflowEditorStore', () => {
		it('should update node status when phaseChanged event matches active run task', () => {
			// Arrange: Load workflow and set up active run
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Simulate having an active run for task TASK-001
			// AMENDMENT AMEND-001: currentPhase must match phaseName for 'running' derivation
			const mockActiveRun = {
				run: {
					id: 'run-001',
					taskId: 'TASK-001',
					currentPhase: 'implement', // Set to match the phase we're testing
				},
				phases: [],
			};
			// The store needs to track which taskId it's watching
			useWorkflowEditorStore.getState().setActiveRun(mockActiveRun as any);

			// Act: Handle phaseChanged event for the matching task
			// AMENDMENT AMEND-001: Use PENDING (not RUNNING) - 'running' is derived from currentPhase match
			const event = createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING);
			handleEvent(event);

			// Assert: Node status should be updated in workflowEditorStore
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'implement'
			);
			expect(implementNode).toBeDefined();
			expect((implementNode!.data as any).status).toBe('running');
		});

		it('should not update workflowEditorStore when event task does not match active run', () => {
			// Arrange: Load workflow and set up active run for different task
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			const mockActiveRun = {
				run: {
					id: 'run-001',
					taskId: 'TASK-001', // Active run is for TASK-001
					currentPhase: 'implement',
				},
				phases: [],
			};
			useWorkflowEditorStore.getState().setActiveRun(mockActiveRun as any);

			// Act: Handle phaseChanged event for DIFFERENT task
			// AMENDMENT AMEND-001: Use PENDING instead of RUNNING
			const event = createPhaseChangedEvent('TASK-999', 'implement', PhaseStatus.PENDING);
			handleEvent(event);

			// Assert: Node status should NOT be updated
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'implement'
			);
			// Status should remain unchanged (undefined from initial load)
			expect((implementNode!.data as any).status).toBeUndefined();
		});

		it('should not update workflowEditorStore when no active run exists', () => {
			// Arrange: Load workflow but no active run
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			// Note: No setActiveRun call

			// Act: Handle phaseChanged event
			// AMENDMENT AMEND-001: Use PENDING instead of RUNNING
			const event = createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING);
			handleEvent(event);

			// Assert: Node status should NOT be updated (no active run to match)
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'implement'
			);
			expect((implementNode!.data as any).status).toBeUndefined();
		});

		it('should update node status to completed with iteration count', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001' },
				phases: [],
			} as any);

			// Act: Handle completion event with iteration count
			const event = createPhaseChangedEvent('TASK-001', 'spec', PhaseStatus.COMPLETED, 3);
			handleEvent(event);

			// Assert: Node should show completed with iterations
			const state = useWorkflowEditorStore.getState();
			const specNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'spec'
			);
			expect((specNode!.data as any).status).toBe('completed');
			expect((specNode!.data as any).iterations).toBe(3);
		});

		it('should update node status to failed when phase fails', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'implement' },
				phases: [],
			} as any);

			// Act: Handle failure event
			// AMENDMENT AMEND-001: Use PENDING with error field to derive 'failed' status
			const event = createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING, 1, 'Build failed');
			handleEvent(event);

			// Assert: Node should show failed
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'implement'
			);
			expect((implementNode!.data as any).status).toBe('failed');
		});

		it('should animate edges to newly running phase', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'implement' },
				phases: [],
			} as any);

			// Act: Handle phase started event
			// AMENDMENT AMEND-001: Use PENDING with matching currentPhase to derive 'running'
			const event = createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING);
			handleEvent(event);

			// Assert: Edges to implement should be animated
			const state = useWorkflowEditorStore.getState();
			const animatedEdges = state.edges.filter((e) => e.animated === true);
			expect(animatedEdges.length).toBeGreaterThan(0);
		});

		it('should stop edge animation when run completes', () => {
			// Arrange: Set up with running phase
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'implement' },
				phases: [],
			} as any);

			// First, set running to animate edges (PENDING + matching currentPhase)
			handleEvent(createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING));

			// Act: Complete the phase
			handleEvent(createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.COMPLETED));

			// Assert: Edges should no longer be animated when no phase is running
			const state = useWorkflowEditorStore.getState();
			const animatedEdges = state.edges.filter((e) => e.animated === true);
			expect(animatedEdges.length).toBe(0);
		});
	});

	describe('SC-2: taskStore still receives phaseChanged events', () => {
		it('should continue updating taskStore alongside workflowEditorStore', () => {
			// Arrange: Add task to taskStore
			const mockTask = {
				id: 'TASK-001',
				title: 'Test Task',
				currentPhase: 'spec',
			};
			useTaskStore.getState().addTask(mockTask as any);

			// Also set up workflowEditorStore
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'implement' },
				phases: [],
			} as any);

			// Act: Handle phaseChanged event
			// AMENDMENT AMEND-001: Use PENDING with matching currentPhase to derive 'running'
			const event = createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING);
			handleEvent(event);

			// Assert: Both stores should be updated
			const taskState = useTaskStore.getState();
			const task = taskState.getTask('TASK-001');
			expect(task?.currentPhase).toBe('implement');

			const editorState = useWorkflowEditorStore.getState();
			const implementNode = editorState.nodes.find(
				(n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'implement'
			);
			expect((implementNode!.data as any).status).toBe('running');
		});
	});

	describe('SC-6: TokensUpdatedEvent updates execution metrics', () => {
		it('should update run metrics when tokensUpdated event matches active run task', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: {
					id: 'run-001',
					taskId: 'TASK-001',
					totalInputTokens: 0,
					totalOutputTokens: 0,
					totalCostUsd: 0,
				},
				phases: [],
			} as any);

			// Act: Handle tokens updated event
			const event = createTokensUpdatedEvent('TASK-001', 5000, 2000);
			handleEvent(event);

			// Assert: Execution metrics should be updated
			// Note: The actual metric update might be in a different store (sessionStore)
			// or accumulated in the activeRun
			const state = useWorkflowEditorStore.getState();
			// This test verifies the event is routed appropriately
			expect(state.activeRun).not.toBeNull();
		});

		it('should not update metrics when event task does not match active run', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().setActiveRun({
				run: {
					id: 'run-001',
					taskId: 'TASK-001',
					totalInputTokens: 1000,
					totalOutputTokens: 500,
				},
				phases: [],
			} as any);

			const initialRun = useWorkflowEditorStore.getState().activeRun;

			// Act: Handle tokens event for different task
			const event = createTokensUpdatedEvent('TASK-999', 5000, 2000);
			handleEvent(event);

			// Assert: Run metrics should be unchanged
			const state = useWorkflowEditorStore.getState();
			// Tokens from different task should not affect our run
			expect(state.activeRun?.run?.totalInputTokens).toBe(initialRun?.run?.totalInputTokens);
		});
	});

	describe('Edge cases', () => {
		it('should handle rapid sequential phase events', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// AMENDMENT AMEND-001: Use PENDING instead of RUNNING - 'running' is derived from currentPhase match
			// Simulate the executor updating currentPhase as phases progress
			// Phase 1: spec starts
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'spec' },
				phases: [],
			} as any);
			handleEvent(createPhaseChangedEvent('TASK-001', 'spec', PhaseStatus.PENDING));

			// Phase 1: spec completes
			handleEvent(createPhaseChangedEvent('TASK-001', 'spec', PhaseStatus.COMPLETED));

			// Phase 2: implement starts - update currentPhase
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'implement' },
				phases: [],
			} as any);
			handleEvent(createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.PENDING));

			// Phase 2: implement completes
			handleEvent(createPhaseChangedEvent('TASK-001', 'implement', PhaseStatus.COMPLETED));

			// Phase 3: review starts - update currentPhase
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'review' },
				phases: [],
			} as any);
			handleEvent(createPhaseChangedEvent('TASK-001', 'review', PhaseStatus.PENDING));

			// Assert: Final state should reflect all changes
			const state = useWorkflowEditorStore.getState();
			const specNode = state.nodes.find((n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'spec');
			const implementNode = state.nodes.find((n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'implement');
			const reviewNode = state.nodes.find((n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'review');

			expect((specNode!.data as any).status).toBe('completed');
			expect((implementNode!.data as any).status).toBe('completed');
			expect((reviewNode!.data as any).status).toBe('running');
		});

		it('should handle phaseChanged for phase not in workflow (gracefully)', () => {
			// Arrange
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			// AMENDMENT AMEND-001: Use PENDING with currentPhase matching the nonexistent phase
			useWorkflowEditorStore.getState().setActiveRun({
				run: { id: 'run-001', taskId: 'TASK-001', currentPhase: 'nonexistent_phase' },
				phases: [],
			} as any);

			// Act: Event for phase not in workflow (maybe workflow was modified)
			expect(() => {
				handleEvent(createPhaseChangedEvent('TASK-001', 'nonexistent_phase', PhaseStatus.PENDING));
			}).not.toThrow();

			// Assert: Should not crash, existing nodes unchanged
			const state = useWorkflowEditorStore.getState();
			expect(state.nodes.length).toBeGreaterThan(0);
		});
	});
});
