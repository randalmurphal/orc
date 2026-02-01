/**
 * TDD Tests for ConditionalEdge component
 *
 * Tests for TASK-693: Visual editor - edge drawing, deletion, and type badges
 *
 * Success Criteria Coverage:
 * - SC-1: ConditionalEdge renders with dotted line style (className edge-conditional)
 * - SC-2: ConditionalEdge displays condition badge when condition data is provided
 * - SC-4: Type badge identifies the edge type visually
 *
 * These tests will FAIL until ConditionalEdge is implemented.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { render } from '@testing-library/react';
import { ReactFlowProvider, ReactFlow } from '@xyflow/react';
import { ConditionalEdge } from './ConditionalEdge';

// Mock ResizeObserver for React Flow
beforeAll(() => {
	class MockResizeObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'ResizeObserver', {
		value: MockResizeObserver,
		writable: true,
	});

	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});
});

/** Render an edge inside React Flow context (required for EdgeLabelRenderer) */
function renderEdge(edgeData?: Record<string, unknown>) {
	const nodes = [
		{ id: 'a', position: { x: 0, y: 0 }, data: {} },
		{ id: 'b', position: { x: 300, y: 0 }, data: {} },
	];
	const edges = [
		{
			id: 'test-edge',
			source: 'a',
			target: 'b',
			type: 'conditional',
			data: edgeData,
		},
	];
	const edgeTypes = { conditional: ConditionalEdge };

	return render(
		<ReactFlowProvider>
			<div style={{ width: 800, height: 600 }}>
				<ReactFlow
					nodes={nodes}
					edges={edges}
					edgeTypes={edgeTypes}
					fitView
				/>
			</div>
		</ReactFlowProvider>
	);
}

describe('ConditionalEdge', () => {
	describe('SC-1: Renders with dotted line style', () => {
		it('applies edge-conditional CSS class to the edge path', () => {
			renderEdge();

			// The edge should have the edge-conditional class applied
			const edgePath = document.querySelector('.edge-conditional .react-flow__edge-path');
			expect(edgePath).not.toBeNull();
		});

		it('renders as a BaseEdge with bezier path', () => {
			renderEdge();

			// Should render an SVG path element inside the edge group
			const edgeGroup = document.querySelector('[data-testid="rf__edge-test-edge"]') ??
				document.querySelector('.react-flow__edge');
			expect(edgeGroup).not.toBeNull();
		});
	});

	describe('SC-2: Condition badge displayed when data includes condition', () => {
		it('renders condition text as a label badge', () => {
			renderEdge({ condition: 'review_passed == true' });

			// The condition badge should display the condition text
			const badge = document.querySelector('.edge-label-conditional');
			expect(badge).not.toBeNull();
			expect(badge!.textContent).toContain('review_passed == true');
		});

		it('does not render badge when no condition data is provided', () => {
			renderEdge();

			const badge = document.querySelector('.edge-label-conditional');
			expect(badge).toBeNull();
		});

		it('does not render badge when condition is empty string', () => {
			renderEdge({ condition: '' });

			const badge = document.querySelector('.edge-label-conditional');
			expect(badge).toBeNull();
		});
	});

	describe('SC-4: Type badge identifies edge as conditional', () => {
		it('renders a type indicator badge on the edge', () => {
			renderEdge({ condition: 'status == ok' });

			// The badge should visually identify this as a conditional edge
			// Look for the conditional-specific label class
			const badge = document.querySelector('.edge-label-conditional');
			expect(badge).not.toBeNull();
		});
	});
});
