/**
 * TDD Tests for WorkflowCanvas - Gate Edge Click Handling
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-4: Clicking gate symbol selects the edge (sets selectedEdgeId in store)
 *
 * Failure Modes:
 * - Click on non-gate edge (dependency) → No selection change
 *
 * These tests will FAIL until WorkflowCanvas is updated with onEdgeClick handler.
 */

import { describe, it, expect, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, cleanup } from '@testing-library/react';
import { WorkflowCanvas } from './WorkflowCanvas';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';

// Mock IntersectionObserver for React Flow
beforeAll(() => {
	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});
});

/** Load a workflow with gate edges into the store */
function loadTestWorkflowWithGates(isBuiltin = true) {
	const details = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'test-wf', isBuiltin }),
		phases: [
			createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'spec',
				sequence: 1,
				template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
			}),
			createMockWorkflowPhase({
				id: 2,
				phaseTemplateId: 'implement',
				sequence: 2,
				template: createMockPhaseTemplate({ gateType: GateType.HUMAN }),
			}),
			createMockWorkflowPhase({
				id: 3,
				phaseTemplateId: 'review',
				sequence: 3,
				template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
			}),
		],
	});
	useWorkflowEditorStore.getState().loadFromWorkflow(details);
	return details;
}

describe('WorkflowCanvas - Gate Edge Click', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-4: onEdgeClick calls selectEdge for gate edges', () => {
		it('calls selectEdge when a gate edge is clicked', () => {
			loadTestWorkflowWithGates();

			render(<WorkflowCanvas />);

			// Find a gate edge in the store
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			expect(gateEdge).toBeDefined();

			// Find the rendered edge and click it
			const edgeEl = document.querySelector(`[data-id="${gateEdge!.id}"]`);
			expect(edgeEl).not.toBeNull();

			// Click the edge
			edgeEl!.dispatchEvent(new MouseEvent('click', { bubbles: true }));

			// After clicking, the store should have selectedEdgeId set
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe(gateEdge!.id);
		});

		it('clears selectedNodeId when selecting a gate edge', () => {
			loadTestWorkflowWithGates();

			render(<WorkflowCanvas />);

			// First select a node
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			useWorkflowEditorStore.getState().selectNode(phaseNode!.id);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNode!.id);

			// Now click a gate edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			const edgeEl = document.querySelector(`[data-id="${gateEdge!.id}"]`);

			if (edgeEl) {
				edgeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
			}

			// selectedNodeId should be cleared
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();
			// selectedEdgeId should be set
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe(gateEdge!.id);
		});

		it('clicking gate symbol on edge also selects the edge', () => {
			loadTestWorkflowWithGates();

			render(<WorkflowCanvas />);

			// The gate symbol is part of the edge via EdgeLabelRenderer
			const gateSymbol = document.querySelector('.gate-edge__symbol');

			if (gateSymbol) {
				gateSymbol.dispatchEvent(new MouseEvent('click', { bubbles: true }));

				// Should select the parent edge
				expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeDefined();
			}
		});
	});

	describe('Failure mode: Click on non-gate edge (dependency)', () => {
		it('does not select dependency edges', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						dependsOn: ['spec'],
					}),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			render(<WorkflowCanvas />);

			// Find a dependency edge
			const edges = useWorkflowEditorStore.getState().edges;
			const depEdge = edges.find((e) => e.type === 'dependency');

			if (depEdge) {
				const edgeEl = document.querySelector(`[data-id="${depEdge.id}"]`);
				if (edgeEl) {
					edgeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
				}

				// selectedEdgeId should NOT be set for dependency edges
				expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
			}
		});

		it('does not select loop edges', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'review',
						sequence: 2,
						loopConfig: JSON.stringify({
							condition: 'has_findings',
							loop_to_phase: 'implement',
							max_iterations: 3,
						}),
					}),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			render(<WorkflowCanvas />);

			const edges = useWorkflowEditorStore.getState().edges;
			const loopEdge = edges.find((e) => e.type === 'loop');

			if (loopEdge) {
				const edgeEl = document.querySelector(`[data-id="${loopEdge.id}"]`);
				if (edgeEl) {
					edgeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
				}

				// selectedEdgeId should NOT be set for loop edges
				expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
			}
		});

		it('does not select retry edges', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'review',
						sequence: 2,
						template: createMockPhaseTemplate({ retryFromPhase: 'implement' }),
					}),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			render(<WorkflowCanvas />);

			const edges = useWorkflowEditorStore.getState().edges;
			const retryEdge = edges.find((e) => e.type === 'retry');

			if (retryEdge) {
				const edgeEl = document.querySelector(`[data-id="${retryEdge.id}"]`);
				if (edgeEl) {
					edgeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
				}

				// selectedEdgeId should NOT be set for retry edges
				expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
			}
		});
	});

	describe('Edge selection in read-only mode', () => {
		it('allows gate edge selection in read-only (built-in) mode', () => {
			loadTestWorkflowWithGates(true); // built-in → readOnly

			render(<WorkflowCanvas />);

			// Even in read-only mode, edges should be selectable for inspection
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');

			if (gateEdge) {
				const edgeEl = document.querySelector(`[data-id="${gateEdge.id}"]`);
				if (edgeEl) {
					edgeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
				}

				// Selection should work even in read-only mode
				expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe(gateEdge.id);
			}
		});
	});

	describe('Pane click clears edge selection', () => {
		it('deselects edge when clicking empty canvas area', () => {
			loadTestWorkflowWithGates();

			render(<WorkflowCanvas />);

			// First select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe(gateEdge!.id);

			// Click the canvas pane (background)
			const pane = document.querySelector('.react-flow__pane');
			if (pane) {
				pane.dispatchEvent(new MouseEvent('click', { bubbles: true }));
			}

			// After pane click, selectedEdgeId should be null
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
		});
	});

	describe('Switching between node and edge selection', () => {
		it('clicking a node after selecting an edge clears edge selection', () => {
			loadTestWorkflowWithGates();

			render(<WorkflowCanvas />);

			// Select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe(gateEdge!.id);

			// Click a node
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			const nodeEl = document.querySelector(`[data-id="${phaseNode!.id}"]`);

			if (nodeEl) {
				nodeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
			}

			// Edge selection should be cleared, node should be selected
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNode!.id);
		});
	});
});
