/**
 * TDD Tests for edge type registration
 *
 * Tests for TASK-693: Visual editor - edge drawing, deletion, and type badges
 *
 * Success Criteria Coverage:
 * - SC-5: All edge types registered: sequential, dependency, loop, retry, conditional
 *
 * Current state: edgeTypes has 4 entries (sequential, dependency, loop, retry).
 * Required: Add 'conditional' type.
 *
 * These tests will FAIL until ConditionalEdge is created and registered.
 */

import { describe, it, expect } from 'vitest';
import { edgeTypes } from './index';

describe('edgeTypes export', () => {
	describe('SC-5: All edge types are registered', () => {
		it('exports edgeTypes object', () => {
			expect(edgeTypes).toBeDefined();
		});

		it('has exactly 6 edge type entries', () => {
			// sequential, dependency, loop, retry, conditional, gate
			expect(Object.keys(edgeTypes)).toHaveLength(6);
		});

		it('includes sequential edge type', () => {
			expect(edgeTypes).toHaveProperty('sequential');
			expect(typeof edgeTypes.sequential).toBe('function');
		});

		it('includes dependency edge type', () => {
			expect(edgeTypes).toHaveProperty('dependency');
			expect(typeof edgeTypes.dependency).toBe('function');
		});

		it('includes loop edge type', () => {
			expect(edgeTypes).toHaveProperty('loop');
			expect(typeof edgeTypes.loop).toBe('function');
		});

		it('includes retry edge type', () => {
			expect(edgeTypes).toHaveProperty('retry');
			expect(typeof edgeTypes.retry).toBe('function');
		});

		it('includes conditional edge type', () => {
			expect(edgeTypes).toHaveProperty('conditional');
			expect(typeof (edgeTypes as Record<string, unknown>).conditional).toBe('function');
		});

		it('is a stable reference (same object on re-import)', async () => {
			const { edgeTypes: edgeTypes2 } = await import('./index');
			expect(edgeTypes).toBe(edgeTypes2);
		});
	});

	describe('SC-5: ConditionalEdge is exported', () => {
		it('exports ConditionalEdge component', async () => {
			const mod = await import('./index');
			expect(mod).toHaveProperty('ConditionalEdge');
			expect(typeof (mod as Record<string, unknown>).ConditionalEdge).toBe('function');
		});
	});
});
