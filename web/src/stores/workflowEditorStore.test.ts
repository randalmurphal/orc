import { describe, it, expect, beforeEach } from 'vitest';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
} from '@/test/factories';
import { useWorkflowEditorStore } from './workflowEditorStore';

describe('workflowEditorStore', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
	});

	describe('initial state', () => {
		it('starts with empty nodes and edges', () => {
			const state = useWorkflowEditorStore.getState();
			expect(state.nodes).toEqual([]);
			expect(state.edges).toEqual([]);
		});

		it('starts with readOnly false', () => {
			const state = useWorkflowEditorStore.getState();
			expect(state.readOnly).toBe(false);
		});

		it('starts with no selected node', () => {
			const state = useWorkflowEditorStore.getState();
			expect(state.selectedNodeId).toBeNull();
		});

		it('starts with no workflow data', () => {
			const state = useWorkflowEditorStore.getState();
			expect(state.workflowDetails).toBeNull();
		});
	});

	describe('loadFromWorkflow', () => {
		it('populates nodes and edges from workflow with details', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test-wf', name: 'Test Workflow' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			const state = useWorkflowEditorStore.getState();
			// 2 phase nodes (no start/end nodes per design spec)
			expect(state.nodes).toHaveLength(2);
			expect(state.edges.length).toBeGreaterThan(0);
		});

		it('stores the workflow details reference', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test-wf' }),
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			const state = useWorkflowEditorStore.getState();
			expect(state.workflowDetails).toBe(details);
		});

		it('sets readOnly true for builtin workflows', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'builtin-wf', isBuiltin: true }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			expect(useWorkflowEditorStore.getState().readOnly).toBe(true);
		});

		it('sets readOnly false for custom workflows', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'custom-wf', isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			expect(useWorkflowEditorStore.getState().readOnly).toBe(false);
		});

		it('handles workflow with zero phases', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'empty' }),
				phases: [],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);

			const state = useWorkflowEditorStore.getState();
			// No phases means no nodes (no start/end nodes per design spec)
			expect(state.nodes).toHaveLength(0);
			// No edges either
			expect(state.edges).toHaveLength(0);
		});

		it('replaces previous state when called again', () => {
			const details1 = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'wf-1' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});
			const details2 = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'wf-2' }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details1);
			expect(useWorkflowEditorStore.getState().nodes).toHaveLength(1); // 1 phase node

			useWorkflowEditorStore.getState().loadFromWorkflow(details2);
			expect(useWorkflowEditorStore.getState().nodes).toHaveLength(3); // 3 phase nodes
		});

		it('clears selected node when loading new workflow', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);
			// Select a node
			useWorkflowEditorStore.getState().selectNode('some-node-id');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('some-node-id');

			// Load new workflow should clear selection
			useWorkflowEditorStore.getState().loadFromWorkflow(details);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();
		});
	});

	describe('setReadOnly', () => {
		it('sets readOnly to true', () => {
			useWorkflowEditorStore.getState().setReadOnly(true);
			expect(useWorkflowEditorStore.getState().readOnly).toBe(true);
		});

		it('sets readOnly to false', () => {
			useWorkflowEditorStore.getState().setReadOnly(true);
			useWorkflowEditorStore.getState().setReadOnly(false);
			expect(useWorkflowEditorStore.getState().readOnly).toBe(false);
		});
	});

	describe('selectNode', () => {
		it('sets selectedNodeId', () => {
			useWorkflowEditorStore.getState().selectNode('node-1');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-1');
		});

		it('sets selectedNodeId to null to deselect', () => {
			useWorkflowEditorStore.getState().selectNode('node-1');
			useWorkflowEditorStore.getState().selectNode(null);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();
		});

		it('replaces previous selection when selecting a different node', () => {
			useWorkflowEditorStore.getState().selectNode('node-1');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-1');

			useWorkflowEditorStore.getState().selectNode('node-2');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-2');

			useWorkflowEditorStore.getState().selectNode('node-3');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-3');
		});

		it('selecting the same node again keeps it selected', () => {
			useWorkflowEditorStore.getState().selectNode('node-1');
			useWorkflowEditorStore.getState().selectNode('node-1');
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('node-1');
		});
	});

	describe('reset', () => {
		it('clears all state back to initial', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test', isBuiltin: true }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			useWorkflowEditorStore.getState().loadFromWorkflow(details);
			useWorkflowEditorStore.getState().selectNode('some-node');

			// Verify state is populated
			expect(useWorkflowEditorStore.getState().nodes.length).toBeGreaterThan(0);
			expect(useWorkflowEditorStore.getState().readOnly).toBe(true);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe('some-node');

			// Reset
			useWorkflowEditorStore.getState().reset();

			const state = useWorkflowEditorStore.getState();
			expect(state.nodes).toEqual([]);
			expect(state.edges).toEqual([]);
			expect(state.readOnly).toBe(false);
			expect(state.selectedNodeId).toBeNull();
			expect(state.workflowDetails).toBeNull();
		});
	});
});
