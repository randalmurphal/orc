/**
 * TDD Tests for layoutWorkflow loop edge sequence integration
 *
 * Tests for TASK-729: Implement loop edges as backward connections
 *
 * Success Criteria Coverage:
 * - SC-6: Loop edge data includes source and target sequence numbers
 * - SC-7: Sequence information enables backward connection detection
 * - SC-8: Loop edges maintain all existing data while adding sequence info
 * - SC-9: Invalid loop configurations handled gracefully with sequence data
 *
 * These tests will FAIL until sequence information is added to loop
 * edge data in the layoutWorkflow function.
 */

import { describe, it, expect } from 'vitest';
import { layoutWorkflow } from './layoutWorkflow';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';

/**
 * Create a workflow with loop configuration for testing
 */
function createWorkflowWithLoop() {
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'loop-test', name: 'Loop Test' }),
		phases: [
			// Phase 1: spec (sequence 1)
			createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'spec',
				sequence: 1,
				template: createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
			}),
			// Phase 2: implement (sequence 2)
			createMockWorkflowPhase({
				id: 2,
				phaseTemplateId: 'implement',
				sequence: 2,
				template: createMockPhaseTemplate({ id: 'implement', name: 'Implementation' }),
			}),
			// Phase 3: review (sequence 3) with loop back to implement
			createMockWorkflowPhase({
				id: 3,
				phaseTemplateId: 'review',
				sequence: 3,
				loopConfig: JSON.stringify({
					condition: 'needs_changes',
					loop_to_phase: 'implement', // Loop back to phase 2 (backward)
					max_iterations: 3,
				}),
				template: createMockPhaseTemplate({ id: 'review', name: 'Review' }),
			}),
			// Phase 4: docs (sequence 4) with loop to spec
			createMockWorkflowPhase({
				id: 4,
				phaseTemplateId: 'docs',
				sequence: 4,
				loopConfig: JSON.stringify({
					condition: 'missing_info',
					loop_to_phase: 'spec', // Loop back to phase 1 (more backward)
					max_iterations: 2,
				}),
				template: createMockPhaseTemplate({ id: 'docs', name: 'Documentation' }),
			}),
		],
	});
}

/**
 * Create workflow with forward loop (unusual but possible)
 */
function createWorkflowWithForwardLoop() {
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'forward-loop', name: 'Forward Loop' }),
		phases: [
			// Phase 1: spec with forward loop to implement
			createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'spec',
				sequence: 1,
				loopConfig: JSON.stringify({
					condition: 'needs_implementation',
					loop_to_phase: 'implement', // Loop forward to phase 2
					max_iterations: 1,
				}),
				template: createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
			}),
			// Phase 2: implement
			createMockWorkflowPhase({
				id: 2,
				phaseTemplateId: 'implement',
				sequence: 2,
				template: createMockPhaseTemplate({ id: 'implement', name: 'Implementation' }),
			}),
		],
	});
}

describe('layoutWorkflow - Loop Edge Sequence Integration', () => {
	describe('SC-6: Loop edge data includes source and target sequence numbers', () => {
		it('adds sequence numbers to backward loop edge data', () => {
			const workflow = createWorkflowWithLoop();
			const result = layoutWorkflow(workflow);

			// Find the review -> implement loop edge
			const reviewToImplementLoop = result.edges.find(
				(edge) =>
					edge.type === 'loop' &&
					edge.source === 'phase-3' &&
					edge.target === 'phase-2'
			);

			expect(reviewToImplementLoop).toBeDefined();
			expect(reviewToImplementLoop?.data).toEqual(
				expect.objectContaining({
					condition: 'needs_changes',
					maxIterations: 3,
					label: 'needs_changes ×3',
					// New sequence information
					sourceSequence: 3,
					targetSequence: 2,
					isBackward: true, // Calculated: 3 > 2
				})
			);
		});

		it('adds sequence numbers to multiple loop edges with different directions', () => {
			const workflow = createWorkflowWithLoop();
			const result = layoutWorkflow(workflow);

			// Find docs -> spec loop edge (more backward)
			const docsToSpecLoop = result.edges.find(
				(edge) =>
					edge.type === 'loop' &&
					edge.source === 'phase-4' &&
					edge.target === 'phase-1'
			);

			expect(docsToSpecLoop).toBeDefined();
			expect(docsToSpecLoop?.data).toEqual(
				expect.objectContaining({
					sourceSequence: 4,
					targetSequence: 1,
					isBackward: true, // 4 > 1, strongly backward
				})
			);

			// Verify both loop edges exist with correct sequence data
			const loopEdges = result.edges.filter(edge => edge.type === 'loop');
			expect(loopEdges).toHaveLength(2);

			loopEdges.forEach(edge => {
				expect(edge.data).toHaveProperty('sourceSequence');
				expect(edge.data).toHaveProperty('targetSequence');
				expect(edge.data).toHaveProperty('isBackward');
			});
		});

		it('identifies forward loop edges correctly', () => {
			const workflow = createWorkflowWithForwardLoop();
			const result = layoutWorkflow(workflow);

			// Find spec -> implement forward loop
			const forwardLoop = result.edges.find(
				(edge) =>
					edge.type === 'loop' &&
					edge.source === 'phase-1' &&
					edge.target === 'phase-2'
			);

			expect(forwardLoop).toBeDefined();
			expect(forwardLoop?.data).toEqual(
				expect.objectContaining({
					sourceSequence: 1,
					targetSequence: 2,
					isBackward: false, // 1 < 2, forward flow
				})
			);
		});
	});

	describe('SC-7: Sequence information enables backward connection detection', () => {
		it('correctly calculates isBackward flag for various sequence combinations', () => {
			const workflow = createWorkflowWithLoop();
			const result = layoutWorkflow(workflow);

			const loopEdges = result.edges.filter(edge => edge.type === 'loop');

			for (const edge of loopEdges) {
				const { sourceSequence, targetSequence, isBackward } = edge.data as any;

				expect(typeof sourceSequence).toBe('number');
				expect(typeof targetSequence).toBe('number');
				expect(typeof isBackward).toBe('boolean');

				// Verify calculation is correct
				const expectedIsBackward = sourceSequence > targetSequence;
				expect(isBackward).toBe(expectedIsBackward);
			}
		});

		it('handles self-loops (same sequence) as non-backward', () => {
			const workflowWithSelfLoop = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'self-loop', name: 'Self Loop' }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'review',
						sequence: 1,
						loopConfig: JSON.stringify({
							condition: 'retry',
							loop_to_phase: 'review', // Self-loop
							max_iterations: 5,
						}),
						template: createMockPhaseTemplate({ id: 'review', name: 'Review' }),
					}),
				],
			});

			const result = layoutWorkflow(workflowWithSelfLoop);
			const selfLoop = result.edges.find(edge => edge.type === 'loop');

			expect(selfLoop?.data).toEqual(
				expect.objectContaining({
					sourceSequence: 1,
					targetSequence: 1,
					isBackward: false, // Same sequence = not backward
				})
			);
		});
	});

	describe('SC-8: Loop edges maintain all existing data while adding sequence info', () => {
		it('preserves existing loop edge properties', () => {
			const workflow = createWorkflowWithLoop();
			const result = layoutWorkflow(workflow);

			const loopEdge = result.edges.find(edge => edge.type === 'loop');
			expect(loopEdge).toBeDefined();

			// Should have all original properties
			expect(loopEdge?.data).toEqual(
				expect.objectContaining({
					condition: expect.any(String),
					maxIterations: expect.any(Number),
					label: expect.any(String),
				})
			);

			// Plus new sequence properties
			expect(loopEdge?.data).toEqual(
				expect.objectContaining({
					sourceSequence: expect.any(Number),
					targetSequence: expect.any(Number),
					isBackward: expect.any(Boolean),
				})
			);
		});

		it('maintains correct edge structure and IDs', () => {
			const workflow = createWorkflowWithLoop();
			const result = layoutWorkflow(workflow);

			const loopEdges = result.edges.filter(edge => edge.type === 'loop');

			loopEdges.forEach(edge => {
				expect(edge).toEqual(
					expect.objectContaining({
						id: expect.stringMatching(/^loop-phase-\d+-phase-\d+$/),
						source: expect.stringMatching(/^phase-\d+$/),
						target: expect.stringMatching(/^phase-\d+$/),
						type: 'loop',
					})
				);
			});
		});
	});

	describe('SC-9: Invalid loop configurations handled gracefully', () => {
		it('handles missing loop_to_phase gracefully', () => {
			const workflowWithInvalidLoop = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'invalid', name: 'Invalid Loop' }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'broken',
						sequence: 1,
						loopConfig: JSON.stringify({
							condition: 'test',
							// missing loop_to_phase
							max_iterations: 1,
						}),
						template: createMockPhaseTemplate({ id: 'broken', name: 'Broken' }),
					}),
				],
			});

			expect(() => layoutWorkflow(workflowWithInvalidLoop)).not.toThrow();
			const result = layoutWorkflow(workflowWithInvalidLoop);

			// Should not create any loop edges
			const loopEdges = result.edges.filter(edge => edge.type === 'loop');
			expect(loopEdges).toHaveLength(0);
		});

		it('handles loop to non-existent phase', () => {
			const workflowWithMissingTarget = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'missing-target', name: 'Missing Target' }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'orphan',
						sequence: 1,
						loopConfig: JSON.stringify({
							condition: 'test',
							loop_to_phase: 'nonexistent', // Phase doesn't exist
							max_iterations: 1,
						}),
						template: createMockPhaseTemplate({ id: 'orphan', name: 'Orphan' }),
					}),
				],
			});

			expect(() => layoutWorkflow(workflowWithMissingTarget)).not.toThrow();
			const result = layoutWorkflow(workflowWithMissingTarget);

			// Should not create loop edge for missing target
			const loopEdges = result.edges.filter(edge => edge.type === 'loop');
			expect(loopEdges).toHaveLength(0);
		});

		it('handles invalid JSON in loopConfig', () => {
			const workflowWithInvalidJSON = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'invalid-json', name: 'Invalid JSON' }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'malformed',
						sequence: 1,
						loopConfig: '{ invalid json }', // Malformed JSON
						template: createMockPhaseTemplate({ id: 'malformed', name: 'Malformed' }),
					}),
				],
			});

			expect(() => layoutWorkflow(workflowWithInvalidJSON)).not.toThrow();
			const result = layoutWorkflow(workflowWithInvalidJSON);

			// Should skip malformed config
			const loopEdges = result.edges.filter(edge => edge.type === 'loop');
			expect(loopEdges).toHaveLength(0);
		});

		it('handles phases without sequence numbers', () => {
			// Edge case: phase missing sequence number
			const workflowWithMissingSequence = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'no-seq', name: 'No Sequence' }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'source',
						sequence: 1,
						loopConfig: JSON.stringify({
							condition: 'test',
							loop_to_phase: 'target',
							max_iterations: 1,
						}),
						template: createMockPhaseTemplate({ id: 'source', name: 'Source' }),
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'target',
						sequence: undefined as any, // Missing sequence
						template: createMockPhaseTemplate({ id: 'target', name: 'Target' }),
					}),
				],
			});

			expect(() => layoutWorkflow(workflowWithMissingSequence)).not.toThrow();
			const result = layoutWorkflow(workflowWithMissingSequence);

			// Should still create edge but handle missing sequence gracefully
			const loopEdges = result.edges.filter(edge => edge.type === 'loop');
			if (loopEdges.length > 0) {
				const edge = loopEdges[0];
				expect(edge.data).toHaveProperty('isBackward');
				// Should default to false when sequence is unknown
				expect(edge.data.isBackward).toBe(false);
			}
		});
	});
});

describe('layoutWorkflow - Sequence Integration Edge Cases', () => {
	it('handles retry edges similarly to loop edges (for consistency)', () => {
		const workflowWithRetry = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({ id: 'retry-test', name: 'Retry Test' }),
			phases: [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'impl',
					sequence: 1,
					template: createMockPhaseTemplate({
						id: 'impl',
						name: 'Implementation',
						retryFromPhase: 'spec', // Retry from spec
					}),
				}),
				createMockWorkflowPhase({
					id: 2,
					phaseTemplateId: 'spec',
					sequence: 2, // Later sequence, so impl->spec would be backward if it were a retry
					template: createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
				}),
			],
		});

		const result = layoutWorkflow(workflowWithRetry);

		// Find retry edge
		const retryEdge = result.edges.find(edge => edge.type === 'retry');

		if (retryEdge) {
			// Retry edges might also benefit from sequence information
			// This test documents the current behavior and can guide future enhancement
			expect(retryEdge.source).toBe('phase-1');
			expect(retryEdge.target).toBe('phase-2');
		}
	});
});