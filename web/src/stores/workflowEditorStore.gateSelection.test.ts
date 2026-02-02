/**
 * TDD Tests for workflowEditorStore - Edge/Gate Selection
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-4: Clicking gate symbol selects the edge (sets selectedEdgeId in store)
 *
 * These tests will FAIL until workflowEditorStore is updated with selectedEdgeId.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
} from '@/test/factories';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';

describe('workflowEditorStore - Edge Selection', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
	});

	describe('SC-4: selectedEdgeId state', () => {
		it('starts with no selected edge', () => {
			const state = useWorkflowEditorStore.getState();
			expect(state.selectedEdgeId).toBeNull();
		});

		it('has selectEdge action that sets selectedEdgeId', () => {
			const store = useWorkflowEditorStore.getState();

			// selectEdge should exist as an action
			expect(typeof store.selectEdge).toBe('function');
		});

		it('selectEdge sets selectedEdgeId', () => {
			useWorkflowEditorStore.getState().selectEdge('edge-1');

			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');
		});

		it('selectEdge with null clears selection', () => {
			useWorkflowEditorStore.getState().selectEdge('edge-1');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');

			useWorkflowEditorStore.getState().selectEdge(null);
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
		});

		it('selecting an edge clears node selection', () => {
			// Select a node first
			useWorkflowEditorStore.getState().selectNode('node-1');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-1');

			// Select an edge
			useWorkflowEditorStore.getState().selectEdge('edge-1');

			// Node selection should be cleared
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');
		});

		it('selecting a node clears edge selection', () => {
			// Select an edge first
			useWorkflowEditorStore.getState().selectEdge('edge-1');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');

			// Select a node
			useWorkflowEditorStore.getState().selectNode('node-1');

			// Edge selection should be cleared
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-1');
		});

		it('replaces previous edge selection when selecting a different edge', () => {
			useWorkflowEditorStore.getState().selectEdge('edge-1');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');

			useWorkflowEditorStore.getState().selectEdge('edge-2');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-2');

			useWorkflowEditorStore.getState().selectEdge('edge-3');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-3');
		});

		it('selecting the same edge again keeps it selected', () => {
			useWorkflowEditorStore.getState().selectEdge('edge-1');
			useWorkflowEditorStore.getState().selectEdge('edge-1');

			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');
		});
	});

	describe('loadFromWorkflow clears edge selection', () => {
		it('clears selectedEdgeId when loading new workflow', () => {
			// Select an edge
			useWorkflowEditorStore.getState().selectEdge('edge-1');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');

			// Load a new workflow
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			// Edge selection should be cleared
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
		});
	});

	describe('reset clears edge selection', () => {
		it('clears selectedEdgeId on reset', () => {
			useWorkflowEditorStore.getState().selectEdge('edge-1');
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe('edge-1');

			useWorkflowEditorStore.getState().reset();

			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
		});
	});

	describe('useEditorSelectedEdgeId selector', () => {
		it('provides a selector hook for selectedEdgeId', async () => {
			// The selector should exist
			const { useEditorSelectedEdgeId } = await import('@/stores/workflowEditorStore');
			expect(typeof useEditorSelectedEdgeId).toBe('function');
		});
	});

	describe('selected edge lookup', () => {
		// Helper to get selected edge from store state
		const getSelectedEdge = () => {
			const state = useWorkflowEditorStore.getState();
			if (!state.selectedEdgeId) return null;
			return state.edges.find((e) => e.id === state.selectedEdgeId) ?? null;
		};

		it('returns null when no edge is selected', () => {
			expect(getSelectedEdge()).toBeNull();
		});

		it('returns the selected edge object when an edge is selected', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			const edges = useWorkflowEditorStore.getState().edges;
			const firstGateEdge = edges.find((e) => e.type === 'gate');

			if (firstGateEdge) {
				useWorkflowEditorStore.getState().selectEdge(firstGateEdge.id);

				const selectedEdge = getSelectedEdge();
				expect(selectedEdge).toBeDefined();
				expect(selectedEdge?.id).toBe(firstGateEdge.id);
			}
		});

		it('returns null if selected edge ID does not exist in edges', () => {
			useWorkflowEditorStore.getState().selectEdge('nonexistent-edge');

			// getSelectedEdge should return null since the edge doesn't exist
			const selectedEdge = getSelectedEdge();
			expect(selectedEdge).toBeNull();
		});
	});
});
