/**
 * TDD Tests for LoopEdge backward connection functionality
 *
 * Tests for TASK-729: Implement loop edges as backward connections
 *
 * Success Criteria Coverage:
 * - SC-1: Loop edges detect backward flow (source sequence > target sequence)
 * - SC-2: Backward loop edges use different path calculation for clear visual distinction
 * - SC-3: Backward loop edges maintain distinctive styling (dashed orange line)
 * - SC-4: Loop edge labels position correctly for backward flowing edges
 * - SC-5: Backward vs forward loop edges are visually distinguishable
 *
 * These tests will FAIL until backward connection detection and special
 * path calculation are implemented in LoopEdge component.
 */

import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render } from '@testing-library/react';
import { ReactFlowProvider, Position } from '@xyflow/react';
import { LoopEdge } from './LoopEdge';
import type { EdgeProps } from '@xyflow/react';

// Mock IntersectionObserver for React Flow
beforeAll(() => {
	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});

	class MockResizeObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'ResizeObserver', {
		value: MockResizeObserver,
		writable: true,
	});
});

/**
 * Create base EdgeProps for testing LoopEdge component
 */
function createBaseEdgeProps(overrides: Partial<EdgeProps> = {}): EdgeProps {
	return {
		id: 'test-loop-edge',
		source: 'phase-2',
		target: 'phase-1',
		sourceX: 200,
		sourceY: 100,
		targetX: 100,
		targetY: 50,
		sourcePosition: Position.Right,
		targetPosition: Position.Left,
		data: {
			condition: 'needs_changes',
			maxIterations: 3,
			label: 'needs_changes ×3',
		},
		...overrides,
	};
}

/**
 * Render LoopEdge within ReactFlowProvider context
 */
function renderLoopEdge(props: EdgeProps) {
	return render(
		<ReactFlowProvider>
			<svg>
				<LoopEdge {...props} />
			</svg>
		</ReactFlowProvider>
	);
}

describe('LoopEdge - Backward Connection Detection', () => {
	describe('SC-1: Loop edges detect backward flow from sequence data', () => {
		it('identifies edge as backward when source sequence > target sequence', () => {
			const props = createBaseEdgeProps({
				data: {
					condition: 'needs_changes',
					maxIterations: 3,
					label: 'needs_changes ×3',
					sourceSequence: 3, // Later phase (review)
					targetSequence: 2, // Earlier phase (implement)
					isBackward: undefined, // Should be calculated by component
				},
			});

			const { container } = renderLoopEdge(props);

			// The edge should have backward-specific styling or class
			const edgePath = container.querySelector('.edge-loop .react-flow__edge-path');
			expect(edgePath).toBeDefined();

			// Should add backward-specific class or styling
			// This will fail until backward detection is implemented
			expect(edgePath?.classList.contains('edge-loop-backward')).toBe(true);
		});

		it('identifies edge as forward when source sequence < target sequence', () => {
			const props = createBaseEdgeProps({
				data: {
					condition: 'success',
					maxIterations: 2,
					label: 'success ×2',
					sourceSequence: 1, // Earlier phase
					targetSequence: 3, // Later phase (unusual but possible)
					isBackward: undefined, // Should be calculated
				},
			});

			const { container } = renderLoopEdge(props);

			const edgePath = container.querySelector('.edge-loop .react-flow__edge-path');
			expect(edgePath).toBeDefined();

			// Should NOT have backward class for forward flow
			expect(edgePath?.classList.contains('edge-loop-backward')).toBe(false);
			expect(edgePath?.classList.contains('edge-loop-forward')).toBe(true);
		});

		it('handles equal sequences as non-backward (self-loop)', () => {
			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 2,
					targetSequence: 2, // Same phase (self-loop)
				},
			});

			const { container } = renderLoopEdge(props);

			const edgePath = container.querySelector('.edge-loop .react-flow__edge-path');
			expect(edgePath?.classList.contains('edge-loop-backward')).toBe(false);
		});
	});

	describe('SC-2: Backward loop edges use different path calculation', () => {
		it('calculates curved path for backward flow to avoid overlap', () => {
			// Mock getBezierPath to capture the curvature parameter
			const mockGetBezierPath = vi.fn(() => ['M 200 100 C 150 75, 150 75, 100 50', 150, 75]);

			// This test verifies that backward edges use higher curvature
			// to create more pronounced curves that visually indicate backward flow
			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 4,
					targetSequence: 1, // Strong backward flow
					condition: 'retry',
					maxIterations: 5,
					label: 'retry ×5',
				},
			});

			// Mock the getBezierPath import
			vi.mock('@xyflow/react', async () => {
				const actual = await vi.importActual('@xyflow/react');
				return {
					...actual,
					getBezierPath: mockGetBezierPath,
				};
			});

			renderLoopEdge(props);

			// Should call getBezierPath with higher curvature for backward edges
			// This will fail until special backward path calculation is implemented
			expect(mockGetBezierPath).toHaveBeenCalledWith(
				expect.objectContaining({
					curvature: expect.any(Number), // Should be > 0.5 for backward
				})
			);

			const calls = mockGetBezierPath.mock.calls as unknown as Array<[{ curvature?: number }]>;
			const curvature = calls[0]?.[0]?.curvature;
			expect(curvature).toBeGreaterThan(0.5); // Backward should have more curve
		});

		it('uses standard curvature for forward flow', () => {
			const mockGetBezierPath = vi.fn(() => ['M 100 50 C 150 75, 150 75, 200 100', 150, 75]);

			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 1,
					targetSequence: 4, // Forward flow
				},
			});

			vi.mock('@xyflow/react', async () => {
				const actual = await vi.importActual('@xyflow/react');
				return {
					...actual,
					getBezierPath: mockGetBezierPath,
				};
			});

			renderLoopEdge(props);

			// Forward flow should use standard curvature
			const calls = mockGetBezierPath.mock.calls as unknown as Array<[{ curvature?: number }]>;
			const curvature = calls[0]?.[0]?.curvature;
			expect(curvature).toBeLessThanOrEqual(0.5); // Standard curvature
		});
	});

	describe('SC-3: Backward loop edges maintain distinctive styling', () => {
		it('preserves orange dashed styling for backward edges', () => {
			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 3,
					targetSequence: 1, // Backward
				},
			});

			const { container } = renderLoopEdge(props);

			const edgePath = container.querySelector('.edge-loop .react-flow__edge-path');
			expect(edgePath).toBeDefined();

			// Should maintain loop styling regardless of direction
			expect(edgePath?.classList.contains('edge-loop')).toBe(true);
			// And add backward-specific class
			expect(edgePath?.classList.contains('edge-loop-backward')).toBe(true);
		});

		it('adds backward visual indicator to edge styling', () => {
			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 4,
					targetSequence: 2, // Backward
				},
			});

			const { container } = renderLoopEdge(props);

			// Should add visual indicator like arrowhead or different dash pattern
			const edgePath = container.querySelector('.edge-loop-backward .react-flow__edge-path');
			expect(edgePath).toBeDefined();

			// Could check for specific styling attributes
			const pathElement = edgePath as SVGPathElement;
			expect(pathElement?.style.strokeDasharray).toBeDefined();
		});
	});

	describe('SC-4: Loop edge labels position correctly for backward flow', () => {
		it('positions label to avoid overlap with forward flow', () => {
			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 3,
					targetSequence: 1, // Backward
					label: 'needs_changes ×3',
				},
			});

			const { container } = renderLoopEdge(props);

			const label = container.querySelector('.edge-label-loop');
			expect(label).toBeDefined();

			// Label should be positioned to avoid main flow
			// This might involve offset calculation based on backward direction
			const labelElement = label as HTMLElement;
			expect(labelElement?.style.transform).toBeDefined();

			// Could verify label positioning logic
			// For backward edges, might offset label differently
		});

		it('includes direction indicator in label for backward edges', () => {
			const props = createBaseEdgeProps({
				data: {
					sourceSequence: 4,
					targetSequence: 1,
					condition: 'retry',
					maxIterations: 2,
					label: 'retry ×2',
				},
			});

			const { container } = renderLoopEdge(props);

			const label = container.querySelector('.edge-label-loop');
			expect(label?.textContent).toContain('retry ×2');

			// Might add directional indicator like arrow or "↩"
			// This will fail until direction indicators are added
			expect(label?.textContent).toContain('↩'); // Backward arrow
		});
	});

	describe('SC-5: Backward vs forward loop edges are visually distinguishable', () => {
		it('renders backward and forward loop edges with clear visual differences', () => {
			// Render both types in same container to compare
			const backwardProps = createBaseEdgeProps({
				id: 'backward-edge',
				data: {
					sourceSequence: 3,
					targetSequence: 1,
					label: 'backward ×3',
				},
			});

			const forwardProps = createBaseEdgeProps({
				id: 'forward-edge',
				data: {
					sourceSequence: 1,
					targetSequence: 3,
					label: 'forward ×2',
				},
			});

			const { container } = render(
				<ReactFlowProvider>
					<svg>
						<LoopEdge {...backwardProps} />
						<LoopEdge {...forwardProps} />
					</svg>
				</ReactFlowProvider>
			);

			const backwardEdge = container.querySelector('.edge-loop-backward');
			const forwardEdge = container.querySelector('.edge-loop-forward');

			expect(backwardEdge).toBeDefined();
			expect(forwardEdge).toBeDefined();

			// Should have different visual characteristics
			// Could be different dash patterns, colors, or curves
			expect(backwardEdge?.classList).not.toEqual(forwardEdge?.classList);
		});

		it('handles missing sequence data gracefully', () => {
			const props = createBaseEdgeProps({
				data: {
					condition: 'test',
					maxIterations: 1,
					label: 'test ×1',
					// No sequence data - should not crash
				},
			});

			expect(() => renderLoopEdge(props)).not.toThrow();

			const { container } = renderLoopEdge(props);
			const edgePath = container.querySelector('.edge-loop .react-flow__edge-path');
			expect(edgePath).toBeDefined();

			// Should default to standard styling when sequence unknown
			expect(edgePath?.classList.contains('edge-loop')).toBe(true);
		});
	});
});

describe('LoopEdge - Error Cases', () => {
	it('handles invalid sequence data types', () => {
		const props = createBaseEdgeProps({
			data: {
				sourceSequence: 'invalid' as any,
				targetSequence: null as any,
				label: 'test',
			},
		});

		expect(() => renderLoopEdge(props)).not.toThrow();

		const { container } = renderLoopEdge(props);
		const edgePath = container.querySelector('.edge-loop .react-flow__edge-path');
		expect(edgePath).toBeDefined();
	});

	it('handles undefined edge data', () => {
		const props = createBaseEdgeProps({
			data: undefined as any,
		});

		expect(() => renderLoopEdge(props)).not.toThrow();
	});
});