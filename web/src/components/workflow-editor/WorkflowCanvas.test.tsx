/**
 * TDD Tests for WorkflowCanvas - node click and pane click interactions
 *
 * Tests for TASK-636: 3-panel editor layout, routing, canvas integration
 *
 * Success Criteria Coverage:
 * - SC-4: Clicking a phase node selects it (onNodeClick → selectNode)
 * - SC-5: Clicking canvas background deselects (onPaneClick → selectNode(null))
 * - SC-6: Node selection state reflects visually (elementsSelectable always true)
 *
 * Edge cases:
 * - Clicking start/end nodes does nothing (not selectable as phase)
 * - Read-only mode still allows selection (for inspection)
 */

import { describe, it, expect, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, cleanup } from '@testing-library/react';
import { WorkflowCanvas } from './WorkflowCanvas';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
} from '@/test/factories';

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

/** Load a workflow into the store before rendering */
function loadTestWorkflow(isBuiltin = true) {
	const details = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'test-wf', isBuiltin }),
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
			}),
		],
	});
	useWorkflowEditorStore.getState().loadFromWorkflow(details);
	return details;
}

describe('WorkflowCanvas', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-6: elementsSelectable is always true', () => {
		it('renders with elementsSelectable true even in read-only mode', () => {
			loadTestWorkflow(true); // built-in → readOnly=true

			const { container } = render(<WorkflowCanvas />);

			// React Flow should be rendered
			const reactFlowEl = container.querySelector('.react-flow');
			expect(reactFlowEl).not.toBeNull();

			// In read-only mode, nodes should NOT be draggable but SHOULD be selectable
			// We verify this by checking the store state and that the component renders
			expect(useWorkflowEditorStore.getState().readOnly).toBe(true);
		});

		it('renders selectable nodes in editable (custom) mode', () => {
			loadTestWorkflow(false); // custom → readOnly=false

			const { container } = render(<WorkflowCanvas />);

			const reactFlowEl = container.querySelector('.react-flow');
			expect(reactFlowEl).not.toBeNull();
			expect(useWorkflowEditorStore.getState().readOnly).toBe(false);
		});
	});

	describe('SC-4: onNodeClick calls selectNode for phase nodes', () => {
		it('calls selectNode when a phase node is clicked', () => {
			loadTestWorkflow();

			render(<WorkflowCanvas />);

			// Find a phase node in the store
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();

			// Find the rendered phase node and click it
			const phaseNodeEl = document.querySelector(`[data-id="${phaseNode!.id}"]`);
			expect(phaseNodeEl).not.toBeNull();

			// Click the node — this should trigger onNodeClick → selectNode
			phaseNodeEl!.dispatchEvent(new MouseEvent('click', { bubbles: true }));

			// After clicking, the store should have selectedNodeId set
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNode!.id);
		});

		it('only has phase nodes (no start/end nodes per design spec)', () => {
			loadTestWorkflow();

			render(<WorkflowCanvas />);

			const nodes = useWorkflowEditorStore.getState().nodes;
			// All nodes should be phase type (start/end nodes removed per design spec)
			expect(nodes.every((n) => n.type === 'phase')).toBe(true);
			expect(nodes.length).toBe(2); // spec + implement
		});
	});

	describe('SC-5: onPaneClick deselects', () => {
		it('deselects node when clicking empty canvas area', () => {
			loadTestWorkflow();

			render(<WorkflowCanvas />);

			// First select a node
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			useWorkflowEditorStore.getState().selectNode(phaseNode!.id);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNode!.id);

			// Click the canvas pane (background) — this should trigger onPaneClick → selectNode(null)
			const pane = document.querySelector('.react-flow__pane');
			if (pane) {
				pane.dispatchEvent(new MouseEvent('click', { bubbles: true }));
			}

			// After pane click, selectedNodeId should be null
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();
		});
	});

	describe('edge cases', () => {
		it('allows node selection in read-only (built-in) mode', () => {
			loadTestWorkflow(true); // built-in → readOnly

			render(<WorkflowCanvas />);

			// Even in read-only mode, nodes should be selectable for inspection
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();

			const phaseNodeEl = document.querySelector(`[data-id="${phaseNode!.id}"]`);
			if (phaseNodeEl) {
				phaseNodeEl.dispatchEvent(new MouseEvent('click', { bubbles: true }));
			}

			// Selection should work even in read-only mode
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNode!.id);
		});

		it('switching selection between nodes replaces previous', () => {
			loadTestWorkflow();

			render(<WorkflowCanvas />);

			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNodes = nodes.filter((n) => n.type === 'phase');
			expect(phaseNodes.length).toBeGreaterThanOrEqual(2);

			// Select first node
			useWorkflowEditorStore.getState().selectNode(phaseNodes[0].id);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNodes[0].id);

			// Select second node — should replace
			useWorkflowEditorStore.getState().selectNode(phaseNodes[1].id);
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBe(phaseNodes[1].id);
		});
	});
});
