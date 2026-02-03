/**
 * TDD Tests for DependencyEdge component updates
 *
 * Tests for TASK-693: Visual editor - edge drawing, deletion, and type badges
 *
 * Success Criteria Coverage:
 * - SC-3: DependencyEdge renders solid arrow with animated flow (not dashed)
 * - SC-4: Type badge identifies the edge type visually
 *
 * Current state: DependencyEdge uses dashed line with accent color (edge-dependency class).
 * Required: Solid arrow with animated flow, matching task description.
 *
 * These tests will FAIL until DependencyEdge is updated.
 */

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { ReactFlowProvider, ReactFlow } from '@xyflow/react';
import { DependencyEdge } from './DependencyEdge';

// NOTE: Browser API mocks (ResizeObserver, IntersectionObserver) provided by global test-setup.ts

/** Render a dependency edge inside React Flow context */
function renderDependencyEdge(edgeData?: Record<string, unknown>) {
	const nodes = [
		{ id: 'a', position: { x: 0, y: 0 }, data: {} },
		{ id: 'b', position: { x: 300, y: 0 }, data: {} },
	];
	const edges = [
		{
			id: 'dep-edge',
			source: 'a',
			target: 'b',
			type: 'dependency',
			data: edgeData,
		},
	];
	const edgeTypes = { dependency: DependencyEdge };

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

describe('DependencyEdge', () => {
	describe('SC-3: Renders solid arrow with animated flow', () => {
		it('renders with edge-dependency class (solid line, not dashed)', () => {
			renderDependencyEdge();

			// The edge-dependency CSS should define a solid line (no stroke-dasharray)
			// This test verifies the class is applied; CSS changes are tested via
			// checking that the CSS file no longer has stroke-dasharray for .edge-dependency
			const edgePath = document.querySelector('.edge-dependency .react-flow__edge-path');
			expect(edgePath).not.toBeNull();
		});

		it('renders animated flow dot when animated data flag is set', () => {
			renderDependencyEdge({ animated: true });

			// Should render an animated dot (circle with animateMotion) like SequentialEdge
			const animDot = document.querySelector('.edge-dependency .edge-dot');
			expect(animDot).not.toBeNull();
		});

		it('renders animation element (animateMotion) on the dot', () => {
			renderDependencyEdge({ animated: true });

			const motion = document.querySelector('.edge-dependency animateMotion');
			expect(motion).not.toBeNull();
		});

		it('does not render animated dot when not animated', () => {
			renderDependencyEdge();

			const animDot = document.querySelector('.edge-dependency .edge-dot');
			expect(animDot).toBeNull();
		});
	});

	describe('SC-4: Type badge for dependency edges', () => {
		it('renders a type badge label identifying the edge as a dependency', () => {
			renderDependencyEdge();

			// DependencyEdge should have a visual badge indicating "dependency" type
			const badge = document.querySelector('.edge-label-dependency');
			expect(badge).not.toBeNull();
		});
	});
});
