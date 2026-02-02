/**
 * TDD Tests for GateEdge component
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-1: GateEdge component renders gate symbol (◆) on edge midpoint
 * - SC-2: Gate symbol color matches gate type (gray=passthrough, blue=auto, yellow=human, purple=AI)
 * - SC-9: Gate symbol shows green color when gate has passed
 * - SC-10: Gate symbol shows red color when gate is blocked/failed
 * - SC-11: Hovering gate symbol shows tooltip with config summary
 *
 * Failure Modes:
 * - Gate data missing from edge → Render as passthrough (gray line)
 *
 * These tests will FAIL until GateEdge is implemented.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { ReactFlowProvider, ReactFlow } from '@xyflow/react';
import { GateType } from '@/gen/orc/v1/workflow_pb';

import { GateEdge } from './GateEdge';

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

/** Gate status for execution tracking */
type GateStatus = 'pending' | 'passed' | 'blocked' | 'failed';

/** Edge data for gate edges */
interface GateEdgeData extends Record<string, unknown> {
	gateType?: GateType;
	gateStatus?: GateStatus;
	phaseId?: number;
	position?: 'entry' | 'exit' | 'between';
	maxRetries?: number;
	failureAction?: string;
}

/** Render a gate edge inside React Flow context */
function renderGateEdge(edgeData?: GateEdgeData) {
	const nodes = [
		{ id: 'a', position: { x: 0, y: 0 }, data: {} },
		{ id: 'b', position: { x: 300, y: 0 }, data: {} },
	];
	const edges = [
		{
			id: 'gate-edge',
			source: 'a',
			target: 'b',
			type: 'gate',
			data: edgeData,
		},
	];
	const edgeTypes = { gate: GateEdge };

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

describe('GateEdge', () => {
	describe('SC-1: Renders gate symbol on edge midpoint', () => {
		it('renders a gate symbol element on the edge', () => {
			renderGateEdge({ gateType: GateType.AUTO });

			// Should render a gate symbol element
			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol).not.toBeNull();
		});

		it('uses EdgeLabelRenderer for the gate symbol', () => {
			renderGateEdge({ gateType: GateType.HUMAN });

			// EdgeLabelRenderer places elements in a specific container or portal
			// In test environment, the structure may differ
			const edgeLabelRenderer = document.querySelector('.react-flow__edge-labels');
			const gateSymbol = document.querySelector('.gate-edge__symbol');

			// Either the label renderer exists with symbol inside, or symbol renders directly
			if (edgeLabelRenderer) {
				expect(edgeLabelRenderer.querySelector('.gate-edge__symbol')).not.toBeNull();
			} else {
				// In test environment, just verify symbol is rendered somewhere
				expect(gateSymbol).not.toBeNull();
			}
		});

		it('renders the diamond symbol (◆) for gates that require approval', () => {
			renderGateEdge({ gateType: GateType.HUMAN });

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			// The diamond should be rendered (either as text content or SVG path)
			expect(
				gateSymbol?.textContent?.includes('◆') ||
				gateSymbol?.querySelector('svg') !== null
			).toBe(true);
		});
	});

	describe('SC-2: Gate symbol color matches gate type', () => {
		it('renders gray color for passthrough (no gate)', () => {
			// Passthrough is represented by GATE_TYPE_UNSPECIFIED or missing gateType
			renderGateEdge({ gateType: GateType.UNSPECIFIED });

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--passthrough')).toBe(true);
		});

		it('renders blue color for auto gate', () => {
			renderGateEdge({ gateType: GateType.AUTO });

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--auto')).toBe(true);
		});

		it('renders yellow color for human gate', () => {
			renderGateEdge({ gateType: GateType.HUMAN });

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--human')).toBe(true);
		});

		it('renders purple color for AI gate', () => {
			renderGateEdge({ gateType: GateType.AI });

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--ai')).toBe(true);
		});

		it('renders passthrough style for skip gate', () => {
			renderGateEdge({ gateType: GateType.SKIP });

			// Skip gates render as passthrough (gray line, no diamond)
			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(
				gateSymbol?.classList.contains('gate-edge__symbol--skip') ||
				gateSymbol?.classList.contains('gate-edge__symbol--passthrough')
			).toBe(true);
		});
	});

	describe('SC-9: Gate symbol shows green color when gate has passed', () => {
		it('renders green color when gateStatus is passed', () => {
			renderGateEdge({
				gateType: GateType.AUTO,
				gateStatus: 'passed',
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--passed')).toBe(true);
		});

		it('status class takes precedence over type class for visual styling', () => {
			renderGateEdge({
				gateType: GateType.HUMAN,
				gateStatus: 'passed',
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			// Should have passed class (green) which overrides the human class (yellow)
			expect(gateSymbol?.classList.contains('gate-edge__symbol--passed')).toBe(true);
		});
	});

	describe('SC-10: Gate symbol shows red color when gate is blocked/failed', () => {
		it('renders red color when gateStatus is blocked', () => {
			renderGateEdge({
				gateType: GateType.HUMAN,
				gateStatus: 'blocked',
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--blocked')).toBe(true);
		});

		it('renders red color when gateStatus is failed', () => {
			renderGateEdge({
				gateType: GateType.AUTO,
				gateStatus: 'failed',
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--failed')).toBe(true);
		});
	});

	describe('SC-11: Hovering gate symbol shows tooltip with config summary', () => {
		it('shows tooltip on hover', () => {
			renderGateEdge({
				gateType: GateType.HUMAN,
				maxRetries: 3,
				failureAction: 'block',
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol).not.toBeNull();

			// Trigger hover
			fireEvent.mouseEnter(gateSymbol!);

			// Tooltip should appear
			const tooltip = document.querySelector('.gate-edge__tooltip');
			expect(tooltip).not.toBeNull();
		});

		it('tooltip shows gate type', () => {
			renderGateEdge({
				gateType: GateType.HUMAN,
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			fireEvent.mouseEnter(gateSymbol!);

			const tooltip = document.querySelector('.gate-edge__tooltip');
			expect(tooltip?.textContent).toContain('Human');
		});

		it('tooltip shows max retries when configured', () => {
			renderGateEdge({
				gateType: GateType.AUTO,
				maxRetries: 5,
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			fireEvent.mouseEnter(gateSymbol!);

			const tooltip = document.querySelector('.gate-edge__tooltip');
			expect(tooltip?.textContent).toContain('5');
		});

		it('tooltip shows failure action when configured', () => {
			renderGateEdge({
				gateType: GateType.HUMAN,
				failureAction: 'retry',
			});

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			fireEvent.mouseEnter(gateSymbol!);

			const tooltip = document.querySelector('.gate-edge__tooltip');
			expect(tooltip?.textContent?.toLowerCase()).toContain('retry');
		});

		it('hides tooltip on mouse leave', () => {
			renderGateEdge({ gateType: GateType.AUTO });

			const gateSymbol = document.querySelector('.gate-edge__symbol');
			fireEvent.mouseEnter(gateSymbol!);

			// Tooltip should be visible
			expect(document.querySelector('.gate-edge__tooltip')).not.toBeNull();

			// Mouse leave
			fireEvent.mouseLeave(gateSymbol!);

			// Tooltip should be hidden
			expect(document.querySelector('.gate-edge__tooltip')).toBeNull();
		});
	});

	describe('Failure mode: Gate data missing from edge', () => {
		it('renders as passthrough when gateType is missing', () => {
			renderGateEdge({});

			// Should render as passthrough (gray, simple line)
			const gateSymbol = document.querySelector('.gate-edge__symbol');
			expect(gateSymbol?.classList.contains('gate-edge__symbol--passthrough')).toBe(true);
		});

		it('renders as passthrough when data is undefined', () => {
			renderGateEdge(undefined);

			// Should render a basic edge without crashing
			const edgePath = document.querySelector('.gate-edge .react-flow__edge-path');
			expect(edgePath).not.toBeNull();
		});

		it('does not crash when edge data is null', () => {
			// This tests defensive programming
			renderGateEdge(null as unknown as GateEdgeData);

			// Should still render the edge path
			const edgePath = document.querySelector('.gate-edge .react-flow__edge-path');
			expect(edgePath).not.toBeNull();
		});
	});

	describe('Edge rendering', () => {
		it('renders the edge path correctly', () => {
			renderGateEdge({ gateType: GateType.AUTO });

			const edgePath = document.querySelector('.gate-edge .react-flow__edge-path');
			expect(edgePath).not.toBeNull();
		});

		it('applies gate-edge class to the edge group', () => {
			renderGateEdge({ gateType: GateType.AUTO });

			const edgeGroup = document.querySelector('.gate-edge');
			expect(edgeGroup).not.toBeNull();
		});

		it('renders interaction path for click handling', () => {
			renderGateEdge({ gateType: GateType.AUTO });

			// Should have an interaction path with larger stroke width for easier clicking
			const interactionPath = document.querySelector('.gate-edge .react-flow__edge-interaction');
			expect(interactionPath).not.toBeNull();
		});
	});

	describe('Position variants', () => {
		it('handles entry position (before first phase)', () => {
			renderGateEdge({
				gateType: GateType.AUTO,
				position: 'entry',
			});

			const gateEdge = document.querySelector('.gate-edge');
			expect(gateEdge?.classList.contains('gate-edge--entry')).toBe(true);
		});

		it('handles exit position (after last phase)', () => {
			renderGateEdge({
				gateType: GateType.AUTO,
				position: 'exit',
			});

			const gateEdge = document.querySelector('.gate-edge');
			expect(gateEdge?.classList.contains('gate-edge--exit')).toBe(true);
		});

		it('handles between position (normal phase-to-phase)', () => {
			renderGateEdge({
				gateType: GateType.AUTO,
				position: 'between',
			});

			const gateEdge = document.querySelector('.gate-edge');
			expect(
				gateEdge?.classList.contains('gate-edge--between') ||
				!gateEdge?.classList.contains('gate-edge--entry')
			).toBe(true);
		});
	});
});
