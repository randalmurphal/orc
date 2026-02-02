/**
 * TDD Tests for layoutWorkflow - Gate Edge Generation
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-3: Sequential edges use GateEdge type instead of SequentialEdge when displaying gates
 * - SC-7: Entry edge renders from left canvas boundary to first phase
 * - SC-8: Exit edge renders from last phase to right canvas boundary
 *
 * Edge Cases:
 * - Single-phase workflow: Entry + exit gates only
 * - Workflow with no phases: No gates rendered
 * - Phase with GATE_TYPE_UNSPECIFIED: Inherits from template or defaults to AUTO
 *
 * These tests will FAIL until layoutWorkflow is updated to generate gate edges.
 */

import { describe, it, expect } from 'vitest';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import { layoutWorkflow } from './layoutWorkflow';

describe('layoutWorkflow - Gate Edges', () => {
	describe('SC-3: Sequential edges use GateEdge type', () => {
		it('generates gate type edges between phases instead of sequential', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test-wf' }),
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
						template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			// Should have gate edges, not sequential edges between phases
			const phaseToPhaseEdges = result.edges.filter(
				(e) => e.source.startsWith('phase-') && e.target.startsWith('phase-')
			);

			expect(phaseToPhaseEdges.length).toBeGreaterThan(0);
			expect(phaseToPhaseEdges.every((e) => e.type === 'gate')).toBe(true);
		});

		it('includes gate data on gate edges', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						template: createMockPhaseTemplate({ gateType: GateType.HUMAN }),
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdge = result.edges.find(
				(e) => e.type === 'gate' && e.source === 'phase-1' && e.target === 'phase-2'
			);

			expect(gateEdge).toBeDefined();
			expect(gateEdge!.data).toBeDefined();
			// Gate type should come from the TARGET phase (the gate that needs to pass to enter that phase)
			expect(gateEdge!.data.gateType).toBeDefined();
		});

		it('gate type on edge comes from target phase', () => {
			const details = createMockWorkflowWithDetails({
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
						gateTypeOverride: GateType.HUMAN,
						template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const specToImplementEdge = result.edges.find(
				(e) => e.type === 'gate' && e.source === 'phase-1' && e.target === 'phase-2'
			);

			// The gate before "implement" phase should be HUMAN (from the target phase)
			expect(specToImplementEdge?.data.gateType).toBe(GateType.HUMAN);
		});

		it('respects gateTypeOverride over template default', () => {
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
						gateTypeOverride: GateType.AI,
						template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdge = result.edges.find(
				(e) => e.type === 'gate' && e.target === 'phase-2'
			);

			expect(gateEdge?.data.gateType).toBe(GateType.AI);
		});
	});

	describe('SC-7: Entry edge renders from left canvas boundary to first phase', () => {
		it('generates entry edge for workflow with phases', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						template: createMockPhaseTemplate({ gateType: GateType.AUTO }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const entryEdge = result.edges.find((e) => e.data?.position === 'entry');
			expect(entryEdge).toBeDefined();
			expect(entryEdge!.type).toBe('gate');
		});

		it('entry edge targets the first phase node', () => {
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
					}),
				],
			});

			const result = layoutWorkflow(details);

			const entryEdge = result.edges.find((e) => e.data?.position === 'entry');
			expect(entryEdge?.target).toBe('phase-1');
		});

		it('entry edge source is a virtual entry point', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const entryEdge = result.edges.find((e) => e.data?.position === 'entry');
			// Source should be a virtual entry node, not a phase node
			expect(entryEdge?.source).toContain('entry');
		});

		it('entry gate type comes from first phase', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						gateTypeOverride: GateType.HUMAN,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const entryEdge = result.edges.find((e) => e.data?.position === 'entry');
			expect(entryEdge?.data.gateType).toBe(GateType.HUMAN);
		});

		it('creates virtual entry node for the edge source', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			// Should have a virtual entry node
			const entryNode = result.nodes.find((n) => n.id.includes('entry'));
			expect(entryNode).toBeDefined();
			expect(entryNode!.type).toBe('virtual');
		});
	});

	describe('SC-8: Exit edge renders from last phase to right canvas boundary', () => {
		it('generates exit edge for workflow with phases', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const exitEdge = result.edges.find((e) => e.data?.position === 'exit');
			expect(exitEdge).toBeDefined();
			expect(exitEdge!.type).toBe('gate');
		});

		it('exit edge source is the last phase node', () => {
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
					}),
					createMockWorkflowPhase({
						id: 3,
						phaseTemplateId: 'review',
						sequence: 3,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const exitEdge = result.edges.find((e) => e.data?.position === 'exit');
			expect(exitEdge?.source).toBe('phase-3');
		});

		it('exit edge target is a virtual exit point', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const exitEdge = result.edges.find((e) => e.data?.position === 'exit');
			expect(exitEdge?.target).toContain('exit');
		});

		it('exit gate uses AUTO type by default', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						gateTypeOverride: GateType.HUMAN, // This shouldn't affect exit gate
					}),
				],
			});

			const result = layoutWorkflow(details);

			const exitEdge = result.edges.find((e) => e.data?.position === 'exit');
			// Exit gate is AUTO by default (workflow completion gate)
			expect(exitEdge?.data.gateType).toBe(GateType.AUTO);
		});

		it('creates virtual exit node for the edge target', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			// Should have a virtual exit node
			const exitNode = result.nodes.find((n) => n.id.includes('exit'));
			expect(exitNode).toBeDefined();
			expect(exitNode!.type).toBe('virtual');
		});
	});

	describe('Edge case: Single-phase workflow', () => {
		it('generates entry + exit gates only (no between gates)', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdges = result.edges.filter((e) => e.type === 'gate');

			// Should have exactly 2 gate edges: entry and exit
			expect(gateEdges.length).toBe(2);

			const entryEdge = gateEdges.find((e) => e.data?.position === 'entry');
			const exitEdge = gateEdges.find((e) => e.data?.position === 'exit');

			expect(entryEdge).toBeDefined();
			expect(exitEdge).toBeDefined();

			// No "between" edges
			const betweenEdges = gateEdges.filter((e) => e.data?.position === 'between');
			expect(betweenEdges.length).toBe(0);
		});
	});

	describe('Edge case: Workflow with no phases', () => {
		it('generates no gates for empty workflow', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'empty' }),
				phases: [],
			});

			const result = layoutWorkflow(details);

			const gateEdges = result.edges.filter((e) => e.type === 'gate');
			expect(gateEdges.length).toBe(0);
		});

		it('generates no nodes for empty workflow', () => {
			const details = createMockWorkflowWithDetails({
				phases: [],
			});

			const result = layoutWorkflow(details);

			// No phase nodes, no virtual entry/exit nodes
			expect(result.nodes.length).toBe(0);
		});
	});

	describe('Edge case: Phase with GATE_TYPE_UNSPECIFIED', () => {
		it('defaults to AUTO when gateType is unspecified', () => {
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
						// No gateTypeOverride, no template gateType
					}),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdge = result.edges.find(
				(e) => e.type === 'gate' && e.target === 'phase-2'
			);

			// Should default to AUTO when unspecified
			expect(gateEdge?.data.gateType).toBe(GateType.AUTO);
		});

		it('uses template gateType when phase override is unspecified', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'review',
						sequence: 2,
						// No gateTypeOverride
						template: createMockPhaseTemplate({
							id: 'review',
							gateType: GateType.HUMAN,
						}),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdge = result.edges.find(
				(e) => e.type === 'gate' && e.target === 'phase-2'
			);

			expect(gateEdge?.data.gateType).toBe(GateType.HUMAN);
		});
	});

	describe('Multi-phase workflow gate count', () => {
		it('generates correct number of gates for 3-phase workflow', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdges = result.edges.filter((e) => e.type === 'gate');

			// 3 phases = 4 gates: entry→spec, spec→implement, implement→review, review→exit
			expect(gateEdges.length).toBe(4);
		});

		it('generates correct number of gates for 5-phase workflow', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'tdd', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'implement', sequence: 3 }),
					createMockWorkflowPhase({ id: 4, phaseTemplateId: 'review', sequence: 4 }),
					createMockWorkflowPhase({ id: 5, phaseTemplateId: 'docs', sequence: 5 }),
				],
			});

			const result = layoutWorkflow(details);

			const gateEdges = result.edges.filter((e) => e.type === 'gate');

			// 5 phases = 6 gates: entry + 4 between + exit
			expect(gateEdges.length).toBe(6);
		});
	});

	describe('Edge data includes phaseId for gate inspector', () => {
		it('between gates include target phaseId', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			const result = layoutWorkflow(details);

			const betweenEdge = result.edges.find(
				(e) => e.type === 'gate' && e.data?.position === 'between'
			);

			expect(betweenEdge?.data.phaseId).toBe(2);
		});

		it('entry gate includes first phase phaseId', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 42, phaseTemplateId: 'spec', sequence: 1 }),
				],
			});

			const result = layoutWorkflow(details);

			const entryEdge = result.edges.find((e) => e.data?.position === 'entry');
			expect(entryEdge?.data.phaseId).toBe(42);
		});
	});

	describe('Non-gate edges are preserved', () => {
		it('preserves dependency edges alongside gate edges', () => {
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

			const result = layoutWorkflow(details);

			const dependencyEdges = result.edges.filter((e) => e.type === 'dependency');
			expect(dependencyEdges.length).toBeGreaterThanOrEqual(1);
		});

		it('preserves loop edges alongside gate edges', () => {
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

			const result = layoutWorkflow(details);

			const loopEdges = result.edges.filter((e) => e.type === 'loop');
			expect(loopEdges.length).toBe(1);
		});

		it('preserves retry edges alongside gate edges', () => {
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

			const result = layoutWorkflow(details);

			const retryEdges = result.edges.filter((e) => e.type === 'retry');
			expect(retryEdges.length).toBe(1);
		});
	});
});
