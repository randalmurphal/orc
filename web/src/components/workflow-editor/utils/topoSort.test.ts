/**
 * Unit tests for topoSort utility.
 *
 * Tests for TASK-705: Auto-recalculate sequence numbers from topology
 *
 * Success Criteria Coverage:
 * - SC-1: Topological sort assigns sequence numbers respecting dependencies
 * - SC-2: Phases at same dependency level get the same sequence number (parallel potential)
 * - SC-3: Edge cases handled correctly (empty input, orphans, cycles)
 */

import { describe, it, expect } from 'vitest';
import { topoSort, type PhaseForSort } from './topoSort';

describe('topoSort', () => {
	describe('SC-1: Dependency ordering', () => {
		it('assigns increasing sequences to dependent phases', () => {
			const phases: PhaseForSort[] = [
				{ id: 'spec', dependsOn: [] },
				{ id: 'implement', dependsOn: ['spec'] },
				{ id: 'review', dependsOn: ['implement'] },
			];

			const result = topoSort(phases);

			expect(result.get('spec')).toBe(1);
			expect(result.get('implement')).toBe(2);
			expect(result.get('review')).toBe(3);
		});

		it('orders phases with multiple dependencies correctly', () => {
			// implement depends on both spec and tdd_write
			// review depends on implement
			const phases: PhaseForSort[] = [
				{ id: 'spec', dependsOn: [] },
				{ id: 'tdd_write', dependsOn: [] },
				{ id: 'implement', dependsOn: ['spec', 'tdd_write'] },
				{ id: 'review', dependsOn: ['implement'] },
			];

			const result = topoSort(phases);

			// spec and tdd_write have no deps - should be sequence 1 (parallel)
			expect(result.get('spec')).toBe(1);
			expect(result.get('tdd_write')).toBe(1);
			// implement waits for both - sequence 2
			expect(result.get('implement')).toBe(2);
			// review waits for implement - sequence 3
			expect(result.get('review')).toBe(3);
		});

		it('handles diamond dependency pattern', () => {
			// Diamond: A -> B, A -> C, B -> D, C -> D
			//      A
			//     / \
			//    B   C
			//     \ /
			//      D
			const phases: PhaseForSort[] = [
				{ id: 'A', dependsOn: [] },
				{ id: 'B', dependsOn: ['A'] },
				{ id: 'C', dependsOn: ['A'] },
				{ id: 'D', dependsOn: ['B', 'C'] },
			];

			const result = topoSort(phases);

			expect(result.get('A')).toBe(1);
			// B and C at same level (both depend only on A)
			expect(result.get('B')).toBe(2);
			expect(result.get('C')).toBe(2);
			// D waits for both B and C
			expect(result.get('D')).toBe(3);
		});
	});

	describe('SC-2: Parallel phases get same sequence', () => {
		it('assigns same sequence to independent phases', () => {
			const phases: PhaseForSort[] = [
				{ id: 'spec', dependsOn: [] },
				{ id: 'research', dependsOn: [] },
				{ id: 'setup', dependsOn: [] },
			];

			const result = topoSort(phases);

			// All independent - all sequence 1
			expect(result.get('spec')).toBe(1);
			expect(result.get('research')).toBe(1);
			expect(result.get('setup')).toBe(1);
		});

		it('groups phases by dependency level', () => {
			// Two parallel chains: spec -> impl, tdd -> test
			// Then merge -> review depends on both chains
			const phases: PhaseForSort[] = [
				{ id: 'spec', dependsOn: [] },
				{ id: 'tdd', dependsOn: [] },
				{ id: 'impl', dependsOn: ['spec'] },
				{ id: 'test', dependsOn: ['tdd'] },
				{ id: 'review', dependsOn: ['impl', 'test'] },
			];

			const result = topoSort(phases);

			// Level 1: spec, tdd (no deps)
			expect(result.get('spec')).toBe(1);
			expect(result.get('tdd')).toBe(1);
			// Level 2: impl, test (each depends on one level-1 phase)
			expect(result.get('impl')).toBe(2);
			expect(result.get('test')).toBe(2);
			// Level 3: review (depends on both level-2 phases)
			expect(result.get('review')).toBe(3);
		});
	});

	describe('SC-3: Edge cases', () => {
		it('returns empty map for empty input', () => {
			const result = topoSort([]);
			expect(result.size).toBe(0);
		});

		it('handles single phase', () => {
			const phases: PhaseForSort[] = [{ id: 'only', dependsOn: [] }];

			const result = topoSort(phases);

			expect(result.get('only')).toBe(1);
		});

		it('ignores dependencies on phases not in the workflow', () => {
			// 'implement' depends on 'external' which doesn't exist
			const phases: PhaseForSort[] = [
				{ id: 'spec', dependsOn: [] },
				{ id: 'implement', dependsOn: ['spec', 'external'] },
			];

			const result = topoSort(phases);

			// 'external' dependency is ignored since it's not in the workflow
			expect(result.get('spec')).toBe(1);
			expect(result.get('implement')).toBe(2);
		});

		it('assigns fallback sequence to phases in a cycle', () => {
			// A -> B -> A (cycle)
			const phases: PhaseForSort[] = [
				{ id: 'A', dependsOn: ['B'] },
				{ id: 'B', dependsOn: ['A'] },
			];

			const result = topoSort(phases);

			// Both should be assigned (cycles get sequence after all others)
			expect(result.has('A')).toBe(true);
			expect(result.has('B')).toBe(true);
			// The implementation assigns cycles to sequence after valid phases
			// Since there are no valid phases, they get sequence 1
			expect(result.get('A')).toBe(1);
			expect(result.get('B')).toBe(1);
		});

		it('handles orphan phases in cycle with valid phases', () => {
			// spec -> implement (valid chain)
			// A -> B -> A (cycle, orphans)
			const phases: PhaseForSort[] = [
				{ id: 'spec', dependsOn: [] },
				{ id: 'implement', dependsOn: ['spec'] },
				{ id: 'A', dependsOn: ['B'] },
				{ id: 'B', dependsOn: ['A'] },
			];

			const result = topoSort(phases);

			expect(result.get('spec')).toBe(1);
			expect(result.get('implement')).toBe(2);
			// Cycle phases get sequence after valid phases
			expect(result.get('A')).toBe(3);
			expect(result.get('B')).toBe(3);
		});
	});
});
