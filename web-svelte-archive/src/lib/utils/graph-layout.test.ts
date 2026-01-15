import { describe, it, expect } from 'vitest';
import { computeLayout, getEdgePath, type LayoutResult } from './graph-layout';

describe('graph-layout', () => {
	describe('computeLayout', () => {
		it('should handle empty graph', () => {
			const result = computeLayout([], []);
			expect(result.nodes.size).toBe(0);
			expect(result.edges).toHaveLength(0);
			expect(result.width).toBeGreaterThan(0);
			expect(result.height).toBeGreaterThan(0);
		});

		it('should layout single node', () => {
			const nodes = [{ id: 'TASK-001' }];
			const edges: Array<{ from: string; to: string }> = [];

			const result = computeLayout(nodes, edges);

			expect(result.nodes.size).toBe(1);
			expect(result.nodes.has('TASK-001')).toBe(true);
			const node = result.nodes.get('TASK-001')!;
			expect(node.layer).toBe(0);
			expect(node.x).toBeGreaterThanOrEqual(0);
			expect(node.y).toBeGreaterThanOrEqual(0);
		});

		it('should layout linear chain correctly', () => {
			const nodes = [{ id: 'TASK-001' }, { id: 'TASK-002' }, { id: 'TASK-003' }];
			const edges = [
				{ from: 'TASK-001', to: 'TASK-002' },
				{ from: 'TASK-002', to: 'TASK-003' }
			];

			const result = computeLayout(nodes, edges);

			expect(result.nodes.size).toBe(3);

			// TASK-001 has no dependencies, should be on top (layer 0)
			const task1 = result.nodes.get('TASK-001')!;
			expect(task1.layer).toBe(0);

			// TASK-002 depends on TASK-001
			const task2 = result.nodes.get('TASK-002')!;
			expect(task2.layer).toBe(1);

			// TASK-003 depends on TASK-002
			const task3 = result.nodes.get('TASK-003')!;
			expect(task3.layer).toBe(2);

			// Verify vertical ordering
			expect(task1.y).toBeLessThan(task2.y);
			expect(task2.y).toBeLessThan(task3.y);
		});

		it('should handle diamond dependency pattern', () => {
			// TASK-001
			//   /   \
			// TASK-002  TASK-003
			//   \   /
			// TASK-004
			const nodes = [
				{ id: 'TASK-001' },
				{ id: 'TASK-002' },
				{ id: 'TASK-003' },
				{ id: 'TASK-004' }
			];
			const edges = [
				{ from: 'TASK-001', to: 'TASK-002' },
				{ from: 'TASK-001', to: 'TASK-003' },
				{ from: 'TASK-002', to: 'TASK-004' },
				{ from: 'TASK-003', to: 'TASK-004' }
			];

			const result = computeLayout(nodes, edges);

			const task1 = result.nodes.get('TASK-001')!;
			const task2 = result.nodes.get('TASK-002')!;
			const task3 = result.nodes.get('TASK-003')!;
			const task4 = result.nodes.get('TASK-004')!;

			// Layer 0: TASK-001
			expect(task1.layer).toBe(0);

			// Layer 1: TASK-002, TASK-003 (both depend on TASK-001)
			expect(task2.layer).toBe(1);
			expect(task3.layer).toBe(1);

			// Layer 2: TASK-004
			expect(task4.layer).toBe(2);
		});

		it('should handle multiple roots (nodes with no dependencies)', () => {
			const nodes = [
				{ id: 'TASK-001' },
				{ id: 'TASK-002' },
				{ id: 'TASK-003' }
			];
			// No edges - all are independent
			const edges: Array<{ from: string; to: string }> = [];

			const result = computeLayout(nodes, edges);

			// All nodes should be on layer 0
			for (const node of result.nodes.values()) {
				expect(node.layer).toBe(0);
			}

			// All should have the same y position
			const yPositions = Array.from(result.nodes.values()).map((n) => n.y);
			expect(new Set(yPositions).size).toBe(1);
		});

		it('should create edge paths for all edges', () => {
			const nodes = [{ id: 'TASK-001' }, { id: 'TASK-002' }];
			const edges = [{ from: 'TASK-001', to: 'TASK-002' }];

			const result = computeLayout(nodes, edges);

			expect(result.edges).toHaveLength(1);
			expect(result.edges[0].from).toBe('TASK-001');
			expect(result.edges[0].to).toBe('TASK-002');
			expect(result.edges[0].points).toHaveLength(2);
		});

		it('should use custom config values', () => {
			const nodes = [{ id: 'TASK-001' }];
			const customConfig = {
				nodeWidth: 200,
				nodeHeight: 100,
				padding: 50
			};

			const result = computeLayout(nodes, [], customConfig);

			const node = result.nodes.get('TASK-001')!;
			expect(node.width).toBe(200);
			expect(node.height).toBe(100);
		});
	});

	describe('getEdgePath', () => {
		it('should return empty string for edges with insufficient points', () => {
			const edge = { from: 'A', to: 'B', points: [] };
			expect(getEdgePath(edge)).toBe('');

			const edge2 = { from: 'A', to: 'B', points: [{ x: 0, y: 0 }] };
			expect(getEdgePath(edge2)).toBe('');
		});

		it('should generate valid SVG path for two points', () => {
			const edge = {
				from: 'A',
				to: 'B',
				points: [
					{ x: 100, y: 50 },
					{ x: 100, y: 150 }
				]
			};

			const path = getEdgePath(edge);

			expect(path).toContain('M 100 50');
			expect(path).toContain('C'); // Should use cubic bezier
			expect(path).toContain('100 150');
		});
	});
});
