/**
 * TDD Tests for edge types registration - GateEdge
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-3 (integration): GateEdge is registered in edgeTypes
 *
 * These tests will FAIL until GateEdge is added to the edgeTypes export.
 */

import { describe, it, expect } from 'vitest';
import { edgeTypes } from './index';

describe('edgeTypes - GateEdge Registration', () => {
	it('includes gate edge type', () => {
		expect(edgeTypes).toHaveProperty('gate');
	});

	it('gate edge type is a React component', () => {
		expect(typeof edgeTypes.gate).toBe('function');
	});

	it('preserves all existing edge types after adding gate', () => {
		// All existing edge types should still be present
		expect(edgeTypes).toHaveProperty('sequential');
		expect(edgeTypes).toHaveProperty('loop');
		expect(edgeTypes).toHaveProperty('retry');
		expect(edgeTypes).toHaveProperty('dependency');
		expect(edgeTypes).toHaveProperty('conditional');
	});

	it('has exactly 6 edge types registered', () => {
		// sequential, loop, retry, dependency, conditional, gate
		const typeCount = Object.keys(edgeTypes).length;
		expect(typeCount).toBe(6);
	});
});
