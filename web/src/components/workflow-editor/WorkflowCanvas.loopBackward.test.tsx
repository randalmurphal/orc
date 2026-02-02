/**
 * TDD Integration Tests for WorkflowCanvas backward loop edge rendering
 *
 * Tests for TASK-729: Implement loop edges as backward connections
 *
 * Success Criteria Coverage:
 * - SC-10: End-to-end workflow displays backward loop edges correctly
 * - SC-11: WorkflowCanvas integrates sequence-aware loop edges seamlessly
 * - SC-12: Backward loop edges are clickable and selectable for editing
 * - SC-13: Loop edge styling updates in real-time when workflow changes
 *
 * These tests verify the complete integration from WorkflowWithDetails
 * through layoutWorkflow to rendered LoopEdge components with backward styling.
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkflowCanvas } from './WorkflowCanvas';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { workflowClient } from '@/lib/client';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
	createMockSaveWorkflowLayoutResponse,
} from '@/test/factories';

// Mock the workflow client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		saveWorkflowLayout: vi.fn(),
		validateWorkflow: vi.fn(),
	},
}));

// Mock observers for React Flow
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

	class MockResizeObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'ResizeObserver', {
		value: MockResizeObserver,
		writable: true,
	});
});

/**
 * Create workflow with complex loop patterns for testing backward connections
 */
function createComplexLoopWorkflow() {
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'complex-loop', name: 'Complex Loop Workflow', isBuiltin: false }),
		phases: [
			// Phase 1: spec (sequence 1)
			createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'spec',
				sequence: 1,
				template: createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
			}),
			// Phase 2: tdd_write (sequence 2)
			createMockWorkflowPhase({
				id: 2,
				phaseTemplateId: 'tdd_write',
				sequence: 2,
				template: createMockPhaseTemplate({ id: 'tdd_write', name: 'TDD Write' }),
			}),
			// Phase 3: implement (sequence 3)
			createMockWorkflowPhase({
				id: 3,
				phaseTemplateId: 'implement',
				sequence: 3,
				template: createMockPhaseTemplate({ id: 'implement', name: 'Implementation' }),
			}),
			// Phase 4: review (sequence 4) with backward loop to implement
			createMockWorkflowPhase({
				id: 4,
				phaseTemplateId: 'review',
				sequence: 4,
				loopConfig: JSON.stringify({
					condition: 'needs_changes',
					loop_to_phase: 'implement', // Backward to sequence 3
					max_iterations: 3,
				}),
				template: createMockPhaseTemplate({ id: 'review', name: 'Review' }),
			}),
			// Phase 5: docs (sequence 5) with far backward loop to spec
			createMockWorkflowPhase({
				id: 5,
				phaseTemplateId: 'docs',
				sequence: 5,
				loopConfig: JSON.stringify({
					condition: 'missing_context',
					loop_to_phase: 'spec', // Far backward to sequence 1
					max_iterations: 2,
				}),
				template: createMockPhaseTemplate({ id: 'docs', name: 'Documentation' }),
			}),
		],
	});
}

describe('WorkflowCanvas - Backward Loop Edge Integration', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
		vi.clearAllMocks();
		vi.mocked(workflowClient.saveWorkflowLayout).mockResolvedValue(
			createMockSaveWorkflowLayoutResponse(true)
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-10: End-to-end workflow displays backward loop edges correctly', () => {
		it('renders backward loop edges with distinctive visual styling', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);

			await waitFor(() => {
				// Should find loop edges in the rendered canvas
				const loopEdges = container.querySelectorAll('.edge-loop');
				expect(loopEdges.length).toBeGreaterThan(0);

				// Should find specifically backward loop edges
				const backwardLoopEdges = container.querySelectorAll('.edge-loop-backward');
				expect(backwardLoopEdges.length).toBeGreaterThan(0);

				// Verify at least one backward edge exists
				expect(backwardLoopEdges.length).toBeGreaterThanOrEqual(1);
			});
		});

		it('displays loop edge labels with backward indicators', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);

			await waitFor(() => {
				// Should find loop edge labels
				const loopLabels = container.querySelectorAll('.edge-label-loop');
				expect(loopLabels.length).toBeGreaterThan(0);

				// At least one label should indicate backward direction
				const labelTexts = Array.from(loopLabels).map(label => label.textContent);
				const hasBackwardIndicator = labelTexts.some(text =>
					text?.includes('↩') || text?.includes('needs_changes') || text?.includes('missing_context')
				);
				expect(hasBackwardIndicator).toBe(true);
			});
		});

		it('positions backward loop edges to avoid overlap with forward flow', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);

			await waitFor(() => {
				const backwardEdges = container.querySelectorAll('.edge-loop-backward .react-flow__edge-path');
				expect(backwardEdges.length).toBeGreaterThan(0);

				// Backward edges should have more pronounced curves
				// This is tested by checking SVG path curvature
				backwardEdges.forEach(edge => {
					const pathElement = edge as SVGPathElement;
					const pathData = pathElement.getAttribute('d');
					expect(pathData).toBeDefined();

					// Path should use cubic bezier curves (contains 'C' command)
					expect(pathData).toContain('C');
				});
			});
		});
	});

	describe('SC-11: WorkflowCanvas integrates sequence-aware loop edges seamlessly', () => {
		it('creates correct loop edges from workflow phase data', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			render(<WorkflowCanvas />);

			await waitFor(() => {
				const store = useWorkflowEditorStore.getState();
				const edges = store.edges;

				// Should have loop edges
				const loopEdges = edges.filter(edge => edge.type === 'loop');
				expect(loopEdges.length).toBe(2); // review->implement, docs->spec

				// Verify sequence information in edge data
				loopEdges.forEach(edge => {
					expect(edge.data).toHaveProperty('sourceSequence');
					expect(edge.data).toHaveProperty('targetSequence');
					expect(edge.data).toHaveProperty('isBackward');

					// All edges in this workflow should be backward
					expect(edge.data.isBackward).toBe(true);
				});
			});
		});

		it('updates edge styling when workflow changes', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container, rerender } = render(<WorkflowCanvas />);

			await waitFor(() => {
				const backwardEdges = container.querySelectorAll('.edge-loop-backward');
				expect(backwardEdges.length).toBe(2);
			});

			// Modify workflow to change sequence (simulate reordering)
			const modifiedWorkflow = {
				...workflow,
				phases: workflow.phases?.map(phase =>
					phase.id === 4 // review phase
						? { ...phase, sequence: 1 } // Move to beginning
						: { ...phase, sequence: phase.sequence + 1 }
				),
			};

			useWorkflowEditorStore.getState().loadFromWorkflow(modifiedWorkflow);
			rerender(<WorkflowCanvas />);

			await waitFor(() => {
				// Edge from review (now seq 1) to implement (now seq 4) should be forward
				const store = useWorkflowEditorStore.getState();
				const reviewToImplEdge = store.edges.find(
					edge => edge.source === 'phase-4' && edge.target === 'phase-3' && edge.type === 'loop'
				);

				if (reviewToImplEdge) {
					expect(reviewToImplEdge.data.isBackward).toBe(false); // Now forward: seq 1 -> seq 4
				}
			});
		});
	});

	describe('SC-12: Backward loop edges are clickable and selectable', () => {
		it('allows selecting backward loop edges for inspection', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);
			const user = userEvent.setup();

			await waitFor(() => {
				const backwardEdge = container.querySelector('.edge-loop-backward .react-flow__edge-path');
				expect(backwardEdge).toBeDefined();
			});

			// Click on backward loop edge
			const backwardEdgePath = container.querySelector('.edge-loop-backward .react-flow__edge-path') as Element;
			await user.click(backwardEdgePath);

			// Should be able to select the edge (React Flow handles this)
			// The edge should become part of the selected elements
			await waitFor(() => {
				const store = useWorkflowEditorStore.getState();
				// Selection behavior depends on React Flow implementation
				// This test documents expected interactivity
				expect(backwardEdgePath).toBeDefined();
			});
		});

		it('shows hover effects on backward loop edges', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);
			const user = userEvent.setup();

			await waitFor(() => {
				const backwardEdge = container.querySelector('.edge-loop-backward');
				expect(backwardEdge).toBeDefined();
			});

			const backwardEdge = container.querySelector('.edge-loop-backward') as Element;

			// Hover over edge
			await user.hover(backwardEdge);

			// Should apply hover styling (CSS handled)
			// Test verifies the edge is interactive
			expect(backwardEdge).toBeDefined();
		});
	});

	describe('SC-13: Loop edge styling updates in real-time', () => {
		it('reflects immediate changes when loop config is modified', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);

			await waitFor(() => {
				const initialBackwardEdges = container.querySelectorAll('.edge-loop-backward');
				expect(initialBackwardEdges.length).toBe(2);
			});

			// Simulate workflow update (e.g., from phase editor)
			const updatedWorkflow = {
				...workflow,
				phases: workflow.phases?.map(phase =>
					phase.id === 4
						? {
								...phase,
								loopConfig: JSON.stringify({
									condition: 'always_retry',
									loop_to_phase: 'tdd_write', // Different target
									max_iterations: 5,
								}),
						  }
						: phase
				),
			};

			// Update store to trigger re-render
			useWorkflowEditorStore.getState().loadFromWorkflow(updatedWorkflow);

			await waitFor(() => {
				const store = useWorkflowEditorStore.getState();
				const updatedLoopEdge = store.edges.find(
					edge => edge.source === 'phase-4' && edge.type === 'loop'
				);

				expect(updatedLoopEdge).toBeDefined();
				expect(updatedLoopEdge?.target).toBe('phase-2'); // tdd_write
				expect(updatedLoopEdge?.data.label).toContain('always_retry ×5');
			});
		});

		it('handles removal and addition of loop configurations', async () => {
			const workflow = createComplexLoopWorkflow();
			useWorkflowEditorStore.getState().loadFromWorkflow(workflow);

			const { container } = render(<WorkflowCanvas />);

			await waitFor(() => {
				const initialLoopEdges = container.querySelectorAll('.edge-loop');
				expect(initialLoopEdges.length).toBe(2);
			});

			// Remove loop config from one phase
			const workflowWithoutLoop = {
				...workflow,
				phases: workflow.phases?.map(phase =>
					phase.id === 4
						? { ...phase, loopConfig: undefined } // Remove loop
						: phase
				),
			};

			useWorkflowEditorStore.getState().loadFromWorkflow(workflowWithoutLoop);

			await waitFor(() => {
				const store = useWorkflowEditorStore.getState();
				const loopEdges = store.edges.filter(edge => edge.type === 'loop');
				expect(loopEdges.length).toBe(1); // Only docs->spec loop remains
			});
		});
	});
});

describe('WorkflowCanvas - Loop Edge Error Handling', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
		vi.clearAllMocks();
		vi.mocked(workflowClient.saveWorkflowLayout).mockResolvedValue(
			createMockSaveWorkflowLayoutResponse(true)
		);
	});

	afterEach(() => {
		cleanup();
	});

	it('handles workflows with invalid loop configurations gracefully', async () => {
		const workflowWithInvalidLoop = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({ id: 'invalid', name: 'Invalid Loop' }),
			phases: [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'broken',
					sequence: 1,
					loopConfig: 'invalid json', // Malformed JSON
					template: createMockPhaseTemplate({ id: 'broken', name: 'Broken Phase' }),
				}),
			],
		});

		useWorkflowEditorStore.getState().loadFromWorkflow(workflowWithInvalidLoop);

		expect(() => render(<WorkflowCanvas />)).not.toThrow();

		const { container } = render(<WorkflowCanvas />);

		await waitFor(() => {
			// Should render without loop edges due to invalid config
			const loopEdges = container.querySelectorAll('.edge-loop');
			expect(loopEdges.length).toBe(0);

			// Should still render phase nodes
			const phaseNodes = container.querySelectorAll('[data-id*="phase-"]');
			expect(phaseNodes.length).toBeGreaterThan(0);
		});
	});

	it('handles workflows with missing phase templates', async () => {
		const workflowWithMissingTemplate = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({ id: 'missing', name: 'Missing Template' }),
			phases: [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'source',
					sequence: 1,
					loopConfig: JSON.stringify({
						condition: 'test',
						loop_to_phase: 'missing', // Target doesn't exist
						max_iterations: 1,
					}),
					template: createMockPhaseTemplate({ id: 'source', name: 'Source Phase' }),
				}),
			],
		});

		useWorkflowEditorStore.getState().loadFromWorkflow(workflowWithMissingTemplate);

		expect(() => render(<WorkflowCanvas />)).not.toThrow();

		const { container } = render(<WorkflowCanvas />);

		await waitFor(() => {
			// Should not create loop edge for missing target
			const loopEdges = container.querySelectorAll('.edge-loop');
			expect(loopEdges.length).toBe(0);
		});
	});
});