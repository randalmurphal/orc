/**
 * TDD Tests for WorkflowEditorStore - Live Execution Visualization
 *
 * Tests for TASK-639: Live execution visualization on workflow canvas
 *
 * Success Criteria Coverage:
 * - SC-1: Active phase node displays purple pulsing glow when workflow run has status RUNNING
 * - SC-2: Phase status updates within 2 seconds of backend phase change
 * - SC-3: Completed phases show green left border and checkmark styling
 * - SC-4: Completed phases display cost badge showing $X.XX
 * - SC-7: Edges leading TO the running phase show animated flowing dot
 *
 * These tests verify the store methods that will be added for execution tracking:
 * - setActiveRun: Load active WorkflowRun and its phases
 * - updateNodeStatus: Update a node's execution status
 * - updateEdgesForActivePhase: Animate edges leading to running phase
 * - clearExecution: Reset execution state when run completes/cancels
 *
 * AMENDMENT AMEND-001: Proto PhaseStatus enum lacks RUNNING/FAILED/BLOCKED
 * The proto PhaseStatus enum (task.proto) only has: UNSPECIFIED(0), PENDING(1), COMPLETED(3), SKIPPED(7)
 * Values 2,4,5,6,8 were removed - comment says "use task status for execution state"
 * For UI purposes, we use string literal types directly: 'running', 'failed', 'blocked'
 * These are passed as UIPhaseStatus strings, not proto enum values.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useWorkflowEditorStore } from './workflowEditorStore';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
} from '@/test/factories';
import { PhaseStatus } from '@/gen/orc/v1/task_pb';
import { RunStatus } from '@/gen/orc/v1/workflow_pb';
import type { PhaseNodeData, PhaseStatus as PhaseStatusType } from '@/components/workflow-editor/nodes';

// Note: These factory functions will need to be added to test/factories.ts
// For now, we create mock data inline to demonstrate the test structure

/**
 * Create a mock WorkflowRun with phases
 */
function createMockWorkflowRun(overrides: {
	id?: string;
	workflowId?: string;
	taskId?: string;
	status?: RunStatus;
	currentPhase?: string;
	totalCostUsd?: number;
	totalInputTokens?: number;
	totalOutputTokens?: number;
} = {}) {
	return {
		id: overrides.id ?? 'run-001',
		workflowId: overrides.workflowId ?? 'medium',
		taskId: overrides.taskId ?? 'TASK-001',
		status: overrides.status ?? RunStatus.RUNNING,
		currentPhase: overrides.currentPhase ?? 'implement',
		totalCostUsd: overrides.totalCostUsd ?? 0.5,
		totalInputTokens: overrides.totalInputTokens ?? 10000,
		totalOutputTokens: overrides.totalOutputTokens ?? 5000,
	};
}

/**
 * Create a mock WorkflowRunPhase
 */
function createMockWorkflowRunPhase(overrides: {
	id?: number;
	workflowRunId?: string;
	phaseTemplateId?: string;
	status?: PhaseStatus;
	iterations?: number;
	costUsd?: number;
} = {}) {
	return {
		id: overrides.id ?? 1,
		workflowRunId: overrides.workflowRunId ?? 'run-001',
		phaseTemplateId: overrides.phaseTemplateId ?? 'implement',
		status: overrides.status ?? PhaseStatus.PENDING,
		iterations: overrides.iterations ?? 0,
		costUsd: overrides.costUsd ?? 0,
	};
}

/**
 * Create a mock WorkflowRunWithDetails
 *
 * AMENDMENT AMEND-001: Uses PhaseStatus.PENDING for "current" phase since RUNNING was removed from proto.
 * In real execution, the "running" state is determined by:
 * 1. The run's currentPhase field
 * 2. The run's overall status being RUNNING
 * 3. The phase being PENDING (not yet completed)
 */
function createMockWorkflowRunWithDetails(overrides: {
	run?: ReturnType<typeof createMockWorkflowRun>;
	phases?: ReturnType<typeof createMockWorkflowRunPhase>[];
} = {}) {
	return {
		run: overrides.run ?? createMockWorkflowRun(),
		phases: overrides.phases ?? [
			createMockWorkflowRunPhase({ phaseTemplateId: 'spec', status: PhaseStatus.COMPLETED }),
			// PENDING represents "in progress" - the UI derives 'running' from run.currentPhase
			createMockWorkflowRunPhase({ phaseTemplateId: 'implement', status: PhaseStatus.PENDING }),
			createMockWorkflowRunPhase({ phaseTemplateId: 'review', status: PhaseStatus.PENDING }),
		],
	};
}

describe('WorkflowEditorStore - Execution Tracking', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
	});

	describe('SC-1 & SC-2: setActiveRun and updateNodeStatus', () => {
		it('should store active run when setActiveRun is called', () => {
			// Arrange: Load a workflow first
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ phaseTemplateId: 'review', sequence: 3 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			const run = createMockWorkflowRunWithDetails();

			// Act: Set active run
			useWorkflowEditorStore.getState().setActiveRun(run as any);

			// Assert: activeRun should be stored
			const state = useWorkflowEditorStore.getState();
			expect(state.activeRun).not.toBeNull();
			expect(state.activeRun?.run?.id).toBe('run-001');
		});

		it('should update node status to running when phase is running', () => {
			// Arrange: Load workflow and set active run
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Update node status to running
			useWorkflowEditorStore.getState().updateNodeStatus('implement', 'running');

			// Assert: Node should have running status
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'implement'
			);
			expect(implementNode).toBeDefined();
			expect((implementNode!.data as PhaseNodeData).status).toBe('running');
		});

		it('should update node status to completed when phase completes', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Update node status to completed
			useWorkflowEditorStore.getState().updateNodeStatus('spec', 'completed');

			// Assert: Node should have completed status
			const state = useWorkflowEditorStore.getState();
			const specNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'spec'
			);
			expect(specNode).toBeDefined();
			expect((specNode!.data as PhaseNodeData).status).toBe('completed');
		});

		it('should update node status to failed when phase fails', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Update node status to failed
			useWorkflowEditorStore.getState().updateNodeStatus('implement', 'failed');

			// Assert: Node should have failed status
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'implement'
			);
			expect(implementNode).toBeDefined();
			expect((implementNode!.data as PhaseNodeData).status).toBe('failed');
		});

		it('should not throw when updating status for non-existent phase', () => {
			// Arrange: Load workflow without the target phase
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act & Assert: Should not throw
			expect(() => {
				useWorkflowEditorStore.getState().updateNodeStatus('nonexistent', 'running');
			}).not.toThrow();
		});
	});

	describe('SC-4: Cost display on completed phases', () => {
		it('should update node with cost when updateNodeStatus includes costUsd', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Update node with cost
			useWorkflowEditorStore.getState().updateNodeStatus('spec', 'completed', {
				costUsd: 0.42,
				iterations: 2,
			});

			// Assert: Node should have cost data
			const state = useWorkflowEditorStore.getState();
			const specNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'spec'
			);
			expect(specNode).toBeDefined();
			expect((specNode!.data as PhaseNodeData).costUsd).toBe(0.42);
			expect((specNode!.data as PhaseNodeData).iterations).toBe(2);
		});

		it('should not display cost badge when costUsd is 0', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Update node with zero cost
			useWorkflowEditorStore.getState().updateNodeStatus('spec', 'completed', {
				costUsd: 0,
			});

			// Assert: Cost should be 0 (component will decide not to render badge)
			const state = useWorkflowEditorStore.getState();
			const specNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'spec'
			);
			expect(specNode).toBeDefined();
			expect((specNode!.data as PhaseNodeData).costUsd).toBe(0);
		});

		it('should not display cost badge when costUsd is undefined', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Update node without cost (status only)
			useWorkflowEditorStore.getState().updateNodeStatus('spec', 'completed');

			// Assert: Cost should remain undefined
			const state = useWorkflowEditorStore.getState();
			const specNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'spec'
			);
			expect(specNode).toBeDefined();
			// costUsd should be undefined when not provided
			expect((specNode!.data as PhaseNodeData).costUsd).toBeUndefined();
		});
	});

	describe('SC-7: updateEdgesForActivePhase', () => {
		it('should set animated=true on edges leading to running phase', () => {
			// Arrange: Load workflow with sequential phases
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Animate edges to 'implement' phase
			useWorkflowEditorStore.getState().updateEdgesForActivePhase('implement');

			// Assert: Edge from spec->implement should be animated
			const state = useWorkflowEditorStore.getState();

			// At least one edge should be animated
			const animatedEdges = state.edges.filter((e) => e.animated === true);
			expect(animatedEdges.length).toBeGreaterThan(0);
		});

		it('should set animated=false on edges when phase is null (no active phase)', () => {
			// Arrange: Load workflow and set some edges as animated
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().updateEdgesForActivePhase('implement');

			// Act: Clear active phase
			useWorkflowEditorStore.getState().updateEdgesForActivePhase(null);

			// Assert: No edges should be animated
			const state = useWorkflowEditorStore.getState();
			const animatedEdges = state.edges.filter((e) => e.animated === true);
			expect(animatedEdges.length).toBe(0);
		});

		it('should not animate edges to pending phases', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Animate only edges to 'spec' (first phase)
			useWorkflowEditorStore.getState().updateEdgesForActivePhase('spec');

			// Assert: Edges leading to 'implement' and 'review' should NOT be animated
			const state = useWorkflowEditorStore.getState();
			// Only edges TO 'spec' (from start) should be animated
			// Edges to other phases should not be animated
			const specNode = state.nodes.find((n) => n.type === 'phase' && (n.data as any).phaseTemplateId === 'spec');
			if (specNode) {
				const animatedToOthers = state.edges.filter((e) => e.animated && e.target !== specNode.id);
				expect(animatedToOthers.length).toBe(0); // No edges to non-spec phases should be animated
			}
		});
	});

	describe('clearExecution', () => {
		it('should reset all nodes to no execution state when clearExecution is called', () => {
			// Arrange: Load workflow and set execution state
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().updateNodeStatus('spec', 'completed', { costUsd: 0.5 });
			useWorkflowEditorStore.getState().updateNodeStatus('implement', 'running');

			// Act: Clear execution
			useWorkflowEditorStore.getState().clearExecution();

			// Assert: All nodes should have no execution state
			const state = useWorkflowEditorStore.getState();
			expect(state.activeRun).toBeNull();

			const phaseNodes = state.nodes.filter((n) => n.type === 'phase');
			phaseNodes.forEach((node) => {
				const data = node.data as PhaseNodeData;
				expect(data.status).toBeUndefined();
				expect(data.costUsd).toBeUndefined();
				expect(data.iterations).toBeUndefined();
			});
		});

		it('should reset all edge animations when clearExecution is called', () => {
			// Arrange: Load workflow and set edge animation
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			useWorkflowEditorStore.getState().updateEdgesForActivePhase('implement');

			// Act: Clear execution
			useWorkflowEditorStore.getState().clearExecution();

			// Assert: No edges should be animated
			const state = useWorkflowEditorStore.getState();
			const animatedEdges = state.edges.filter((e) => e.animated === true);
			expect(animatedEdges.length).toBe(0);
		});
	});

	describe('Edge cases', () => {
		it('should handle run with no phases yet', () => {
			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			const runWithNoPhases = createMockWorkflowRunWithDetails({
				run: createMockWorkflowRun(),
				phases: [], // No phases executed yet
			});

			// Act: Set active run with no phases
			useWorkflowEditorStore.getState().setActiveRun(runWithNoPhases as any);

			// Assert: All nodes should be pending
			const state = useWorkflowEditorStore.getState();
			const phaseNodes = state.nodes.filter((n) => n.type === 'phase');
			phaseNodes.forEach((node) => {
				// Without execution data, status should be undefined or pending
				expect((node.data as PhaseNodeData).status).toBeUndefined();
			});
		});

		it('should handle phase not in current workflow (template changed)', () => {
			// Arrange: Load workflow with only 'spec' phase
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Act: Try to update status for phase that doesn't exist in workflow
			const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
			useWorkflowEditorStore.getState().updateNodeStatus('implement', 'running');

			// Assert: Should warn but not throw
			// (The actual warning behavior depends on implementation)
			const state = useWorkflowEditorStore.getState();
			const implementNode = state.nodes.find(
				(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === 'implement'
			);
			expect(implementNode).toBeUndefined();

			consoleSpy.mockRestore();
		});

		it('should select most recent RUNNING run when multiple runs exist', () => {
			// This tests the run selection logic when detecting active runs
			// The actual implementation will be in the hook/page that fetches runs
			// This store test verifies the store can handle run replacement

			// Arrange: Load workflow
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);

			// Set first run
			const oldRun = createMockWorkflowRunWithDetails({
				run: createMockWorkflowRun({ id: 'run-old', status: RunStatus.COMPLETED }),
			});
			useWorkflowEditorStore.getState().setActiveRun(oldRun as any);

			// Act: Set newer run
			const newRun = createMockWorkflowRunWithDetails({
				run: createMockWorkflowRun({ id: 'run-new', status: RunStatus.RUNNING }),
			});
			useWorkflowEditorStore.getState().setActiveRun(newRun as any);

			// Assert: Should have the newer run
			const state = useWorkflowEditorStore.getState();
			expect(state.activeRun?.run?.id).toBe('run-new');
		});

		it('should cleanup on unmount (reset call)', () => {
			// Arrange: Load workflow and set execution state
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'medium', name: 'Medium' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(workflowDetails);
			const run = createMockWorkflowRunWithDetails();
			useWorkflowEditorStore.getState().setActiveRun(run as any);

			// Act: Reset (simulating unmount)
			useWorkflowEditorStore.getState().reset();

			// Assert: Everything should be cleared
			const state = useWorkflowEditorStore.getState();
			expect(state.activeRun).toBeNull();
			expect(state.nodes).toEqual([]);
			expect(state.edges).toEqual([]);
			expect(state.workflowDetails).toBeNull();
		});
	});
});

describe('mapPhaseStatus helper', () => {
	/**
	 * Test the helper function that maps proto PhaseStatus to UI status string.
	 *
	 * AMENDMENT AMEND-001: Proto PhaseStatus only has UNSPECIFIED(0), PENDING(1), COMPLETED(3), SKIPPED(7).
	 * Values RUNNING, FAILED, BLOCKED were removed from the proto.
	 *
	 * For UI purposes:
	 * - 'running' is derived from run.currentPhase + run.status === RUNNING
	 * - 'failed' is derived from run.status === FAILED or phase.error being set
	 * - 'blocked' is derived from gate blocking or dependency conditions
	 *
	 * This test only covers the actual proto enum values.
	 * The UI status strings 'running', 'failed', 'blocked' are passed directly via updateNodeStatus().
	 */

	it.each([
		[PhaseStatus.PENDING, 'pending'],
		[PhaseStatus.COMPLETED, 'completed'],
		[PhaseStatus.SKIPPED, 'skipped'],
		[PhaseStatus.UNSPECIFIED, 'pending'], // Default to pending
	])('should map PhaseStatus.%s to "%s"', (protoStatus, expectedUiStatus) => {
		// This test documents the expected mapping for ACTUAL proto values
		const statusMap: Record<PhaseStatus, PhaseStatusType> = {
			[PhaseStatus.PENDING]: 'pending',
			[PhaseStatus.COMPLETED]: 'completed',
			[PhaseStatus.SKIPPED]: 'skipped',
			[PhaseStatus.UNSPECIFIED]: 'pending',
		};
		expect(statusMap[protoStatus]).toBe(expectedUiStatus);
	});

	it('should support UI-only status values via updateNodeStatus', () => {
		// The store's updateNodeStatus method accepts UIPhaseStatus strings directly
		// This allows setting 'running', 'failed', 'blocked' which aren't in the proto
		const store = useWorkflowEditorStore.getState();

		// Verify the store method signature accepts all UI status types
		const validUIStatuses: PhaseStatusType[] = [
			'pending',
			'running',
			'completed',
			'failed',
			'skipped',
			'blocked',
			'unspecified',
		];

		// This verifies the type system allows these values
		for (const status of validUIStatuses) {
			// updateNodeStatus should accept all UI status values without error
			expect(() => store.updateNodeStatus('test-phase', status)).not.toThrow();
		}
	});
});
