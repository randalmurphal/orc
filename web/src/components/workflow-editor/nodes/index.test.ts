import { describe, it, expect } from 'vitest';
import { nodeTypes } from './index';

describe('nodeTypes export', () => {
	describe('SC-10: module-level nodeTypes object', () => {
		it('exports nodeTypes with phase key', () => {
			expect(nodeTypes).toBeDefined();
			expect(nodeTypes).toHaveProperty('phase');
		});

		it('has exactly 2 node type entries (phase + virtual for gate edges)', () => {
			expect(Object.keys(nodeTypes)).toHaveLength(2);
		});

		it('exports nodeTypes with virtual key', () => {
			expect(nodeTypes).toHaveProperty('virtual');
		});

		it('is a stable reference (same object on re-import)', async () => {
			// Re-import to verify it's the same module-level constant
			const { nodeTypes: nodeTypes2 } = await import('./index');
			expect(nodeTypes).toBe(nodeTypes2);
		});

		it('phase value is a component (function)', () => {
			expect(typeof nodeTypes.phase).toBe('function');
		});
	});
});
