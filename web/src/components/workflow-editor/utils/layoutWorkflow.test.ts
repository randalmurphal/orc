import { describe, it, expect } from 'vitest';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import { layoutWorkflow } from './layoutWorkflow';

describe('layoutWorkflow', () => {
	describe('node generation', () => {
		it('produces startEnd nodes for a workflow with no phases', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'empty-wf', name: 'Empty' }),
				phases: [],
			});

			const result = layoutWorkflow(details);

			const nodeTypes = result.nodes.map((n) => n.type);
			expect(nodeTypes).toContain('startEnd');
			expect(result.nodes).toHaveLength(2);
			// Both start and end use 'startEnd' type
			expect(result.nodes.filter((n) => n.type === 'startEnd')).toHaveLength(2);
		});

		it('produces startEnd + phase nodes for a single-phase workflow', () => {
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'single', name: 'Single' }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
					}),
				],
			});

			const result = layoutWorkflow(details);

			expect(result.nodes).toHaveLength(3);
			const types = result.nodes.map((n) => n.type);
			expect(types).toContain('startEnd');
			expect(types).toContain('phase');
		});

		it('produces correct node count for multi-phase workflow', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'tdd_write', sequence: 2 }),
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'implement', sequence: 3 }),
					createMockWorkflowPhase({ id: 4, phaseTemplateId: 'review', sequence: 4 }),
				],
			});

			const result = layoutWorkflow(details);

			// 4 phases + start + end = 6
			expect(result.nodes).toHaveLength(6);

			const phaseNodes = result.nodes.filter((n) => n.type === 'phase');
			expect(phaseNodes).toHaveLength(4);
		});

		it('stores phase template ID as node data', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();
			expect(phaseNode!.data).toHaveProperty('phaseTemplateId', 'implement');
		});

		it('assigns unique IDs to all nodes', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			const result = layoutWorkflow(details);

			const ids = result.nodes.map((n) => n.id);
			const uniqueIds = new Set(ids);
			expect(uniqueIds.size).toBe(ids.length);
		});
	});

	describe('node positioning', () => {
		it('assigns positions from dagre layout (not all zero)', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			const result = layoutWorkflow(details);

			// With LR layout, nodes should have different x positions
			const xPositions = result.nodes.map((n) => n.position.x);
			const uniqueX = new Set(xPositions);
			expect(uniqueX.size).toBeGreaterThan(1);
		});

		it('lays out nodes left-to-right (start has smallest x)', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			const result = layoutWorkflow(details);

			const startNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'start'
			)!;
			const endNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'end'
			)!;
			expect(startNode.position.x).toBeLessThan(endNode.position.x);
		});
	});

	describe('sequential edges', () => {
		it('creates a single edge between start and end for 0-phase workflow', () => {
			const details = createMockWorkflowWithDetails({ phases: [] });

			const result = layoutWorkflow(details);

			expect(result.edges).toHaveLength(1);
			const startNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'start'
			)!;
			const endNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'end'
			)!;
			expect(result.edges[0].source).toBe(startNode.id);
			expect(result.edges[0].target).toBe(endNode.id);
		});

		it('creates sequential edges connecting start → phase → end for single phase', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			const result = layoutWorkflow(details);

			const startNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'start'
			)!;
			const phaseNode = result.nodes.find((n) => n.type === 'phase')!;
			const endNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'end'
			)!;

			// start → phase
			const startEdge = result.edges.find(
				(e) => e.source === startNode.id && e.target === phaseNode.id
			);
			expect(startEdge).toBeDefined();

			// phase → end
			const endEdge = result.edges.find(
				(e) => e.source === phaseNode.id && e.target === endNode.id
			);
			expect(endEdge).toBeDefined();
		});

		it('creates sequential chain for multi-phase workflow sorted by sequence', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					// Intentionally out of order to test sorting
					createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			const result = layoutWorkflow(details);

			const startNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'start'
			)!;
			const endNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'end'
			)!;

			// Sequential edges: start → spec → implement → review → end = 4 edges (minimum)
			const sequentialEdges = result.edges.filter(
				(e) => !e.type || e.type === 'default'
			);
			expect(sequentialEdges.length).toBeGreaterThanOrEqual(4);

			// Verify chain connectivity: start reaches end through phase nodes
			const hasStartToFirst = result.edges.some(
				(e) => e.source === startNode.id
			);
			const hasLastToEnd = result.edges.some(
				(e) => e.target === endNode.id
			);
			expect(hasStartToFirst).toBe(true);
			expect(hasLastToEnd).toBe(true);
		});
	});

	describe('dependency edges', () => {
		it('creates dependency edge when phase has dependsOn', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						dependsOn: [],
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

			const depEdges = result.edges.filter((e) => e.type === 'dependency');
			expect(depEdges.length).toBeGreaterThanOrEqual(1);
		});

		it('skips dependency edge when dependsOn references non-existent phase', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
						dependsOn: ['nonexistent_phase'],
					}),
				],
			});

			const result = layoutWorkflow(details);

			const depEdges = result.edges.filter((e) => e.type === 'dependency');
			expect(depEdges).toHaveLength(0);
		});
	});

	describe('loop-back edges', () => {
		it('creates loop edge when phase has retryFromPhase', () => {
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

			const loopEdges = result.edges.filter((e) => e.type === 'loop');
			expect(loopEdges).toHaveLength(1);
		});

		it('does not create loop edge when retryFromPhase references non-existent phase', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'review',
						sequence: 1,
						template: createMockPhaseTemplate({ retryFromPhase: 'nonexistent' }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const loopEdges = result.edges.filter((e) => e.type === 'loop');
			expect(loopEdges).toHaveLength(0);
		});
	});

	describe('edge cases', () => {
		it('handles phases with duplicate sequence numbers', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			const result = layoutWorkflow(details);

			// Should still produce valid output with all nodes
			expect(result.nodes).toHaveLength(4); // 2 phases + start + end
			expect(result.edges.length).toBeGreaterThanOrEqual(3);
		});

		it('returns valid structure shape', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 }),
				],
			});

			const result = layoutWorkflow(details);

			// Verify return shape
			expect(result).toHaveProperty('nodes');
			expect(result).toHaveProperty('edges');
			expect(Array.isArray(result.nodes)).toBe(true);
			expect(Array.isArray(result.edges)).toBe(true);

			// Verify node shape
			for (const node of result.nodes) {
				expect(node).toHaveProperty('id');
				expect(node).toHaveProperty('position');
				expect(node.position).toHaveProperty('x');
				expect(node.position).toHaveProperty('y');
				expect(node).toHaveProperty('data');
				expect(node).toHaveProperty('type');
			}

			// Verify edge shape
			for (const edge of result.edges) {
				expect(edge).toHaveProperty('id');
				expect(edge).toHaveProperty('source');
				expect(edge).toHaveProperty('target');
			}
		});
	});

	describe('custom node data shapes', () => {
		it('produces StartEndNodeData with variant=start for start node', () => {
			const details = createMockWorkflowWithDetails({ phases: [] });

			const result = layoutWorkflow(details);

			const startNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'start'
			);
			expect(startNode).toBeDefined();
			expect(startNode!.data).toHaveProperty('variant', 'start');
			expect(startNode!.data).toHaveProperty('label', 'Start');
		});

		it('produces StartEndNodeData with variant=end for end node', () => {
			const details = createMockWorkflowWithDetails({ phases: [] });

			const result = layoutWorkflow(details);

			const endNode = result.nodes.find(
				(n) => n.type === 'startEnd' && n.data.variant === 'end'
			);
			expect(endNode).toBeDefined();
			expect(endNode!.data).toHaveProperty('variant', 'end');
			expect(endNode!.data).toHaveProperty('label', 'End');
		});

		it('produces PhaseNodeData with templateName from joined template', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						template: createMockPhaseTemplate({
							id: 'spec',
							name: 'Specification',
						}),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();
			expect(phaseNode!.data).toHaveProperty('templateName', 'Specification');
			expect(phaseNode!.data).toHaveProperty('phaseTemplateId', 'spec');
			expect(phaseNode!.data).toHaveProperty('sequence', 1);
			expect(phaseNode!.data).toHaveProperty('phaseId', 1);
		});

		it('falls back to phaseTemplateId for templateName when template is missing', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'custom_phase',
						sequence: 1,
						// No template attached
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();
			expect(phaseNode!.data).toHaveProperty('templateName', 'custom_phase');
		});

		it('computes effective gateType from override, falling back to template default', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						gateTypeOverride: GateType.HUMAN,
						template: createMockPhaseTemplate({
							id: 'spec',
							gateType: GateType.AUTO,
						}),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();
			expect(phaseNode!.data).toHaveProperty('gateType', GateType.HUMAN);
		});

		it('computes effective gateType from template when no override', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						template: createMockPhaseTemplate({
							id: 'spec',
							gateType: GateType.HUMAN,
						}),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();
			expect(phaseNode!.data).toHaveProperty('gateType', GateType.HUMAN);
		});

		it('computes effective maxIterations from override', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
						maxIterationsOverride: 5,
						template: createMockPhaseTemplate({
							id: 'implement',
							maxIterations: 3,
						}),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode!.data).toHaveProperty('maxIterations', 5);
		});

		it('includes modelOverride when set on phase', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
						modelOverride: 'claude-opus-4',
						template: createMockPhaseTemplate({ id: 'implement' }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode!.data).toHaveProperty('modelOverride', 'claude-opus-4');
		});

		it('does not include modelOverride when not set', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'implement',
						sequence: 1,
						template: createMockPhaseTemplate({ id: 'implement' }),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			// modelOverride should be undefined or falsy when not set
			expect(phaseNode!.data.modelOverride).toBeFalsy();
		});

		it('includes all PhaseNodeData fields for a fully-configured phase', () => {
			const details = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 42,
						phaseTemplateId: 'review',
						sequence: 5,
						gateTypeOverride: GateType.HUMAN,
						maxIterationsOverride: 10,
						modelOverride: 'claude-opus-4',
						template: createMockPhaseTemplate({
							id: 'review',
							name: 'Code Review',
							description: 'Multi-agent review',
							gateType: GateType.AUTO,
							maxIterations: 3,
						}),
					}),
				],
			});

			const result = layoutWorkflow(details);

			const phaseNode = result.nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();

			const data = phaseNode!.data;
			expect(data.phaseTemplateId).toBe('review');
			expect(data.templateName).toBe('Code Review');
			expect(data.sequence).toBe(5);
			expect(data.phaseId).toBe(42);
			expect(data.gateType).toBe(GateType.HUMAN);
			expect(data.maxIterations).toBe(10);
			expect(data.modelOverride).toBe('claude-opus-4');
		});
	});
});
