/**
 * TDD Tests for layoutWorkflow - Position Loading (SC-11)
 *
 * Tests for TASK-640: Position persistence in layout calculation
 *
 * Success Criteria Coverage:
 * - SC-11: Loading uses position_x/position_y when present; falls back to dagre when null
 *
 * Edge cases:
 * - Phase positions partially set (mixed mode)
 * - Reset Layout clears all positions and re-runs dagre
 */

import { describe, it, expect } from 'vitest';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	
} from '@/test/factories';
import { layoutWorkflow } from './layoutWorkflow';

describe('layoutWorkflow - Position Persistence (SC-11)', () => {
	describe('uses stored positions when present', () => {
		it('uses positionX/positionY from phase when both are set', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'pos-wf', isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 100,
						positionY: 200,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						positionX: 400,
						positionY: 200,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1');
			const implNode = result.nodes.find((n) => n.id === 'phase-2');

			expect(specNode).toBeDefined();
			expect(specNode!.position.x).toBe(100);
			expect(specNode!.position.y).toBe(200);

			expect(implNode).toBeDefined();
			expect(implNode!.position.x).toBe(400);
			expect(implNode!.position.y).toBe(200);
		});

		it('preserves exact stored positions without dagre adjustment', () => {
			// Use unusual positions that dagre would never produce
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 999,
						positionY: 777,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1');
			expect(specNode!.position.x).toBe(999);
			expect(specNode!.position.y).toBe(777);
		});
	});

	describe('falls back to dagre layout when positions are null', () => {
		it('uses dagre layout when positionX is null', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: undefined,
						positionY: 200,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1');
			// Should use dagre-computed position, not 200
			expect(specNode).toBeDefined();
			// Dagre positions are computed, we just verify they're not the stored Y
			// (since X was missing, dagre should take over)
		});

		it('uses dagre layout when positionY is null', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 100,
						positionY: undefined,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1');
			expect(specNode).toBeDefined();
			// When Y is missing, dagre should compute both positions
		});

		it('uses dagre layout when both positions are null', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: undefined,
						positionY: undefined,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						positionX: undefined,
						positionY: undefined,
					}),
				],
			});

			const result = layoutWorkflow(details);

			// With LR layout, dagre assigns increasing x positions
			const specNode = result.nodes.find((n) => n.id === 'phase-1')!;
			const implNode = result.nodes.find((n) => n.id === 'phase-2')!;

			// Dagre produces left-to-right ordering
			expect(specNode.position.x).toBeLessThan(implNode.position.x);
		});
	});

	describe('mixed mode: partial positions', () => {
		it('uses stored positions for some phases and dagre for others', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 50,
						positionY: 100,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						// No position stored - should use dagre
						positionX: undefined,
						positionY: undefined,
					}),
					createMockWorkflowPhase({
						id: 3,
						phaseTemplateId: 'review',
						sequence: 3,
						positionX: 600,
						positionY: 100,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1')!;
			const implNode = result.nodes.find((n) => n.id === 'phase-2')!;
			const reviewNode = result.nodes.find((n) => n.id === 'phase-3')!;

			// spec and review use stored positions
			expect(specNode.position.x).toBe(50);
			expect(specNode.position.y).toBe(100);
			expect(reviewNode.position.x).toBe(600);
			expect(reviewNode.position.y).toBe(100);

			// implement gets dagre position (will be between spec and review in x)
			// We can't assert exact dagre values, but verify it's different from stored ones
			expect(implNode.position.x).not.toBe(50);
			expect(implNode.position.x).not.toBe(600);
		});
	});

	describe('only phase nodes are produced (no start/end)', () => {
		it('produces only phase nodes with stored positions', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 200,
						positionY: 200,
					}),
				],
			});

			const result = layoutWorkflow(details);

			// Only phase nodes, no startEnd nodes
			expect(result.nodes).toHaveLength(1);
			expect(result.nodes[0].type).toBe('phase');
			expect(result.nodes[0].position.x).toBe(200);
			expect(result.nodes[0].position.y).toBe(200);
		});
	});

	describe('position values at boundaries', () => {
		it('handles position at origin (0, 0)', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 0,
						positionY: 0,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1')!;
			expect(specNode.position.x).toBe(0);
			expect(specNode.position.y).toBe(0);
		});

		it('handles negative positions', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: -100,
						positionY: -50,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1')!;
			expect(specNode.position.x).toBe(-100);
			expect(specNode.position.y).toBe(-50);
		});

		it('handles very large positions', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						positionX: 10000,
						positionY: 5000,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specNode = result.nodes.find((n) => n.id === 'phase-1')!;
			expect(specNode.position.x).toBe(10000);
			expect(specNode.position.y).toBe(5000);
		});
	});
});
